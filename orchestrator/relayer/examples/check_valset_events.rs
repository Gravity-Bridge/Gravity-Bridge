use std::{str::FromStr, time::Duration};

use actix::System;
use gravity_utils::types::{
    event_signatures::VALSET_UPDATED_EVENT_SIG, EthereumEvent, ValsetUpdatedEvent,
};
use log::info;
use web30::{client::Web3, TronAddress};

fn main() {
    env_logger::Builder::from_env("RUST_LOG").init();
    let web3 = Web3::new("https://nile.trongrid.io/jsonrpc", Duration::from_secs(120));
    let gravity_contract_address = TronAddress::from_str("TBGmjkUMQszHisGp99WNecYd2ZwzkSFapX")
        .unwrap()
        .into();

    let runner = System::new();
    let starting_block = 0u8.into();
    runner.block_on(async move {
        let latest_block = web3.eth_block_number().await.unwrap();

        let valsets = web3
            .check_for_events(
                starting_block,
                Some(latest_block),
                vec![gravity_contract_address],
                vec![VALSET_UPDATED_EVENT_SIG],
            )
            .await
            .unwrap();
        let valsets = ValsetUpdatedEvent::from_logs(&valsets).unwrap();
        info!("parsed valsets {:?}", valsets);
    });
}
