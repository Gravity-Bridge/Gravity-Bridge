//! this crate, namely runs all up integration tests of the Gravity code against
//! several scenarios, happy path and non happy path. This is essentially meant
//! to be executed in our specific CI docker container and nowhere else. If you
//! find some function useful pull it up into the more general gravity_utils or the like

#[macro_use]
extern crate log;

use crate::airdrop_proposal::airdrop_proposal_test;
use crate::bootstrapping::*;
use crate::deposit_overflow::deposit_overflow_test;
use crate::ethereum_blacklist_test::ethereum_blacklist_test;
use crate::ibc_metadata::ibc_metadata_proposal_test;
use crate::invalid_events::invalid_events;
use crate::pause_bridge::pause_bridge_test;
use crate::signature_slashing::signature_slashing_test;
use crate::slashing_delegation::slashing_delegation_test;
use crate::tx_cancel::send_to_eth_and_cancel;
use crate::upgrade::{upgrade_part_1, upgrade_part_2};
use crate::utils::*;
use crate::valset_rewards::valset_rewards_test;
use clarity::PrivateKey as EthPrivateKey;
use clarity::{Address as EthAddress, Uint256};
use deep_space::coin::Coin;
use deep_space::Contact;
use erc_721_happy_path::erc721_happy_path_test;
use evidence_based_slashing::evidence_based_slashing;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use happy_path::happy_path_test;
use happy_path_v2::happy_path_test_v2;
use lazy_static::lazy_static;
use orch_keys::orch_keys;
use relay_market::relay_market_test;
use std::{env, time::Duration};
use transaction_stress_test::transaction_stress_test;
use unhalt_bridge::unhalt_bridge_test;
use valset_stress::validator_set_stress_test;

mod airdrop_proposal;
mod bootstrapping;
mod deposit_overflow;
mod erc_721_happy_path;
mod ethereum_blacklist_test;
mod evidence_based_slashing;
mod happy_path;
mod happy_path_v2;
mod ibc_metadata;
mod invalid_events;
mod orch_keys;
mod pause_bridge;
mod relay_market;
mod signature_slashing;
mod slashing_delegation;
mod transaction_stress_test;
mod tx_cancel;
mod unhalt_bridge;
mod upgrade;
mod utils;
mod valset_rewards;
mod valset_stress;

/// the timeout for individual requests
const OPERATION_TIMEOUT: Duration = Duration::from_secs(30);
/// the timeout for the total system
const TOTAL_TIMEOUT: Duration = Duration::from_secs(300);

// Retrieve values from runtime ENV vars
lazy_static! {
    static ref ADDRESS_PREFIX: String =
        env::var("ADDRESS_PREFIX").unwrap_or_else(|_| "gravity".to_string());
    static ref STAKING_TOKEN: String =
        env::var("STAKING_TOKEN").unwrap_or_else(|_| "stake".to_owned());
    static ref COSMOS_NODE_GRPC: String =
        env::var("COSMOS_NODE_GRPC").unwrap_or_else(|_| "http://localhost:9090".to_owned());
    static ref COSMOS_NODE_ABCI: String =
        env::var("COSMOS_NODE_ABCI").unwrap_or_else(|_| "http://localhost:26657".to_owned());
    static ref ETH_NODE: String =
        env::var("ETH_NODE").unwrap_or_else(|_| "http://localhost:8545".to_owned());
}

/// this value reflects the contents of /tests/container-scripts/setup-validator.sh
/// and is used to compute if a stake change is big enough to trigger a validator set
/// update since we want to make several such changes intentionally
pub const STAKE_SUPPLY_PER_VALIDATOR: u128 = 1000000000;
/// this is the amount each validator bonds at startup
pub const STARTING_STAKE_PER_VALIDATOR: u128 = STAKE_SUPPLY_PER_VALIDATOR / 2;

lazy_static! {
    // this key is the private key for the public key defined in tests/assets/ETHGenesis.json
    // where the full node / miner sends its rewards. Therefore it's always going
    // to have a lot of ETH to pay for things like contract deployments
    static ref MINER_PRIVATE_KEY: EthPrivateKey =
        "0xb1bab011e03a9862664706fc3bbaa1b16651528e5f0e7fbfcbfdd8be302a13e7"
            .parse()
            .unwrap();
    static ref MINER_ADDRESS: EthAddress = MINER_PRIVATE_KEY.to_address();
}

/// Gets the standard non-token fee for the testnet. We deploy the test chain with STAKE
/// and FOOTOKEN balances by default, one footoken is sufficient for any Cosmos tx fee except
/// fees for send_to_eth messages which have to be of the same bridged denom so that the relayers
/// on the Ethereum side can be paid in that token.
pub fn get_fee(denom: Option<String>) -> Coin {
    match denom {
        None => Coin {
            denom: get_test_token_name(),
            amount: 1u32.into(),
        },
        Some(denom) => Coin {
            denom,
            amount: 1u32.into(),
        },
    }
}

pub fn get_deposit() -> Coin {
    Coin {
        denom: STAKING_TOKEN.to_string(),
        amount: 1_000_000_000u64.into(),
    }
}

pub fn get_test_token_name() -> String {
    "footoken".to_string()
}

pub fn get_chain_id() -> String {
    "gravity-test".to_string()
}

pub fn one_eth() -> Uint256 {
    1000000000000000000u128.into()
}

pub fn one_hundred_eth() -> Uint256 {
    (1000000000000000000u128 * 100).into()
}

pub fn should_deploy_contracts() -> bool {
    match env::var("DEPLOY_CONTRACTS") {
        Ok(s) => s == "1" || s.to_lowercase() == "yes" || s.to_lowercase() == "true",
        _ => false,
    }
}

#[actix_rt::main]
pub async fn main() {
    env_logger::init();
    info!("Starting Gravity test-runner");
    let contact = Contact::new(
        COSMOS_NODE_GRPC.as_str(),
        OPERATION_TIMEOUT,
        ADDRESS_PREFIX.as_str(),
    )
    .unwrap();

    info!("Waiting for Cosmos chain to come online");
    wait_for_cosmos_online(&contact, TOTAL_TIMEOUT).await;

    let grpc_client = GravityQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .unwrap();
    let web30 = web30::client::Web3::new(ETH_NODE.as_str(), OPERATION_TIMEOUT);
    let keys = get_keys();

    // if we detect this env var we are only deploying contracts, do that then exit.
    if should_deploy_contracts() {
        info!("test-runner in contract deploying mode, deploying contracts, then exiting");
        deploy_contracts(&contact).await;
        return;
    }

    let contracts = parse_contract_addresses();
    // the address of the deployed Gravity contract
    let gravity_address = contracts.gravity_contract;
    // the address of the deployed GravityERC721 contract
    let gravity_erc721_address = contracts.gravity_erc721_contract;
    // addresses of deployed ERC20 token contracts to be used for testing
    let erc20_addresses = contracts.erc20_addresses.clone();
    // addresses of deployed ERC721 token contracts to be used for testing
    let erc721_addresses = contracts.erc721_addresses.clone();
    // before we start the orchestrators send them some funds so they can pay
    // for things
    send_eth_to_orchestrators(&keys, &web30).await;

    assert!(contact
        .get_balance(
            keys[0]
                .validator_key
                .to_address(&contact.get_prefix())
                .unwrap(),
            get_test_token_name(),
        )
        .await
        .unwrap()
        .is_some());

    // This segment contains optional tests, by default we run a happy path test
    // this tests all major functionality of Gravity once or twice.
    // VALSET_STRESS sends in 1k valsets to sign and update
    // BATCH_STRESS fills several batches and executes an out of order batch
    // VALIDATOR_OUT simulates a validator not participating in the happy path test
    // V2_HAPPY_PATH runs the happy path tests but focusing on moving Cosmos assets to Ethereum
    // ARBITRARY_LOGIC tests the arbitrary logic functionality, where an arbitrary contract call
    //                 is created and deployed vai the bridge.
    let test_type = env::var("TEST_TYPE");
    info!("Starting tests with {:?}", test_type);
    if let Ok(test_type) = test_type {
        if test_type == "VALIDATOR_OUT" {
            info!("Starting Validator out test");
            happy_path_test(
                &web30,
                grpc_client,
                &contact,
                keys,
                gravity_address,
                erc20_addresses[0],
                true,
            )
            .await;
            return;
        } else if test_type == "BATCH_STRESS" {
            let contact = Contact::new(
                COSMOS_NODE_GRPC.as_str(),
                TOTAL_TIMEOUT,
                ADDRESS_PREFIX.as_str(),
            )
            .unwrap();
            transaction_stress_test(
                &web30,
                &contact,
                grpc_client,
                keys,
                gravity_address,
                erc20_addresses,
            )
            .await;
            return;
        } else if test_type == "VALSET_STRESS" {
            info!("Starting Valset update stress test");
            validator_set_stress_test(&web30, grpc_client, &contact, keys, gravity_address).await;
            return;
        } else if test_type == "VALSET_REWARDS" {
            info!("Starting Valset rewards test");
            valset_rewards_test(&web30, grpc_client, &contact, keys, gravity_address).await;
            return;
        } else if test_type == "V2_HAPPY_PATH" || test_type == "HAPPY_PATH_V2" {
            info!("Starting happy path for Gravity v2");
            happy_path_test_v2(
                &web30,
                grpc_client,
                &contact,
                keys,
                gravity_address,
                false,
                None,
            )
            .await;
            return;
        } else if test_type == "RELAY_MARKET" {
            info!("Starting relay market tests!");
            relay_market_test(&web30, grpc_client, &contact, keys, gravity_address).await;
            return;
        } else if test_type == "ORCHESTRATOR_KEYS" {
            info!("Starting orchestrator key update tests!");
            orch_keys(grpc_client, &contact, keys).await;
            return;
        } else if test_type == "EVIDENCE" {
            info!("Starting evidence based slashing tests!");
            evidence_based_slashing(&web30, &contact, keys, gravity_address).await;
            return;
        } else if test_type == "TXCANCEL" {
            info!("Starting SendToEth cancellation test!");
            send_to_eth_and_cancel(
                &contact,
                grpc_client,
                &web30,
                keys,
                gravity_address,
                erc20_addresses[0],
            )
            .await;
            return;
        } else if test_type == "INVALID_EVENTS" {
            info!("Starting invalid events test!");
            invalid_events(
                &web30,
                &contact,
                keys,
                gravity_address,
                erc20_addresses[0],
                grpc_client,
            )
            .await;
            return;
        } else if test_type == "UNHALT_BRIDGE" {
            info!("Starting unhalt bridge tests");
            unhalt_bridge_test(
                &web30,
                grpc_client,
                &contact,
                keys,
                gravity_address,
                erc20_addresses[0],
            )
            .await;
            return;
        } else if test_type == "PAUSE_BRIDGE" {
            info!("Starting pause bridge tests");
            pause_bridge_test(
                &web30,
                grpc_client,
                &contact,
                keys,
                gravity_address,
                erc20_addresses[0],
            )
            .await;
            return;
        } else if test_type == "DEPOSIT_OVERFLOW" {
            info!("Starting deposit overflow test!");
            deposit_overflow_test(&web30, &contact, keys, erc20_addresses, grpc_client).await;
            return;
        } else if test_type == "ETHEREUM_BLACKLIST" {
            info!("Starting ethereum blacklist test");
            ethereum_blacklist_test(grpc_client, &contact, keys).await;
            return;
        } else if test_type == "AIRDROP_PROPOSAL" {
            info!("Starting airdrop governance proposal test");
            airdrop_proposal_test(&contact, keys).await;
            return;
        } else if test_type == "SIGNATURE_SLASHING" {
            info!("Starting Signature Slashing test");
            signature_slashing_test(&web30, grpc_client, &contact, keys, gravity_address).await;
            return;
        } else if test_type == "SLASHING_DELEGATION" {
            info!("Starting Slashing Delegation test");
            slashing_delegation_test(&web30, grpc_client, &contact, keys, gravity_address).await;
            return;
        } else if test_type == "IBC_METADATA" {
            info!("Starting IBC metadata proposal test");
            ibc_metadata_proposal_test(gravity_address, keys, grpc_client, &contact, &web30).await;
            return;
        } else if test_type == "ERC721_HAPPY_PATH" {
            info!("Starting ERC 721 transfer test");
            erc721_happy_path_test(
                &web30,
                &contact,
                keys,
                gravity_address,
                gravity_erc721_address,
                erc721_addresses[0],
                false,
            )
            .await;
            return;
        } else if test_type == "UPGRADE_PART_1" {
            info!("Starting Gravity Upgrade test Part 1");
            let contact = Contact::new(
                COSMOS_NODE_GRPC.as_str(),
                TOTAL_TIMEOUT,
                ADDRESS_PREFIX.as_str(),
            )
            .unwrap();
            upgrade_part_1(
                &web30,
                &contact,
                grpc_client,
                keys,
                gravity_address,
                erc20_addresses,
            )
            .await;
            return;
        } else if test_type == "UPGRADE_PART_2" {
            info!("Starting Gravity Upgrade test Part 2");
            let contact = Contact::new(
                COSMOS_NODE_GRPC.as_str(),
                TOTAL_TIMEOUT,
                ADDRESS_PREFIX.as_str(),
            )
            .unwrap();
            upgrade_part_2(
                &web30,
                &contact,
                grpc_client,
                keys,
                gravity_address,
                erc20_addresses,
            )
            .await;
            return;
        } else if !test_type.is_empty() {
            panic!("Err Unknown test type")
        }
    }
    let grpc_client = GravityQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .unwrap();
    let keys = get_keys();
    let erc20_addresses = contracts.erc20_addresses;

    info!("Starting Happy path test");
    happy_path_test(
        &web30,
        grpc_client,
        &contact,
        keys,
        gravity_address,
        erc20_addresses[0],
        false,
    )
    .await;

    // this checks that the chain is continuing at the end of each test.
    contact
        .wait_for_next_block(TOTAL_TIMEOUT)
        .await
        .expect("Error chain has halted unexpectedly!");
}
