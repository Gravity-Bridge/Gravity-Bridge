use crate::{
    airdrop_proposal::wait_for_proposals_to_execute,
    get_fee,
    happy_path::test_erc20_deposit_panic,
    one_eth, one_hundred_eth,
    transaction_stress_test::{
        lock_funds_in_pool, prep_users_for_deposit, test_bulk_send_to_cosmos, STARTING_ETH,
    },
    utils::*,
    TOTAL_TIMEOUT,
};
use clarity::Address as EthAddress;
use cosmos_gravity::{
    query::get_gravity_params, send::send_request_batch, utils::get_reasonable_send_to_eth_fee,
};
use deep_space::Contact;
use gravity_proto::{
    cosmos_sdk_proto::cosmos::params::v1beta1::ParamChange,
    gravity::query_client::QueryClient as GravityQueryClient,
};
use std::time::{Duration, Instant};
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use web30::client::Web3;

/// sixty seconds in milliseconds
const BATCH_TIMEOUT_SHORT: u64 = 60000;
const BATCH_TIMEOUT_LONG: u64 = 6000000;

const NUM_USERS: usize = 100;

/// Perform a stress test by setting the batch timeout period
/// to an extremely small value and then sending many different transfers
#[allow(clippy::too_many_arguments)]
pub async fn batch_timeout_test(
    web30: &Web3,
    contact: &Contact,
    grpc_client: GravityQueryClient<Channel>,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_addresses: Vec<EthAddress>,
) {
    let mut grpc_client = grpc_client;

    let no_relay_market_config = create_no_batch_requests_config();
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;

    // first we reduce batch timeout (denominated in miliseconds)
    // to one minute, the absolute minimum value

    set_batch_timeout(&keys, BATCH_TIMEOUT_SHORT, contact, &mut grpc_client).await;

    // now that we have set the batch timeout we're going to generate a lot of batches

    // Generate NUM_USERS user keys to send ETH and multiple types of tokens
    let mut user_keys = Vec::new();
    for _ in 0..NUM_USERS {
        user_keys.push(get_user_key(None));
    }

    prep_users_for_deposit(&user_keys, &erc20_addresses, web30).await;

    let sent_amounts = test_bulk_send_to_cosmos(
        &user_keys,
        gravity_address,
        &erc20_addresses,
        web30,
        contact,
    )
    .await;

    // now that the users have tokens on cosmos we're going to generate many small batches of progressively
    // increasing size, since a larger fee value is required to generate a new batch
    let denoms = lock_funds_in_pool(
        &sent_amounts,
        &keys,
        &user_keys,
        &erc20_addresses,
        contact,
        true,
    )
    .await;

    // vote to increase the timeout, all the batchs already created will not be affected
    // this is so we can relay later, the voting period is long enough to timeout anything that's left
    set_batch_timeout(&keys, BATCH_TIMEOUT_LONG, contact, &mut grpc_client).await;

    // now send a deposit to complete the timeout of all those batches by updating the eth height
    let user = get_user_key(None);
    test_erc20_deposit_panic(
        web30,
        contact,
        &mut grpc_client,
        user.cosmos_address,
        gravity_address,
        erc20_addresses[0],
        one_eth(),
        None,
        None,
    )
    .await;

    // request batches and check that everyone got their expected amounts even after the timeout/reconsolidation

    for denom in denoms {
        info!("Requesting batch for {}", denom);
        let res = send_request_batch(keys[0].validator_key, denom, Some(get_fee(None)), contact)
            .await
            .unwrap();
        info!("batch request response is {:?}", res);
    }

    let starting_eth = one_eth() * STARTING_ETH.into();
    let max_nonrefundable_amount = get_reasonable_send_to_eth_fee(contact, one_hundred_eth())
        .await
        .expect("Unable to get reasonable fee!");
    let start = Instant::now();
    let mut good = true;
    while Instant::now() - start < TOTAL_TIMEOUT {
        good = true;
        for keys in user_keys.iter() {
            let e_dest_addr = keys.eth_dest_address;
            for token in erc20_addresses.iter() {
                let bal = get_erc20_balance_safe(*token, web30, e_dest_addr)
                    .await
                    .unwrap();

                // we sent a random amount below 100 tokens to each user, now we're sending
                // it all back, so we should only be down 500 and whatever ChainFees we paid
                let min_expected_balance = starting_eth - 500u16.into() - max_nonrefundable_amount;

                if bal < min_expected_balance {
                    good = false;
                }
            }
        }
        if good {
            info!(
                "All {} withdraws to Ethereum bridged successfully after timeout/rebatch!",
                NUM_USERS * erc20_addresses.len()
            );
            break;
        }
        delay_for(Duration::from_secs(5)).await;
    }
    if !(good) {
        panic!(
            "Failed to perform all {} withdraws to Ethereum!",
            NUM_USERS * erc20_addresses.len()
        );
    }
}

async fn set_batch_timeout(
    keys: &[ValidatorKeys],
    timeout: u64,
    contact: &Contact,
    grpc_client: &mut GravityQueryClient<Channel>,
) {
    info!("Voting to change batch timeout!");
    let mut params_to_change = Vec::new();
    let halt = ParamChange {
        subspace: "gravity".to_string(),
        key: "TargetBatchTimeout".to_string(),
        value: format!("\"{}\"", timeout),
    };
    params_to_change.push(halt);

    // next we create a governance proposal to
    create_parameter_change_proposal(
        contact,
        keys[0].validator_key,
        params_to_change,
        get_fee(None),
    )
    .await;

    vote_yes_on_proposals(contact, keys, None).await;

    // wait for the voting period to pass
    wait_for_proposals_to_execute(contact).await;
    // confirm the parameter has actually been changed
    let params = get_gravity_params(grpc_client).await.unwrap();
    assert_eq!(params.target_batch_timeout, timeout);
}
