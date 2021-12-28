//! This file tests signature slashing, which is when a validator is slashed for not submitted Ethereum signatures in time
//! the default timeline for signature slashing is quite long (10k blocks) so this test reduces that with a governance
//! proposal and then waits for slashing code to execute before performing a final test to ensure everything is good

use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::happy_path::test_valset_update;
use crate::utils::{
    create_default_test_config, create_parameter_change_proposal, start_orchestrators,
    vote_yes_on_proposals, ValidatorKeys,
};
use crate::TOTAL_TIMEOUT;
use clarity::Address as EthAddress;
use cosmos_gravity::query::get_gravity_params;
use deep_space::client::types::ChainStatus;
use deep_space::Contact;
use gravity_proto::cosmos_sdk_proto::cosmos::params::v1beta1::ParamChange;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use std::time::{Duration, Instant};
use tokio::time::sleep;
use tonic::transport::Channel;
use web30::client::Web3;

pub async fn signature_slashing_test(
    web30: &Web3,
    grpc_client: GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
) {
    let mut grpc_client = grpc_client;

    let no_relay_market_config = create_default_test_config();
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;

    test_valset_update(web30, contact, &mut grpc_client, &keys, gravity_address).await;

    reduce_slashing_window(contact, &mut grpc_client, &keys).await;

    // check that the block height is greater than 20 if not wait until it is
    // there's some logic here to handle the chian progressing slowly over blocks
    // which probably isn't needed for block height 20
    wait_for_height(20, contact).await;

    // make sure everything is still moving!
    test_valset_update(web30, contact, &mut grpc_client, &keys, gravity_address).await;
}

pub async fn wait_for_height(target_height: u64, contact: &Contact) {
    let mut last_update = Instant::now();
    let mut last_seen_block = 0;
    while get_latest_block(contact).await < 20 {
        let latest = get_latest_block(contact).await;
        if last_seen_block != latest {
            last_seen_block = latest;
            last_update = Instant::now()
        }

        if Instant::now() - last_update > TOTAL_TIMEOUT {
            panic!(
                "Chain has halted while waiting for height {}",
                target_height
            )
        }
        sleep(Duration::from_secs(10)).await;
    }
}

pub async fn get_latest_block(contact: &Contact) -> u64 {
    let block = contact.get_chain_status().await.unwrap();
    match block {
        ChainStatus::Moving { block_height } => block_height,
        ChainStatus::Syncing | ChainStatus::WaitingToStart => panic!("Cosmos chain not running!"),
    }
}

/// Reduces the slashing window for validator sets and batches
/// from 10k blocks to 10 blocks in order to trigger Gravity's slashing
/// code in the integration test environment. This also reduces the
/// unbonding time down to 60 seconds so that unbonding can be tested
pub async fn reduce_slashing_window(
    contact: &Contact,
    grpc_client: &mut GravityQueryClient<Channel>,
    keys: &[ValidatorKeys],
) {
    let mut params_to_change = Vec::new();
    let signed_valsets_window = ParamChange {
        subspace: "gravity".to_string(),
        key: "SignedValsetsWindow".to_string(),
        value: r#""10""#.to_string(),
    };
    params_to_change.push(signed_valsets_window);
    let signed_batches_window = ParamChange {
        subspace: "gravity".to_string(),
        key: "SignedBatchesWindow".to_string(),
        value: r#""10""#.to_string(),
    };
    params_to_change.push(signed_batches_window);
    let signed_logic_call_window = ParamChange {
        subspace: "gravity".to_string(),
        key: "SignedLogicCallsWindow".to_string(),
        value: r#""10""#.to_string(),
    };
    params_to_change.push(signed_logic_call_window);

    // next we create a governance proposal to use the newly bridged asset as the reward
    // and vote to pass the proposal
    info!("Creating parameter change governance proposal");
    create_parameter_change_proposal(contact, keys[0].validator_key, params_to_change).await;

    vote_yes_on_proposals(contact, keys, None).await;

    // wait for the voting period to pass
    wait_for_proposals_to_execute(contact).await;

    let params = get_gravity_params(grpc_client).await.unwrap();
    // check that params have changed
    assert_eq!(params.signed_valsets_window, 10);
    assert_eq!(params.signed_batches_window, 10);
    assert_eq!(params.signed_logic_calls_window, 10);
}
