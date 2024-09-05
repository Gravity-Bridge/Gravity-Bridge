use clarity::Uint256;
use num_traits::ToPrimitive;

const ONE_ETH: u128 = 1000000000000000000;
const ONE_ETH_FLOAT: f64 = ONE_ETH as f64;
pub fn one_eth() -> Uint256 {
    ONE_ETH.into()
}

const ONE_GWEI: u128 = 1000000000;
const ONE_GWEI_FLOAT: f64 = ONE_GWEI as f64;
pub fn one_gwei() -> Uint256 {
    ONE_GWEI.into()
}

const ONE_ATOM: u128 = 1000000;
const ONE_ATOM_FLOAT: f64 = ONE_ATOM as f64;
pub fn one_atom() -> Uint256 {
    ONE_ATOM.into()
}

pub fn downcast_uint256(input: Uint256) -> Option<u64> {
    input.to_u64()
}

pub fn downcast_to_u128(input: Uint256) -> Option<u128> {
    input.to_u128()
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
    let res = float / ONE_ETH_FLOAT;
    format!("{:.4}", res)
}

pub fn print_atom(input: Uint256) -> String {
    let float: f64 = input.to_string().parse().unwrap();
    let res = float / ONE_ATOM_FLOAT;
    format!("{:.4}", res)
}

pub fn print_gwei(input: Uint256) -> String {
    let float: f64 = input.to_string().parse().unwrap();
    let res = float / ONE_GWEI_FLOAT;
    format!("{:}", res)
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

#[test]
fn test_downcast_nonce() {
    let mut i = 0u64;
    while i < 100_000 {
        assert_eq!(i, downcast_uint256(i.into()).unwrap());
        i += 1
    }
    let mut i: u64 = u32::MAX.into();
    i -= 100;
    let end = i + 100_000;
    while i < end {
        assert_eq!(i, downcast_uint256(i.into()).unwrap());
        i += 1
    }
}

#[test]
fn test_downcast_to_u128() {
    let mut i = 0u128;
    while i < 100_000 {
        assert_eq!(i, downcast_to_u128(i.into()).unwrap());
        i += 1
    }
    let mut i: u128 = u64::MAX.into();
    i -= 100;
    let end = i + 100_000;
    while i < end {
        assert_eq!(i, downcast_to_u128(i.into()).unwrap());
        i += 1
    }
}
