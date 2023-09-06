use std::{str::FromStr, time::Duration};

use actix::System;
use deep_space::address::Address as CosmosAddress;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use orchestrator::oracle_resync::get_last_checked_block;
use web30::{client::Web3, EthAddress};

use lazy_static::lazy_static;
use std::env;

// Retrieve values from runtime ENV vars
lazy_static! {
    // GRAVITY CHAIN CONSTANTS
    // These constants all apply to the gravity instance running (gravity-test-1)
    static ref COSMOS_NODE_GRPC: String =
        env::var("COSMOS_NODE_GRPC").unwrap_or_else(|_| "http://localhost:9090".to_owned());
    static ref ETH_NODE: String =
        env::var("ETH_NODE").unwrap_or_else(|_| "http://localhost:8545".to_owned());
    static ref EVM_CHAIN_PREFIX: String =
        env::var("EVM_CHAIN_PREFIX").unwrap_or_else(|_| "oraib".to_owned());
    static ref GRAVITY_CONTRACT_ADDR: String =
        env::var("GRAVITY_CONTRACT_ADDR").unwrap_or_else(|_| "0xb40C364e70bbD98E8aaab707A41a52A2eAF5733f".to_owned()); // bsc gravity
}

fn main() {
    let runner = System::new();
    let web3 = Web3::new(ETH_NODE.as_str(), Duration::from_secs(30));
    let contract_addr = EthAddress::from_str(GRAVITY_CONTRACT_ADDR.as_str()).unwrap();

    runner.block_on(async move {
        let grpc_client = GravityQueryClient::connect(COSMOS_NODE_GRPC.as_str())
            .await
            .unwrap();

        println!("before getting last checked block");
        let result = get_last_checked_block(
            grpc_client,
            EVM_CHAIN_PREFIX.as_str(),
            CosmosAddress::from_bech32("oraib16mw6u5624m70apug6fh9a9avezf67y562r0dsn".to_string())
                .unwrap(),
            "oraib".to_string(),
            contract_addr,
            &web3,
        )
        .await;

        println!("{:?}", result);
    });
}
