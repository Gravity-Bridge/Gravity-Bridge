use crate::happy_path::{test_erc20_deposit_bool, test_erc20_deposit_panic};
use crate::utils::*;
use crate::{get_fee, one_eth, OPERATION_TIMEOUT, STAKING_TOKEN, TOTAL_TIMEOUT};
use bytes::BytesMut;
use clarity::{Address as EthAddress, Uint256};
use cosmos_gravity::query::{get_attestations, get_last_event_nonce_for_validator};
use cosmos_gravity::send::MEMO;
use deep_space::address::Address as CosmosAddress;
use deep_space::coin::Coin;
use deep_space::error::CosmosGrpcError;
use deep_space::utils::encode_any;
use deep_space::{Contact, Fee, Msg};
use ethereum_gravity::utils::downcast_uint256;
use futures::future::join_all;
use gravity_proto::cosmos_sdk_proto::cosmos::{
    params::v1beta1::{ParamChange, ParameterChangeProposal},
    tx::v1beta1::BroadcastMode,
};
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::MsgSendToCosmosClaim;
use prost::Message;
use std::str::FromStr;
use std::thread::sleep;
use std::time::{Duration, Instant};
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
    validator_out: bool,
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

    start_orchestrators(
        keys.clone(),
        gravity_address,
        validator_out,
        no_relay_market_config.clone(),
    )
    .await;

    print_validator_stake(contact).await;

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

    let fee = Fee {
        amount: vec![get_fee()],
        gas_limit: 500_000_000u64,
        granter: None,
        payer: None,
    };
    // These are the nonces each validator is aware of before false claims are submitted
    let initial_nonces = get_nonces(&mut grpc_client, &keys, &prefix).await;
    // All nonces should be the same right now
    assert!(
        initial_nonces[0] == initial_nonces[1]
            && initial_nonces[0] == initial_nonces[2]
            && initial_nonces[0] == initial_nonces[3],
        "The initial nonces differed!"
    );
    let initial_block_height =
        downcast_uint256(web30.eth_get_latest_block().await.unwrap().number).unwrap();
    // At this point we can use any nonce since all the validators have the same state
    let initial_valid_nonce = initial_nonces[0];

    info!("Two validators submitting false claims!");
    submit_false_claims(
        &keys,
        initial_valid_nonce + 1,
        initial_block_height + 1,
        &bridge_user,
        &prefix,
        erc20_address,
        contact,
        &fee,
    )
    .await;

    info!("Getting latest nonce after false claims for each validator");
    let after_false_claims_nonces = get_nonces(&mut grpc_client, &keys, &prefix).await;
    info!(
        "initial_nonce: {} after_false_claims_nonces: {:?}",
        initial_valid_nonce, after_false_claims_nonces,
    );

    // validator_1 nonce and validator_2 nonce should be initial + 1 but val1_nonce should not
    assert!(
        after_false_claims_nonces[1] == initial_valid_nonce + 1
            && after_false_claims_nonces[1] == after_false_claims_nonces[2],
        "The false claims validators do not have updated nonces"
    );
    assert_eq!(
        after_false_claims_nonces[0], initial_valid_nonce,
        "The honest validator should not have an updated nonce!"
    );

    info!("Checking that bridge is halted!");

    let halted_bridge_amt = Uint256::from_str("100_000_000_000_000_000").unwrap();
    // Attempt transaction on halted bridge
    let success = test_erc20_deposit_bool(
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
    if success {
        panic!("Bridge not halted!")
    }

    sleep(Duration::from_secs(30));

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
    let deposit = Coin {
        denom: STAKING_TOKEN.to_string(),
        amount: 1_000_000_000u64.into(),
    };
    let _ = submit_and_pass_gov_proposal(initial_valid_nonce, &deposit, contact, &keys)
        .await
        .expect("Governance proposal failed");
    let start = Instant::now();
    loop {
        let new_nonces = get_nonces(&mut grpc_client, &keys, &prefix).await;
        if new_nonces.iter().min() == after_halt_nonces.iter().max() {
            info!(
                "Nonces have not changed: {:?}=>{:?} sleeping before retry",
                after_halt_nonces, new_nonces
            );
            if Instant::now()
                .checked_duration_since(start)
                .unwrap()
                .gt(&Duration::from_secs(10 * 60))
            {
                panic!("10 minutes have elapsed trying to get the validator last nonces to change for val1 and val2!");
            }
            sleep(Duration::from_secs(10));
            continue;
        } else {
            info!("Nonces changed! {:?}=>{:?}", after_halt_nonces, new_nonces);
            break;
        }
    }
    let after_unhalt_nonces = get_nonces(&mut grpc_client, &keys, &prefix).await;
    assert!(
        after_unhalt_nonces
            .iter()
            .all(|&nonce| nonce == initial_valid_nonce),
        "The post-reset nonces are not equal to the initial nonce",
    );

    // After the governance proposal the resync will happen on the next loop.
    info!("Sleeping so that resync can start!");
    sleep(Duration::from_secs(10));

    info!("Observing attestations before bridging asset to cosmos!");
    print_sends_to_cosmos(&grpc_client, true).await;

    let fixed_bridge_amt = Uint256::from_str("50_000_000_000_000_000").unwrap();
    info!("Attempting to resend now that the bridge should be fixed");
    // After the reset, our earlier halted_bridge_amt tx on the halted bridge will go through while our new
    // fixed_bridge_amt tx goes through, we need to pass in the expected amount so the function knows what to watch for
    let expected_increase = Some(halted_bridge_amt.clone() + fixed_bridge_amt.clone());
    let res = test_erc20_deposit_bool(
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
    match res {
        true => info!("Successfully bridged asset!"),
        false => panic!("Failed to bridge ERC20!"),
    }
}

// Submits a governance proposal, votes yes for each validator, waits for votes to be submitted
async fn submit_and_pass_gov_proposal(
    nonce: u64,
    deposit: &Coin,
    contact: &Contact,
    keys: &[ValidatorKeys],
) -> Result<bool, CosmosGrpcError> {
    let proposal = create_unhalt_proposal(nonce);

    let any = encode_any(
        proposal,
        "/cosmos.params.v1beta1.ParameterChangeProposal".to_string(),
    );

    let res = contact
        .create_gov_proposal(
            any.clone(),
            deposit.clone(),
            get_fee(),
            keys[0].validator_key,
            Some(TOTAL_TIMEOUT),
        )
        .await;
    info!("Proposal response is {:?}", res);
    if res.is_err() {
        return Err(res.unwrap_err());
    }

    vote_yes_on_proposals(contact, keys, None).await;
    Ok(true)
}

// creates a proposal to reset the bridge back to event_nonce = `nonce`
fn create_unhalt_proposal(nonce: u64) -> ParameterChangeProposal {
    let mut params_to_change: Vec<ParamChange> = Vec::new();
    // this does not
    let reset_state = ParamChange {
        subspace: "gravity".to_string(),
        key: "ResetBridgeState".to_string(),
        value: serde_json::to_string(&true).unwrap(),
    };
    info!("Submit and pass gov proposal: nonce is {}", nonce);
    params_to_change.push(reset_state);
    let reset_nonce = ParamChange {
        subspace: "gravity".to_string(),
        key: "ResetBridgeNonce".to_string(),
        value: format!("\"{}\"", nonce),
    };
    params_to_change.push(reset_nonce);
    ParameterChangeProposal {
        title: "Reset Bridge State".to_string(),
        description: "Test resetting bridge state to before things were messed up".to_string(),
        changes: params_to_change,
    }
}

// gets the last event nonce for each validator
async fn get_nonces(
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

// Creates a MsgSendToCosmosClaim
fn create_claim(
    nonce: u64,
    height: u64,
    token_contract: &EthAddress,
    initiator_eth_addr: &EthAddress,
    receiver_cosmos_addr: &CosmosAddress,
    orchestrator_addr: &CosmosAddress,
) -> MsgSendToCosmosClaim {
    MsgSendToCosmosClaim {
        event_nonce: nonce,
        block_height: height,
        token_contract: token_contract.to_string(),
        amount: one_eth().to_string(),
        cosmos_receiver: receiver_cosmos_addr.to_string(),
        ethereum_sender: initiator_eth_addr.to_string(),
        orchestrator: orchestrator_addr.to_string(),
    }
}

// Creates a signed send to cosmos message from a claim
async fn create_message(
    claim: &MsgSendToCosmosClaim,
    contact: &Contact,
    key: &ValidatorKeys,
    fee: &Fee,
    prefix: &str,
) -> Vec<u8> {
    let msg_url = "/gravity.v1.MsgSendToCosmosClaim";

    let msg = Msg::new(msg_url, claim.clone());
    let args = contact
        .get_message_args(key.orch_key.to_address(prefix).unwrap(), fee.clone())
        .await
        .unwrap();
    let msgs = vec![msg];
    key.orch_key.sign_std_msg(&msgs, args, MEMO).unwrap()
}

// Submits a false send to cosmos for all the lying validators
#[allow(clippy::too_many_arguments)]
async fn submit_false_claims(
    keys: &[ValidatorKeys],
    nonce: u64,
    height: u64,
    bridge_user: &BridgeUserKey,
    prefix: &str,
    erc20_address: EthAddress,
    contact: &Contact,
    fee: &Fee,
) {
    let mut futures = vec![];
    for (i, k) in keys.iter().enumerate() {
        if i == 0 || i == 3 {
            info!("Skipping validator {} for false claims", i);
            continue;
        }
        let claim = create_claim(
            nonce,
            height,
            &erc20_address,
            &bridge_user.eth_address,
            &bridge_user.cosmos_address,
            &k.orch_key.to_address(prefix).unwrap(),
        );
        info!("Oracle number {} submitting false deposit {:?}", i, &claim);
        let msg_bytes = create_message(&claim, contact, k, fee, prefix).await;

        let response = contact
            .send_transaction(msg_bytes, BroadcastMode::Sync)
            .await
            .unwrap();
        futures.push(contact.wait_for_tx(response, OPERATION_TIMEOUT));
    }

    let join_res = join_all(futures).await;
    info!("join_res: {:?}", join_res);
}

async fn print_sends_to_cosmos(grpc_client: &GravityQueryClient<Channel>, print_others: bool) {
    let mut grpc_client = &mut grpc_client.clone();
    let attestations = get_attestations(&mut grpc_client, None).await.unwrap();
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
