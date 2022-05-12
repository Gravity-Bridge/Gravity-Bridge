use crate::ibc_auto_forwarding::ibc_auto_forward_loop;
use crate::temporary_gas_tracker::GasTracker;
use crate::{
    batch_relaying::relay_batches, find_latest_valset::find_latest_valset,
    logic_call_relaying::relay_logic_calls, request_batches::batch_request_loop,
    valset_relaying::relay_valsets,
};
use clarity::address::Address as EthAddress;
use clarity::PrivateKey as EthPrivateKey;
use clarity::Uint256;
use deep_space::{Coin, Contact, PrivateKey as CosmosPrivateKey};
use futures::future::{join, join3};
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_utils::types::{BatchRelayingMode, RelayerConfig, ValsetRelayingMode};
use std::sync::{Arc, RwLock};
use std::time::{Duration, Instant};
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use web30::client::Web3;

/// The general network request and operation timeout
pub const TIMEOUT: Duration = Duration::from_secs(10);
/// The Amount of time to wait for an Ethereum transaction submission to enter the chain
pub const ETH_SUBMIT_WAIT_TIME: Duration = Duration::from_secs(600);

// Altruistic relaying is a mode for relayers that tries to minimize the gas price on
// the donor, while providing maximum utility to the blockchain. Other modes are profitable only
// and just to always relay everything which is mostly used for tests.

/// If we are relaying in altruistic mode only relay during the lowest 1% of gas prices
/// in ALTRUISTIC_SAMPLES
pub const ALTRUISTIC_GAS_PERCENTAGE: f32 = 0.01;
/// The number of samples over which ALTRUISTIC_GAS_PERCENTAGE is computed, note this is determined
/// by the relay loop time.
pub const ALTRUISTIC_SAMPLES: usize = 2000;

lazy_static! {
    static ref GAS_TRACKER: Arc<RwLock<GasTracker>> =
        Arc::new(RwLock::new(GasTracker::new(ALTRUISTIC_SAMPLES)));
}

pub fn get_acceptable_gas_price() -> Option<Uint256> {
    GAS_TRACKER
        .read()
        .unwrap()
        .get_acceptable_gas_price(ALTRUISTIC_GAS_PERCENTAGE)
}

pub fn get_current_gas_price() -> Option<Uint256> {
    GAS_TRACKER.read().unwrap().current_gas_price()
}

pub async fn update_gas_tracker(web3: &Web3) -> Option<Uint256> {
    GAS_TRACKER.write().unwrap().update(web3).await
}

/// bundles the batch_request_loop, relayer_main_loop, and ibc_auto_forward_loop into a single future
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
    let a = relayer_main_loop(
        ethereum_key,
        web3.clone(),
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
    if let (Some(key), Some(cosmos_fee)) = (cosmos_key, fee) {
        let c = batch_request_loop(
            contact.clone(),
            &web3,
            grpc_client.clone(),
            config.clone(),
            ethereum_key,
            key,
            cosmos_fee.clone(),
        );

        join3(a, b, c).await;
    } else {
        join(a, b).await;
    }
}

/// This function contains the relayer primary loop, it is broken out of the main loop so that
/// it can be called in the test runner for easier orchestration of multi-node tests
#[allow(clippy::too_many_arguments)]
pub async fn relayer_main_loop(
    ethereum_key: EthPrivateKey,
    web3: Web3,
    grpc_client: GravityQueryClient<Channel>,
    gravity_contract_address: EthAddress,
    gravity_id: String,
    relayer_config: RelayerConfig,
) {
    let mut grpc_client = grpc_client;

    // Separately the batch_request_loop should be running immediately, we delay this main loop in order to
    // prevent newly requested batches from being temporarily unrelayable. Batch requests take 20-25 seconds to process,
    // meaning we could miss new ones, taking 10+ minutes to relay it
    delay_for(Duration::from_secs(
        relayer_config.batch_request_relay_offset,
    ))
    .await;

    loop {
        let loop_start = Instant::now();

        // update the gas estimator and determine if we should relay altruistically
        let current_gas_price = update_gas_tracker(&web3).await;
        let ideal_gas = get_acceptable_gas_price();
        let should_relay_altruistic =
            if let (Some(current_price), Some(good_price)) = (current_gas_price, ideal_gas) {
                current_price <= good_price
            } else {
                false
            };

        // we should relay if we're not altruistic or if we are and the gas price is good
        let should_relay_valsets = relayer_config.valset_relaying_mode
            != ValsetRelayingMode::Altruistic
            || should_relay_altruistic;
        let should_relay_batches = relayer_config.batch_relaying_mode
            != BatchRelayingMode::Altruistic
            || should_relay_altruistic;

        let current_valset =
            find_latest_valset(&mut grpc_client, gravity_contract_address, &web3).await;
        if current_valset.is_err() {
            error!("Could not get current valset! {:?}", current_valset);
            continue;
        }
        let current_valset = current_valset.unwrap();

        if should_relay_valsets {
            relay_valsets(
                current_valset.clone(),
                ethereum_key,
                &web3,
                &mut grpc_client,
                gravity_contract_address,
                gravity_id.clone(),
                relayer_config.clone(),
            )
            .await;
        }
        if should_relay_batches {
            relay_batches(
                current_valset.clone(),
                ethereum_key,
                &web3,
                &mut grpc_client,
                gravity_contract_address,
                gravity_id.clone(),
                relayer_config.clone(),
            )
            .await;
        }

        relay_logic_calls(
            current_valset,
            ethereum_key,
            &web3,
            &mut grpc_client,
            gravity_contract_address,
            gravity_id.clone(),
            relayer_config.clone(),
        )
        .await;

        // a bit of logic that tries to keep things running every relayer_loop_speed seconds exactly
        // this is not required for any specific reason. In fact we expect and plan for
        // the timing being off significantly
        let elapsed = Instant::now() - loop_start;
        let loop_speed = Duration::from_secs(relayer_config.relayer_loop_speed);
        if elapsed < loop_speed {
            delay_for(loop_speed - elapsed).await;
        }
    }
}
