//! This is a test for the ibc metadata proposal, which is a way to set the denom metadata for a given

use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::happy_path_v2::deploy_cosmos_representing_erc20_and_check_adoption;
use crate::utils::{vote_yes_on_proposals, ValidatorKeys};
use crate::{get_deposit, get_fee, TOTAL_TIMEOUT};
use clarity::Address;
use cosmos_gravity::proposals::submit_ibc_metadata_proposal;
use deep_space::Contact;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::{DenomUnit, Metadata};
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::IbcMetadataProposal;
use tonic::transport::Channel;
use web30::client::Web3;

pub async fn ibc_metadata_proposal_test(
    gravity_address: Address,
    keys: Vec<ValidatorKeys>,
    grpc_client: GravityQueryClient<Channel>,
    contact: &Contact,
    web30: &Web3,
) {
    let mut grpc_client = grpc_client;
    let metadata_start = contact.get_all_denoms_metadata().await.unwrap();
    info!("Starting metadata {:?}", metadata_start);
    // the token we want to try and set the metadata of
    let target_denom = "ibc/nometadatatoken".to_string();
    // already has metadta set, should fail
    let invalid_denom = "footoken".to_string();

    // totally invalid metadata to test
    let invalid_metadata = Metadata {
        description: "invalid".to_string(),
        denom_units: Vec::new(),
        base: "no good".to_string(),
        display: "nada".to_string(),
        name: "invalid".to_string(),
        symbol: "INV".to_string(),
    };

    // valid metadata to test, but would overwrite existing metadata
    let foo_metadata = Metadata {
        description: "A testing token".to_string(),
        denom_units: vec![
            DenomUnit {
                denom: "footoken".to_string(),
                exponent: 0,
                aliases: vec![],
            },
            DenomUnit {
                denom: "mfootoken".to_string(),
                exponent: 6,
                aliases: vec![],
            },
        ],
        base: "footoken".to_string(),
        display: "mfootoken".to_string(),
        name: "Footoken".to_string(),
        symbol: "FOO".to_string(),
    };

    // valid metadata to test, but would overwrite existing metadata
    let test_metadata = Metadata {
        description: "this token gets metadata".to_string(),
        denom_units: vec![
            DenomUnit {
                denom: target_denom.clone(),
                exponent: 0,
                aliases: vec![],
            },
            DenomUnit {
                denom: "mnometadatatoken".to_string(),
                exponent: 6,
                aliases: vec![],
            },
        ],
        base: target_denom.clone(),
        display: "mnometadatatoken".to_string(),
        name: "Metadata token".to_string(),
        symbol: "META".to_string(),
    };

    // check that a totally invalid version does not work
    submit_and_fail_ibc_metadata_proposal(
        "invalid".to_string(),
        invalid_metadata.clone(),
        contact,
        &keys,
    )
    .await;
    // check to make sure we target denom and metadata denom matches
    submit_and_fail_ibc_metadata_proposal(invalid_denom, foo_metadata.clone(), contact, &keys)
        .await;

    // try and overwrite the footoken metadata
    submit_and_fail_ibc_metadata_proposal(foo_metadata.base.clone(), foo_metadata, contact, &keys)
        .await;

    // the actual valid proposal
    submit_and_pass_ibc_metadata_proposal(target_denom.clone(), test_metadata, contact, &keys)
        .await;
    let metadata_end = contact.get_all_denoms_metadata().await.unwrap();
    info!("Ending metadata {:?}", metadata_end);
    let mut found = None;
    for m in metadata_end {
        if m.base == target_denom {
            found = Some(m);
        }
    }
    assert!(found.is_some());
    let found = found.unwrap();
    info!("Successfully set IBC metadata");

    info!("Deploying representative ERC20");
    deploy_cosmos_representing_erc20_and_check_adoption(
        gravity_address,
        web30,
        Some(keys),
        &mut grpc_client,
        false,
        found,
    )
    .await;
}

async fn submit_and_pass_ibc_metadata_proposal(
    denom: String,
    metadata: Metadata,
    contact: &Contact,
    keys: &[ValidatorKeys],
) {
    let proposal_content = IbcMetadataProposal {
        title: format!("Proposal to set metadata on {}", denom),
        description: "IBC METADATA!".to_string(),
        ibc_denom: denom,
        metadata: Some(metadata),
    };
    let res = submit_ibc_metadata_proposal(
        proposal_content,
        get_deposit(),
        get_fee(),
        contact,
        keys[0].validator_key,
        Some(TOTAL_TIMEOUT),
    )
    .await;
    vote_yes_on_proposals(contact, keys, None).await;
    wait_for_proposals_to_execute(contact).await;
    trace!("Gov proposal executed with {:?}", res);
}

async fn submit_and_fail_ibc_metadata_proposal(
    denom: String,
    metadata: Metadata,
    contact: &Contact,
    keys: &[ValidatorKeys],
) {
    let proposal_content = IbcMetadataProposal {
        title: format!("Proposal to set metadata on {}", denom),
        description: "IBC METADATA!".to_string(),
        ibc_denom: denom,
        metadata: Some(metadata),
    };
    let res = submit_ibc_metadata_proposal(
        proposal_content,
        get_deposit(),
        get_fee(),
        contact,
        keys[0].validator_key,
        Some(TOTAL_TIMEOUT),
    )
    .await;
    assert!(res.is_err());
}
