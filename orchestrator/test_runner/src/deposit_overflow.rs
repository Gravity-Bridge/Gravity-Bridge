//! This tests Uint256 max value deposits to the module (it does NOT test deposits to the ethereum
//! contract which are later relayed by the orchestrator).
//! NOTE: In the process of testing the module, the bridge is desync'd due to false validator claims,
//! therefore adding new tests at the end of this one may fail.
use crate::unhalt_bridge::get_nonces;
use crate::utils::{check_cosmos_balances, get_user_key, submit_false_claims, ValidatorKeys};
use crate::OPERATION_TIMEOUT;
use crate::{get_fee, MINER_ADDRESS};
use clarity::{Address as EthAddress, Uint256};
use deep_space::private_key::PrivateKey as CosmosPrivateKey;
use deep_space::{Coin, Contact, Fee};
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::QueryErc20ToDenomRequest;
use gravity_utils::num_conversion::downcast_uint256;
use num::Bounded;
use tonic::transport::Channel;
use web30::client::Web3;

// Tests end to end bridge function, then asserts a Uint256 max value deposit of overflowing_erc20 succeeds,
// then asserts that other token deposits are unaffected by this transfer,
// and future deposits of overflowing_erc20 are blocked
pub async fn deposit_overflow_test(
    web30: &Web3,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    erc20_addresses: Vec<EthAddress>,
    grpc_client: GravityQueryClient<Channel>,
) {
    let mut grpc_client = grpc_client;
    let orchestrator_keys: Vec<CosmosPrivateKey> =
        keys.clone().into_iter().map(|key| key.orch_key).collect();
    ///////////////////// SETUP /////////////////////
    let user_keys = get_user_key();
    let dest = user_keys.cosmos_address;
    let dest2 = get_user_key().cosmos_address;
    // this is a valid eth address we are pretending is an erc20
    // since this test is not completely end to end (we don't have an actual overflowing erc20 solidity contract)
    // we just pretend that this random address i an erc20 that overflows with false events.
    let overflowing_erc20 = user_keys.eth_address;
    let check_module_erc20 = erc20_addresses[0]; // unrelated erc20 to check module functions
    let overflowing_denom = grpc_client
        .erc20_to_denom(QueryErc20ToDenomRequest {
            erc20: overflowing_erc20.clone().to_string(),
        })
        .await
        .unwrap()
        .into_inner()
        .denom;
    let check_module_denom = grpc_client
        .erc20_to_denom(QueryErc20ToDenomRequest {
            erc20: check_module_erc20.clone().to_string(),
        })
        .await
        .unwrap()
        .into_inner()
        .denom;
    let mut grpc_client = grpc_client.clone();
    let max_amount = Uint256::max_value(); // 2^256 - 1 (max amount possible to send)
    let normal_amount = Uint256::from(30_000_000u64); // an amount we would expect to easily transfer
    let fee = Fee {
        amount: vec![get_fee()],
        gas_limit: 500_000_000u64,
        granter: None,
        payer: None,
    };

    ///////////////////// EXECUTION /////////////////////
    let initial_nonce = get_nonces(&mut grpc_client, &keys, &contact.get_prefix()).await[0];
    let initial_block_height =
        downcast_uint256(web30.eth_get_latest_block().await.unwrap().number).unwrap();
    info!("Initial transfer complete, nonce is {}", initial_nonce);

    // NOTE: the dest user's balance should be 1 * normal_amount of check_module_erc20 token
    let mut expected_cosmos_coins = vec![Coin {
        amount: normal_amount.clone(),
        denom: check_module_denom.clone(),
    }];
    check_cosmos_balances(contact, dest, &expected_cosmos_coins).await;

    // Don't judge me! it's really difficult to get a 2^256-1 ERC20 transfer to happen, so we fake it
    // We would want to deploy a custom ERC20 which allows transfers of any amounts for convenience
    // But this simulates that without testing the solidity portion
    submit_false_claims(
        &orchestrator_keys,
        initial_nonce + 1,
        initial_block_height + 1,
        max_amount.clone(),
        dest,
        *MINER_ADDRESS,
        overflowing_erc20,
        contact,
        &fee,
        Some(OPERATION_TIMEOUT),
    )
    .await;

    // NOTE: the dest user's balance should be 1 * normal_amount of check_module_erc20 token and
    // Uint256 max of false_claims_erc20 token
    expected_cosmos_coins.push(Coin {
        amount: max_amount.clone(),
        denom: overflowing_denom.clone(),
    });
    check_cosmos_balances(contact, dest, &expected_cosmos_coins).await;

    // Note: Now the bridge is broken since the ethereum side's event nonce does not match the
    // Cosmos side's event nonce, we are forced to continue lying to keep the charade going

    // Expect this one to succeed as we're using an unrelated token
    submit_false_claims(
        &orchestrator_keys,
        initial_nonce + 2,
        initial_block_height + 2,
        normal_amount.clone(),
        dest,
        *MINER_ADDRESS,
        check_module_erc20,
        contact,
        &fee,
        Some(OPERATION_TIMEOUT),
    )
    .await;
    // NOTE: the dest user's balance should now be 2 * normal_amount of check_module_erc20 token and
    // Uint256 max of false_claims_erc20 token
    expected_cosmos_coins = vec![
        Coin {
            amount: normal_amount.clone() + normal_amount.clone(),
            denom: check_module_denom.clone(),
        },
        Coin {
            amount: max_amount,
            denom: overflowing_denom,
        },
    ];
    check_cosmos_balances(contact, dest, &expected_cosmos_coins).await;

    // Expect this one to fail, there's no supply left of the false_claims_erc20!
    submit_false_claims(
        &orchestrator_keys,
        initial_nonce + 3,
        initial_block_height + 3,
        normal_amount.clone(),
        dest,
        *MINER_ADDRESS,
        overflowing_erc20,
        contact,
        &fee,
        Some(OPERATION_TIMEOUT),
    )
    .await;
    // NOTE: the dest user's balance should still be 2 * normal_amount of check_module_erc20 token and
    // still be Uint256 max of false_claims_erc20 token
    check_cosmos_balances(contact, dest, &expected_cosmos_coins).await;

    // Expect this one to also fail, there's no supply left of the false_claims_erc20, even though account has changed
    submit_false_claims(
        &orchestrator_keys,
        initial_nonce + 4,
        initial_block_height + 4,
        normal_amount.clone(),
        dest2,
        *MINER_ADDRESS,
        overflowing_erc20,
        contact,
        &fee,
        Some(OPERATION_TIMEOUT),
    )
    .await;
    let dest2_bals = contact.get_balances(dest2).await.unwrap();
    assert!(
        dest2_bals.is_empty(),
        "dest2 should have no coins, but they have {:?}",
        dest2_bals
    );
    info!("Successful send of Uint256 max value to cosmos user, unable to overflow the supply!");
}
