use clarity::Uint256;
use gravity_utils::types::{BatchRequestMode, RelayerConfig, ValsetRelayingMode};
use std::time::Duration;

pub const TIMEOUT: Duration = Duration::from_secs(60);

pub fn one_eth() -> f64 {
    1000000000000000000f64
}

pub fn one_atom() -> f64 {
    1000000f64
}

/// TODO revisit this for higher precision while
/// still representing the number to the user as a float
/// this takes a number like 0.37 eth and turns it into wei
/// or any erc20 with arbitrary decimals
pub fn fraction_to_exponent(num: f64, exponent: u8) -> Uint256 {
    let mut res = num;
    // in order to avoid floating point rounding issues we
    // multiply only by 10 each time. this reduces the rounding
    // errors enough to be ignored
    for _ in 0..exponent {
        res *= 10f64
    }
    (res as u128).into()
}

pub fn print_eth(input: Uint256) -> String {
    let float: f64 = input.to_string().parse().unwrap();
    let res = float / one_eth();
    format!("{}", res)
}

pub fn print_atom(input: Uint256) -> String {
    let float: f64 = input.to_string().parse().unwrap();
    let res = float / one_atom();
    format!("{}", res)
}

#[test]
fn even_f32_rounding() {
    let one_eth: Uint256 = 1000000000000000000u128.into();
    let one_point_five_eth: Uint256 = 1500000000000000000u128.into();
    let one_point_one_five_eth: Uint256 = 1150000000000000000u128.into();
    let a_high_precision_number: Uint256 = 1150100000000000000u128.into();
    let res = fraction_to_exponent(1f64, 18);
    assert_eq!(one_eth, res);
    let res = fraction_to_exponent(1.5f64, 18);
    assert_eq!(one_point_five_eth, res);
    let res = fraction_to_exponent(1.15f64, 18);
    assert_eq!(one_point_one_five_eth, res);
    let res = fraction_to_exponent(1.1501f64, 18);
    assert_eq!(a_high_precision_number, res);
}

/// Explains the relaying config to users
pub fn print_relaying_explanation(input: &RelayerConfig, batch_requests: bool) {
    info!("Relaying from Cosmos => Ethereum is enabled, this will cost ETH");
    match input.valset_relaying_mode {
        ValsetRelayingMode::ProfitableOnly => info!(
            "This relayer will only relay validator set updates if they have a profitable reward"
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
    }
    match input.batch_market_enabled {
        true => info!("This relayer will only relay batches to Ethereum if the rewards have a greater value in weth than the gas cost"),
        false => warn!("This relayer will relay any batch. This will cost a lof of ETH!")
    }
}
