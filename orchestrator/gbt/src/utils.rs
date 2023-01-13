use clarity::address::Address;
use clarity::constants::ZERO_ADDRESS;
use gravity_utils::types::{BatchRequestMode, RelayerConfig, ValsetRelayingMode};
use std::{process::exit, time::Duration};

pub const TIMEOUT: Duration = Duration::from_secs(60);

/// Explains the relaying config to users
pub fn print_relaying_explanation(input: &RelayerConfig, batch_requests: bool) {
    info!("Relaying from Cosmos => Ethereum is enabled, this will cost ETH");
    match input.valset_relaying_mode {
        ValsetRelayingMode::ProfitableOnly {margin} => info!(
            "This relayer will only relay validator set updates if they have a profitable reward with at least {} margin", margin
        ),
        ValsetRelayingMode::Altruistic => info!(
            "This relayer will relay validator set updates altruistically if required by the network"
        ),
        ValsetRelayingMode::EveryValset => warn!(
            "This relayer will relay every validator set update. This will cost a lot of ETH!"
        ),
    }
    match (input.batch_request_mode, batch_requests) {
        (_, false) => info!(
            "This relayer will not automatically request batches because the Graviton private key and fees are not provided",
        ),
        (BatchRequestMode::None, _) => info!(
            "This relayer will not automatically request batches, to enable this modify your configs `batch_request_mode`",
        ),
        (BatchRequestMode::ProfitableOnly, true) => info!(
            "This relayer will automatically spend Graviton tx fees to request the creation of batches that may be profitable",
        ),
        (BatchRequestMode::EveryBatch, true) => info!(
            "This relayer will automatically spend Graviton tx fees to request a batch when any tx are available",
        ),
        (BatchRequestMode::Altruistic, true) => info!(
            "This relayer will automatically spend Graviton tx fees to request a batch during the lowest {}% of gas prices over {} samples", input.altruistic_acceptable_gas_price_percentage * 100.0, input.altruistic_gas_price_samples,
        ),

    }
    match &input.batch_relaying_mode {
        gravity_utils::types::BatchRelayingMode::EveryBatch => info!("This relayer will relay every batch. This will cost a lot of ETH!"),
        gravity_utils::types::BatchRelayingMode::Altruistic => info!("This relayer will relay batches during the lowest {}% of gas prices over {} samples", input.altruistic_acceptable_gas_price_percentage * 100.0, input.altruistic_gas_price_samples),
        gravity_utils::types::BatchRelayingMode::ProfitableOnly { margin } => info!("This relayer will only relay batches if they have a profitable reward with at least {} margin", margin),
        gravity_utils::types::BatchRelayingMode::ProfitableWithWhitelist { margin, whitelist } =>
            info!("This relayer will relay profitable matches with {} margin, and the following tokens with the provided amounts {:?}", margin, whitelist)
    }
}

/// Try parse ethereum address and check empty
pub fn parse_bridge_ethereum_address_with_exit(address: &str) -> Address {
    if let Ok(v) = address.parse() {
        if v != *ZERO_ADDRESS {
            return v;
        }
    }
    error!("The Gravity address is not yet set as a chain parameter! You must specify --gravity-contract-address");
    exit(1)
}
