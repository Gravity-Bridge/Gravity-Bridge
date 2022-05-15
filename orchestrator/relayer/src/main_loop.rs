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
use futures::future::{join3, join4};
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_utils::num_conversion::print_gwei;
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

lazy_static! {
    // Define a gas tracker with a small sample size, the size can be increased later as needed
    static ref GAS_TRACKER: Arc<RwLock<GasTracker>> = Arc::new(RwLock::new(GasTracker::new(1)));
    // Tracks the relayer_main_loop's starting point, used to delay batch requests so the gas tracker can store some data
    static ref RELAYER_START: Instant = Instant::now();
}

/// fetches the lowest ALTRUISTIC_GAS_PERCENTAGE % of gas prices in the gas tracker's store, use in conjunction with
/// get_current_gas_price() to determine if now is an acceptable time to relay altruistically
pub fn get_acceptable_gas_price(percentage: f32) -> Option<Uint256> {
    let tracker = GAS_TRACKER.try_read();
    match tracker {
        Err(_) => None,
        Ok(gas_tracker) => gas_tracker.get_acceptable_gas_price(percentage),
    }
}

/// fetches the last recorded gas price (accurate within relayer_config.gas_tracker_loop_speed seconds) in the
/// gas tracker's store. Compare against get_acceptable_gas_price() to determine if this is a relatively low gas price
pub fn get_current_gas_price() -> Option<Uint256> {
    let tracker = GAS_TRACKER.try_read();
    match tracker {
        Err(_) => None,
        Ok(gas_tracker) => gas_tracker.current_gas_price(),
    }
}

/// writes a new gas price entry into the tracker with the current ethereum gas price
/// WARNING: only the gas_tracker_loop should call this to track gas uniformly across time.
/// Adjust relayer_config.gas_tracker_loop_speed and ALTRUISTIC_SAMPLES to control the tracker
/// panics if the GAS_TRACKER lock is currently held by another thread or is poisoned
pub async fn update_gas_tracker(web3: &Web3) -> Option<Uint256> {
    GAS_TRACKER.write().unwrap().update(web3).await
}

/// updates the gas tracker's number of samples stored, panics if the input is smaller than the current tracker's size
/// ideally this function will only be called once from a single thread
/// panics if the GAS_TRACKER lock is currently held by another thread or is poisoned
pub fn update_gas_history_samples(size: usize) {
    GAS_TRACKER
        .write()
        .unwrap()
        .expand_history_size(size)
        .unwrap();
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
    // Update the tracker with the now-known desired number of samples
    update_gas_history_samples(config.altruistic_gas_price_samples as usize);
    debug!("Starting all relayer loops");

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
    let c = gas_tracker_loop(&web3, config.clone());

    if let (Some(key), Some(cosmos_fee)) = (cosmos_key, fee) {
        let d = batch_request_loop(
            contact.clone(),
            &web3,
            grpc_client.clone(),
            config.clone(),
            ethereum_key,
            key,
            cosmos_fee.clone(),
        );

        join4(a, b, c, d).await;
    } else {
        join3(a, b, c).await;
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
    {
        // Initialize RELAYER_START via the very first deref
        let start = *RELAYER_START;
        debug!("Relayer Started at {:?}", start);
    }
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

        // use the gas estimator to determine if we should relay altruistically
        // TODO: Check to see when the gas price was sampled? Only a significant issue if the gas tracker loop speed is large
        let current_gas_price = get_current_gas_price();
        let ideal_gas =
            get_acceptable_gas_price(relayer_config.altruistic_acceptable_gas_price_percentage);
        debug!(
            "Relayer main loop: current gas {:?}, ideal gas {:?}",
            current_gas_price.clone().map(print_gwei),
            ideal_gas.clone().map(print_gwei)
        );
        let should_relay_altruistic = if let (Some(current_price), Some(good_price)) =
            (current_gas_price.clone(), ideal_gas.clone())
        {
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
        let delay_altruistic_relayer = relayer_config.batch_relaying_mode
            == BatchRelayingMode::Altruistic
            && Instant::now() - *RELAYER_START
                < Duration::from_secs(relayer_config.altruistic_batch_relaying_delay);
        if delay_altruistic_relayer {
            info!(
                "Delaying relayer because {} seconds have not elapsed",
                relayer_config.altruistic_batch_relaying_delay
            )
        }
        if should_relay_batches && !delay_altruistic_relayer {
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

        delay_until_next_iteration(loop_start, relayer_config.relayer_loop_speed).await;
    }
}

/// continually updates the gas tracker with a new gas price entry to enable altruistic batch requests and batch relaying
pub async fn gas_tracker_loop(web3: &Web3, relayer_config: RelayerConfig) {
    loop {
        let loop_start = Instant::now();

        let current = update_gas_tracker(web3).await;
        debug!(
            "Gas Tracker: Updated gas price history {:?}",
            current.clone().map(print_gwei),
        );

        delay_until_next_iteration(loop_start, relayer_config.gas_tracker_loop_speed).await;
    }
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
