//! This is the happy path test for Cosmos to Ethereum asset transfers, meaning assets originated on Cosmos

use crate::utils::create_default_test_config;
use crate::utils::footoken_metadata;
use crate::utils::get_decimals;
use crate::utils::get_erc20_balance_safe;
use crate::utils::get_event_nonce_safe;
use crate::utils::get_user_key;
use crate::utils::send_one_eth;
use crate::utils::start_orchestrators;
use crate::MINER_ADDRESS;
use crate::MINER_PRIVATE_KEY;
use crate::TOTAL_TIMEOUT;
use crate::{get_fee, utils::ValidatorKeys};
use clarity::Address as EthAddress;
use clarity::Uint256;
use cosmos_gravity::send::send_to_eth;
use deep_space::coin::Coin;
use deep_space::Contact;
use ethereum_gravity::deploy_erc20::deploy_erc20;
use ethereum_gravity::utils::get_valset_nonce;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::Metadata;
use gravity_proto::gravity::{
    query_client::QueryClient as GravityQueryClient, QueryDenomToErc20Request,
};
use std::time::{Duration, Instant};
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use web30::client::Web3;
use web30::types::SendTxOption;

pub async fn happy_path_test_v2(
    web30: &Web3,
    grpc_client: GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    validator_out: bool,
    ibc_metadata: Option<Metadata>,
) {
    let mut grpc_client = grpc_client;

    let ibc_metadata = match ibc_metadata {
        Some(metadata) => metadata,
        None => footoken_metadata(contact).await,
    };

    let erc20_contract = deploy_cosmos_representing_erc20_and_check_adoption(
        gravity_address,
        web30,
        Some(keys.clone()),
        &mut grpc_client,
        validator_out,
        ibc_metadata.clone(),
    )
    .await;

    let token_to_send_to_eth = ibc_metadata.base.clone();

    // one foo token
    let amount_to_bridge: Uint256 = 1_000_000u64.into();
    let send_to_user_coin = Coin {
        denom: token_to_send_to_eth.clone(),
        amount: amount_to_bridge.clone() + 100u8.into(),
    };
    let send_to_eth_coin = Coin {
        denom: token_to_send_to_eth.clone(),
        amount: amount_to_bridge.clone(),
    };

    let user = get_user_key();
    // send the user some footoken
    contact
        .send_coins(
            send_to_user_coin.clone(),
            Some(get_fee(None)),
            user.cosmos_address,
            Some(TOTAL_TIMEOUT),
            keys[0].validator_key,
        )
        .await
        .unwrap();

    let balances = contact.get_balances(user.cosmos_address).await.unwrap();
    let mut found = false;
    for coin in balances {
        if coin.denom == token_to_send_to_eth.clone() {
            found = true;
            break;
        }
    }
    if !found {
        panic!(
            "Failed to send {} to the user address",
            token_to_send_to_eth
        );
    }
    info!(
        "Sent some {} to user address {}",
        token_to_send_to_eth, user.cosmos_address
    );
    // send the user some eth, they only need this to check their
    // erc20 balance, so a pretty minor usecase
    send_one_eth(user.eth_address, web30).await;
    info!("Sent 1 eth to user address {}", user.eth_address);

    let res = send_to_eth(
        user.cosmos_key,
        user.eth_address,
        send_to_eth_coin,
        get_fee(Some(ibc_metadata.base.clone())),
        get_fee(Some(ibc_metadata.base.clone())),
        contact,
    )
    .await
    .unwrap();
    info!("Send to eth res {:?}", res);
    info!(
        "Locked up {} {} to send to Cosmos",
        amount_to_bridge, token_to_send_to_eth
    );

    info!("Waiting for batch to be signed and relayed to Ethereum");

    let start = Instant::now();
    // overly complicated retry logic allows us to handle the possibility that gas prices change between blocks
    // and cause any individual request to fail.

    while Instant::now() - start < TOTAL_TIMEOUT {
        let new_balance = get_erc20_balance_safe(erc20_contract, web30, user.eth_address).await;
        // only keep trying if our error is gas related
        if new_balance.is_err() {
            continue;
        }
        let balance = new_balance.unwrap();
        if balance == amount_to_bridge {
            info!(
                "Successfully bridged {} Cosmos asset {} to Ethereum!",
                amount_to_bridge, token_to_send_to_eth
            );
            assert!(balance == amount_to_bridge.clone());
            break;
        } else if balance != 0u8.into() {
            panic!(
                "Expected {} {} but got {} instead",
                amount_to_bridge, token_to_send_to_eth, balance
            );
        }
        delay_for(Duration::from_secs(1)).await;
    }
}

/// This segment is broken out because it's used in two different tests
/// once here where we verify that tokens bridge correctly and once in valset_rewards
/// where we do a governance update to enable rewards
pub async fn deploy_cosmos_representing_erc20_and_check_adoption(
    gravity_address: EthAddress,
    web30: &Web3,
    keys: Option<Vec<ValidatorKeys>>,
    grpc_client: &mut GravityQueryClient<Channel>,
    validator_out: bool,
    token_metadata: Metadata,
) -> EthAddress {
    get_valset_nonce(gravity_address, *MINER_ADDRESS, web30)
        .await
        .expect("Incorrect Gravity Address or otherwise unable to contact Gravity");

    let starting_event_nonce = get_event_nonce_safe(gravity_address, web30, *MINER_ADDRESS)
        .await
        .unwrap();

    let cosmos_decimals = get_decimals(&token_metadata);
    deploy_erc20(
        token_metadata.base.clone(),
        token_metadata.name.clone(),
        token_metadata.symbol.clone(),
        cosmos_decimals,
        gravity_address,
        web30,
        Some(TOTAL_TIMEOUT),
        *MINER_PRIVATE_KEY,
        vec![
            SendTxOption::GasLimitMultiplier(2.0),
            SendTxOption::GasPriceMultiplier(2.0),
        ],
    )
    .await
    .unwrap();
    let ending_event_nonce = get_event_nonce_safe(gravity_address, web30, *MINER_ADDRESS)
        .await
        .unwrap();

    assert!(starting_event_nonce != ending_event_nonce);
    info!(
        "Successfully deployed new ERC20 representing FooToken on Cosmos with event nonce {}",
        ending_event_nonce
    );

    // if no keys are provided we assume the caller does not want to spawn
    // orchestrators as part of the test
    if let Some(keys) = keys {
        let no_relay_market_config = create_default_test_config();
        start_orchestrators(
            keys.clone(),
            gravity_address,
            validator_out,
            no_relay_market_config,
        )
        .await;
    }

    let start = Instant::now();
    // the erc20 representing the cosmos asset on Ethereum
    let mut erc20_contract = None;
    while Instant::now() - start < TOTAL_TIMEOUT {
        let res = grpc_client
            .denom_to_erc20(QueryDenomToErc20Request {
                denom: token_metadata.base.clone(),
            })
            .await;
        if let Ok(res) = res {
            let erc20 = res.into_inner().erc20;
            info!(
                "Successfully adopted {} token contract of {}",
                token_metadata.base, erc20
            );
            erc20_contract = Some(erc20);
            break;
        }
        delay_for(Duration::from_secs(1)).await;
    }
    if erc20_contract.is_none() {
        panic!(
            "Cosmos did not adopt the ERC20 contract for {} it must be invalid in some way",
            token_metadata.base
        );
    }
    let erc20_contract: EthAddress = erc20_contract.unwrap().parse().unwrap();

    // now that we have the contract, validate that it has the properties we want
    let got_decimals = web30
        .get_erc20_decimals(erc20_contract, *MINER_ADDRESS)
        .await
        .unwrap();
    assert_eq!(Uint256::from(cosmos_decimals), got_decimals);

    let got_name = web30
        .get_erc20_name(erc20_contract, *MINER_ADDRESS)
        .await
        .unwrap();
    assert_eq!(got_name, token_metadata.name);

    let got_symbol = web30
        .get_erc20_symbol(erc20_contract, *MINER_ADDRESS)
        .await
        .unwrap();
    assert_eq!(got_symbol, token_metadata.symbol);

    let got_supply = web30
        .get_erc20_supply(erc20_contract, *MINER_ADDRESS)
        .await
        .unwrap();
    assert_eq!(got_supply, 0u8.into());

    erc20_contract
}
