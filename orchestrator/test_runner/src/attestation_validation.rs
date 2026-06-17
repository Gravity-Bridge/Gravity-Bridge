use crate::get_fee;
use crate::happy_path::wait_for_nonzero_valset;
use crate::ibc_auto_forward::{get_channel_id, get_ibc_balance};
use crate::utils::{
    create_default_test_config, get_event_nonce_safe, get_user_key, start_orchestrators,
    ValidatorKeys,
};
use crate::{
    get_gravity_chain_id, get_ibc_chain_id, ADDRESS_PREFIX, COSMOS_NODE_GRPC, IBC_ADDRESS_PREFIX,
    IBC_NODE_GRPC, MINER_ADDRESS, MINER_PRIVATE_KEY, OPERATION_TIMEOUT, TOTAL_TIMEOUT,
};
use clarity::Address as EthAddress;
use clarity::Uint256;
use cosmos_gravity::query::{get_attestations, get_last_event_nonce_for_validator};
use cosmos_gravity::send::send_ethereum_claims;
use deep_space::client::type_urls::MSG_TRANSFER_TYPE_URL;
use deep_space::private_key::{CosmosPrivateKey, PrivateKey};
use deep_space::Address as CosmosAddress;
use deep_space::{Coin as DSCoin, Contact, Msg};
use ethereum_gravity::deploy_erc20::deploy_erc20;
use ethereum_gravity::send_to_cosmos::send_to_cosmos;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::query_client::QueryClient as BankQueryClient;
use gravity_proto::cosmos_sdk_proto::cosmos::base::v1beta1::Coin;
use gravity_proto::cosmos_sdk_proto::ibc::applications::transfer::v1::query_client::QueryClient as IbcTransferQueryClient;
use gravity_proto::cosmos_sdk_proto::ibc::applications::transfer::v1::MsgTransfer;
use gravity_proto::cosmos_sdk_proto::ibc::core::channel::v1::query_client::QueryClient as IbcChannelQueryClient;
use gravity_proto::gravity::v1::claim_hash_components::Components;
use gravity_proto::gravity::v1::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::v1::{
    Attestation, ClaimHashComponents, ClaimType, MsgBatchSendToEthClaim, MsgErc20DeployedClaim,
    MsgLogicCallExecutedClaim, MsgSendToCosmosClaim, MsgValsetUpdatedClaim,
};
use gravity_utils::types::event_signatures::ERC20_DEPLOYED_EVENT_SIG;
use gravity_utils::types::{Erc20DeployedEvent, EthereumEvent, SendToCosmosEvent};
use prost::Message;
use sha2::{Digest, Sha256};
use std::convert::TryFrom;
use std::ops::Add;
use std::time::{Duration, Instant, SystemTime};
use tokio::time::sleep;
use tonic::transport::Channel;
use web30::client::Web3;
use web30::types::SendTxOption;

// AttestationSeparator is the same Unicode combining character sequence used in Go code
const ATTESTATION_SEPARATOR: &str = "G\u{0304}\u{0310}\u{030f}\u{030d}\u{0304}\u{0313}\u{0303}\u{0308}\u{030e}\u{0322}\u{0308}\u{030e}\u{0322}\u{0300}\u{0308}\u{030e}\u{0322}\u{0301}\u{0352}\u{0357}\u{0320}\u{0300}\u{0357}\u{0340}\u{0309}\u{0321}\u{0357}\u{0330}\u{0350}\u{0308}\u{0301}";

pub async fn attestation_claim_voting_test(
    web30: &Web3,
    grpc_client: GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_address: EthAddress,
) {
    let mut grpc_client = grpc_client;

    start_orchestrators(
        keys.clone(),
        gravity_address,
        false,
        create_default_test_config(),
    )
    .await;
    wait_for_nonzero_valset(web30, gravity_address).await;

    let user = get_user_key(None);
    let amount: Uint256 = 100u64.into();
    let denom = format!("gravity{erc20_address}");

    info!("ATTESTATION_VALIDATION: happy path send_to_cosmos with components");
    let before_deposit = get_balance_amount(contact, user.cosmos_address, &denom).await;

    let tx_id = send_to_cosmos(
        erc20_address,
        gravity_address,
        amount,
        user.cosmos_address,
        *MINER_PRIVATE_KEY,
        Some(OPERATION_TIMEOUT),
        web30,
        vec![],
    )
    .await
    .expect("failed to submit send_to_cosmos tx");

    web30
        .wait_for_transaction(tx_id, OPERATION_TIMEOUT, None)
        .await
        .expect("send_to_cosmos tx not included");

    let att = wait_for_matching_send_to_cosmos_attestation(
        &mut grpc_client,
        erc20_address,
        user.cosmos_address,
        *MINER_ADDRESS,
    )
    .await;

    assert_eq!(
        att.claim_type,
        ClaimType::SendToCosmos as i32,
        "expected claim_type SEND_TO_COSMOS, got {}",
        att.claim_type
    );
    let components = att
        .claim_components
        .as_ref()
        .expect("send_to_cosmos attestation missing claim_components");
    let send_components = match components.components.as_ref() {
        Some(Components::SendToCosmos(c)) => c,
        _ => panic!("expected send_to_cosmos claim components variant"),
    };

    assert_eq!(send_components.token_contract, erc20_address.to_string());
    assert_eq!(send_components.amount, amount.to_string());
    assert_eq!(
        send_components.cosmos_receiver,
        user.cosmos_address.to_string()
    );
    assert_eq!(send_components.ethereum_sender, MINER_ADDRESS.to_string());

    let decoded_for_nonce: MsgSendToCosmosClaim =
        decode_claim_any(&att).expect("failed to decode claim Any for nonce assertions");
    assert_eq!(
        send_components.event_nonce, decoded_for_nonce.event_nonce,
        "send_to_cosmos event_nonce mismatch: components={} claim={}",
        send_components.event_nonce, decoded_for_nonce.event_nonce
    );
    assert_eq!(
        send_components.eth_block_height, decoded_for_nonce.eth_block_height,
        "send_to_cosmos eth_block_height mismatch: components={} claim={}",
        send_components.eth_block_height, decoded_for_nonce.eth_block_height
    );

    let hash_from_claim = hash_from_claim_any(&att)
        .expect("failed to compute send_to_cosmos hash from decoded claim Any");
    let hash_from_components_value = hash_from_components(components, att.claim_type)
        .expect("failed to compute send_to_cosmos hash from claim components");
    assert_eq!(
        hash_from_claim, hash_from_components_value,
        "hash mismatch between decoded claim and claim components"
    );

    wait_for_balance_delta(contact, user.cosmos_address, &denom, before_deposit, amount).await;

    // all orchestrators submit the same event and aggregate into one attestation
    info!("ATTESTATION_VALIDATION: duplicate aggregation - all orchestrators voted in one attestation");
    {
        let expected_votes = keys.len();
        let deadline = Instant::now() + TOTAL_TIMEOUT;
        loop {
            let atts = get_attestations(&mut grpc_client, Some(1000))
                .await
                .expect("failed querying attestations for vote check");
            let maybe_att = atts.into_iter().find(|a| {
                if a.claim_type != ClaimType::SendToCosmos as i32 {
                    return false;
                }
                match decode_claim_any::<MsgSendToCosmosClaim>(a) {
                    Ok(c) => {
                        c.token_contract == erc20_address.to_string()
                            && c.cosmos_receiver == user.cosmos_address.to_string()
                            && c.ethereum_sender == MINER_ADDRESS.to_string()
                    }
                    Err(_) => false,
                }
            });
            if let Some(found_att) = maybe_att {
                if found_att.votes.len() >= expected_votes {
                    assert_eq!(
                        found_att.votes.len(),
                        expected_votes,
                        "expected exactly {} orchestrator votes in one attestation, found {}",
                        expected_votes,
                        found_att.votes.len()
                    );
                    info!(
                        "Confirmed {} orchestrator votes in single attestation",
                        expected_votes
                    );
                    break;
                }
            }
            assert!(
                Instant::now() < deadline,
                "timeout waiting for all {} orchestrator votes on send_to_cosmos attestation",
                expected_votes
            );
            sleep(Duration::from_secs(3)).await;
        }
    }

    // Byzantine fork — orch0 and orch1 submit conflicting SendToCosmos claims at
    // the same event nonce but with different amounts. Gravity's attestation store is keyed by
    // (nonce, claim_hash); a claim with a different hash opens a *separate* attestation rather
    // than being rejected. Both submissions succeed (code=0) and the two competing attestations
    // coexist on-chain. Neither can reach the >2/3 quorum needed to be marked "observed"
    // because each has only one validator's vote out of four total.
    info!(
        "ATTESTATION_VALIDATION: Byzantine fork - conflicting claims create separate attestations"
    );
    {
        let prefix = contact.get_prefix();
        let orch0_addr = keys[0]
            .orch_key
            .to_address(&prefix)
            .expect("orch0 key to address");
        let orch1_addr = keys[1]
            .orch_key
            .to_address(&prefix)
            .expect("orch1 key to address");
        let nonce0 =
            get_last_event_nonce_for_validator(&mut grpc_client, orch0_addr, prefix.clone())
                .await
                .expect("failed to get orch0 last event nonce");
        let nonce1 =
            get_last_event_nonce_for_validator(&mut grpc_client, orch1_addr, prefix.clone())
                .await
                .expect("failed to get orch1 last event nonce");
        assert_eq!(
            nonce0, nonce1,
            "orchestrators 0 and 1 are at different chain nonces ({} vs {}); Byzantine fork test cannot run",
            nonce0, nonce1
        );
        let next_nonce = nonce0 + 1;

        // Use a fixed fake Ethereum sender distinct from MINER_ADDRESS so real orchestrators
        // relaying actual Ethereum chain state will not interfere with this injected claim.
        let fake_eth_sender: EthAddress = "0x912fd21d7a69678227fe6d08c64222db41477ba0"
            .parse()
            .expect("valid hex address");
        let base_amount: Uint256 = 777u64.into();
        let tampered_amount: Uint256 = 778u64.into();

        let base_event = SendToCosmosEvent {
            event_nonce: next_nonce,
            block_height: 500u64.into(),
            erc20: erc20_address,
            sender: fake_eth_sender,
            destination: user.cosmos_address.to_string(),
            validated_destination: Some(user.cosmos_address),
            amount: base_amount,
        };

        // Step 1: orch0 submits base event → creates attestation keyed by (next_nonce, H₇₇₇).
        let res0 = send_ethereum_claims(
            contact,
            keys[0].orch_key,
            vec![base_event.clone()],
            vec![],
            vec![],
            vec![],
            vec![],
            get_fee(None),
        )
        .await
        .expect("base claim submission from orch0 failed unexpectedly");
        assert_eq!(
            res0.code(),
            0,
            "expected orch0 base claim to succeed, code={}: {}",
            res0.code(),
            res0.raw_log()
        );

        contact
            .wait_for_next_block(TOTAL_TIMEOUT)
            .await
            .expect("failed to wait for block after base claim");

        // Step 2: orch1 submits a conflicting event (same nonce, different amount).
        // Because hash(amount=778) ≠ hash(amount=777), the keeper looks up
        // (next_nonce, H₇₇₈), finds nothing, and creates a second attestation.
        // This is correct Byzantine fork behaviour — the chain does not reject it.
        let conflicting_event = SendToCosmosEvent {
            amount: tampered_amount,
            ..base_event
        };
        let res1 = send_ethereum_claims(
            contact,
            keys[1].orch_key,
            vec![conflicting_event],
            vec![],
            vec![],
            vec![],
            vec![],
            get_fee(None),
        )
        .await
        .expect("conflicting claim submission from orch1 failed unexpectedly");
        assert_eq!(
            res1.code(),
            0,
            "expected orch1 conflicting claim to succeed (creates competing attestation), code={}: {}",
            res1.code(),
            res1.raw_log()
        );

        contact
            .wait_for_next_block(TOTAL_TIMEOUT)
            .await
            .expect("failed to wait for block after conflicting claim");

        // Step 3: verify exactly two competing attestations exist for next_nonce,
        // each with exactly one validator's vote and neither observed.
        let all_atts = get_attestations(&mut grpc_client, Some(1000))
            .await
            .expect("Failed to query attestations");
        let competing: Vec<_> = all_atts
            .iter()
            .filter(|att| {
                matches!(
                    att.claim_components
                        .as_ref()
                        .and_then(|cc| cc.components.as_ref()),
                    Some(Components::SendToCosmos(c)) if c.event_nonce == next_nonce
                )
            })
            .collect();
        assert_eq!(
            competing.len(),
            2,
            "Expected 2 competing attestations for nonce {next_nonce}, found {}",
            competing.len()
        );
        for att in &competing {
            assert!(
                !att.observed,
                "competing attestation at nonce {} should not be observed \
                 (only 1 of 4 validators voted)",
                next_nonce
            );
            assert_eq!(
                att.votes.len(),
                1,
                "expected exactly 1 vote in each competing attestation, got {:?}",
                att.votes
            );
        }
        info!(
            "confirmed 2 competing attestations at nonce {next_nonce}, \
             each with 1 vote, neither observed"
        );
    }

    info!("ATTESTATION_VALIDATION: nonce-contiguous enforcement - replay of already-submitted nonce should fail");
    let before_replay = get_balance_amount(contact, user.cosmos_address, &denom).await;
    let replay_nonce = get_event_nonce_safe(gravity_address, web30, *MINER_ADDRESS)
        .await
        .expect("failed to read current event nonce from gravity contract");

    let replay_event = SendToCosmosEvent {
        event_nonce: replay_nonce,
        block_height: 500u64.into(),
        erc20: erc20_address,
        sender: *MINER_ADDRESS,
        destination: user.cosmos_address.to_string(),
        validated_destination: Some(user.cosmos_address),
        amount,
    };

    for key in &keys {
        let res = send_ethereum_claims(
            contact,
            key.orch_key,
            vec![replay_event.clone()],
            vec![],
            vec![],
            vec![],
            vec![],
            get_fee(None),
        )
        .await;

        match res {
            Ok(tx) => {
                assert_ne!(
                    tx.code(),
                    0,
                    "expected duplicate replay tx to fail, got success: {:?}",
                    tx
                );
                let log = tx.raw_log();
                assert!(
                    log.to_lowercase().contains("noncontiguous")
                        || log.to_lowercase().contains("non contiguous")
                        || log.to_lowercase().contains("expected")
                        || log.to_lowercase().contains("event nonce"),
                    "expected clear non-contiguous nonce error, got log: {}",
                    log
                );
            }
            Err(err) => {
                let msg = err.to_string().to_lowercase();
                assert!(
                    msg.contains("noncontiguous")
                        || msg.contains("non contiguous")
                        || msg.contains("event nonce")
                        || msg.contains("broadcast"),
                    "expected clear replay failure message, got error: {}",
                    err
                );
            }
        }
    }

    contact
        .wait_for_next_block(TOTAL_TIMEOUT)
        .await
        .expect("failed waiting for block after replay attempts");

    let after_replay = get_balance_amount(contact, user.cosmos_address, &denom).await;
    assert_eq!(
        before_replay, after_replay,
        "malicious replay changed user balance; before={} after={}",
        before_replay, after_replay
    );

    info!("ATTESTATION_CLAIM_VOTING complete: passed");
}

#[allow(clippy::too_many_arguments)]
pub async fn attestation_hash_integrity_test(
    web30: &Web3,
    grpc_client: GravityQueryClient<Channel>,
    contact: &Contact,
    ibc_contact: &Contact,
    keys: Vec<ValidatorKeys>,
    ibc_keys: Vec<CosmosPrivateKey>,
    gravity_address: EthAddress,
    erc20_address: EthAddress,
) {
    let mut grpc_client = grpc_client;

    start_orchestrators(
        keys.clone(),
        gravity_address,
        false,
        create_default_test_config(),
    )
    .await;
    wait_for_nonzero_valset(web30, gravity_address).await;

    // Setup: seed one observed SendToCosmos attestation for hash reconstruction validation
    // non-valset claim to reconstruct the hash for.  Without this, all on-chain attestations
    // would be ValsetUpdated, causing the checked > 0 assertion to panic.
    {
        let seed_user = get_user_key(None);
        let seed_amount: Uint256 = 1u64.into();
        let seed_denom = format!("gravity{erc20_address}");
        let tx_id = send_to_cosmos(
            erc20_address,
            gravity_address,
            seed_amount,
            seed_user.cosmos_address,
            *MINER_PRIVATE_KEY,
            Some(OPERATION_TIMEOUT),
            web30,
            vec![],
        )
        .await
        .expect("ATTESTATION_HASH_INTEGRITY setup: failed to submit seed send_to_cosmos tx");
        web30
            .wait_for_transaction(tx_id, OPERATION_TIMEOUT, None)
            .await
            .expect("ATTESTATION_HASH_INTEGRITY setup: seed send_to_cosmos tx not included");
        wait_for_balance_delta(
            contact,
            seed_user.cosmos_address,
            &seed_denom,
            0u8.into(),
            seed_amount,
        )
        .await;
        info!("ATTESTATION_HASH_INTEGRITY setup: seed SendToCosmos observed, proceeding with hash reconstruction validation");
    }

    info!("ATTESTATION_HASH_INTEGRITY: rust-side hash reconstruction for all supported observed attestations");
    let attestations = get_attestations(&mut grpc_client, Some(1000))
        .await
        .expect("failed querying attestations for reconstruction checks");
    let mut checked = 0usize;
    let mut skipped_valset = 0usize;

    for att in attestations {
        if att.claim_components.is_none() || att.claim_type == ClaimType::Unspecified as i32 {
            continue;
        }

        let claim_hash = hash_from_claim_any(&att);
        let component_hash = hash_from_components(
            att.claim_components
                .as_ref()
                .expect("claim_components unexpectedly missing"),
            att.claim_type,
        );

        match (claim_hash, component_hash) {
            (Ok(h1), Ok(h2)) => {
                assert_eq!(
                    h1,
                    h2,
                    "hash mismatch for attestation type {}",
                    ClaimType::try_from(att.claim_type)
                        .map(|c| c.as_str_name())
                        .unwrap_or("UNKNOWN")
                );
                checked += 1;
            }
            (Err(e1), Err(e2)) => {
                if att.claim_type == ClaimType::ValsetUpdated as i32 {
                    warn!(
                        "skipping valset hash strict reconstruction due Go fmt %x struct formatting dependency: claim_err='{}' component_err='{}'",
                        e1, e2
                    );
                    skipped_valset += 1;
                } else {
                    panic!(
                        "failed reconstructing attestation hash: claim_err='{}' component_err='{}'",
                        e1, e2
                    );
                }
            }
            (Err(e), _) | (_, Err(e)) => {
                if att.claim_type == ClaimType::ValsetUpdated as i32 {
                    warn!("skipping valset hash strict reconstruction: {}", e);
                    skipped_valset += 1;
                } else {
                    panic!("failed reconstructing attestation hash: {}", e);
                }
            }
        }
    }

    assert!(
        checked > 0,
        "expected to validate at least one attestation hash"
    );

    // valset member sort order verification
    info!("ATTESTATION_HASH_INTEGRITY: valset member sort order verification");
    {
        let all_atts = get_attestations(&mut grpc_client, Some(1000))
            .await
            .expect("failed querying attestations for valset sort check");
        let mut valset_checked = 0usize;
        for att in &all_atts {
            if att.claim_type != ClaimType::ValsetUpdated as i32 {
                continue;
            }
            let components = match att.claim_components.as_ref() {
                Some(c) => c,
                None => continue,
            };
            let valset_components = match components.components.as_ref() {
                Some(Components::ValsetUpdated(v)) => v,
                _ => continue,
            };
            // Assert members are sorted: power descending, ethereum_address ascending on ties
            for i in 1..valset_components.members.len() {
                let prev = &valset_components.members[i - 1];
                let curr = &valset_components.members[i];
                assert!(
                    prev.power > curr.power
                        || (prev.power == curr.power
                            && prev.ethereum_address <= curr.ethereum_address),
                    "valset members not sorted at index {}: \
                     prev=(power={}, addr={}) curr=(power={}, addr={})",
                    i,
                    prev.power,
                    prev.ethereum_address,
                    curr.power,
                    curr.ethereum_address
                );
            }
            valset_checked += 1;
        }
        assert!(
            valset_checked > 0,
            "expected at least one ValsetUpdated attestation for sort-order check"
        );
        info!(
            "Verified member ordering for {} ValsetUpdated attestations",
            valset_checked
        );
    }

    info!(
        "ATTESTATION_HASH_INTEGRITY complete: validated {} attestation hashes (skipped valset strict reconstruction for {})",
        checked, skipped_valset
    );

    // ERC20DeployedClaim hash collision targeting bartoken ERC20
    info!("ATTESTATION_HASH_INTEGRITY: ERC20DeployedClaim hash collision");
    erc20_deployed_claim_hash_collision(
        web30,
        contact,
        ibc_contact,
        &mut grpc_client,
        &keys,
        &ibc_keys,
        gravity_address,
        erc20_address,
    )
    .await;
}

/// ERC20DeployedClaim claim disagreement test.
///
/// Verifies that:
/// 1. A denom containing AttestationSeparator is rejected by ValidateStrictDenom
/// 2. A claim disagreement claim creates its own attestation (1 vote)
/// 3. other validators' claims form a separate attestation that reaches quorum
/// 4. The disagreed attestation never reaches quorum
#[allow(clippy::too_many_arguments)]
async fn erc20_deployed_claim_hash_collision(
    web30: &Web3,
    contact: &Contact,
    ibc_contact: &Contact,
    grpc_client: &mut GravityQueryClient<Channel>,
    keys: &[ValidatorKeys],
    ibc_keys: &[CosmosPrivateKey],
    gravity_address: EthAddress,
    bartoken_address: EthAddress,
) {
    const BARTOKEN_ADDR: &str = "0x0412C7c846bb6b7DC462CF6B453f76D8440b2609";
    const MALICIOUS_DENOM: &str = "attack/0x0412C7c846bb6b7DC462CF6B453f76D8440b2609/tok";

    // Sanity-check: the erc20_address passed in from the test harness must match
    // the constant address the collision was constructed for.
    assert_eq!(
        bartoken_address.to_string(),
        BARTOKEN_ADDR,
        "bartoken_address parameter ({}) does not match BARTOKEN_ADDR constant — \
         check erc20_addresses[0] in main.rs",
        bartoken_address
    );

    // ── Phase 0: escrow bartoken funds on Ethereum ────────────────────────────
    // Send bartoken ERC20s from Ethereum into Gravity.sol escrow so there are
    // real funds at stake.
    let escrow_amount: Uint256 = 1_000u64.into();
    let escrow_receiver = get_user_key(None);
    let tx_id = send_to_cosmos(
        bartoken_address,
        gravity_address,
        escrow_amount,
        escrow_receiver.cosmos_address,
        *MINER_PRIVATE_KEY,
        Some(OPERATION_TIMEOUT),
        web30,
        vec![],
    )
    .await
    .expect("Phase 0: failed to send bartoken to cosmos");
    web30
        .wait_for_transaction(tx_id, TOTAL_TIMEOUT, None)
        .await
        .expect("Phase 0: send_to_cosmos tx not included");

    let bartoken_denom = format!("gravity{bartoken_address}");
    wait_for_balance_delta(
        contact,
        escrow_receiver.cosmos_address,
        &bartoken_denom,
        0u8.into(),
        escrow_amount,
    )
    .await;
    info!(
        "Phase 0: {escrow_amount} bartoken escrowed in Gravity.sol; \
         cosmos vouchers confirmed"
    );

    // ── Phase 1: IBC-transfer the malicious token from ibc-test-1 → gravity ──
    // This causes ibc-go's OnRecvPacket to call setDenomMetadata automatically,
    // producing the exact Name/Symbol that the collision requires.

    // 1a. Find gravity-side channel ID (used in the IBC denom path).
    let gravity_channel_qc = IbcChannelQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Phase 1: failed to connect gravity IbcChannelQueryClient");
    let gravity_channel_id = get_channel_id(
        gravity_channel_qc,
        get_ibc_chain_id(), // "ibc-test-1"
        Some(TOTAL_TIMEOUT),
    )
    .await
    .expect("Phase 1: could not find gravity-side IBC channel connecting to ibc-test-1");

    // 1b. Compute the ibc/* denom and metadata that gravity chain will assign.
    let full_denom_path = format!("transfer/{gravity_channel_id}/{MALICIOUS_DENOM}");
    let raw_hash = Sha256::digest(full_denom_path.as_bytes());
    let ibc_denom = format!(
        "ibc/{}",
        raw_hash
            .iter()
            .map(|b| format!("{b:02X}"))
            .collect::<String>()
    );
    let ibc_token_name = format!("{full_denom_path} IBC token");
    let ibc_token_symbol = MALICIOUS_DENOM.to_uppercase();
    info!("computed ibc_denom={ibc_denom} name='{ibc_token_name}' symbol='{ibc_token_symbol}'");

    // 1c. Find ibc-test-1 channel ID connecting back to the gravity chain.
    let ibc_channel_qc = IbcChannelQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Phase 1: failed to connect ibc-test-1 IbcChannelQueryClient");
    let ibc_channel_id = get_channel_id(
        ibc_channel_qc,
        get_gravity_chain_id(), // "gravity-test-1"
        Some(TOTAL_TIMEOUT),
    )
    .await
    .expect("Phase 1: could not find ibc-test-1 channel connecting to gravity chain");

    // 1d. Send the malicious token from ibc-test-1 to the gravity chain.
    let malicious_receiver = keys[0].validator_key.to_address(&ADDRESS_PREFIX).unwrap();
    let timeout_ts = SystemTime::now()
        .add(TOTAL_TIMEOUT)
        .duration_since(SystemTime::UNIX_EPOCH)
        .unwrap()
        .as_nanos() as u64;
    let malicious_coin = Coin {
        denom: MALICIOUS_DENOM.to_string(),
        amount: "1000000".to_string(),
    };
    let msg_transfer = MsgTransfer {
        source_port: "transfer".to_string(),
        source_channel: ibc_channel_id.clone(),
        token: Some(malicious_coin),
        sender: ibc_keys[0]
            .to_address(&IBC_ADDRESS_PREFIX)
            .unwrap()
            .to_string(),
        receiver: malicious_receiver.to_string(),
        timeout_height: None,
        timeout_timestamp: timeout_ts,
        ..Default::default()
    };
    ibc_contact
        .send_message(
            &[Msg::new(MSG_TRANSFER_TYPE_URL, msg_transfer)],
            None,
            &[DSCoin {
                amount: 100u16.into(),
                denom: "stake".to_string(), // ibc-test-1 native denom
            }],
            Some(TOTAL_TIMEOUT),
            None,
            ibc_keys[0],
        )
        .await
        .expect("Phase 1: IBC transfer of malicious token failed");

    // 1e. Wait for the gravity chain to receive the packet and register bank metadata.
    info!(
        "Phase 1: waiting for ibc_denom={ibc_denom} to arrive on malicious_receiver={malicious_receiver}"
    );
    let gravity_bank_qc = BankQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Phase 1: failed to connect gravity BankQueryClient");
    let gravity_transfer_qc = IbcTransferQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Phase 1: failed to connect gravity IbcTransferQueryClient");
    let ibc_balance = get_ibc_balance(
        malicious_receiver,
        MALICIOUS_DENOM.to_string(),
        None,
        gravity_bank_qc,
        gravity_transfer_qc,
        Some(TOTAL_TIMEOUT),
    )
    .await
    .expect(
        "Phase 1: malicious token never arrived on gravity chain — \
         check IBC relayer is running",
    );
    info!(
        "Phase 1: malicious token arrived as {ibc_denom} ({}); \
         bank metadata registered automatically by ibc-go OnRecvPacket",
        ibc_balance.amount
    );

    // ── Phase 2: deploy the malicious ERC20 on Ethereum ──────────────────────
    let deploy_tx_hash = deploy_erc20(
        ibc_denom.clone(),
        ibc_token_name.clone(),
        ibc_token_symbol.clone(),
        0u32,
        gravity_address,
        web30,
        None, // don't block inside deploy_erc20; we parse the receipt ourselves
        *MINER_PRIVATE_KEY,
        vec![SendTxOption::GasLimitMultiplier(2.0)],
    )
    .await
    .expect("Phase 2: deployERC20 call failed");

    let deploy_tx = web30
        .wait_for_transaction(deploy_tx_hash, TOTAL_TIMEOUT, None)
        .await
        .expect("Phase 2: deployERC20 tx not included");

    let deploy_block = deploy_tx
        .get_block_number()
        .expect("Phase 2: deployERC20 tx hash has block number");
    let raw_logs = web30
        .check_for_events(
            deploy_block,
            Some(deploy_block),
            vec![gravity_address],
            vec![ERC20_DEPLOYED_EVENT_SIG],
        )
        .await
        .expect("Phase 2: failed to query ERC20DeployedEvent logs");

    let deploy_events = Erc20DeployedEvent::from_logs(&raw_logs)
        .expect("Phase 2: failed to parse Erc20DeployedEvent from logs");
    let deploy_event = deploy_events
        .iter()
        .find(|e| e.cosmos_denom == ibc_denom)
        .expect("Phase 2: ERC20DeployedEvent for ibc_denom not found in block logs");
    let new_erc20_addr = deploy_event.erc20_address;
    let eth_block_height = deploy_event.block_height;
    // Use the nonce directly from the parsed event — more reliable than starting_nonce+1.
    let event_nonce = deploy_event.event_nonce;
    let event_decimals = deploy_event.decimals;

    info!(
        "Phase 2: deployERC20 confirmed. \
         new_erc20_addr={new_erc20_addr}, event_nonce={event_nonce}, block={eth_block_height}"
    );

    // ── Phase 2.5: verify AttestationSeparator is blocked by denom validation ─
    //
    // The Go-side ClaimHash() uses AttestationSeparator (non-ASCII combining
    // marks) as the field delimiter.  If a denom containing the separator were
    // accepted, an attacker could shift field boundaries to engineer a hash
    // collision.  ValidateStrictDenom blocks this via its isASCII check.
    let separator_denom = format!("test{ATTESTATION_SEPARATOR}denom");
    let separator_event = Erc20DeployedEvent {
        cosmos_denom: separator_denom.clone(),
        erc20_address: new_erc20_addr,
        name: ibc_token_name.clone(),
        symbol: ibc_token_symbol.clone(),
        decimals: event_decimals,
        event_nonce,
        block_height: eth_block_height,
    };
    let sep_result = send_ethereum_claims(
        contact,
        keys[0].orch_key,
        vec![],
        vec![],
        vec![separator_event],
        vec![],
        vec![],
        get_fee(None),
    )
    .await;
    match sep_result {
        Err(e) => {
            info!(
                "Phase 2.5: separator-denom claim correctly rejected at transport level: {e}"
            );
        }
        Ok(tx) => {
            assert_ne!(
                tx.code(),
                0,
                "Phase 2.5: claim with AttestationSeparator in denom must be rejected, \
                 but got code=0. raw_log: {}",
                tx.raw_log()
            );
            let log = tx.raw_log().to_lowercase();
            assert!(
                log.contains("non-ascii") || log.contains("invalid"),
                "Phase 2.5: separator-denom rejected with unexpected error: {}",
                tx.raw_log()
            );
            info!(
                "Phase 2.5: separator-denom claim correctly rejected with code={}: {}",
                tx.code(),
                tx.raw_log()
            );
        }
    }

    // ── Phase 3: submit all claims manually (forged first, honest second) ─────
    //
    // With AttestationSeparator as the ClaimHash field delimiter (non-ASCII
    // combining marks), hash collisions through field-boundary shifting are
    // impossible for ASCII-only denoms.  The forged and honest claims will
    // produce DIFFERENT ClaimHashes on-chain, creating separate attestations.
    //
    // This phase tests claim *disagreement*: the malicious validator submits a
    // claim with fabricated field values.  Honest validators submit the real
    // values.  The honest attestation reaches quorum; the forged one does not.
    let forged_cosmos_denom = "forged-attack-denom".to_string();

    let bartoken_eth_addr: EthAddress = BARTOKEN_ADDR.parse().unwrap();
    let forged_event = Erc20DeployedEvent {
        cosmos_denom: forged_cosmos_denom.clone(),
        erc20_address: bartoken_eth_addr,
        name: "forged-token-name".to_string(),
        symbol: "FORGED".to_string(),
        decimals: event_decimals,
        event_nonce,
        block_height: eth_block_height,
    };

    // keys[0] is the malicious validator — submits the forged claim first.
    let forged_res = send_ethereum_claims(
        contact,
        keys[0].orch_key,
        vec![],
        vec![],
        vec![forged_event],
        vec![],
        vec![],
        get_fee(None),
    )
    .await
    .expect("Phase 3: forged claim submission failed at transport level");
    assert_eq!(
        forged_res.code(),
        0,
        "Phase 3: forged claim must be accepted as the first vote: {}",
        forged_res.raw_log()
    );
    info!(
        "Phase 3: forged claim accepted — attestation created with \
         TokenContract={BARTOKEN_ADDR}"
    );

    // Submit honest claims from all remaining validators manually, before
    // orchestrators loop.  Because the separator change makes hash collisions
    // impossible, these claims have a DIFFERENT ClaimHash from the forged one
    // and create their own separate attestation.  They should all be accepted.
    let new_erc20_addr_str = new_erc20_addr.to_string();
    for honest_key in &keys[1..] {
        let orch_addr = honest_key
            .orch_key
            .to_address(&contact.get_prefix())
            .unwrap();
        let honest_event = Erc20DeployedEvent {
            cosmos_denom: ibc_denom.clone(),
            erc20_address: new_erc20_addr,
            name: ibc_token_name.clone(),
            symbol: ibc_token_symbol.clone(),
            decimals: event_decimals,
            event_nonce,
            block_height: eth_block_height,
        };
        let send_result = send_ethereum_claims(
            contact,
            honest_key.orch_key,
            vec![],
            vec![],
            vec![honest_event],
            vec![],
            vec![],
            get_fee(None),
        )
        .await;

        // Honest claims create a separate attestation (different ClaimHash from
        // the forged one) and should be accepted.
        let tx = send_result.unwrap_or_else(|e| {
            panic!(
                "Phase 3: honest claim from {} failed at transport level: {}", orch_addr, e
            )
        });
        assert_eq!(
            tx.code(),
            0,
            "Phase 3: honest claim from {orch_addr} should be accepted (code=0), \
             but got code={}: {}",
            tx.code(),
            tx.raw_log()
        );
        info!(
            "Phase 3: honest claim from {orch_addr} accepted — \
             joined honest attestation for TokenContract={new_erc20_addr_str}"
        );
    }

    // ── Phase 4 / 5: wait, then assert ───────────────────────────────────────
    // Allow 3 full orchestrator loop cycles (10s each) for any background submissions.
    sleep(Duration::from_secs(30)).await;

    let all_atts = get_attestations(grpc_client, Some(1000))
        .await
        .expect("Phase 5: failed to query attestations");

    // ── Forged attestation: must exist with exactly 1 vote, NOT observed ──────
    let forged_att = all_atts.iter().find(|a| {
        if a.claim_type != ClaimType::Erc20Deployed as i32 {
            return false;
        }
        let Some(ref any_att) = a.claim else {
            return false;
        };
        MsgErc20DeployedClaim::decode(any_att.value.as_slice())
            .ok()
            .map(|c| c.token_contract == BARTOKEN_ADDR)
            .unwrap_or(false)
    });

    let forged_att = forged_att.expect(
        "Phase 5: forged attestation was never created — \
         the forged claim was rejected before creating an attestation",
    );

    assert_eq!(
        forged_att.votes.len(),
        1,
        "Phase 5: forged attestation should have exactly 1 vote (the malicious validator), \
         but has {}. Votes: {:?}",
        forged_att.votes.len(),
        forged_att.votes,
    );

    assert!(
        !forged_att.observed,
        "REGRESSION: forged attestation was marked observed (2/3 threshold reached). \
         handleErc20Deployed ran with forged_cosmos_denom and bartokenAddr — \
         denom→ERC20 mapping is corrupted."
    );

    // ── Honest attestation: must exist with 3 votes, observed ─────────────────
    let honest_att = all_atts.iter().find(|a| {
        if a.claim_type != ClaimType::Erc20Deployed as i32 {
            return false;
        }
        let Some(ref any_att) = a.claim else {
            return false;
        };
        MsgErc20DeployedClaim::decode(any_att.value.as_slice())
            .ok()
            .map(|c| c.token_contract == new_erc20_addr_str)
            .unwrap_or(false)
    });

    let honest_att = honest_att.expect(
        "Phase 5: honest attestation was never created — \
         honest claims were rejected before creating an attestation",
    );

    assert!(
        honest_att.votes.len() >= 3,
        "Phase 5: honest attestation should have at least 3 votes, \
         but has {}. Votes: {:?}",
        honest_att.votes.len(),
        honest_att.votes,
    );

    assert!(
        honest_att.observed,
        "Phase 5: honest attestation should be observed (quorum reached with 3/4 validators), \
         but it was not. Votes: {:?}",
        honest_att.votes,
    );

    info!(
        "ERC20DeployedClaim claim disagreement VERIFIED: forged attestation stuck at {} vote(s), \
         honest attestation observed with {} vote(s). \
         AttestationSeparator prevents hash collisions; honest majority wins.",
        forged_att.votes.len(),
        honest_att.votes.len(),
    );
}

async fn get_balance_amount(contact: &Contact, addr: CosmosAddress, denom: &str) -> Uint256 {
    contact
        .get_balance(addr, denom.to_string())
        .await
        .expect("failed querying balance")
        .map(|c| c.amount)
        .unwrap_or_else(|| 0u8.into())
}

async fn wait_for_balance_delta(
    contact: &Contact,
    addr: CosmosAddress,
    denom: &str,
    start: Uint256,
    expected_delta: Uint256,
) {
    let deadline = Instant::now() + TOTAL_TIMEOUT;
    while Instant::now() < deadline {
        let current = get_balance_amount(contact, addr, denom).await;
        if current == start + expected_delta {
            return;
        }
        sleep(Duration::from_secs(3)).await;
    }
    panic!(
        "timeout waiting for balance delta; start={} expected_delta={} denom={}",
        start, expected_delta, denom
    );
}

async fn wait_for_matching_send_to_cosmos_attestation(
    grpc_client: &mut GravityQueryClient<Channel>,
    token_contract: EthAddress,
    cosmos_receiver: CosmosAddress,
    ethereum_sender: EthAddress,
) -> Attestation {
    let deadline = Instant::now() + TOTAL_TIMEOUT;
    while Instant::now() < deadline {
        let attestations = get_attestations(grpc_client, Some(1000))
            .await
            .expect("failed querying attestations");

        for att in attestations {
            if att.claim_type != ClaimType::SendToCosmos as i32 {
                continue;
            }

            let claim = match decode_claim_any::<MsgSendToCosmosClaim>(&att) {
                Ok(c) => c,
                Err(_) => continue,
            };

            let matches = claim.token_contract == token_contract.to_string()
                && claim.cosmos_receiver == cosmos_receiver.to_string()
                && claim.ethereum_sender == ethereum_sender.to_string();

            // Only return once the attestation is observed (>2/3 votes in) to give
            // the subsequent votes-count assertion a stable starting point.
            if matches && att.observed {
                return att;
            }
        }

        sleep(Duration::from_secs(3)).await;
    }

    panic!("timeout waiting for matching MsgSendToCosmosClaim attestation");
}

fn decode_claim_any<T: Message + Default>(att: &Attestation) -> Result<T, String> {
    let any = att
        .claim
        .as_ref()
        .ok_or_else(|| "attestation claim is missing".to_string())?;
    T::decode(any.value.as_slice()).map_err(|e| format!("failed to decode claim Any: {e}"))
}

fn tmhash(data: &[u8]) -> Vec<u8> {
    let digest = Sha256::digest(data);
    digest[..20].to_vec()
}

fn hash_from_components(
    components: &ClaimHashComponents,
    claim_type: i32,
) -> Result<Vec<u8>, String> {
    let ctype = ClaimType::try_from(claim_type)
        .map_err(|_| format!("unknown claim_type value {claim_type}"))?;
    match (ctype, components.components.as_ref()) {
        (ClaimType::SendToCosmos, Some(Components::SendToCosmos(c))) => {
            let path = format!(
                "{}{}{}{}{}{}{}{}{}{}{}",
                c.event_nonce,
                ATTESTATION_SEPARATOR,
                c.eth_block_height,
                ATTESTATION_SEPARATOR,
                c.token_contract,
                ATTESTATION_SEPARATOR,
                c.amount,
                ATTESTATION_SEPARATOR,
                c.ethereum_sender,
                ATTESTATION_SEPARATOR,
                c.cosmos_receiver
            );
            Ok(tmhash(path.as_bytes()))
        }
        (ClaimType::BatchSendToEth, Some(Components::BatchSendToEth(c))) => {
            let path = format!(
                "{}{}{}{}{}{}{}",
                c.event_nonce,
                ATTESTATION_SEPARATOR,
                c.eth_block_height,
                ATTESTATION_SEPARATOR,
                c.batch_nonce,
                ATTESTATION_SEPARATOR,
                c.token_contract
            );
            Ok(tmhash(path.as_bytes()))
        }
        (ClaimType::Erc20Deployed, Some(Components::Erc20Deployed(c))) => {
            let path = format!(
                "{}{}{}{}{}{}{}{}{}{}{}{}{}",
                c.event_nonce,
                ATTESTATION_SEPARATOR,
                c.eth_block_height,
                ATTESTATION_SEPARATOR,
                c.cosmos_denom,
                ATTESTATION_SEPARATOR,
                c.token_contract,
                ATTESTATION_SEPARATOR,
                c.name,
                ATTESTATION_SEPARATOR,
                c.symbol,
                ATTESTATION_SEPARATOR,
                c.decimals
            );
            Ok(tmhash(path.as_bytes()))
        }
        (ClaimType::LogicCallExecuted, Some(Components::LogicCallExecuted(c))) => {
            // Go uses fmt.Sprintf("%s", []byte) which is a raw byte cast, not UTF-8 validated.
            // Replicate faithfully by concatenating raw bytes instead of going through a String.
            let prefix = format!("{}{}{}", c.event_nonce, ATTESTATION_SEPARATOR, c.eth_block_height);
            let mut path = prefix.into_bytes();
            path.extend_from_slice(ATTESTATION_SEPARATOR.as_bytes());
            path.extend_from_slice(&c.invalidation_id);
            path.extend_from_slice(ATTESTATION_SEPARATOR.as_bytes());
            path.extend_from_slice(c.invalidation_nonce.to_string().as_bytes());
            Ok(tmhash(&path))
        }
        (ClaimType::ValsetUpdated, Some(Components::ValsetUpdated(_))) => Err(
            "strict Rust valset hash reconstruction is skipped because Go uses fmt '%x' over struct slices"
                .to_string(),
        ),
        (_, None) => Err("claim_components.components is missing".to_string()),
        (expected, Some(found)) => Err(format!(
            "claim_type {:?} does not match components variant {:?}",
            expected, found
        )),
    }
}

fn hash_from_claim_any(att: &Attestation) -> Result<Vec<u8>, String> {
    let ctype = ClaimType::try_from(att.claim_type)
        .map_err(|_| format!("unknown claim_type value {}", att.claim_type))?;

    match ctype {
        ClaimType::SendToCosmos => {
            let c: MsgSendToCosmosClaim = decode_claim_any(att)?;
            let path = format!(
                "{}{}{}{}{}{}{}{}{}{}{}",
                c.event_nonce,
                ATTESTATION_SEPARATOR,
                c.eth_block_height,
                ATTESTATION_SEPARATOR,
                c.token_contract,
                ATTESTATION_SEPARATOR,
                c.amount,
                ATTESTATION_SEPARATOR,
                c.ethereum_sender,
                ATTESTATION_SEPARATOR,
                c.cosmos_receiver
            );
            Ok(tmhash(path.as_bytes()))
        }
        ClaimType::BatchSendToEth => {
            let c: MsgBatchSendToEthClaim = decode_claim_any(att)?;
            let path = format!(
                "{}{}{}{}{}{}{}",
                c.event_nonce,
                ATTESTATION_SEPARATOR,
                c.eth_block_height,
                ATTESTATION_SEPARATOR,
                c.batch_nonce,
                ATTESTATION_SEPARATOR,
                c.token_contract
            );
            Ok(tmhash(path.as_bytes()))
        }
        ClaimType::Erc20Deployed => {
            let c: MsgErc20DeployedClaim = decode_claim_any(att)?;
            let path = format!(
                "{}{}{}{}{}{}{}{}{}{}{}{}{}",
                c.event_nonce,
                ATTESTATION_SEPARATOR,
                c.eth_block_height,
                ATTESTATION_SEPARATOR,
                c.cosmos_denom,
                ATTESTATION_SEPARATOR,
                c.token_contract,
                ATTESTATION_SEPARATOR,
                c.name,
                ATTESTATION_SEPARATOR,
                c.symbol,
                ATTESTATION_SEPARATOR,
                c.decimals
            );
            Ok(tmhash(path.as_bytes()))
        }
        ClaimType::LogicCallExecuted => {
            let c: MsgLogicCallExecutedClaim = decode_claim_any(att)?;
            // Go uses fmt.Sprintf("%s", []byte) which is a raw byte cast, not UTF-8 validated.
            // Replicate faithfully by concatenating raw bytes instead of going through a String.
            let prefix = format!(
                "{}{}{}",
                c.event_nonce, ATTESTATION_SEPARATOR, c.eth_block_height
            );
            let mut path = prefix.into_bytes();
            path.extend_from_slice(ATTESTATION_SEPARATOR.as_bytes());
            path.extend_from_slice(&c.invalidation_id);
            path.extend_from_slice(ATTESTATION_SEPARATOR.as_bytes());
            path.extend_from_slice(c.invalidation_nonce.to_string().as_bytes());
            Ok(tmhash(&path))
        }
        ClaimType::ValsetUpdated => {
            let _c: MsgValsetUpdatedClaim = decode_claim_any(att)?;
            Err("strict Rust valset hash reconstruction is skipped because Go uses fmt '%x' over struct slices".to_string())
        }
        ClaimType::Unspecified => Err("unspecified claim type".to_string()),
    }
}
