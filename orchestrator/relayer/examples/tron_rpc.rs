use std::time::Duration;
use web30::client::Web3;

#[actix_rt::main]
pub async fn main() {
    let mut web3 = Web3::from_headers(
        "https://api.trongrid.io/jsonrpc",
        Duration::from_secs(120),
        vec![("TRON-PRO-API-KEY", option_env!("API_KEY").unwrap())],
    );

    web3.set_check_sync(false);

    let ret = web3
        .eth_get_balance(
            "0xf2846a1e4dafaea38c1660a618277d67605bd2b5"
                .parse()
                .unwrap(),
        )
        .await;

    println!("{:?}", ret);
}
