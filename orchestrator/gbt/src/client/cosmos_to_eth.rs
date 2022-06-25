use crate::args::CosmosToEthOpts;
use crate::utils::TIMEOUT;
use cosmos_gravity::query::get_denom_to_erc20;
use cosmos_gravity::send::send_to_eth;
use deep_space::PrivateKey;
use gravity_proto::gravity::QueryDenomToErc20Request;
use gravity_utils::{
    connection_prep::{check_for_fee, create_rpc_connections},
    num_conversion::{print_atom, print_eth},
};
use std::process::exit;

pub async fn cosmos_to_eth(args: CosmosToEthOpts, address_prefix: String) {
    let cosmos_key = args.cosmos_phrase;
    let gravity_coin = args.amount;
    let fee = args.fee;
    let cosmos_grpc = args.cosmos_grpc;
    let eth_dest = args.eth_destination;
    let bridge_fee = args.bridge_fee;

    let cosmos_address = cosmos_key.to_address(&address_prefix).unwrap();

    info!("Sending from Cosmos address {}", cosmos_address);
    let connections =
        create_rpc_connections(address_prefix, Some(cosmos_grpc), None, TIMEOUT).await;
    let contact = connections.contact.unwrap();
    let mut grpc = connections.grpc.unwrap();

    let res = get_denom_to_erc20(&mut grpc, gravity_coin.denom.clone()).await;
    let is_cosmos_originated = match res {
        Ok(v) => v.cosmos_originated,
        Err(e) => {
            error!("Could not lookup denom is it valid? {:?}", e);
            exit(1);
        }
    };

    let res = grpc
        .denom_to_erc20(QueryDenomToErc20Request {
            denom: gravity_coin.denom.clone(),
        })
        .await;
    match res {
        Ok(val) => info!(
            "Asset {} has ERC20 representation {}",
            gravity_coin.denom,
            val.into_inner().erc20
        ),
        Err(_e) => {
            info!(
                "Asset {} has no ERC20 representation, you may need to deploy an ERC20 for it!",
                gravity_coin.denom
            );
            exit(1);
        }
    }

    let amount = gravity_coin.clone();
    check_for_fee(&gravity_coin, cosmos_address, &contact).await;
    check_for_fee(&fee, cosmos_address, &contact).await;

    let balance = contact
        .get_balance(cosmos_address, gravity_coin.denom.clone())
        .await
        .expect("Failed to get balances!");

    match balance {
        Some(balance) => {
            if balance.amount < amount.amount.clone() + bridge_fee.amount.clone() {
                if is_cosmos_originated {
                    error!("Your transfer of {} {} tokens is greater than your balance of {} tokens. Remember you need some to pay for fees!", print_atom(amount.amount), gravity_coin.denom, print_atom(balance.amount));
                } else {
                    error!("Your transfer of {} {} tokens is greater than your balance of {} tokens. Remember you need some to pay for fees!", print_eth(amount.amount), gravity_coin.denom, print_eth(balance.amount));
                }
                exit(1);
            }
        }
        None => {
            error!("You don't have any {} tokens!", gravity_coin.denom);
            exit(1);
        }
    }

    info!(
        "Locking {} / {} into the batch pool",
        amount.denom, gravity_coin.denom
    );
    let res = send_to_eth(
        cosmos_key,
        eth_dest,
        amount.clone(),
        bridge_fee.clone(),
        fee.clone(),
        &contact,
    )
    .await;
    match res {
        Ok(tx_id) => info!("Send to Eth txid {}", tx_id.txhash),
        Err(e) => info!("Failed to send tokens! {:?}", e),
    }
    info!("Your funds are now waiting to be sent to Ethereum in a transaction batch!");
    info!("Depending on how much you and others attached in fees, this might take a while!");
    info!("You can retrieve your funds using a CancelSendToEth message, up until they are sent to Ethereum");
}
