use crate::args::CosmosToEthOpts;
use crate::utils::TIMEOUT;
use clarity::Address as EthAddress;
use cosmos_gravity::query::get_denom_to_erc20;
use cosmos_gravity::send::send_to_eth;
use deep_space::{Address as CosmosAddress, Coin, Contact, PrivateKey};
use gravity_proto::gravity::query_client::QueryClient;
use gravity_proto::gravity::QueryDenomToErc20Request;
use gravity_utils::{
    connection_prep::{check_for_fee, create_rpc_connections},
    num_conversion::{print_atom, print_eth},
};
use std::process::exit;
use tonic::transport::Channel;

pub async fn cosmos_to_eth_cmd(args: CosmosToEthOpts, address_prefix: String) {
    let cosmos_key = args.cosmos_phrase;
    let gravity_coin = args.amount;
    let fee = args.fee;
    let cosmos_grpc = args.cosmos_grpc;
    let eth_dest = args.eth_destination;
    let bridge_fee = args.bridge_fee;
    let chain_fee = args.chain_fee;

    let cosmos_address = cosmos_key.to_address(&address_prefix).unwrap();

    info!("Sending from Cosmos address {}", cosmos_address);
    let connections =
        create_rpc_connections(address_prefix, Some(cosmos_grpc), None, TIMEOUT).await;
    let contact = connections.contact.unwrap();
    let grpc = connections.grpc.unwrap();

    cosmos_to_eth(
        &contact,
        grpc,
        cosmos_key,
        cosmos_address,
        gravity_coin,
        fee,
        bridge_fee,
        chain_fee,
        eth_dest,
    )
    .await;
}

#[allow(clippy::too_many_arguments)]
pub async fn cosmos_to_eth(
    contact: &Contact,
    grpc: QueryClient<Channel>,
    sender_key: impl PrivateKey,
    sender_address: CosmosAddress,
    to_bridge: Coin,
    cosmos_fee: Coin,
    chain_fee: Coin,
    bridge_fee: Coin,
    receiver_address: EthAddress,
) {
    let mut grpc = grpc;
    let res = get_denom_to_erc20(&mut grpc, to_bridge.denom.clone()).await;
    let is_cosmos_originated = match res {
        Ok(v) => v.cosmos_originated,
        Err(e) => {
            error!("Could not lookup denom is it valid? {:?}", e);
            exit(1);
        }
    };

    let res = grpc
        .denom_to_erc20(QueryDenomToErc20Request {
            denom: to_bridge.denom.clone(),
        })
        .await;
    match res {
        Ok(val) => info!(
            "Asset {} has ERC20 representation {}",
            to_bridge.denom,
            val.into_inner().erc20
        ),
        Err(_e) => {
            info!(
                "Asset {} has no ERC20 representation, you may need to deploy an ERC20 for it!",
                to_bridge.denom
            );
            exit(1);
        }
    }

    let amount = to_bridge.clone();
    let full_amount = Coin {
        amount: to_bridge.amount + chain_fee.amount,
        denom: to_bridge.denom.clone(),
    };
    check_for_fee(&full_amount, sender_address, contact).await;
    check_for_fee(&cosmos_fee, sender_address, contact).await;

    let balance = contact
        .get_balance(sender_address, to_bridge.denom.clone())
        .await
        .expect("Failed to get balances!");

    match balance {
        Some(balance) => {
            if balance.amount < amount.amount + bridge_fee.amount {
                if is_cosmos_originated {
                    error!("Your transfer of {} {} tokens with chain fee {} is greater than your balance of {} tokens. Remember you need some to pay for fees!", print_atom(amount.amount), to_bridge.denom, print_atom(chain_fee.amount), print_atom(balance.amount));
                } else {
                    error!("Your transfer of {} {} tokens with chain fee {} is greater than your balance of {} tokens. Remember you need some to pay for fees!", print_eth(amount.amount), to_bridge.denom, print_eth(chain_fee.amount), print_eth(balance.amount));
                }
                exit(1);
            }
        }
        None => {
            error!("You don't have any {} tokens!", to_bridge.denom);
            exit(1);
        }
    }

    info!(
        "Locking {} / {} into the batch pool",
        amount.denom, to_bridge.denom
    );
    let res = send_to_eth(
        sender_key,
        receiver_address,
        amount.clone(),
        bridge_fee.clone(),
        Some(chain_fee),
        cosmos_fee.clone(),
        contact,
    )
    .await;
    match res {
        Ok(tx_id) => info!("Send to Eth txid {}", tx_id.txhash()),
        Err(e) => info!("Failed to send tokens! {:?}", e),
    }
    info!("Your funds are now waiting to be sent to Ethereum in a transaction batch!");
    info!("Depending on how much you and others attached in fees, this might take a while!");
    info!("You can retrieve your funds using a CancelSendToEth message, up until they are sent to Ethereum");
}
