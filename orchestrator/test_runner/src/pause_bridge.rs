//! this test simulates pausing and unpausing the bridge via governance action. This would be used in an emergency
//! situation to prevent the bridge from being drained of funds
//!
use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::happy_path::{test_erc20_deposit_panic, test_erc20_deposit_result};
use crate::utils::*;
use crate::MINER_ADDRESS;
use crate::{get_fee, OPERATION_TIMEOUT, TOTAL_TIMEOUT};
use clarity::Address as EthAddress;
use cosmos_gravity::query::get_gravity_params;
use cosmos_gravity::send::{send_request_batch, send_to_eth};
use deep_space::coin::Coin;
use deep_space::Contact;
use ethereum_gravity::utils::get_tx_batch_nonce;
use gravity_proto::cosmos_sdk_proto::cosmos::params::v1beta1::ParamChange;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use std::time::{Duration, Instant};
use tokio::time::sleep;
use tonic::transport::Channel;
use web30::client::Web3;

/// Tests the bridge pause function, which allows a governance vote
/// to temporarily stop token transfers while vulnerabilities are dealt with
pub async fn pause_bridge_test(
    web30: &Web3,
    grpc_client: GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_address: EthAddress,
) {
    let mut grpc_client = grpc_client.clone();

    // check that the bridge is active to start the test, this test is especially
    // helpful if the last run crashed and you're trying to run a second time, not
    // realizing the starting state is incorrect
    let params = get_gravity_params(&mut grpc_client).await.unwrap();
    assert!(params.bridge_active);

    let no_relay_market_config = create_no_batch_requests_config();
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;

    // generate an address for coin sending tests, this ensures test imdepotency
    let user_keys = get_user_key();

    // send some tokens to Cosmos, so that we can try to send them back later
    // this won't complete until the tokens cross the bridge
    test_erc20_deposit_panic(
        web30,
        contact,
        &mut grpc_client,
        user_keys.cosmos_address,
        gravity_address,
        erc20_address,
        100u64.into(),
        None,
        None,
    )
    .await;

    // next we pause the bridge via governance

    info!("Voting to pause the bridge!");
    let mut params_to_change = Vec::new();
    let halt = ParamChange {
        subspace: "gravity".to_string(),
        key: "BridgeActive".to_string(),
        value: format!("{}", false),
    };
    params_to_change.push(halt);

    // next we create a governance proposal halt the bridge temporarily
    create_parameter_change_proposal(contact, keys[0].validator_key, params_to_change).await;

    vote_yes_on_proposals(contact, &keys, None).await;

    // wait for the voting period to pass
    wait_for_proposals_to_execute(contact).await;
    let params = get_gravity_params(&mut grpc_client).await.unwrap();
    assert!(!params.bridge_active);

    // now we try to bridge some tokens
    let result = test_erc20_deposit_result(
        web30,
        contact,
        &mut grpc_client,
        user_keys.cosmos_address,
        gravity_address,
        erc20_address,
        100u64.into(),
        Some(OPERATION_TIMEOUT),
        None,
    )
    .await;
    if result.is_ok() {
        panic!("Deposit succeeded after bridge pause!")
    } else {
        info!("Bridge pause successfully stopped deposit");
    }

    // Try to create a batch and send tokens to Ethereum
    let coin = contact
        .get_balance(
            user_keys.cosmos_address,
            format!("gravity{}", erc20_address),
        )
        .await
        .unwrap()
        .unwrap();
    let token_name = coin.denom;
    let amount = coin.amount;

    let bridge_denom_fee = Coin {
        denom: token_name.clone(),
        amount: 1u64.into(),
    };
    let amount = amount - 5u64.into();
    send_to_eth(
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
    let res = send_request_batch(
        keys[0].orch_key,
        token_name.clone(),
        Some(get_fee()),
        contact,
    )
    .await;
    assert!(res.is_err());

    contact
        .wait_for_next_block(OPERATION_TIMEOUT)
        .await
        .unwrap();

    // we have to send this address one eth so that it can perform contract calls
    send_one_eth(user_keys.eth_address, web30).await;

    assert!(
        get_erc20_balance_safe(erc20_address, web30, user_keys.eth_address)
            .await
            .unwrap()
            == 0u8.into()
    );
    info!("Batch creation was blocked by bridge pause!");

    info!("Voting to resume bridge operations!");
    let mut params_to_change = Vec::new();
    let unhalt = ParamChange {
        subspace: "gravity".to_string(),
        key: "BridgeActive".to_string(),
        value: format!("{}", true),
    };
    params_to_change.push(unhalt);

    // crate a governance proposal to resume the bridge
    create_parameter_change_proposal(contact, keys[0].validator_key, params_to_change).await;

    vote_yes_on_proposals(contact, &keys, None).await;

    // wait for the voting period to pass
    sleep(Duration::from_secs(65)).await;
    let params = get_gravity_params(&mut grpc_client).await.unwrap();
    assert!(params.bridge_active);

    // finally we check that our batch executes and our new withdraw processes
    let res = contact
        .get_balance(
            user_keys.cosmos_address,
            format!("gravity{}", erc20_address),
        )
        .await
        .unwrap()
        .unwrap();
    // check that our balance is equal to 200 (two deposits) minus 95 (sent to eth) - 1 (fee) - 1 (fee for batch request)
    // NOTE this makes the test not imdepotent but it's not anyways, a crash may leave the bridge halted
    assert_eq!(res.amount, 103u8.into());

    let mut current_eth_batch_nonce =
        get_tx_batch_nonce(gravity_address, erc20_address, *MINER_ADDRESS, web30)
            .await
            .expect("Failed to get current eth valset");

    // now we make sure our tokens in the batch queue make it across
    send_request_batch(
        keys[0].orch_key,
        token_name.clone(),
        Some(get_fee()),
        contact,
    )
    .await
    .unwrap();

    let starting_batch_nonce = current_eth_batch_nonce;

    let start = Instant::now();
    while starting_batch_nonce == current_eth_batch_nonce {
        info!(
            "Batch is not yet submitted {}>, waiting",
            starting_batch_nonce
        );
        current_eth_batch_nonce =
            get_tx_batch_nonce(gravity_address, erc20_address, *MINER_ADDRESS, web30)
                .await
                .expect("Failed to get current eth tx batch nonce");
        sleep(Duration::from_secs(4)).await;
        if Instant::now() - start > TOTAL_TIMEOUT {
            panic!("Failed to submit transaction batch set");
        }
    }
}
