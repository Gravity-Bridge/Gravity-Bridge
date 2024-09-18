use crate::ibc_metadata::submit_and_pass_ibc_metadata_proposal;
use crate::{happy_path_test, happy_path_test_v2, utils::*};
use clarity::Address as EthAddress;
use deep_space::client::ChainStatus;
use deep_space::utils::decode_any;
use deep_space::{Contact, CosmosPrivateKey};
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::Metadata;
use gravity_proto::gravity::query_client::{QueryClient as GravityQueryClient, QueryClient};
use gravity_proto::gravity::{
    ClaimType, EthereumClaim, MsgBatchSendToEthClaim, MsgErc20DeployedClaim,
    MsgLogicCallExecutedClaim, MsgSendToCosmosClaim, MsgValsetUpdatedClaim,
    QueryAttestationsRequest,
};
use gravity_utils::types::{
    MSG_BATCH_SEND_TO_ETH_TYPE_URL, MSG_ERC20_DEPLOYED_CLAIM_TYPE_URL,
    MSG_LOGIC_CALL_EXECUTED_CLAIM_TYPE_URL, MSG_SEND_TO_COSMOS_CLAIM_TYPE_URL,
    MSG_VALSET_UPDATED_CLAIM_TYPE_URL,
};
use std::time::Duration;
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use web30::client::Web3;

pub const UPGRADE_NAME: &str = "neutrino";

// The number of attestations run_all_recoverable_tests and run_upgrade_specific_tests should create
const MINIMUM_ATTESTATIONS: u64 = 10;
// Those attestations broken down by type
const EXPECTED_SENDS_TO_COSMOS: u64 = 3;
const EXPECTED_BATCHES: u64 = 2;
const EXPECTED_ERC20S: u64 = 1;
const EXPECTED_LOGIC_CALLS: u64 = 0;
const MINIMUM_VALSETS: u64 = 4; // There may be more valsets depending on how long the test takes

/// Perform a series of integration tests to seed the system with data, then submit and pass a chain
/// upgrade proposal
/// NOTE: To run this test, use the tests/run-upgrade-test.sh command with an old binary, then in
/// a separate terminal execute tests/run-tests.sh with V2_UPGRADE_PART_1 as the test type.
/// After the test executes, you will need to wait for the chain to reach the halt height, which is
/// output in a comment at the end of this test
#[allow(clippy::too_many_arguments)]
pub async fn upgrade_part_1(
    web30: &Web3,
    gravity_contact: &Contact,
    ibc_contact: &Contact,
    grpc_client: GravityQueryClient<Channel>,
    keys: Vec<ValidatorKeys>,
    ibc_keys: Vec<CosmosPrivateKey>,
    gravity_address: EthAddress,
    erc20_addresses: Vec<EthAddress>,
) {
    info!("Starting upgrade test part 1");
    let metadata = footoken_metadata(gravity_contact).await;
    submit_and_pass_ibc_metadata_proposal(
        metadata.name.clone(),
        metadata.clone(),
        gravity_contact,
        &keys,
    )
    .await;
    run_all_recoverable_tests(
        web30,
        gravity_contact,
        grpc_client.clone(),
        keys.clone(),
        gravity_address,
        erc20_addresses.clone(),
        metadata.clone(),
    )
    .await;
    run_upgrade_specific_tests(
        web30,
        gravity_contact,
        ibc_contact,
        grpc_client.clone(),
        keys.clone(),
        ibc_keys.clone(),
        gravity_address,
        erc20_addresses.clone(),
        false,
    )
    .await;

    let upgrade_height = run_upgrade(gravity_contact, keys, UPGRADE_NAME.to_string(), false).await;

    // Check that the expected attestations exist
    check_attestations(grpc_client.clone(), MINIMUM_ATTESTATIONS).await;

    info!(
        "Ready to run the new binary, waiting for chain panic at upgrade height of {}!",
        upgrade_height
    );
    // Wait for the block before the upgrade height, we won't get a response from the chain
    let res = wait_for_block(gravity_contact, (upgrade_height - 1) as u64).await;
    if res.is_err() {
        panic!("Unable to wait for upgrade! {}", res.err().unwrap());
    }

    delay_for(Duration::from_secs(10)).await; // wait for the new block to halt the chain
    let status = gravity_contact.get_chain_status().await;
    info!(
        "Done waiting, chain should be halted, status response: {:?}",
        status
    );
}

/// Perform a series of integration tests after an upgrade has executed
/// NOTE: To run this test, follow the instructions for v2_upgrade_part_1 and WAIT FOR CHAIN HALT,
/// then finally run tests/run-tests.sh with V2_UPGRADE_PART_2 as the test type.
#[allow(clippy::too_many_arguments)]
pub async fn upgrade_part_2(
    web30: &Web3,
    gravity_contact: &Contact,
    ibc_contact: &Contact,
    grpc_client: GravityQueryClient<Channel>,
    keys: Vec<ValidatorKeys>,
    ibc_keys: Vec<CosmosPrivateKey>,
    gravity_address: EthAddress,
    erc20_addresses: Vec<EthAddress>,
) {
    info!("Starting upgrade_part_2 test");
    // Check that the expected attestations exist
    check_attestations(grpc_client.clone(), MINIMUM_ATTESTATIONS).await;

    let mut metadata: Option<Metadata> = None;
    {
        let all_metadata = gravity_contact.get_all_denoms_metadata().await.unwrap();
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

    submit_and_pass_ibc_metadata_proposal(
        metadata.name.clone(),
        metadata.clone(),
        gravity_contact,
        &keys,
    )
    .await;
    run_all_recoverable_tests(
        web30,
        gravity_contact,
        grpc_client.clone(),
        keys.clone(),
        gravity_address,
        erc20_addresses.clone(),
        metadata.clone(),
    )
    .await;
    run_upgrade_specific_tests(
        web30,
        gravity_contact,
        ibc_contact,
        grpc_client.clone(),
        keys.clone(),
        ibc_keys,
        gravity_address,
        erc20_addresses.clone(),
        true,
    )
    .await;
}

pub async fn run_upgrade(
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    plan_name: String,
    wait_for_upgrade: bool,
) -> i64 {
    let curr_height = contact.get_chain_status().await.unwrap();
    let curr_height = if let ChainStatus::Moving { block_height } = curr_height {
        block_height
    } else {
        panic!("Chain is not moving!");
    };
    let upgrade_height = (curr_height + 40) as i64;
    let upgrade_prop_params = UpgradeProposalParams {
        upgrade_height,
        plan_name,
        plan_info: "upgrade info here".to_string(),
        proposal_title: "proposal title here".to_string(),
        proposal_desc: "proposal description here".to_string(),
    };
    info!(
        "Starting upgrade vote with params name: {}, height: {}",
        upgrade_prop_params.plan_name.clone(),
        upgrade_height
    );
    execute_upgrade_proposal(contact, &keys, None, upgrade_prop_params).await;

    if wait_for_upgrade {
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
    upgrade_height
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

// These tests should fail in upgrade_part_1() but pass in upgrade_part_2()
#[allow(clippy::too_many_arguments)]
pub async fn run_upgrade_specific_tests(
    _web30: &Web3,
    _gravity_contact: &Contact,
    _ibc_contact: &Contact,
    _grpc_client: GravityQueryClient<Channel>,
    _keys: Vec<ValidatorKeys>,
    _ibc_keys: Vec<CosmosPrivateKey>,
    _gravity_address: EthAddress,
    _erc20_addresses: Vec<EthAddress>,
    _post_upgrade: bool,
) {
}

/// Checks that the expected attestations are returned from the grpc endpoint
async fn check_attestations(grpc_client: QueryClient<Channel>, expected_attestations: u64) {
    info!("Checking attestations before the upgrade");
    let mut grpc_client = grpc_client;
    let attestations = grpc_client
        .get_attestations(QueryAttestationsRequest {
            limit: 0,
            order_by: "desc".to_string(),
            claim_type: "".to_string(),
            nonce: 0,
            height: 0,
            use_v1_key: false,
        })
        .await
        .expect("Failed to get attestations pre-upgrade")
        .into_inner()
        .attestations;
    assert!(!attestations.is_empty());
    assert!(attestations.len() >= expected_attestations as usize);
    let mut claims: Vec<Box<dyn EthereumClaim>> = vec![];
    for (i, att) in attestations.iter().enumerate() {
        assert!(att.observed);
        let claim_any = att
            .claim
            .clone()
            .unwrap_or_else(|| panic!("Attestation {} had no claims!", i));
        claims.push(unpack_and_print_claim_info(claim_any, i));
    }
    let mut sends_to_cosmos = 0;
    let mut batches = 0;
    let mut erc20s_deployed = 0;
    let mut logic_calls = 0;
    let mut valsets_updated = 0;
    for claim in claims {
        match claim.get_type() {
            ClaimType::Unspecified => panic!("Unexpected claim!"),
            ClaimType::SendToCosmos => sends_to_cosmos += 1,
            ClaimType::BatchSendToEth => batches += 1,
            ClaimType::Erc20Deployed => erc20s_deployed += 1,
            ClaimType::LogicCallExecuted => logic_calls += 1,
            ClaimType::ValsetUpdated => valsets_updated += 1,
        }
    }
    assert_eq!(sends_to_cosmos, EXPECTED_SENDS_TO_COSMOS);
    assert_eq!(batches, EXPECTED_BATCHES);
    assert_eq!(erc20s_deployed, EXPECTED_ERC20S);
    assert_eq!(logic_calls, EXPECTED_LOGIC_CALLS);
    assert!(valsets_updated >= MINIMUM_VALSETS); // New valsets can be created at any time, we want at least as many as we had
}

fn unpack_and_print_claim_info(claim_any: prost_types::Any, i: usize) -> Box<dyn EthereumClaim> {
    let claim: Box<dyn EthereumClaim>;
    if claim_any.type_url == MSG_SEND_TO_COSMOS_CLAIM_TYPE_URL {
        claim = Box::new(decode_any::<MsgSendToCosmosClaim>(claim_any).unwrap());
        info!(
            "Claim {} is {} at height {} with nonce {}",
            i,
            claim.get_type().to_string(),
            claim.get_eth_block_height(),
            claim.get_event_nonce()
        )
    } else if claim_any.type_url == MSG_BATCH_SEND_TO_ETH_TYPE_URL {
        claim = Box::new(decode_any::<MsgBatchSendToEthClaim>(claim_any).unwrap());
        info!(
            "Claim {} is {} at height {} with nonce {}",
            i,
            claim.get_type().to_string(),
            claim.get_eth_block_height(),
            claim.get_event_nonce()
        )
    } else if claim_any.type_url == MSG_ERC20_DEPLOYED_CLAIM_TYPE_URL {
        claim = Box::new(decode_any::<MsgErc20DeployedClaim>(claim_any).unwrap());
        info!(
            "Claim {} is {} at height {} with nonce {}",
            i,
            claim.get_type().to_string(),
            claim.get_eth_block_height(),
            claim.get_event_nonce()
        )
    } else if claim_any.type_url == MSG_LOGIC_CALL_EXECUTED_CLAIM_TYPE_URL {
        claim = Box::new(decode_any::<MsgLogicCallExecutedClaim>(claim_any).unwrap());
        info!(
            "Claim {} is {} at height {} with nonce {}",
            i,
            claim.get_type().to_string(),
            claim.get_eth_block_height(),
            claim.get_event_nonce()
        )
    } else if claim_any.type_url == MSG_VALSET_UPDATED_CLAIM_TYPE_URL {
        claim = Box::new(decode_any::<MsgValsetUpdatedClaim>(claim_any).unwrap());
        info!(
            "Claim {} is {} at height {} with nonce {}",
            i,
            claim.get_type().to_string(),
            claim.get_eth_block_height(),
            claim.get_event_nonce()
        )
    } else {
        panic!("Unexpected claim type detected! {:?}", claim_any);
    }

    claim
}
