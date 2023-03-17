use std::{convert::TryInto, str::FromStr, time::Duration};

use actix::System;
use clarity::abi::encode_call;
use web30::{client::Web3, EthAddress, TronAddress};

fn main() {
    let web3 = Web3::new("https://nile.trongrid.io/jsonrpc", Duration::from_secs(120));
    let tron_addr = TronAddress::from_str("TMjswVjeapQ73yZUrZbHq3rPAHJoexMcZy").unwrap();
    let caller_address =
        EthAddress::from_str("0x993d06FC97F45f16e4805883b98a6c20BAb54964").unwrap();
    let payload = encode_call("getAdminAddress()", &[]).unwrap();

    let runner = System::new();

    runner.block_on(async move {
        let val = web3
            .simulate_transaction(tron_addr.into(), 0u8.into(), payload, caller_address, None)
            .await
            .unwrap();

        // uint256 => 32 bytes, get last 20 byte
        let admin_addr = EthAddress::from_slice(val[12..].try_into().unwrap()).unwrap();

        let admin_tron_addr: TronAddress = admin_addr.into();

        assert_eq!(
            TronAddress::from_str("TVf8hwYiMa91Pd5y424xz4ybMhbqDTVN4B").unwrap(),
            admin_tron_addr
        );

        let last_event_nonce =
            ethereum_gravity::utils::get_event_nonce(tron_addr.into(), caller_address, &web3)
                .await
                .unwrap();

        println!("last_event_nonce: {}", last_event_nonce);
    })
}
