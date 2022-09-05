use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::happy_path::send_erc20_deposit;
use crate::utils::*;
use crate::OPERATION_TIMEOUT;
use crate::{
    get_ibc_chain_id,ADDRESS_PREFIX, COSMOS_NODE_GRPC, IBC_ADDRESS_PREFIX, IBC_NODE_GRPC,
    STAKING_TOKEN,
};
use clarity::Address as EthAddress;
use cosmos_gravity::proposals::UPDATE_HRP_IBC_CHANNEL_PROPOSAL;
use cosmos_gravity::send::MSG_EXECUTE_IBC_AUTO_FORWARDS_TYPE_URL;
use deep_space::address::Address as CosmosAddress;
use deep_space::client::msgs::MSG_TRANSFER_TYPE_URL;
use deep_space::error::CosmosGrpcError;
use deep_space::private_key::{CosmosPrivateKey, PrivateKey};
use deep_space::{Coin as DSCoin, Contact, Msg};
use gravity_proto::cosmos_sdk_proto::bech32ibc::bech32ibc::v1::UpdateHrpIbcChannelProposal;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::{
    v1beta1 as Bank, v1beta1::query_client::QueryClient as BankQueryClient,
};
use gravity_proto::cosmos_sdk_proto::cosmos::base::v1beta1::Coin;
use gravity_proto::cosmos_sdk_proto::ibc::applications::transfer::v1::MsgTransfer;
use gravity_proto::cosmos_sdk_proto::ibc::applications::transfer::{
    v1 as IbcTransferV1, v1::query_client::QueryClient as IbcTransferQueryClient,
};
use gravity_proto::cosmos_sdk_proto::ibc::core::connection::v1::query_client::QueryClient as ConnectionQueryClient;
use gravity_proto::cosmos_sdk_proto::ibc::core::connection::v1::QueryConnectionsRequest;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_utils::num_conversion::one_atom;
use num256::Uint256;
use std::cmp::Ordering;
use std::ops::{Add, Mul};
use std::str::FromStr;
use std::time::Instant;
use std::time::{Duration, SystemTime};
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
) {
    let no_relay_market_config = create_default_test_config();
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;

    let ibc_user_keys = get_user_key(Some("cosmos"));

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
            println!("{}{}", connection.id);
            return Ok(connection.id);
        }
    }
    Err(CosmosGrpcError::BadResponse("No such connection".to_string()))
}
