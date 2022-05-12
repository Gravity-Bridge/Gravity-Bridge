// TODO: Remove this file, upstream all changes
//! This file contains a gas estimator struct one that can be generally used in any case where
//! waiting for lower than average gas prices is an advantage.
use clarity::Uint256;
use std::cmp::Ordering;
use std::collections::VecDeque;
use std::iter::FromIterator;
use std::time::Instant;
use web30::client::Web3;

/// internal storage type for the GasTracker struct right now the
/// sample_time is only used for stale identification but it should
/// be generally useful in improving accuracy elsewhere
#[derive(Debug, Clone, PartialEq, Eq)]
struct GasPriceEntry {
    sample_time: Instant,
    sample: Uint256,
}

// implement ord ignoring sample_time
impl Ord for GasPriceEntry {
    fn cmp(&self, other: &Self) -> Ordering {
        let size1 = &self.sample;
        let size2 = &other.sample;
        if size1 > size2 {
            return Ordering::Less;
        }
        if size1 < size2 {
            return Ordering::Greater;
        }
        Ordering::Equal
    }
}

// boilerplate partial ord impl using above Ord
impl PartialOrd for GasPriceEntry {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        match self.sample_time.partial_cmp(&other.sample_time) {
            Some(core::cmp::Ordering::Equal) => {}
            ord => return ord,
        }
        self.sample.partial_cmp(&other.sample)
    }
}

/// A struct for storing gas prices and estimating when it's a good
/// idea to perform some gas intensive operation
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct GasTracker {
    history: VecDeque<GasPriceEntry>,
    size: usize,
}

impl GasTracker {
    /// create a new gas tracker with size
    /// internal sample size and number of samples before which
    /// it will not give an estimate
    pub fn new(size: usize) -> Self {
        GasTracker {
            history: VecDeque::new(),
            size,
        }
    }

    // TODO: Upstream this function!
    // Gets the most recently stored gas price
    pub fn current_gas_price(&self) -> Option<Uint256> {
        self.history.front().map(|price| price.sample.clone())
    }

    /// Gets the latest gas price and adds it to the array if this fails
    /// the sample is skipped, returns a gas price if one is successfully added
    pub async fn update(&mut self, web30: &Web3) -> Option<Uint256> {
        let sample = web30.eth_gas_price().await;
        match sample {
            Ok(sample) => {
                let sample_time = Instant::now();
                let entry = GasPriceEntry {
                    sample_time,
                    sample: sample.clone(),
                };
                match self.history.len().cmp(&self.size) {
                    Ordering::Less => {
                        self.history.push_front(entry);
                        Some(sample)
                    }
                    Ordering::Equal => {
                        //vec is full, remove oldest entry
                        self.history.pop_back();
                        self.history.push_front(entry);
                        Some(sample)
                    }
                    Ordering::Greater => {
                        panic!("Vec size greater than max size, error in GasTracker vecDeque logic")
                    }
                }
            }
            Err(e) => {
                warn!("Failed to update gas price sample with {:?}", e);
                None
            }
        }
    }

    /// Look through all the gas prices in the history range and determine the highest
    /// acceptable price to pay as provided by a user percentage
    pub fn get_acceptable_gas_price(&self, percentage: f32) -> Option<Uint256> {
        // if there are no entries, return that no gas price should currently
        // be taken
        if self.history.is_empty() {
            return None;
        }

        let mut vector: Vec<&GasPriceEntry> = Vec::from_iter(self.history.iter());
        vector.sort();

        // this should never panic as percentage is less than 1 and vector len is
        // included as a factor
        let lowest: usize = (percentage * vector.len() as f32).floor() as usize;
        Some(vector[lowest].sample.clone())
    }
}
