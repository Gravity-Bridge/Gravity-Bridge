use ethabi::ParamType;
use heliosphere::{MethodCall, RpcClient};
use std::{str::FromStr, time::Duration};
use web30::{client::Web3, utils::get_evm_address};

#[actix_rt::main]
pub async fn main() {
    let api = "https://api.trongrid.io";
    let mut web3 = Web3::new(&format!("{}/jsonrpc", api), Duration::from_secs(120));

    web3.set_header("TRON-PRO-API-KEY", option_env!("API_KEY").unwrap());
    web3.set_check_sync(false);

    let tron_usdt: clarity::Address = get_evm_address("TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t")
        .parse()
        .unwrap();

    let caller_address =
        clarity::Address::from_str("0xf2846a1E4dAFaeA38C1660a618277d67605bd2B5").unwrap();

    let ret = web3
        .get_erc20_decimals(tron_usdt, caller_address)
        .await
        .unwrap();

    println!("{:?}", ret);

    let mut client = RpcClient::new(api, Duration::from_secs(120)).unwrap();
    client.set_header("TRON-PRO-API-KEY", option_env!("API_KEY").unwrap());
    let method_call = MethodCall {
        caller: &heliosphere::core::Address::from_str("TY5X9ocQACH9YGAyiK3WUxLcLw3t2ethnc")
            .unwrap(),
        contract: &heliosphere::core::Address::from_str("TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t")
            .unwrap(),
        selector: "decimals()",
        parameter: &ethabi::encode(&[]),
    };

    let ret = &ethabi::decode(
        &[ParamType::Uint(256)],
        &client
            .query_contract(&method_call)
            .await
            .unwrap()
            .constant_result(0)
            .unwrap(),
    )
    .unwrap()[0];
    println!("{:?}", ret);
}
