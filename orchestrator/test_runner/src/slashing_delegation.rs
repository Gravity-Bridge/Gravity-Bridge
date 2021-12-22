//! This file tests delegation as it interacts with slashing in the Gravity bridge module. This test was created
//! after a bug caused delegations and delegation withdraws to fail when a validator was slashed in a specific way
//! and exists to prevent regressions and hopefully find any new bugs of the same nature

use crate::happy_path::test_valset_update;
use crate::signature_slashing::{reduce_slashing_window, wait_for_height};
use crate::utils::{
    create_default_test_config, get_operator_address, get_user_key, start_orchestrators,
    ValidatorKeys,
};
use crate::{get_fee, STAKING_TOKEN, TOTAL_TIMEOUT};
use clarity::Address as EthAddress;
use deep_space::{Coin, Contact};
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use tonic::transport::Channel;
use web30::client::Web3;

pub async fn slashing_delegation_test(
    web30: &Web3,
    grpc_client: GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
) {
    let mut grpc_client = grpc_client;

    let amount_to_delegate = Coin {
        denom: STAKING_TOKEN.clone(),
        amount: 100_000_000u32.into(),
    };
    let mut fee_send = get_fee();
    fee_send.amount *= 1000u16.into();

    // create a user and send them some coins to delegate
    // user A delegates before slashing
    let user_a = get_user_key();
    // user B delegates after slashing
    let user_b = get_user_key();
    contact
        .send_coins(
            amount_to_delegate.clone(),
            Some(get_fee()),
            user_a.cosmos_address,
            Some(TOTAL_TIMEOUT),
            keys[0].validator_key,
        )
        .await
        .unwrap();
    contact
        .send_coins(
            fee_send.clone(),
            Some(get_fee()),
            user_a.cosmos_address,
            Some(TOTAL_TIMEOUT),
            keys[0].validator_key,
        )
        .await
        .unwrap();
    contact
        .send_coins(
            amount_to_delegate.clone(),
            Some(get_fee()),
            user_b.cosmos_address,
            Some(TOTAL_TIMEOUT),
            keys[0].validator_key,
        )
        .await
        .unwrap();
    contact
        .send_coins(
            fee_send,
            Some(get_fee()),
            user_b.cosmos_address,
            Some(TOTAL_TIMEOUT),
            keys[0].validator_key,
        )
        .await
        .unwrap();

    let no_relay_market_config = create_default_test_config();
    // by setting validator out to true, the last validator will not have an orchestrator, will not submit
    // signatures and is sure to be slashed
    start_orchestrators(keys.clone(), gravity_address, true, no_relay_market_config).await;
    let slashed_validator = keys.iter().last().unwrap().clone();

    // delegate to the validator that is about to be slashed from a new address

    // create two valsets for the validator to be slashed for
    test_valset_update(web30, contact, &mut grpc_client, &keys, gravity_address).await;
    test_valset_update(web30, contact, &mut grpc_client, &keys, gravity_address).await;

    // delegate to validator that is not yet slashed
    let res = contact
        .delegate_to_validator(
            get_operator_address(slashed_validator.validator_key),
            amount_to_delegate.clone(),
            get_fee(),
            user_a.cosmos_key,
            Some(TOTAL_TIMEOUT),
        )
        .await
        .unwrap();
    info!("Delegation result is {:?}", res);

    // reduce slashing window so that slashing occurs quickly
    reduce_slashing_window(contact, &mut grpc_client, &keys).await;

    // wait for slashing to occur
    wait_for_height(20, contact).await;

    // test delegating to the slashed validator
    let res = contact
        .delegate_to_validator(
            get_operator_address(slashed_validator.validator_key),
            amount_to_delegate,
            get_fee(),
            user_b.cosmos_key,
            Some(TOTAL_TIMEOUT),
        )
        .await
        .unwrap();
    info!("Delegation result is {:?}", res);

    info!("Waiting to withdraw delegation rewards");
    wait_for_height(40, contact).await;

    // test withdrawing rewards from both users A and B
    let res = contact
        .withdraw_delegator_rewards(
            get_operator_address(slashed_validator.validator_key),
            get_fee(),
            user_a.cosmos_key,
            Some(TOTAL_TIMEOUT),
        )
        .await
        .unwrap();
    info!(
        "Rewards withdraw result for pre-slashing delegation is {:?}",
        res
    );
    let res = contact
        .withdraw_delegator_rewards(
            get_operator_address(slashed_validator.validator_key),
            get_fee(),
            user_b.cosmos_key,
            Some(TOTAL_TIMEOUT),
        )
        .await
        .unwrap();
    info!(
        "Rewards withdraw result for post-slashing delegation is {:?}",
        res
    );
    let res = contact
        .withdraw_validator_commission(
            get_operator_address(slashed_validator.validator_key),
            get_fee(),
            slashed_validator.validator_key,
            Some(TOTAL_TIMEOUT),
        )
        .await
        .unwrap();
    info!("Validator commission withdraw result is {:?}", res);

    info!("Successfully completed Slashing Delegation test!");
}
