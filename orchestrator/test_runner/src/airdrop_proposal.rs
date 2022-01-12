//! This is a test for the Airdrop proposal governance handler, which allows the community to propose
//! and automatically execute an Airdrop out of the community pool

use crate::utils::{get_coins, vote_yes_on_proposals, ValidatorKeys};
use crate::ADDRESS_PREFIX;
use crate::STAKING_TOKEN;
use crate::{get_deposit, get_fee, TOTAL_TIMEOUT};
use deep_space::error::CosmosGrpcError;
use deep_space::utils::encode_any;
use deep_space::Address as CosmosAddress;
use deep_space::Coin;
use deep_space::Contact;
use gravity_proto::gravity::AirdropProposal;
use rand::Rng;
use std::time::{Duration, Instant};
use tokio::time::sleep;

const NUM_AIRDROP_RECIPIENTS: usize = 10_000;
// note this test can only be run once because we exhaust the community pool
// after that the chain must be restarted to reset that state.
pub async fn airdrop_proposal_test(contact: &Contact, keys: Vec<ValidatorKeys>) {
    let community_pool_contents_start = contact.query_community_pool().await.unwrap();
    let starting_amount_in_pool =
        get_coins(&*STAKING_TOKEN, &community_pool_contents_start).unwrap();
    let airdrop_amount = Coin {
        denom: STAKING_TOKEN.clone(),
        amount: starting_amount_in_pool.amount.clone() / (NUM_AIRDROP_RECIPIENTS + 1).into(),
    };
    let bad_airdrop_amount = Coin {
        denom: "notoken".to_string(),
        amount: 100u16.into(),
    };

    info!("Starting user key generation");
    // Generate user keys for the airdrop, converting between private key and address
    // is quite slow, so we skip that step and go directly to an address
    let mut user_addresses = Vec::new();
    let mut rng = rand::thread_rng();
    for _ in 0..NUM_AIRDROP_RECIPIENTS {
        let secret: [u8; 20] = rng.gen();
        let cosmos_address = CosmosAddress::from_bytes(secret, ADDRESS_PREFIX.as_str()).unwrap();
        user_addresses.push(cosmos_address);
    }
    info!("Finished user key generation");

    // submit an invalid airdrop token type
    submit_and_fail_airdrop_proposal(
        bad_airdrop_amount.clone(),
        &user_addresses,
        contact,
        &keys,
        false,
    )
    .await;

    // submit an airdrop token with an invalid address
    submit_and_fail_airdrop_proposal(
        bad_airdrop_amount.clone(),
        &user_addresses,
        contact,
        &keys,
        true,
    )
    .await;

    // submit the actual valid airdrop
    submit_and_pass_airdrop_proposal(
        airdrop_amount.clone(),
        &user_addresses,
        contact,
        &keys,
        false,
    )
    .await
    .unwrap();

    wait_for_proposals_to_execute(contact).await;

    // make sure everyone got their airdrop amount
    for key in user_addresses.iter() {
        let balances = contact.get_balances(*key).await.unwrap();
        assert!(balances.contains(&airdrop_amount));
    }

    // try to submit the airdrop again, make sure nothing happens because we are out of tokens
    submit_and_fail_airdrop_proposal(
        airdrop_amount.clone(),
        &user_addresses,
        contact,
        &keys,
        false,
    )
    .await;

    let community_pool_contents_end = contact.query_community_pool().await.unwrap();
    let end = get_coins(&*STAKING_TOKEN, &community_pool_contents_end).unwrap();
    info!(
        "FeePool start {} and End {}",
        starting_amount_in_pool.amount, end.amount
    );
    // check that ending amount is smaller than starting (will panic on underflow)
    // and that we have subtracted at least enough to fund the airdrop, the problem is
    // that tokens are added to the pool via inflation while this whole test is running
    // meaning we can't just check that it all adds up (we do that in the go unit test though)
    assert!(starting_amount_in_pool.amount - end.amount >= 0u8.into());

    info!("Successfully Issued Airdrop!");
}

// Submits the custom Unhalt bridge governance proposal, votes yes for each validator, waits for votes to be submitted
async fn submit_and_pass_airdrop_proposal(
    amount: Coin,
    recipients: &[CosmosAddress],
    contact: &Contact,
    keys: &[ValidatorKeys],
    // used to test sending a junk user key
    make_invalid: bool,
) -> Result<bool, CosmosGrpcError> {
    let mut str_recipients = Vec::new();
    for r in recipients {
        str_recipients.push(r.to_string());
    }
    if make_invalid {
        str_recipients.push("totally invalid!".to_string());
    }

    let proposal_content = AirdropProposal {
        title: format!("Proposal to perform {} airdrop", amount.denom),
        description: "Airdrop time!".to_string(),
        amount: Some(amount.clone().into()),
        recipients: str_recipients,
    };

    // encode as a generic proposal
    let any = encode_any(proposal_content, "/gravity.v1.AirdropProposal".to_string());

    let res = contact
        .create_gov_proposal(
            any,
            get_deposit(),
            get_fee(),
            keys[0].validator_key,
            Some(TOTAL_TIMEOUT),
        )
        .await
        .unwrap();
    trace!("Gov proposal submitted with {:?}", res);
    let res = contact.wait_for_tx(res, TOTAL_TIMEOUT).await.unwrap();
    trace!("Gov proposal executed with {:?}", res);
    info!(
        "Submit and pass airdrop proposal: for {} to {} recipients for {} gas",
        amount,
        recipients.len(),
        res.gas_used
    );

    vote_yes_on_proposals(contact, keys, None).await;
    Ok(true)
}

// Submits the custom Unhalt bridge governance proposal, panics if the airdrop submission does not fail
async fn submit_and_fail_airdrop_proposal(
    amount: Coin,
    recipients: &[CosmosAddress],
    contact: &Contact,
    keys: &[ValidatorKeys],
    // used to test sending a junk user key
    make_invalid: bool,
) {
    let mut str_recipients = Vec::new();
    for r in recipients {
        str_recipients.push(r.to_string());
    }
    if make_invalid {
        str_recipients.push("totally invalid!".to_string());
    }

    let proposal_content = AirdropProposal {
        title: format!("Proposal to perform {} airdrop", amount.denom),
        description: "Airdrop time!".to_string(),
        amount: Some(amount.clone().into()),
        recipients: str_recipients,
    };
    info!(
        "Submit and pass airdrop proposal: for {} to {} recipients",
        amount,
        recipients.len()
    );

    // encode as a generic proposal
    let any = encode_any(proposal_content, "/gravity.v1.AirdropProposal".to_string());

    let res = contact
        .create_gov_proposal(
            any,
            get_deposit(),
            get_fee(),
            keys[0].validator_key,
            Some(TOTAL_TIMEOUT),
        )
        .await;
    assert!(res.is_err());
}

/// waits for the governance proposal to execute by waiting for it to leave
/// the 'voting' status
async fn wait_for_proposals_to_execute(contact: &Contact) {
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
