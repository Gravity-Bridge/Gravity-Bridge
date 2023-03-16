use std::{str::FromStr, time::Duration};

use actix::System;
use clarity::{PrivateKey, Uint256};
use web30::{client::Web3, types::SendTxOption, EthAddress, TronAddress};

fn main() {
    env_logger::Builder::from_env("RUST_LOG").init();

    let web3 = Web3::new("https://nile.trongrid.io/jsonrpc", Duration::from_secs(120));

    let sender_private_key = PrivateKey::from_str(option_env!("PRIVATE_KEY").unwrap()).unwrap();
    let erc20: EthAddress = TronAddress::from_str("TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj")
        .unwrap()
        .into();
    let runner = System::new();

    let recipient: EthAddress = TronAddress::from_str("TZAjoY9H62kHkLkDtMuPc7U86UdqrCT52T")
        .unwrap()
        .into();

    runner.block_on(async move {
        let tx_hash = web3
            .send_transaction(
                erc20,
                "transfer(address,uint256)",
                &[recipient.into(), Uint256::from_str("1000").unwrap().into()],
                0u32.into(),
                sender_private_key.to_address(),
                sender_private_key,
                vec![SendTxOption::GasLimitMultiplier(1.2f32)],
            )
            .await
            .unwrap();
        println!("tx hash: {:?}", format!("{tx_hash:064x}"));
    });
}

// PRIVATE_KEY= cargo run --package relayer --example send_trc20
