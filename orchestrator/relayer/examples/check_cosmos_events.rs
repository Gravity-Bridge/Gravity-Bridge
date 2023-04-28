use std::{str::FromStr, time::Duration};

use actix::System;
use clarity::Uint256;
use gravity_utils::types::{event_signatures::SENT_TO_COSMOS_EVENT_SIG, SendToCosmosEvent};
use web30::{client::Web3, EthAddress};

fn main() {
    let runner = System::new();
    let web3 = Web3::new("https://api.trongrid.io/jsonrpc", Duration::from_secs(30));
    let start_block = Uint256::from(49524536u128);
    let end_block = None; //Some(Uint256::from(49529536u128));
    let contract_addr = EthAddress::from_str("0x73Ddc880916021EFC4754Cb42B53db6EAB1f9D64").unwrap();

    runner.block_on(async move {
        let val: Vec<SendToCosmosEvent> = web3
            .parse_event(
                start_block,
                end_block,
                contract_addr,
                SENT_TO_COSMOS_EVENT_SIG,
            )
            .await
            .unwrap();

        println!("{:?}", val);
    });
}
