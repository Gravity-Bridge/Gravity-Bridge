use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::happy_path::{test_erc20_deposit_panic, test_erc20_deposit_result};
use crate::{get_deposit, utils::*, TOTAL_TIMEOUT};
use crate::{get_fee, one_eth, OPERATION_TIMEOUT};
use bytes::BytesMut;
use clarity::{Address as EthAddress, Uint256};
use cosmos_gravity::proposals::submit_unhalt_bridge_proposal;
use cosmos_gravity::query::{get_attestations, get_last_event_nonce_for_validator};
use deep_space::private_key::PrivateKey as CosmosPrivateKey;
use deep_space::{Contact, Fee};
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::MsgSendToCosmosClaim;
use gravity_proto::gravity::UnhaltBridgeProposal;
use gravity_utils::num_conversion::downcast_uint256;
use prost::Message;
use std::str::FromStr;
use std::time::{Duration, Instant};
use tokio::time::sleep;
use tonic::transport::Channel;
use web30::client::Web3;

// Halts the bridge by having some validators lie about a SendToCosmos claim, asserts bridge is halted,
// then resets the bridge back to the last valid nonce via governance vote, asserts bridge resumes functioning
pub async fn unhalt_bridge_test(
    web30: &Web3,
    grpc_client: GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_address: EthAddress,
) {
    let prefix = contact.get_prefix();
    let mut grpc_client = grpc_client;
    let no_relay_market_config = create_default_test_config();
    let bridge_user = get_user_key();

    info!("Sending bridge user some tokens");
    send_one_eth(bridge_user.eth_address, web30).await;
    send_erc20_bulk(
        one_eth() * 10u64.into(),
        erc20_address,
        &[bridge_user.eth_address],
        web30,
    )
    .await;
    let fee = Fee {
        amount: vec![get_fee()],
        gas_limit: 500_000_000u64,
        granter: None,
        payer: None,
    };

    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;
    let lying_validators: Vec<CosmosPrivateKey> =
        keys[1..3].iter().map(|key| key.orch_key).collect();

    print_validator_stake(contact).await;

    while get_event_nonce_safe(gravity_address, web30, bridge_user.eth_address)
        .await
        .unwrap()
        == 0
    {
        // this prevents race conditions by allowing the orchestrators to submit the events they
        // have seen so far, since events are submitted in batches we only need to wait for a value
        // that's not zero to be sure that all events so far have been submitted.
        info!("Waiting for Orchestrators to warm up");
        sleep(Duration::from_secs(5)).await;
    }

    info!("Test bridge before false claims!");
    // Test a deposit to increment the event nonce before false claims happen
    test_erc20_deposit_panic(
        web30,
        contact,
        &mut grpc_client,
        bridge_user.cosmos_address,
        gravity_address,
        erc20_address,
        10_000_000_000_000_000u64.into(),
        None,
        None,
    )
    .await;

    // wait for several blocks to pass to reduce orchestrator race conditions
    for _ in 0..15 {
        contact
            .wait_for_next_block(OPERATION_TIMEOUT)
            .await
            .unwrap();
    }

    let start = Instant::now();
    let mut initial_nonces_same = false;
    let mut initial_valid_nonce = None;
    while Instant::now() - start < TOTAL_TIMEOUT {
        // These are the nonces each validator is aware of before false claims are submitted
        let initial_nonces = get_nonces(&mut grpc_client, &keys, &prefix).await;
        initial_nonces_same = initial_nonces[0] == initial_nonces[1]
            && initial_nonces[0] == initial_nonces[2]
            && initial_nonces[0] == initial_nonces[3];
        if initial_nonces_same {
            initial_valid_nonce = Some(initial_nonces[0]);
            break;
        }
        sleep(Duration::from_secs(1)).await;
    }
    // All nonces should be the same right now
    assert!(initial_nonces_same, "The initial nonces differed!");

    let initial_block_height =
        downcast_uint256(web30.eth_get_latest_block().await.unwrap().number).unwrap();
    // At this point we can use any nonce since all the validators have the same state
    let initial_valid_nonce = initial_valid_nonce.unwrap();

    info!("Two validators submitting false claims!");
    submit_false_claims(
        &lying_validators,
        initial_valid_nonce + 1,
        initial_block_height + 1,
        one_eth(),
        bridge_user.cosmos_address,
        bridge_user.eth_address,
        erc20_address,
        contact,
        &fee,
        Some(OPERATION_TIMEOUT),
    )
    .await;

    contact
        .wait_for_next_block(OPERATION_TIMEOUT)
        .await
        .unwrap();

    info!("Checking that bridge is halted!");

    let halted_bridge_amt = Uint256::from_str("100_000_000_000_000_000").unwrap();
    // Attempt transaction on halted bridge
    let res = test_erc20_deposit_result(
        web30,
        contact,
        &mut grpc_client,
        bridge_user.cosmos_address,
        gravity_address,
        erc20_address,
        halted_bridge_amt.clone(),
        Some(Duration::from_secs(30)),
        None,
    )
    .await;
    if res.is_ok() {
        panic!("Bridge not halted!")
    }

    sleep(Duration::from_secs(30)).await;

    info!("Getting latest nonce after bridge halt check");
    let after_halt_nonces = get_nonces(&mut grpc_client, &keys, &prefix).await;
    info!(
        "initial_nonce: {} after_halt_nonces: {:?}",
        initial_valid_nonce, after_halt_nonces,
    );

    info!(
        "Bridge successfully locked, starting governance vote to reset nonce to {}.",
        initial_valid_nonce
    );

    info!("Preparing governance proposal!!");
    // Unhalt the bridge
    submit_and_pass_unhalt_bridge_proposal(initial_valid_nonce, contact, &keys).await;
    wait_for_proposals_to_execute(contact).await;

    let after_unhalt_nonces = get_nonces(&mut grpc_client, &keys, &prefix).await;
    let not_equal = after_unhalt_nonces
        .iter()
        .all(|&nonce| nonce == initial_valid_nonce);
    assert!(
        not_equal,
        "The post-reset nonces should not be equal to the initial nonce",
    );

    // After the governance proposal the resync will happen on the next loop.
    info!("Sleeping so that resync can start!");
    sleep(Duration::from_secs(10)).await;

    info!("Observing attestations before bridging asset to cosmos!");
    print_sends_to_cosmos(&grpc_client, true).await;

    let fixed_bridge_amt = Uint256::from_str("50_000_000_000_000_000").unwrap();
    info!("Attempting to resend now that the bridge should be fixed");
    // After the reset, our earlier halted_bridge_amt tx on the halted bridge will go through while our new
    // fixed_bridge_amt tx goes through, we need to pass in the expected amount so the function knows what to watch for
    let expected_increase = Some(halted_bridge_amt.clone() + fixed_bridge_amt.clone());
    let res = test_erc20_deposit_result(
        web30,
        contact,
        &mut grpc_client,
        bridge_user.cosmos_address,
        gravity_address,
        erc20_address,
        fixed_bridge_amt.clone(),
        None,
        expected_increase,
    )
    .await;
    match res.is_ok() {
        true => info!("Successfully bridged asset!"),
        false => panic!("Failed to bridge ERC20!"),
    }
}

// Submits the custom Unhalt bridge governance proposal, votes yes for each validator, waits for votes to be submitted
async fn submit_and_pass_unhalt_bridge_proposal(
    nonce: u64,
    contact: &Contact,
    keys: &[ValidatorKeys],
) {
    let proposal_content = UnhaltBridgeProposal {
        title: "Proposal to reset the oracle".to_string(),
        description: "this resets the oracle to an earlier nonce".to_string(),
        target_nonce: nonce,
    };
    info!("Submit and pass gov proposal: nonce is {}", nonce);
    let res = submit_unhalt_bridge_proposal(
        proposal_content,
        get_deposit(),
        get_fee(),
        contact,
        keys[0].validator_key,
        Some(TOTAL_TIMEOUT),
    )
    .await
    .unwrap();
    trace!("Gov proposal executed with {:?}", res);

    vote_yes_on_proposals(contact, keys, None).await;
}

// gets the last event nonce for each validator
pub async fn get_nonces(
    grpc_client: &mut GravityQueryClient<Channel>,
    keys: &[ValidatorKeys],
    prefix: &str,
) -> Vec<u64> {
    let mut nonces = vec![];
    for validator_keys in keys {
        nonces.push(
            get_last_event_nonce_for_validator(
                grpc_client,
                validator_keys.orch_key.to_address(prefix).unwrap(),
                prefix.to_string(),
            )
            .await
            .unwrap(),
        );
    }
    nonces
}

async fn print_sends_to_cosmos(grpc_client: &GravityQueryClient<Channel>, print_others: bool) {
    let grpc_client = &mut grpc_client.clone();
    let attestations = get_attestations(grpc_client, None).await.unwrap();
    for (i, attestation) in attestations.into_iter().enumerate() {
        let claim = attestation.clone().claim.unwrap();
        if print_others && claim.type_url != "/gravity.v1.MsgSendToCosmosClaim" {
            info!("attestation {}: {:?}", i, &attestation);
            continue;
        }
        let mut buf = BytesMut::with_capacity(claim.value.len());
        buf.extend_from_slice(&claim.value);

        // Here we use the `T` type to decode whatever type of message this attestation holds
        // for use in the `f` function
        let decoded = MsgSendToCosmosClaim::decode(buf);

        info!(
            "attestation {}: votes {:?}\n decoded{:?}",
            i, &attestation.votes, decoded
        );
    }
}
