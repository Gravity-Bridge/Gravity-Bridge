//! This is a test for the Airdrop proposal governance handler, which allows the community to propose
//! and automatically execute an Airdrop out of the community pool

use crate::utils::{
    create_parameter_change_proposal, get_coins, vote_yes_on_proposals, ValidatorKeys,
};
use crate::ADDRESS_PREFIX;
use crate::STAKING_TOKEN;
use crate::{get_deposit, get_fee, TOTAL_TIMEOUT};
use clarity::Uint256;
use cosmos_gravity::proposals::{
    submit_airdrop_proposal, AirdropProposalJson, AIRDROP_PROPOSAL_TYPE_URL,
};
use deep_space::error::CosmosGrpcError;
use deep_space::utils::encode_any;
use deep_space::Address as CosmosAddress;
use deep_space::Contact;
use gravity_proto::cosmos_sdk_proto::cosmos::params::v1beta1::ParamChange;
use gravity_proto::gravity::AirdropProposal as AirdropProposalMsg;
use rand::prelude::ThreadRng;
use rand::Rng;
use std::time::{Duration, Instant};
use tokio::time::sleep;

/// the maximum possible number of airdrop recipients
const NUM_AIRDROP_RECIPIENTS: u16 = 35_000;

/// Produce and send a number of valid and invalid airdrops in order to demonstrate
/// correct behavior.
pub async fn airdrop_proposal_test(contact: &Contact, keys: Vec<ValidatorKeys>) {
    // we start by disabling inflation, this allows us to check that the correct
    // amount of tokens was withdrawn from the community pool, if we don't do this inflation
    // in the community pool may be larger than our actual airdrop amount spent.
    disable_inflation(contact, &keys).await;

    // now that inflation is disabled we can query the community pool and expect the value to only go down
    let bad_airdrop_denom = "notoken".to_string();

    // submit an invalid airdrop token type
    submit_and_fail_airdrop_proposal(bad_airdrop_denom.clone(), &keys, contact, false, false).await;

    // submit an airdrop token with an invalid address
    submit_and_fail_airdrop_proposal(STAKING_TOKEN.clone(), &keys, contact, true, false).await;

    // submit the actual valid airdrop, max number of possible recipients
    submit_and_pass_airdrop_proposal(
        STAKING_TOKEN.clone(),
        NUM_AIRDROP_RECIPIENTS,
        &keys,
        contact,
    )
    .await
    .unwrap();

    // submit one more airdrop with a random number of recipients
    let mut rng = rand::thread_rng();
    let recipients: u16 = rng.gen_range(1..NUM_AIRDROP_RECIPIENTS);
    submit_and_pass_airdrop_proposal(STAKING_TOKEN.clone(), recipients, &keys, contact)
        .await
        .unwrap();

    // submit an airdrop that contains more tokens than are in the community pool
    submit_and_fail_airdrop_proposal(STAKING_TOKEN.clone(), &keys, contact, false, true).await;
}

// Submits the custom airdrop governance proposal, votes yes for each validator, waits for votes to be submitted
async fn submit_and_pass_airdrop_proposal(
    denom: String,
    num_recipients: u16,
    keys: &[ValidatorKeys],
    contact: &Contact,
) -> Result<bool, CosmosGrpcError> {
    let community_pool_contents_start = contact.query_community_pool().await.unwrap();
    let total_supply_start = contact.query_supply_of(denom.clone()).await.unwrap();
    let starting_amount_in_pool =
        get_coins(&*STAKING_TOKEN, &community_pool_contents_start).unwrap();
    let (recipients, amounts) =
        generate_valid_accounts_and_amounts(num_recipients, starting_amount_in_pool.amount.clone());

    let proposal_content = AirdropProposalJson {
        title: format!("Proposal to perform {} airdrop", denom),
        description: "Airdrop time!".to_string(),
        denom: denom.clone(),
        amounts: amounts.to_vec(),
        recipients: recipients.to_vec(),
    };

    let res = submit_airdrop_proposal(
        proposal_content,
        get_deposit(),
        get_fee(None),
        contact,
        keys[0].validator_key,
        Some(TOTAL_TIMEOUT),
    )
    .await
    .unwrap();

    info!(
        "Submit and pass airdrop proposal: for {} to {} recipients for {} gas",
        total_array(&amounts),
        recipients.len(),
        res.gas_used
    );

    vote_yes_on_proposals(contact, keys, None).await;
    wait_for_proposals_to_execute(contact).await;

    let mut error = false;
    // make sure everyone got their airdrop amount
    for (key, amount) in recipients.iter().zip(amounts.iter()) {
        let balances = contact
            .get_balance(*key, STAKING_TOKEN.to_string())
            .await
            .unwrap();

        assert!(balances.is_some());
        match balances {
            Some(balance) => {
                let big_amount: Uint256 = (*amount).into();
                // assert_eq!(balances.unwrap().amount, big_amount);
                if balance.amount != big_amount {
                    error!(
                        "Address {} did not get the correct amount {} vs {}",
                        key, balance.amount, big_amount
                    );
                    error = true;
                }
            }
            None => {
                error!(
                    "Address {} did not get an airdrop, should have {} got nothing",
                    key, amount
                );
                error = true;
            }
        }
    }
    if error {
        panic!("Failed to distribute airdrop amounts correctly!")
    }

    // assert that no tokens where created by this airdrop, remember inflation is disabled
    let total_supply_end = contact.query_supply_of(denom.clone()).await.unwrap();
    assert_eq!(total_supply_start, total_supply_end);

    // assert that the community pool has been properly reduced, remember inflation is disabled
    let community_pool_contents_end = contact.query_community_pool().await.unwrap();
    let ending_amount_in_pool = get_coins(&*STAKING_TOKEN, &community_pool_contents_end).unwrap();
    info!(
        "FeePool start {} and End {}",
        starting_amount_in_pool.amount, ending_amount_in_pool.amount
    );
    assert_eq!(
        starting_amount_in_pool.amount - total_array(&amounts),
        ending_amount_in_pool.amount
    );

    info!("Successfully Issued Airdrop!");
    Ok(true)
}

// Submits the custom airdrop governance proposal, panics if the airdrop submission does not fail
async fn submit_and_fail_airdrop_proposal(
    denom: String,
    keys: &[ValidatorKeys],
    contact: &Contact,
    // used to test sending a junk user key
    make_invalid: bool,
    // makes the proposal amounts total larger than the community pool supply
    make_too_many: bool,
) {
    // ensures we are generating a set of recipients and amounts that is possible to execute
    let community_pool_contents_start = contact.query_community_pool().await.unwrap();
    let starting_amount_in_pool =
        get_coins(&*STAKING_TOKEN, &community_pool_contents_start).unwrap();

    let (recipients, amounts) = if make_too_many {
        generate_invalid_accounts_and_amounts(
            NUM_AIRDROP_RECIPIENTS,
            starting_amount_in_pool.amount.clone(),
        )
    } else {
        generate_valid_accounts_and_amounts(
            NUM_AIRDROP_RECIPIENTS,
            starting_amount_in_pool.amount.clone(),
        )
    };

    let mut byte_recipients = Vec::new();
    for r in recipients {
        byte_recipients.extend_from_slice(r.as_bytes())
    }
    if make_invalid {
        byte_recipients.extend_from_slice(&[0, 1, 2, 3, 4]);
    }

    let proposal_content = AirdropProposalMsg {
        title: format!("Proposal to perform {} airdrop", denom),
        description: "Airdrop time!".to_string(),
        denom,
        amounts: amounts.to_vec(),
        recipients: byte_recipients,
    };
    info!(
        "Submit and fail airdrop proposal: for {} to {} recipients",
        total_array(&amounts),
        amounts.len()
    );

    // encode as a generic proposal
    let any = encode_any(proposal_content, AIRDROP_PROPOSAL_TYPE_URL.to_string());

    let res = contact
        .create_gov_proposal(
            any,
            get_deposit(),
            get_fee(None),
            keys[0].validator_key,
            Some(TOTAL_TIMEOUT),
        )
        .await;

    // proposal fails tx simulation and does not enter the chain
    assert!(res.is_err());
}

/// waits for the governance proposal to execute by waiting for it to leave
/// the 'voting' status
pub async fn wait_for_proposals_to_execute(contact: &Contact) {
    let start = Instant::now();
    loop {
        let proposals = contact
            .get_governance_proposals_in_voting_period()
            .await
            .unwrap();
        if Instant::now() - start > TOTAL_TIMEOUT {
            panic!("Gov proposal did not execute")
        } else if proposals.proposals.is_empty() {
            return;
        }
        sleep(Duration::from_secs(5)).await;
    }
}

/// Generates random airdrops up to NUM_AIRDROP_RECIPIENTS random participants with
/// random amounts
fn generate_accounts_and_amounts(
    num_recipients: u16,
    rng: &mut ThreadRng,
) -> (Vec<CosmosAddress>, Vec<u64>) {
    // Generate user keys for the airdrop, converting between private key and address
    // is quite slow, so we skip that step and go directly to an address
    let mut user_addresses = Vec::new();
    let mut amounts: Vec<u64> = Vec::new();
    for _ in 0..num_recipients {
        let secret: [u8; 20] = rng.gen();
        let amount: u64 = rng.gen();
        let cosmos_address = CosmosAddress::from_bytes(secret, ADDRESS_PREFIX.as_str()).unwrap();
        user_addresses.push(cosmos_address);
        amounts.push(amount)
    }
    (user_addresses, amounts)
}

/// This function generates a random list of accounts and amounts for the airdrop
/// then it goes back over this array and replaces random indexes with smaller amounts
/// until the total size of the airdrop is less than the size of the community pool
fn generate_valid_accounts_and_amounts(
    num_recipients: u16,
    max: Uint256,
) -> (Vec<CosmosAddress>, Vec<u64>) {
    info!("Staring valid accounts + amounts generation");
    let mut rng = rand::thread_rng();
    let (user_addresses, mut amounts) = generate_accounts_and_amounts(num_recipients, &mut rng);
    while total_array(&amounts) > max {
        let random_idx = rng.gen_range(0..amounts.len());
        let new_val: u16 = rng.gen();
        amounts[random_idx] = new_val.into();
    }
    info!("Finished valid accounts + amounts generation");
    (user_addresses, amounts)
}

/// This function generates a random list of accounts and amounts for the airdrop
/// then it goes back over this array and replaces random indexes with larger amounts
/// until the total size of the airdrop is greater than the size of the community pool
fn generate_invalid_accounts_and_amounts(
    num_recipients: u16,
    max: Uint256,
) -> (Vec<CosmosAddress>, Vec<u64>) {
    info!("Started invalid accounts + amounts generation");
    let mut rng = rand::thread_rng();
    let (user_addresses, mut amounts) = generate_accounts_and_amounts(num_recipients, &mut rng);
    while total_array(&amounts) < max {
        let random_idx = rng.gen_range(0..amounts.len());
        let new_val: u64 = u64::MAX;
        amounts[random_idx] = new_val;
    }
    info!("Finished invalid accounts + amounts generation");
    (user_addresses, amounts)
}

fn total_array(input: &[u64]) -> Uint256 {
    let mut out = 0u8.into();
    for v in input {
        out += (*v).into()
    }
    out
}

/// submits and passes a proposal to disable inflation on the chain
pub async fn disable_inflation(contact: &Contact, keys: &[ValidatorKeys]) {
    info!("Submitting and passing a proposal to zero out inflation");
    let mut params_to_change = Vec::new();
    let change = ParamChange {
        subspace: "mint".to_string(),
        key: "InflationRateChange".to_string(),
        value: r#""0""#.to_string(),
    };
    params_to_change.push(change);
    let change = ParamChange {
        subspace: "mint".to_string(),
        key: "InflationMin".to_string(),
        value: r#""0""#.to_string(),
    };
    params_to_change.push(change);
    let change = ParamChange {
        subspace: "mint".to_string(),
        key: "InflationMax".to_string(),
        value: r#""0""#.to_string(),
    };
    params_to_change.push(change);
    create_parameter_change_proposal(contact, keys[0].validator_key, params_to_change).await;
    vote_yes_on_proposals(contact, keys, None).await;
    wait_for_proposals_to_execute(contact).await;
}
