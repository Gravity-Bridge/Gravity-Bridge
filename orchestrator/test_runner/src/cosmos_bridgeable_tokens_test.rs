//! End-to-end test for the CosmosBridgeableTokens governance-controlled allowlist.
//!
//! This test verifies:
//! - Empty allowlist blocks cosmos-originated tokens from SendToEth
//! - Allowlisted denoms can be bridged; non-listed ones cannot
//! - Ethereum-originated tokens are always allowed regardless of the allowlist
//! - MsgSetCosmosBridgeableTokensProposal and MsgDeleteCosmosBridgeableTokensProposal are
//!   rejected when authority is not the gov module
//! - A valid, gov-executed MsgDeleteCosmosBridgeableTokensProposal actually blocks the
//!   deleted denom from the bridge afterwards

use clarity::Address as EthAddress;
use cosmos_gravity::query::{get_cosmos_bridgeable_tokens, get_pending_send_to_eth};
use cosmos_gravity::send::{send_to_eth, MSG_SEND_TO_ETH_TYPE_URL};
use deep_space::coin::Coin;
use deep_space::{Contact, Msg, PrivateKey};
use gravity_proto::gravity::v1::DeleteCosmosBridgeableTokensProposal;
use gravity_proto::gravity::v1::SetCosmosBridgeableTokensProposal;
use gravity_proto::gravity::v1::{query_client::QueryClient as GravityQueryClient, MsgSendToEth};
use gravity_proto::gravity::v2::MsgDeleteCosmosBridgeableTokensProposal;
use gravity_proto::gravity::v2::MsgSetCosmosBridgeableTokensProposal;
use tonic::transport::Channel;
use web30::client::Web3;

use crate::happy_path::test_erc20_deposit_panic;
use crate::happy_path_v2::deploy_cosmos_representing_erc20_and_check_adoption;
use crate::utils::create_default_test_config;
use crate::utils::{
    footoken2_metadata, footoken_metadata, get_user_key, remove_cosmos_bridgeable_tokens,
    send_one_eth, set_cosmos_bridgeable_tokens, start_orchestrators, ValidatorKeys,
};
use crate::{get_fee, one_eth, ADDRESS_PREFIX, OPERATION_TIMEOUT, TOTAL_TIMEOUT};

pub async fn cosmos_bridgeable_tokens_test(
    web30: &Web3,
    contact: &Contact,
    grpc_client: GravityQueryClient<Channel>,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_address: EthAddress,
) {
    let mut grpc_client = grpc_client;
    let no_relay_market_config = create_default_test_config();
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;

    // ------------------------------------------------------------------
    // Step 1: deploy footoken and footoken2 ERC20 representations.
    // Both denoms must be on the CosmosBridgeableTokens allowlist before
    // deploying their ERC20s — handleErc20Deployed always enforces the
    // allowlist, even when the list is otherwise empty.
    // After deployment we clear the list so that Step 2 can verify
    // SendToEth is blocked with an empty allowlist.
    // ------------------------------------------------------------------
    let footoken = footoken_metadata(contact).await;
    let footoken2 = footoken2_metadata(contact).await;
    set_cosmos_bridgeable_tokens(contact, &keys, vec![footoken.clone(), footoken2.clone()]).await;

    let _footoken_erc20 = deploy_cosmos_representing_erc20_and_check_adoption(
        gravity_address,
        web30,
        Some(keys.clone()),
        &mut grpc_client,
        false,
        footoken.clone(),
    )
    .await;

    let _footoken2_erc20 = deploy_cosmos_representing_erc20_and_check_adoption(
        gravity_address,
        web30,
        Some(keys.clone()),
        &mut grpc_client,
        false,
        footoken2.clone(),
    )
    .await;

    // Clear the allowlist so Step 2 can verify SendToEth is blocked.
    remove_cosmos_bridgeable_tokens(contact, &keys, vec![footoken.clone(), footoken2.clone()])
        .await;

    // Set up a test user funded with footoken, footoken2, and some ETH
    let user = get_user_key(None);
    let fund_amount: u64 = 10_000_000;
    contact
        .send_coins(
            Coin {
                denom: footoken.base.clone(),
                amount: fund_amount.into(),
            },
            Some(get_fee(None)),
            user.cosmos_address,
            Some(TOTAL_TIMEOUT),
            keys[0].validator_key,
        )
        .await
        .expect("Failed to fund user with footoken");
    contact
        .send_coins(
            Coin {
                denom: footoken2.base.clone(),
                amount: fund_amount.into(),
            },
            Some(get_fee(None)),
            user.cosmos_address,
            Some(TOTAL_TIMEOUT),
            keys[0].validator_key,
        )
        .await
        .expect("Failed to fund user with footoken2");
    send_one_eth(user.eth_address, web30).await;

    // Also deposit an Ethereum-originated ERC20 to the user on Cosmos
    test_erc20_deposit_panic(
        web30,
        contact,
        &mut grpc_client,
        user.cosmos_address,
        gravity_address,
        erc20_address,
        one_eth(),
        None,
        None,
    )
    .await;

    // ------------------------------------------------------------------
    // Step 2: Assert cosmos-originated token is blocked with empty allowlist.
    // Use send_message directly to bypass the client-side balance checks in
    // the send_to_eth wrapper (which would return a BadInput for denom mismatch
    // but not the on-chain allowlist rejection).
    // ------------------------------------------------------------------
    info!("Step 2: Verify SendToEth is blocked for cosmos-originated token with empty allowlist");
    let bridge_fee = Coin {
        denom: footoken.base.clone(),
        amount: 1u64.into(),
    };
    let chain_fee = Coin {
        denom: footoken.base.clone(),
        amount: 1000u64.into(),
    };
    let send_coin = Coin {
        denom: footoken.base.clone(),
        amount: 1_000_000u64.into(),
    };
    let msg_send_to_eth = MsgSendToEth {
        sender: user.cosmos_address.to_string(),
        eth_dest: user.eth_address.to_string(),
        amount: Some(send_coin.clone().into()),
        bridge_fee: Some(bridge_fee.clone().into()),
        chain_fee: Some(chain_fee.clone().into()),
    };
    let msg = Msg::new(MSG_SEND_TO_ETH_TYPE_URL, msg_send_to_eth);
    let res = contact
        .send_message(
            &[msg],
            None,
            &[get_fee(None)],
            Some(OPERATION_TIMEOUT),
            None,
            user.cosmos_key,
        )
        .await;
    assert!(
        res.is_err(),
        "SendToEth for unlisted cosmos token should have been rejected but succeeded: {:?}",
        res
    );
    info!("Confirmed: SendToEth correctly rejected for unlisted cosmos token");

    // ------------------------------------------------------------------
    // Step 3: Add footoken to the allowlist via governance.
    // ------------------------------------------------------------------
    info!("Step 3: Add footoken to allowlist via governance");
    set_cosmos_bridgeable_tokens(contact, &keys, vec![footoken.clone()]).await;

    let bridgeable_tokens = get_cosmos_bridgeable_tokens(&mut grpc_client)
        .await
        .unwrap();
    assert_eq!(
        bridgeable_tokens,
        vec![footoken.clone()],
        "Expected cosmos_bridgeable_tokens to contain only footoken"
    );
    info!(
        "Confirmed: cosmos_bridgeable_tokens = {:?}",
        bridgeable_tokens
    );

    // ------------------------------------------------------------------
    // Step 4: Assert cosmos-originated footoken is now allowed.
    // ------------------------------------------------------------------
    info!("Step 4: Verify SendToEth succeeds for allowlisted footoken");
    let res = send_to_eth(
        user.cosmos_key,
        user.eth_address,
        send_coin.clone(),
        bridge_fee.clone(),
        Some(chain_fee.clone()),
        get_fee(None),
        contact,
    )
    .await;
    assert!(
        res.is_ok(),
        "SendToEth for allowlisted footoken should succeed: {:?}",
        res
    );

    let pending = get_pending_send_to_eth(&mut grpc_client, user.cosmos_address)
        .await
        .expect("Failed to query pending SendToEth");
    assert!(
        !pending.transfers_in_batches.is_empty() || !pending.unbatched_transfers.is_empty(),
        "Expected pending SendToEth transaction in outgoing pool"
    );
    info!("Confirmed: footoken SendToEth accepted and visible in outgoing pool");

    // ------------------------------------------------------------------
    // Step 5: Assert Ethereum-originated token is always allowed.
    // ------------------------------------------------------------------
    info!("Step 5: Verify Ethereum-originated token is always allowed (allowlist = [footoken])");
    // The gravity0x... denom was deposited to the user in Step 1 setup
    let balances = contact.get_balances(user.cosmos_address).await.unwrap();
    let eth_originated_coin = balances
        .iter()
        .find(|c| c.denom.starts_with("gravity0x"))
        .cloned()
        .expect("User should have an Ethereum-originated gravity0x... balance");

    let bridge_amount = 100000000000000000u128; // 0.1 token with 18 decimals
    let eth_bridge_fee = Coin {
        denom: eth_originated_coin.denom.clone(),
        amount: 1u64.into(),
    };
    let eth_chain_fee = Coin {
        denom: eth_originated_coin.denom.clone(),
        amount: (bridge_amount / 10u128).into(),
    }; // 10% of bridge amount
    let eth_send_coin = Coin {
        denom: eth_originated_coin.denom.clone(),
        amount: bridge_amount.into(),
    };
    let res = send_to_eth(
        user.cosmos_key,
        user.eth_address,
        eth_send_coin,
        eth_bridge_fee,
        Some(eth_chain_fee),
        get_fee(None),
        contact,
    )
    .await;
    assert!(
        res.is_ok(),
        "SendToEth for Ethereum-originated token should always succeed regardless of allowlist: {:?}",
        res
    );
    info!("Confirmed: Ethereum-originated token SendToEth accepted");

    // ------------------------------------------------------------------
    // Step 6: Assert footoken2 is still blocked (not in allowlist).
    // ------------------------------------------------------------------
    info!("Step 6: Verify footoken2 is still blocked (allowlist only contains footoken)");
    let bridge_fee2 = Coin {
        denom: footoken2.base.clone(),
        amount: 1u64.into(),
    };
    let chain_fee2 = Coin {
        denom: footoken2.base.clone(),
        amount: 1000u64.into(),
    };
    let send_coin2 = Coin {
        denom: footoken2.base.clone(),
        amount: 1_000_000u64.into(),
    };
    let msg_send_to_eth2 = MsgSendToEth {
        sender: user.cosmos_address.to_string(),
        eth_dest: user.eth_address.to_string(),
        amount: Some(send_coin2.into()),
        bridge_fee: Some(bridge_fee2.into()),
        chain_fee: Some(chain_fee2.into()),
    };
    let msg2 = Msg::new(MSG_SEND_TO_ETH_TYPE_URL, msg_send_to_eth2);
    let res = contact
        .send_message(
            &[msg2],
            None,
            &[get_fee(None)],
            Some(OPERATION_TIMEOUT),
            None,
            user.cosmos_key,
        )
        .await;
    assert!(
        res.is_err(),
        "SendToEth for unlisted footoken2 should have been rejected but succeeded: {:?}",
        res
    );
    info!("Confirmed: footoken2 correctly rejected while not on allowlist");

    // ------------------------------------------------------------------
    // Step 7: Clear the allowlist and verify footoken is blocked again.
    // ------------------------------------------------------------------
    info!("Step 7: Clear allowlist and verify footoken is blocked again");
    remove_cosmos_bridgeable_tokens(contact, &keys, vec![footoken.clone()]).await;

    let bridgeable_tokens = get_cosmos_bridgeable_tokens(&mut grpc_client)
        .await
        .unwrap();
    assert!(
        bridgeable_tokens.is_empty(),
        "Expected cosmos_bridgeable_tokens to be empty after clearing"
    );

    // Re-fund the user (previous footoken was sent in steps 2/4)
    contact
        .send_coins(
            Coin {
                denom: footoken.base.clone(),
                amount: fund_amount.into(),
            },
            Some(get_fee(None)),
            user.cosmos_address,
            Some(TOTAL_TIMEOUT),
            keys[0].validator_key,
        )
        .await
        .expect("Failed to re-fund user with footoken");

    let msg_send_to_eth3 = MsgSendToEth {
        sender: user.cosmos_address.to_string(),
        eth_dest: user.eth_address.to_string(),
        amount: Some(send_coin.clone().into()),
        bridge_fee: Some(bridge_fee.clone().into()),
        chain_fee: Some(chain_fee.clone().into()),
    };
    let msg3 = Msg::new(MSG_SEND_TO_ETH_TYPE_URL, msg_send_to_eth3);
    let res = contact
        .send_message(
            &[msg3],
            None,
            &[get_fee(None)],
            Some(OPERATION_TIMEOUT),
            None,
            user.cosmos_key,
        )
        .await;
    assert!(
        res.is_err(),
        "SendToEth for footoken should be rejected after allowlist was cleared: {:?}",
        res
    );
    info!("Confirmed: footoken correctly blocked after allowlist cleared");

    // ------------------------------------------------------------------
    // Step 8: Assert governance is the only authority.
    // Send MsgSetCosmosBridgeableTokensProposal directly (not via gov) with a non-gov sender.
    // ------------------------------------------------------------------
    info!("Step 8: Verify MsgSetCosmosBridgeableTokensProposal is rejected for non-gov authority");
    let user_address = keys[0]
        .validator_key
        .to_address(ADDRESS_PREFIX.as_str())
        .unwrap()
        .to_string();
    let msg_update_tokens = MsgSetCosmosBridgeableTokensProposal {
        authority: user_address.clone(),
        proposal: Some(SetCosmosBridgeableTokensProposal {
            title: "Malicious CosmosBridgeableTokens proposal".to_string(),
            description: "should be rejected".to_string(),
            metadatas: vec![footoken.clone()],
        }),
    };
    let msg_gov = Msg::new(
        "/gravity.v2.MsgSetCosmosBridgeableTokensProposal",
        msg_update_tokens,
    );
    let res = contact
        .send_message(
            &[msg_gov],
            None,
            &[],
            Some(OPERATION_TIMEOUT),
            None,
            keys[0].validator_key,
        )
        .await;
    assert!(
        res.is_err(),
        "MsgSetCosmosBridgeableTokensProposal with non-gov authority should be rejected but succeeded: {:?}",
        res
    );
    info!("Confirmed: MsgSetCosmosBridgeableTokensProposal correctly rejected for non-gov sender");

    // ------------------------------------------------------------------
    // Step 9: Assert governance is the only authority for deletion too.
    // Send MsgDeleteCosmosBridgeableTokensProposal directly (not via gov) with a non-gov sender.
    // ------------------------------------------------------------------
    info!(
        "Step 9: Verify MsgDeleteCosmosBridgeableTokensProposal is rejected for non-gov authority"
    );
    let msg_delete_tokens = MsgDeleteCosmosBridgeableTokensProposal {
        authority: user_address,
        proposal: Some(DeleteCosmosBridgeableTokensProposal {
            title: "Malicious CosmosBridgeableTokens deletion".to_string(),
            description: "should be rejected".to_string(),
            metadatas: vec![footoken.clone()],
        }),
    };
    let msg_gov_delete = Msg::new(
        "/gravity.v2.MsgDeleteCosmosBridgeableTokensProposal",
        msg_delete_tokens,
    );
    let res = contact
        .send_message(
            &[msg_gov_delete],
            None,
            &[],
            Some(OPERATION_TIMEOUT),
            None,
            keys[0].validator_key,
        )
        .await;
    assert!(
        res.is_err(),
        "MsgDeleteCosmosBridgeableTokensProposal with non-gov authority should be rejected but succeeded: {:?}",
        res
    );
    info!(
        "Confirmed: MsgDeleteCosmosBridgeableTokensProposal correctly rejected for non-gov sender"
    );

    // ------------------------------------------------------------------
    // Step 10: Assert that a *valid* delete proposal (submitted via governance)
    // actually blocks the token from the bridge afterwards.
    // First re-add footoken to the allowlist and confirm it can be bridged,
    // then delete it via governance and confirm it can no longer be bridged.
    // ------------------------------------------------------------------
    info!("Step 10: Verify footoken is blocked from the bridge after a valid governance delete proposal");
    set_cosmos_bridgeable_tokens(contact, &keys, vec![footoken.clone()]).await;

    let bridgeable_tokens = get_cosmos_bridgeable_tokens(&mut grpc_client)
        .await
        .unwrap();
    assert_eq!(
        bridgeable_tokens,
        vec![footoken.clone()],
        "Expected cosmos_bridgeable_tokens to contain only footoken after re-adding"
    );

    let res = send_to_eth(
        user.cosmos_key,
        user.eth_address,
        send_coin.clone(),
        bridge_fee.clone(),
        Some(chain_fee.clone()),
        get_fee(None),
        contact,
    )
    .await;
    assert!(
        res.is_ok(),
        "SendToEth for footoken should succeed while it is allowlisted: {:?}",
        res
    );
    info!("Confirmed: footoken SendToEth accepted while allowlisted");

    remove_cosmos_bridgeable_tokens(contact, &keys, vec![footoken.clone()]).await;

    let bridgeable_tokens = get_cosmos_bridgeable_tokens(&mut grpc_client)
        .await
        .unwrap();
    assert!(
        bridgeable_tokens.is_empty(),
        "Expected cosmos_bridgeable_tokens to be empty after the valid governance delete proposal"
    );

    let msg_send_to_eth4 = MsgSendToEth {
        sender: user.cosmos_address.to_string(),
        eth_dest: user.eth_address.to_string(),
        amount: Some(send_coin.clone().into()),
        bridge_fee: Some(bridge_fee.clone().into()),
        chain_fee: Some(chain_fee.clone().into()),
    };
    let msg4 = Msg::new(MSG_SEND_TO_ETH_TYPE_URL, msg_send_to_eth4);
    let res = contact
        .send_message(
            &[msg4],
            None,
            &[get_fee(None)],
            Some(OPERATION_TIMEOUT),
            None,
            user.cosmos_key,
        )
        .await;
    assert!(
        res.is_err(),
        "SendToEth for footoken should be rejected after the valid governance delete proposal: {:?}",
        res
    );
    info!("Confirmed: footoken correctly blocked from the bridge after the valid governance delete proposal");

    info!("CosmosBridgeableTokens test passed!");
}
