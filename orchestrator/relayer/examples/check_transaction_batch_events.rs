use std::{str::FromStr, time::Duration};

use actix::System;
use clarity::Uint256;
use gravity_utils::types::{
    event_signatures::TRANSACTION_BATCH_EXECUTED_EVENT_SIG, TransactionBatchExecutedEvent,
};

use web30::{client::Web3, EthAddress};

fn main() {
    let runner = System::new();
    let web3 = Web3::new("https://api.trongrid.io/jsonrpc", Duration::from_secs(30));
    let start_block = Uint256::from(49524536u128);
    let end_block = None;
    let contract_addr = EthAddress::from_str("0x2f1e13A482af1cc89553cDFB8BdF999155D13C35").unwrap();

    runner.block_on(async move {
        let val: Vec<TransactionBatchExecutedEvent> = web3
            .parse_event(
                start_block,
                end_block,
                contract_addr,
                TRANSACTION_BATCH_EXECUTED_EVENT_SIG,
            )
            .await
            .unwrap();

        println!("{:?}", val);
    });
}
