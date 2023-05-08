use deep_space::{Contact, CosmosPrivateKey};
use gravity_proto::gravity::query_client::QueryClient;
use tonic::transport::Channel;
use web30::client::Web3;
use clarity::Address as EthAddress;

use crate::{batch_stress::batch_stress_test, batch_timeout::batch_timeout_test, tx_cancel::send_to_eth_and_cancel, send_to_eth_fees::send_to_eth_fees_test, cross_bridge_balances::cross_bridge_balance_test, utils::ValidatorKeys};

pub async fn the_gauntlet(
    web30: &Web3,
    grpc: QueryClient<Channel>,
    contact: &Contact,
    ibc_contact: &Contact,
    keys: Vec<ValidatorKeys>,
    ibc_keys: Vec<CosmosPrivateKey>,
    gravity_address: EthAddress,
    erc20_addresses: Vec<EthAddress>,
    vulnerable_erc20_address: EthAddress,
) {
    batch_stress_test(
        &web30,
        &contact,
        grpc.clone(),
        keys.clone(),
        gravity_address,
        erc20_addresses.clone(),
    )
    .await;
    info!("Starting Batch Timeout/Timeout Stress test");
    batch_timeout_test(
        &web30,
        &contact,
        grpc.clone(),
        keys.clone(),
        gravity_address,
        erc20_addresses.clone(),
    )
    .await;
    info!("Starting SendToEth cancellation test!");
    send_to_eth_and_cancel(
        &contact,
        grpc.clone(),
        &web30,
        keys.clone(),
        gravity_address,
        erc20_addresses[0].clone(),
    )
    .await;
    send_to_eth_fees_test(
        &web30,
        &contact,
        grpc.clone(),
        keys.clone(),
        gravity_address,
        erc20_addresses.clone(),
    )
    .await;
    cross_bridge_balance_test(
        &web30,
        grpc.clone(),
        &contact,
        &ibc_contact,
        keys.clone(),
        ibc_keys.clone(),
        gravity_address,
        erc20_addresses.clone(),
        vulnerable_erc20_address,
    )
    .await;
}