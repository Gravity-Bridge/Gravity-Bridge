use std::{str::FromStr, time::Duration};

use actix::System;
use deep_space::address::Address as CosmosAddress;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use orchestrator::oracle_resync::get_last_checked_block;
use web30::{client::Web3, EthAddress};

fn main() {
    let runner = System::new();
    let web3 = Web3::new(
        "https://blue-floral-dream.bsc.quiknode.pro",
        Duration::from_secs(30),
    );
    let contract_addr = EthAddress::from_str("0xb40C364e70bbD98E8aaab707A41a52A2eAF5733f").unwrap();

    runner.block_on(async move {
        let grpc_client = GravityQueryClient::connect("http://localhost:9090")
            .await
            .unwrap();

        println!("before getting last checked block");
        let result = get_last_checked_block(
            grpc_client,
            "oraib",
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
