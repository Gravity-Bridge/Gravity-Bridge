use crate::request_batches::request_batches;
use crate::{
    batch_relaying::relay_batches, find_latest_valset::find_latest_valset,
    logic_call_relaying::relay_logic_calls, valset_relaying::relay_valsets,
};
use clarity::address::Address as EthAddress;
use clarity::PrivateKey as EthPrivateKey;
use deep_space::{Coin, Contact, PrivateKey as CosmosPrivateKey};
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_utils::types::RelayerConfig;
use std::time::{Duration, Instant};
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use web30::client::Web3;

pub const TIMEOUT: Duration = Duration::from_secs(10);

/// This function contains the orchestrator primary loop, it is broken out of the main loop so that
/// it can be called in the test runner for easier orchestration of multi-node tests
#[allow(clippy::too_many_arguments)]
pub async fn relayer_main_loop(
    ethereum_key: EthPrivateKey,
    cosmos_key: Option<CosmosPrivateKey>,
    cosmos_fee: Option<Coin>,
    web3: Web3,
    contact: Contact,
    grpc_client: GravityQueryClient<Channel>,
    gravity_contract_address: EthAddress,
    gravity_id: String,
    relayer_config: RelayerConfig,
) {
    let mut grpc_client = grpc_client;
    loop {
        let loop_start = Instant::now();

        let current_valset =
            find_latest_valset(&mut grpc_client, gravity_contract_address, &web3).await;
        if current_valset.is_err() {
            error!("Could not get current valset! {:?}", current_valset);
            continue;
        }
        let current_valset = current_valset.unwrap();

        relay_valsets(
            current_valset.clone(),
            ethereum_key,
            &web3,
            &mut grpc_client,
            gravity_contract_address,
            gravity_id.clone(),
            TIMEOUT,
            relayer_config.clone(),
        )
        .await;

        relay_batches(
            current_valset.clone(),
            ethereum_key,
            &web3,
            &mut grpc_client,
            gravity_contract_address,
            gravity_id.clone(),
            TIMEOUT,
            relayer_config.clone(),
        )
        .await;

        relay_logic_calls(
            current_valset,
            ethereum_key,
            &web3,
            &mut grpc_client,
            gravity_contract_address,
            gravity_id.clone(),
            TIMEOUT,
            relayer_config.clone(),
        )
        .await;

        if let (Some(cosmos_key), Some(cosmos_fee)) = (cosmos_key, cosmos_fee.clone()) {
            request_batches(
                &contact,
                &web3,
                &mut grpc_client,
                relayer_config.batch_request_mode,
                ethereum_key.to_address(),
                cosmos_key,
                cosmos_fee,
            )
            .await
        }

        // a bit of logic that tires to keep things running every relayer_loop_speed seconds exactly
        // this is not required for any specific reason. In fact we expect and plan for
        // the timing being off significantly
        let elapsed = Instant::now() - loop_start;
        let loop_speed = Duration::from_secs(relayer_config.relayer_loop_speed);
        if elapsed < loop_speed {
            delay_for(loop_speed - elapsed).await;
        }
    }
}
