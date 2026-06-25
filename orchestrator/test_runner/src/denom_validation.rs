//! Integration test for strict denom validation rules.
//!
//! This test validates that `ValidateStrictDenom` rejects bad denoms at every
//! entry point: `SendToEth`, `DenomToERC20` query, and Governance proposals.
//! It uses the **existing** tokens that already exist in genesis to verify that
//! valid denoms still work.

use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::happy_path_v2::happy_path_test_v2;
use crate::utils::{
    create_default_test_config, start_orchestrators, vote_yes_on_proposals, ValidatorKeys,
};
use crate::{get_deposit, get_fee, ADDRESS_PREFIX, TOTAL_TIMEOUT};
use clarity::Address as EthAddress;
use cosmos_gravity::proposals::{submit_airdrop_proposal, AirdropProposalJson};
use cosmos_gravity::send::{send_request_batch, send_to_eth, MSG_SEND_TO_ETH_TYPE_URL};
use deep_space::{Address as CosmosAddress, Coin, Contact, Msg, PrivateKey};
use gravity_proto::gravity::v1::{
    query_client::QueryClient as GravityQueryClient,
    MsgSendToEth as ProtoMsgSendToEth, QueryDenomToErc20Request,
};
use gravity_utils::num_conversion::one_atom;
use tonic::transport::Channel;
use web30::client::Web3;

/// Denoms that MUST be rejected by ValidateStrictDenom wherever they appear.
/// These violate the strict rules:
///   * `ibc/gravity…`  (forbidden substring in IBC) → bad
///   * `gravity/0xbad`  (non-IBC contains `/`) → bad
pub const BAD_IBC_DENOM: &str = "ibc/gravity0xbad0000000000000000000000000000000000bad";
pub const BAD_SLASH_DENOM: &str = "gravity/0xbad";

/// Stable substring that identifies strict-denom rejection
const STRICT_DENOM_MARKERS: &str = "invalid denom";

/// Assert that `err` contains the strict-denom marker.
///
/// Panics with full diagnostic context when the marker is not found, making it immediately clear
/// that the wrong error path fired (e.g. fee mismatch, missing ERC20 index) instead of the
/// strict-denom rejection path.
fn assert_strict_denom_rejection(err: impl std::fmt::Display, context: &str, bad_denom: &str) {
    let msg = err.to_string().to_lowercase();
    if !msg.contains(STRICT_DENOM_MARKERS) {
        panic!(
            "STRICT-DENOM DETECTION FAILED [{context}]\n\
             Expected '{marker}' in error for denom '{bad_denom}'\n\
             Actual error (lowercased): {msg}",
            context = context,
            marker = STRICT_DENOM_MARKERS,
            bad_denom = bad_denom,
            msg = msg,
        );
    }
}

pub async fn denom_validation_test(
    web30: &Web3,
    contact: &Contact,
    gravity_client: GravityQueryClient<Channel>,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    _erc20_address: EthAddress,
) {
    info!("\n\n========== Denom Validation Integration Test ==========\n");

    let no_relay_market_config = create_default_test_config();
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;

    // ------------------------------------------------------
    // Part A: Arrived IBC token cannot bridge (no mapping)
    // ------------------------------------------------------
    info!("\n--- Part A: Arrived IBC token cannot bridge ---");
    let arrived_denom = "ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2";
    test_send_to_eth_fails_no_mapping(contact, &keys, arrived_denom).await;

    // ------------------------------------------------------
    // Part B: Gravity-native bad denoms (ValidateStrictDenom)
    // ------------------------------------------------------
    info!("\n--- Part B: Gravity-native bad denoms ---");
    for bad_denom in [BAD_IBC_DENOM, BAD_SLASH_DENOM] {
        info!("Testing bad denom: {}", bad_denom);
        test_send_to_eth_rejected(contact, &keys, bad_denom).await;
        test_request_batch_rejected(contact, &keys, bad_denom).await;
        test_denom_to_erc20_query_rejected(gravity_client.clone(), bad_denom).await;
    }

    // ------------------------------------------------------
    // Part C: Governance proposal validations
    // ------------------------------------------------------
    info!("\n--- Part C: Governance proposal validations ---");
    test_airdrop_proposal_rejected(contact, &keys, BAD_IBC_DENOM).await;

    // ------------------------------------------------------
    // Part D: Control test — normal tokens still work
    // ------------------------------------------------------
    info!("\n--- Part D: Control test ---");
    happy_path_test_v2(
        web30,
        gravity_client.clone(),
        contact,
        keys.clone(),
        gravity_address,
        false,
        None,
    )
    .await;

    info!("\n\n========== Denom Validation Test Complete ==========\n");
}

/// Assert that SendToEth fails because there is no ERC20 mapping for this IBC hash.
async fn test_send_to_eth_fails_no_mapping(
    contact: &Contact,
    keys: &[ValidatorKeys],
    arrived_denom: &str,
) {
    let sender = keys[0].validator_key;
    let eth_dest = keys[0].eth_key.to_address();

    let send_to_eth_coin = Coin {
        denom: arrived_denom.to_string(),
        amount: one_atom(),
    };
    let fee = get_fee(None);
    let bridge_fee = get_fee(None);
    let chain_fee = get_fee(None);

    let res = send_to_eth(
        sender,
        eth_dest,
        send_to_eth_coin.clone(),
        bridge_fee,
        Some(chain_fee),
        fee,
        contact,
    )
    .await;

    match res {
        Ok(tx_res) => {
            panic!(
                "SendToEth should fail for unmapped IBC denom {}: tx hash {}",
                arrived_denom,
                tx_res.txhash()
            )
        }
        Err(e) => info!("SendToEth correctly failed (no mapping): {e}"),
    }
}

/// MsgSendToEth using a bad denom must be rejected by ValidateStrictDenom.
///
/// Constructs the message directly and submits via `contact.send_message` to bypass the
/// client-side balance and fee-denom-mismatch checks inside `cosmos_gravity::send::send_to_eth`.
/// Those checks would otherwise short-circuit with unrelated errors before the tx reaches the
/// module's `SendToEth` handler where `ValidateStrictDenom` is called.
///
/// All coin fields (amount, bridge_fee, chain_fee) use `bad_denom` so the on-chain denom-match
/// check in `ValidateBasic` passes; the cosmos tx fee is paid in footoken so the AnteHandler
/// can deduct it normally.
async fn test_send_to_eth_rejected(contact: &Contact, keys: &[ValidatorKeys], bad_denom: &str) {
    let sender_key = keys[0].validator_key;
    let sender_addr = sender_key.to_address(&ADDRESS_PREFIX).unwrap().to_string();
    let eth_dest = keys[0].eth_key.to_address().to_string();

    let bad_coin = Coin {
        denom: bad_denom.to_string(),
        amount: one_atom(),
    };
    let msg_send_to_eth = ProtoMsgSendToEth {
        sender: sender_addr,
        eth_dest,
        amount: Some(bad_coin.clone().into()),
        bridge_fee: Some(bad_coin.clone().into()),
        chain_fee: Some(bad_coin.into()),
    };
    let msg = Msg::new(MSG_SEND_TO_ETH_TYPE_URL, msg_send_to_eth);
    // Cosmos tx fee is paid in footoken — independent of the bridge coin denom.
    let tx_fee = get_fee(None);

    let res = contact
        .send_message(
            &[msg],
            None,
            &[tx_fee],
            Some(TOTAL_TIMEOUT),
            None,
            sender_key,
        )
        .await;

    match res {
        Ok(tx_res) => panic!(
            "SendToEth should reject bad denom {}: tx hash {}",
            bad_denom,
            tx_res.txhash()
        ),
        Err(e) => {
            assert_strict_denom_rejection(&e, "SendToEth", bad_denom);
            info!(
                "SendToEth correctly rejected bad denom '{}' with strict-denom marker: {}",
                bad_denom, e
            );
        }
    }
}

/// MsgRequestBatch with a bad denom must be rejected by ValidateStrictDenom.
async fn test_request_batch_rejected(contact: &Contact, keys: &[ValidatorKeys], bad_denom: &str) {
    let res = send_request_batch(
        keys[0].validator_key,
        bad_denom.to_string(),
        Some(get_fee(None)),
        contact,
    )
    .await;

    match res {
        Ok(tx_res) => panic!(
            "RequestBatch should reject bad denom {}: tx hash {}",
            bad_denom,
            tx_res.txhash()
        ),
        Err(e) => {
            assert_strict_denom_rejection(&e, "RequestBatch", bad_denom);
            info!(
                "RequestBatch correctly rejected bad denom '{}' with strict-denom marker: {}",
                bad_denom, e
            );
        }
    }
}

/// DenomToERC20 gRPC query must reject a bad `req.Denom` with a strict-denom error.
async fn test_denom_to_erc20_query_rejected(
    mut gravity_client: GravityQueryClient<Channel>,
    bad_denom: &str,
) {
    let req = QueryDenomToErc20Request {
        denom: bad_denom.to_string(),
    };

    match gravity_client.denom_to_erc20(req).await {
        Ok(resp) => panic!(
            "DenomToERC20 query should reject bad denom {}: {:?}",
            bad_denom, resp
        ),
        Err(e) => {
            assert_strict_denom_rejection(&e, "DenomToERC20 query", bad_denom);
            info!(
                "DenomToERC20 query correctly rejected bad denom '{}' with strict-denom marker: {}",
                bad_denom, e
            );
        }
    }
}

/// An AirdropProposal with a bad denom must fail either at submission or at governance execution.
///
/// * If the chain rejects submission, the error must contain a strict-denom marker.
/// * If submission is accepted (governance validates at execution time), we vote and wait,
///   then assert the recipient balance has not increased — proving execution was rejected by
///   `HandleAirdropProposal`'s `ValidateStrictDenom` call.
async fn test_airdrop_proposal_rejected(
    contact: &Contact,
    keys: &[ValidatorKeys],
    bad_denom: &str,
) {
    let recipient_addr = keys[0].validator_key.to_address(&ADDRESS_PREFIX).unwrap();

    // Snapshot the recipient's balance before submission so we can detect unexpected minting.
    let pre_balances = contact
        .get_balances(recipient_addr)
        .await
        .unwrap_or_default();

    let recipients: Vec<CosmosAddress> = vec![recipient_addr];
    let amounts = vec![1_000u64];

    let proposal = AirdropProposalJson {
        title: format!("Bad airdrop with {}", bad_denom),
        description: "Should fail".to_string(),
        denom: bad_denom.to_string(),
        amounts,
        recipients,
    };

    let res = submit_airdrop_proposal(
        proposal,
        get_deposit(None),
        get_fee(None),
        contact,
        keys[0].validator_key,
        Some(TOTAL_TIMEOUT),
    )
    .await;

    match res {
        Err(e) => {
            assert_strict_denom_rejection(&e, "Airdrop proposal submission", bad_denom);
            info!(
                "Airdrop proposal correctly rejected on submission with strict-denom marker: {e}"
            );
        }
        Ok(_) => {
            // Submission accepted — governance execution is where ValidateStrictDenom fires.
            vote_yes_on_proposals(contact, keys, None).await;
            wait_for_proposals_to_execute(contact).await;

            // Verify execution was rejected: no bad-denom tokens must have been minted.
            let post_balances = contact
                .get_balances(recipient_addr)
                .await
                .unwrap_or_default();
            let had_denom_before = pre_balances.iter().any(|c| c.denom == bad_denom);
            let has_denom_after = post_balances.iter().any(|c| c.denom == bad_denom);
            assert!(
                !has_denom_after || had_denom_before,
                "Airdrop proposal with bad denom '{}' must not have minted tokens to the recipient \
                 (strict-denom execution rejection expected, but a new balance appeared \
                 post-execution — reverting strict-denom module code can cause this failure)",
                bad_denom
            );
            info!(
                "Airdrop proposal: execution correctly rejected (no new '{}' balance created)",
                bad_denom
            );
        }
    }
}
