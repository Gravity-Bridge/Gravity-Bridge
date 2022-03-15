use crate::ibc_metadata::submit_and_pass_ibc_metadata_proposal;
use crate::{happy_path_test, happy_path_test_v2, utils::*};
use clarity::Address as EthAddress;
use deep_space::client::ChainStatus;
use deep_space::Contact;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::Metadata;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use std::time::Duration;
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use web30::client::Web3;

/// Perform a series of integration tests to seed the system with data, then submit and pass a chain
/// upgrade proposal
/// NOTE: To run this test, use the tests/run-upgrade-test.sh command with an old binary, then in
/// a separate terminal execute tests/run-tests.sh with V2_UPGRADE_PART_1 as the test type.
/// After the test executes, you will need to wait for the chain to reach the halt height, which is
/// output in a comment at the end of this test
#[allow(clippy::too_many_arguments)]
pub async fn upgrade_part_1(
    web30: &Web3,
    contact: &Contact,
    grpc_client: GravityQueryClient<Channel>,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_addresses: Vec<EthAddress>,
) {
    info!("Starting upgrade test part 1");
    let metadata = footoken_metadata(contact).await;
    submit_and_pass_ibc_metadata_proposal(metadata.name.clone(), metadata.clone(), contact, &keys)
        .await;
    run_all_recoverable_tests(
        web30,
        contact,
        grpc_client.clone(),
        keys.clone(),
        gravity_address,
        erc20_addresses.clone(),
        metadata,
    )
    .await;

    let curr_height = contact.get_chain_status().await.unwrap();
    let curr_height = if let ChainStatus::Moving { block_height } = curr_height {
        block_height
    } else {
        panic!("Chain is not moving!");
    };
    let upgrade_height = (curr_height + 40) as i64;
    execute_upgrade_proposal(
        contact,
        &keys,
        None,
        UpgradeProposalParams {
            upgrade_height,
            plan_name: "mercury".to_string(),
            plan_info: "mercury upgrade info here".to_string(),
            proposal_title: "mercury upgrade proposal title here".to_string(),
            proposal_desc: "mercury upgrade proposal description here".to_string(),
        },
    )
    .await;

    info!(
        "Ready to run the new binary, waiting for chain panic at upgrade height of {}!",
        upgrade_height
    );
    // Wait for the block before the upgrade height, we won't get a response from the chain
    let res = wait_for_block(contact, (upgrade_height - 1) as u64).await;
    if res.is_err() {
        panic!("Unable to wait for upgrade! {}", res.err().unwrap());
    }

    delay_for(Duration::from_secs(10)).await; // wait for the new block to halt the chain
    let status = contact.get_chain_status().await;
    info!(
        "Done waiting, chain should be halted, status response: {:?}",
        status
    );
}

/// Perform a series of integration tests after an upgrade has executed
/// NOTE: To run this test, follow the instructions for v2_upgrade_part_1 and WAIT FOR CHAIN HALT,
/// then finally run tests/run-tests.sh with V2_UPGRADE_PART_2 as the test type.
pub async fn upgrade_part_2(
    web30: &Web3,
    contact: &Contact,
    grpc_client: GravityQueryClient<Channel>,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_addresses: Vec<EthAddress>,
) {
    info!("Starting upgrade_part_2 test");
    let mut metadata: Option<Metadata> = None;
    {
        let all_metadata = contact.get_all_denoms_metadata().await.unwrap();
        for m in all_metadata {
            if m.base == "footoken2" {
                metadata = Some(m)
            }
        }
    };
    if metadata.is_none() {
        panic!("footoken2 metadata does not exist!");
    }
    let metadata = metadata.unwrap();

    submit_and_pass_ibc_metadata_proposal(metadata.name.clone(), metadata.clone(), contact, &keys)
        .await;
    run_all_recoverable_tests(
        web30,
        contact,
        grpc_client.clone(),
        keys.clone(),
        gravity_address,
        erc20_addresses.clone(),
        metadata,
    )
    .await;
}

/// Runs many integration tests, but only the ones which DO NOT corrupt state
/// TODO: Add more tests here and determine they are 1. Reliable and 2. Executable twice on the same chain
pub async fn run_all_recoverable_tests(
    web30: &Web3,
    contact: &Contact,
    grpc_client: GravityQueryClient<Channel>,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_addresses: Vec<EthAddress>,
    ibc_metadata: Metadata,
) {
    info!("Starting Happy Path test");
    happy_path_test(
        web30,
        grpc_client.clone(),
        contact,
        keys.clone(),
        gravity_address,
        erc20_addresses[0],
        false,
    )
    .await;
    info!("Starting Happy Path test v2");
    happy_path_test_v2(
        web30,
        grpc_client.clone(),
        contact,
        keys.clone(),
        gravity_address,
        false,
        Some(ibc_metadata),
    )
    .await;
}
