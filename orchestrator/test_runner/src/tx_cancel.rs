use crate::happy_path::test_erc20_deposit_panic;
use crate::one_eth;
use crate::utils::*;
use clarity::Address as EthAddress;
use cosmos_gravity::query::get_pending_send_to_eth;
use cosmos_gravity::send::cancel_send_to_eth;
use cosmos_gravity::send::send_to_eth;
use deep_space::coin::Coin;
use deep_space::Contact;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use tonic::transport::Channel;
use web30::client::Web3;

// Justin: Here's the method I set up to test out sending and cancelling, but I have not been able to get any transaction ids
// So I have not been able to generate the cancel request
pub async fn send_to_eth_and_cancel(
    contact: &Contact,
    grpc_client: GravityQueryClient<Channel>,
    web30: &Web3,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_address: EthAddress,
) {
    let mut grpc_client = grpc_client;

    let no_relay_market_config = create_default_test_config();
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;

    // a pair of cosmos and Ethereum keys + addresses to use for this test
    let user_keys = get_user_key();

    test_erc20_deposit_panic(
        web30,
        contact,
        &mut grpc_client,
        user_keys.cosmos_address,
        gravity_address,
        erc20_address,
        one_eth(),
        None,
        None,
    )
    .await;

    let token_name = format!("gravity{}", erc20_address);

    let bridge_denom_fee = Coin {
        denom: token_name.clone(),
        amount: 500u64.into(),
    };
    let amount = one_eth() - 1_500u64.into();
    info!(
        "Sending {}{} from {} on Cosmos back to Ethereum",
        amount, token_name, user_keys.cosmos_address
    );

    // Generate the tx (this part is working for me)
    let res = send_to_eth(
        user_keys.cosmos_key,
        user_keys.eth_address,
        Coin {
            denom: token_name.clone(),
            amount: amount.clone(),
        },
        bridge_denom_fee.clone(),
        bridge_denom_fee.clone(),
        contact,
    )
    .await
    .unwrap();
    info!("{:?}", res);
    for thing in res.logs {
        for event in thing.events {
            info!("attribute for {:?}", event.attributes);
        }
    }

    let res = get_pending_send_to_eth(&mut grpc_client, user_keys.cosmos_address)
        .await
        .unwrap();

    let send_to_eth_id = res.unbatched_transfers[0].id;

    cancel_send_to_eth(
        user_keys.cosmos_key,
        bridge_denom_fee,
        contact,
        send_to_eth_id,
    )
    .await
    .unwrap();

    let res = get_pending_send_to_eth(&mut grpc_client, user_keys.cosmos_address)
        .await
        .unwrap();

    assert!(res.unbatched_transfers.is_empty());
    info!("Successfully canceled SendToEth!")
}
