/// Contains logic specific to altruistic relaying, including gas tracking
use clarity::Uint256;
use gravity_utils::num_conversion::print_gwei;
use gravity_utils::types::RelayerConfig;
use std::sync::{Arc, RwLock};
use std::time::Instant;
use web30::client::Web3;
use web30::gas_estimator::GasTracker;

use crate::main_loop::delay_until_next_iteration;

// Altruistic relaying is a mode for relayers that tries to minimize the gas price on
// the donor, while providing maximum utility to the blockchain. Other modes are profitable only
// and just to always relay everything which is mostly used for tests.

lazy_static! {
    // Define a gas tracker with a small sample size, the size can be increased later as needed
    static ref GAS_TRACKER: Arc<RwLock<GasTracker>> = Arc::new(RwLock::new(GasTracker::new(1)));
}

/// writes a new gas price entry into the tracker with the current ethereum gas price
/// WARNING: only the gas_tracker_loop should call this to track gas uniformly across time.
/// Adjust relayer_config.gas_tracker_loop_speed and ALTRUISTIC_SAMPLES to control the tracker
/// panics if the GAS_TRACKER lock is currently held by another thread or is poisoned
async fn update_gas_tracker(web3: &Web3) -> Option<Uint256> {
    let sample = GasTracker::sample(web3).await;
    match sample {
        None => {
            warn!("Failed to update gas price sample");
            None
        }
        Some(price) => {
            GAS_TRACKER.write().unwrap().update(price.clone());
            Some(price.sample)
        }
    }
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

pub fn get_num_gas_tracker_samples() -> Option<usize> {
    let tracker = GAS_TRACKER.try_read();
    match tracker {
        Err(_) => None,
        Ok(gas_tracker) => Some(gas_tracker.get_current_size()),
    }
}

/// fetches the last recorded gas price (accurate within relayer_config.gas_tracker_loop_speed seconds) in the
/// gas tracker's store. Compare against get_acceptable_gas_price() to determine if this is a relatively low gas price
pub fn get_current_gas_price() -> Option<Uint256> {
    let tracker = GAS_TRACKER.try_read();
    match tracker {
        Err(_) => None,
        Ok(gas_tracker) => gas_tracker.latest_gas_price(),
    }
}

/// updates the gas tracker's number of samples stored, panics if the input is smaller than the current tracker's size
/// ideally this function will only be called once from a single thread
/// panics if the GAS_TRACKER lock is currently held by another thread or is poisoned
pub fn update_gas_history_samples(size: usize) {
    GAS_TRACKER.write().unwrap().expand_history_size(size)
}

/// continually updates the gas tracker with a new gas price entry to enable altruistic batch requests and batch relaying
pub async fn gas_tracker_loop(web3: &Web3, relayer_config: RelayerConfig) {
    loop {
        let loop_start = Instant::now();

        let current = update_gas_tracker(web3).await;
        debug!("Updated gas price history {:?}", current.map(print_gwei),);

        delay_until_next_iteration(loop_start, relayer_config.gas_tracker_loop_speed).await;
    }
}
