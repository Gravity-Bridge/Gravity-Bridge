use crate::altruistic::{
    gas_tracker_loop, get_acceptable_gas_price, get_current_gas_price, get_num_gas_tracker_samples,
    update_gas_history_samples,
};
use crate::ibc_auto_forwarding::ibc_auto_forward_loop;
use crate::request_batches::request_batches;
use crate::{
    batch_relaying::relay_batches, find_latest_valset::find_latest_valset,
    logic_call_relaying::relay_logic_calls, valset_relaying::relay_valsets,
};
use clarity::address::Address as EthAddress;
use clarity::PrivateKey as EthPrivateKey;
use deep_space::{Coin, Contact, CosmosPrivateKey};
use futures::future::join3;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_utils::num_conversion::print_gwei;
use gravity_utils::types::{BatchRelayingMode, RelayerConfig, ValsetRelayingMode};
use std::time::{Duration, Instant};
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use web30::client::Web3;

/// The general network request and operation timeout
pub const TIMEOUT: Duration = Duration::from_secs(10);
/// The Amount of time to wait for an Ethereum transaction submission to enter the chain
pub const ETH_SUBMIT_WAIT_TIME: Duration = Duration::from_secs(600);

/// bundles the relayer_main_loop, ibc_auto_forward_loop, and gas_tracker_loop together into a single future
#[allow(clippy::too_many_arguments)]
pub async fn all_relayer_loops(
    cosmos_key: Option<CosmosPrivateKey>,
    ethereum_key: EthPrivateKey,
    web3: Web3,
    contact: Contact,
    grpc_client: GravityQueryClient<Channel>,
    gravity_contract_address: EthAddress,
    gravity_id: String,
    fee: Option<Coin>,
    config: RelayerConfig,
) {
    if config.gas_tracker_loop_speed > 60u64 {
        panic!(
            "Invalid configured gas_tracker_loop_speed ({}): must be 60 seconds or less",
            config.gas_tracker_loop_speed
        )
    }
    // Update the tracker with the now-known desired number of samples
    update_gas_history_samples(config.altruistic_gas_price_samples as usize);
    debug!("Starting all relayer loops");

    let a = relayer_main_loop(
        ethereum_key,
        cosmos_key,
        fee.clone(),
        web3.clone(),
        contact.clone(),
        grpc_client.clone(),
        gravity_contract_address,
        gravity_id,
        config.clone(),
    );
    let b = ibc_auto_forward_loop(
        cosmos_key,
        &contact,
        grpc_client.clone(),
        fee.clone(),
        config.clone(),
    );
    let c = gas_tracker_loop(&web3, config.clone());

    join3(a, b, c).await;
}

/// This function contains the relayer primary loop, it is broken out of the main loop so that
/// it can be called in the test runner for easier orchestration of multi-node tests
#[allow(clippy::too_many_arguments)]
pub async fn relayer_main_loop(
    ethereum_key: EthPrivateKey,
    cosmos_key: Option<CosmosPrivateKey>,
    cosmos_fee: Option<Coin>,
    web3: Web3,
    contact: Contact,
    grpc_client: GravityQueryClient<Channel>,
    gravity_contract_address: EthAddress,
    gravity_id: String,
    relayer_config: RelayerConfig,
) {
    let grpc_client = grpc_client;

    loop {
        let loop_start = Instant::now();

        // use the gas estimator to determine if we should relay altruistically
        let current_gas_price = get_current_gas_price();
        let ideal_gas =
            get_acceptable_gas_price(relayer_config.altruistic_acceptable_gas_price_percentage);
        debug!(
            "current ethereum gas price: {:?}, ideal gas price: {:?}",
            current_gas_price.map(print_gwei),
            ideal_gas.map(print_gwei)
        );
        let should_relay_altruistic =
            if let (Some(current_price), Some(good_price)) = (current_gas_price, ideal_gas) {
                current_price <= good_price
            } else {
                false
            };

        single_relayer_iteration(
            ethereum_key,
            cosmos_key,
            cosmos_fee.clone(),
            &contact,
            &web3,
            &grpc_client,
            gravity_contract_address,
            &gravity_id,
            &relayer_config,
            should_relay_altruistic,
        )
        .await;

        delay_until_next_iteration(loop_start, relayer_config.relayer_loop_speed).await;
    }
}

/// Performs a single execution of all the main_loop relayer functions:
/// * Batch Requests
/// * Valset Relaying
/// * Batch Relaying
/// * Logic Call Relaying
#[allow(clippy::too_many_arguments)]
pub async fn single_relayer_iteration(
    ethereum_key: EthPrivateKey,
    cosmos_key: Option<CosmosPrivateKey>,
    cosmos_fee: Option<Coin>,
    contact: &Contact,
    web3: &Web3,
    grpc_client: &GravityQueryClient<Channel>,
    gravity_contract_address: EthAddress,
    gravity_id: &str,
    relayer_config: &RelayerConfig,
    should_relay_altruistic: bool,
) {
    let mut grpc_client: GravityQueryClient<Channel> = grpc_client.clone();
    if let (Some(cosmos_key), Some(cosmos_fee)) = (cosmos_key, cosmos_fee.clone()) {
        // Batches are only requested if it is a good time to do so, no checks needed here
        request_batches(
            contact,
            web3,
            &mut grpc_client,
            relayer_config,
            ethereum_key.to_address(),
            cosmos_key,
            cosmos_fee,
        )
        .await
    }

    // we should relay if we're not altruistic or if we are and the gas price is good
    let should_relay_valsets = relayer_config.valset_relaying_mode
        != ValsetRelayingMode::Altruistic
        || should_relay_altruistic;
    let should_relay_batches = relayer_config.batch_relaying_mode != BatchRelayingMode::Altruistic
        || should_relay_altruistic;

    let current_valset = find_latest_valset(&mut grpc_client, gravity_contract_address, web3).await;
    if current_valset.is_err() {
        error!("Could not get current valset! {:?}", current_valset);
        return;
    }
    let current_valset = current_valset.unwrap();

    if should_relay_valsets {
        relay_valsets(
            current_valset.clone(),
            ethereum_key,
            web3,
            &mut grpc_client,
            gravity_contract_address,
            gravity_id.to_string(),
            relayer_config.clone(),
        )
        .await;
    }
    let current_gas_samples = get_num_gas_tracker_samples();
    let delay_altruistic_relayer = relayer_config.batch_relaying_mode
        == BatchRelayingMode::Altruistic
        && current_gas_samples.is_some()
        && current_gas_samples.unwrap()
            < relayer_config.altruistic_batch_relaying_samples_delay as usize;
    if delay_altruistic_relayer {
        info!(
            "Delaying relayer because the gas tracker has not collected {} samples",
            relayer_config.altruistic_batch_relaying_samples_delay
        )
    }
    if should_relay_batches && !delay_altruistic_relayer {
        relay_batches(
            current_valset.clone(),
            ethereum_key,
            web3,
            &mut grpc_client,
            gravity_contract_address,
            gravity_id.to_string(),
            relayer_config.clone(),
        )
        .await;
    }

    relay_logic_calls(
        current_valset,
        ethereum_key,
        web3,
        &mut grpc_client,
        gravity_contract_address,
        gravity_id.to_string(),
        relayer_config.clone(),
    )
    .await;
}

/// a bit of logic that tries to keep things running every relayer_loop_speed seconds exactly
/// do not depend heavily on this being accurate, it will roughly get you what you want
pub async fn delay_until_next_iteration(loop_start: Instant, loop_speed: u64) {
    let elapsed = Instant::now() - loop_start;
    let loop_speed = Duration::from_secs(loop_speed);
    if elapsed < loop_speed {
        delay_for(loop_speed - elapsed).await;
    }
}
