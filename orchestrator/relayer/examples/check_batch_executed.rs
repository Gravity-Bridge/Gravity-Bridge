use std::{str::FromStr, time::Duration};

use actix::System;
use clarity::Uint256;
use gravity_utils::types::event_signatures::TRANSACTION_BATCH_EXECUTED_EVENT_SIG;
use web30::{client::Web3, EthAddress, Web3Event};

use lazy_static::lazy_static;
use std::env;

// Retrieve values from runtime ENV vars
lazy_static! {
    // GRAVITY CHAIN CONSTANTS
    // These constants all apply to the gravity instance running (gravity-test-1)
    static ref COSMOS_NODE_GRPC: String =
        env::var("COSMOS_NODE_GRPC").unwrap_or_else(|_| "http://localhost:9090".to_owned());
    static ref ETH_NODE: String =
        env::var("ETH_NODE").unwrap_or_else(|_| "http://local
        host:8545".to_owned());
    static ref EVM_CHAIN_PREFIX: String =
        env::var("EVM_CHAIN_PREFIX").unwrap_or_else(|_| "oraib".to_owned());
    static ref GRAVITY_CONTRACT_ADDR: String =
        env::var("GRAVITY_CONTRACT_ADDR").unwrap_or_else(|_| "0xb40C364e70bbD98E8aaab707A41a52A2eAF5733f".to_owned()); // bsc gravity

        static ref START_BLOCK: String =
        env::var("START_BLOCK").unwrap_or_else(|_| "1".to_owned());
    static ref END_BLOCK: String =
        env::var("END_BLOCK").unwrap_or_else(|_| "1".to_owned()); // bsc gravity
}

fn main() {
    let runner = System::new();
    let web3 = Web3::new(ETH_NODE.as_str(), Duration::from_secs(30));
    let contract_addr = EthAddress::from_str(GRAVITY_CONTRACT_ADDR.as_str()).unwrap();

    runner.block_on(async move {
        let _web3_event = match web3
            .check_for_event(
                Uint256::from_str(START_BLOCK.as_str()).unwrap(),
                Some(Uint256::from_str(END_BLOCK.as_str()).unwrap()),
                contract_addr,
                TRANSACTION_BATCH_EXECUTED_EVENT_SIG,
            )
            .await
        {
            Err(e) => {
                print!("error web3 event: {:?}", e);
            }
            Ok(events) => match &events {
                Web3Event::Logs(logs) => println!("event logsss: {:?}", logs.clone()),
                Web3Event::Events(evs) => println!("events: {:?}", evs.clone()),
            },
        };
    });
}
