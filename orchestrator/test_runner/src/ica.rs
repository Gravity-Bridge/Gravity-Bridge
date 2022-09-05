use crate::utils::*;
use crate::OPERATION_TIMEOUT;
use crate::{
    COSMOS_NODE_GRPC, IBC_NODE_GRPC,
};
use clarity::Address as EthAddress;
use deep_space::error::CosmosGrpcError;
use deep_space::private_key::{CosmosPrivateKey};
use deep_space::Contact;
use gravity_proto::cosmos_sdk_proto::ibc::core::connection::v1::query_client::QueryClient as ConnectionQueryClient;
use gravity_proto::cosmos_sdk_proto::ibc::core::connection::v1::QueryConnectionsRequest;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use std::time::Instant;
use std::time::Duration;
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use web30::client::Web3;

// Test Interchain accounts host / controller. Create , Send , Delegate
pub async fn ica_test(
    web30: &Web3,
    gravity_client: GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    ibc_keys: Vec<CosmosPrivateKey>,
    gravity_address: EthAddress,
) {
    let no_relay_market_config = create_default_test_config();
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;

    let gravity_connection_qc = ConnectionQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect channel query client");
    let ibc_connection_qc = ConnectionQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect ibc-transfer query client");

    // Wait for the ibc channel to be created and find the connection ids
    let connection_id_timeout = Duration::from_secs(60 * 5);
    let gravity_connection_id = get_connection_id(
        gravity_connection_qc,
        Some(connection_id_timeout),
    )
    .await
    .expect("Could not find gravity-test-1 connection id");
    let gravity_counterparty_connection_id = get_connection_id(
        ibc_connection_qc,
        Some(connection_id_timeout),
    )
    .await
    .expect("Could not find gravity-test-1 counterparty connection id");
    println!("{}{}", gravity_connection_id,gravity_counterparty_connection_id)
}

// Retrieves the channel connecting the chain behind `ibc_channel_qc` and the chain with id `foreign_chain_id`
// Retries up to `timeout` (or OPERATION_TIMEOUT if None), checking each channel's client state to find the foreign chain's id
pub async fn get_connection_id(
    ibc_connection_qc: ConnectionQueryClient<Channel>, // The Src chain's IbcChannelQueryClient                    // The chain-id of the Dst chain
    timeout: Option<Duration>,
) -> Result<String, CosmosGrpcError> {
    let mut ibc_connection_qc = ibc_connection_qc;
    let timeout = match timeout {
        Some(t) => t,
        None => OPERATION_TIMEOUT,
    };

    let start = Instant::now();
    while Instant::now() - start < timeout {
        let connections = ibc_connection_qc
            .connections(QueryConnectionsRequest { pagination: None })
            .await;
        if connections.is_err() {
            delay_for(Duration::from_secs(5)).await;
            continue;
        }
        let connections = connections.unwrap().into_inner().connections;
        for connection in connections {
            println!("{}", connection.id);
            return Ok(connection.id);
        }
    }
    Err(CosmosGrpcError::BadResponse("No such connection".to_string()))
}
