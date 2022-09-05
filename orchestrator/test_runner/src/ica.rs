use crate::utils::*;
use crate::OPERATION_TIMEOUT;
use crate::{
    COSMOS_NODE_GRPC, IBC_NODE_GRPC,
};
use deep_space::error::CosmosGrpcError;
use deep_space::PrivateKey;
use deep_space::private_key::CosmosPrivateKey;
use deep_space::Contact;
use gravity_proto::cosmos_sdk_proto::ibc::core::connection::v1::query_client::QueryClient as ConnectionQueryClient;
use gravity_proto::cosmos_sdk_proto::ibc::core::connection::v1::QueryConnectionsRequest;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::msg_client::MsgClient as GravityMsgClient;
use std::time::Instant;
use std::time::Duration;
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use gravity_proto::gravity::{
    QueryInterchainAccountFromAddressRequest, MsgRegisterAccount,
};

/// Test Interchain accounts host / controller. Create , Send , Delegate
/// Plan is 
/// 1. get connection_id , counterparty_connection_id 
/// 2. register interchain account gravity -> ibc 
/// 3. check account registered
/// 4. send some stake tokens
/// 5. delegate 
/// 6. repeat 1-5 for ibc -> gravity
pub async fn ica_test(
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    ibc_keys: Vec<CosmosPrivateKey>,
) {
    let gravity_connection_qc = ConnectionQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect channel query client");
    let ibc_connection_qc = ConnectionQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect ibc-transfer query client");

    // Wait for the ibc channel to be created and find the connection ids
    let connection_id_timeout = Duration::from_secs(60 * 5);
    let connection_id = get_connection_id(
        gravity_connection_qc,
        Some(connection_id_timeout),
    )
    .await
    .expect("Could not find gravity-test-1 connection id");
    let counterparty_connection_id = get_connection_id(
        ibc_connection_qc,
        Some(connection_id_timeout),
    )
    .await
    .expect("Could not find gravity-test-1 counterparty connection id");
    println!(
        "Found valid connections: connection_id: {} counterparty_connection_id {}", 
        connection_id,
        counterparty_connection_id,
    );
    //create gravity&gaia interchain accounts
    let ok = create_interchain_account(
        contact,
        keys.clone(),
        ibc_keys.clone(),
        connection_id.clone(),
        counterparty_connection_id,
    )
    .await;
    if !ok.is_err() {
        println!("Accounts registered");
        let timeout = Duration::from_secs(60 * 5);
        let ccqc = GravityQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect channel query client");
        let counterchain_account = get_interchain_account(
            contact,
            ibc_keys[0],
            ccqc,
            Some(timeout),
            connection_id.clone(),
        )
        .await
        .expect("Account for gravity not created or something went wrong");

        let qc = GravityQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect channel query client");
        let grav_account = get_interchain_account(
            contact,
            keys[0].validator_key,
            qc,
            Some(timeout),
            connection_id.clone(),
        )
        .await
        .expect("Account for gravity not created or something went wrong");

        println!("gravity interchain account: {} , counterchain interchain account: {}",grav_account,counterchain_account)
    }

}

// Retrieves the connecting the chain behind `ibc_channel_qc` and the chain with id `foreign_chain_id`
// Retries up to `timeout` (or OPERATION_TIMEOUT if None)
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
            return Ok(connection.id);
        }
    }
    Err(CosmosGrpcError::BadResponse("No such connection".to_string()))
}

pub async fn create_interchain_account(
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    ibc_keys: Vec<CosmosPrivateKey>,
    connection_id: String,
    counterparty_connection_id: String,
) -> Result<String,CosmosGrpcError> { 
    
    let msg_register_account = MsgRegisterAccount {
        owner: keys[0].validator_key.to_address(&contact.get_prefix()).unwrap().to_string(),
        connection_id,
        version: "".to_string(),
    };
    info!("Submitting MsgRegisterAccount {:?}", msg_register_account);

    let mut grpc_gravity = GravityMsgClient::connect(COSMOS_NODE_GRPC.as_str())
    .await
    .expect("Can't create a channel");
    let msg = grpc_gravity.register_account(msg_register_account)
    .await
    .expect("Could not create interchain account for gravity");
    info!("Sent MsgTransfer with response {:?}", msg);

    let msg_register_account_counter_chain = MsgRegisterAccount {
        owner: ibc_keys[0].to_address(&contact.get_prefix()).unwrap().to_string(),
        connection_id: counterparty_connection_id,
        version: "".to_string(),
    };
    info!("Submitting MsgRegisterAccount {:?}", msg_register_account_counter_chain);

    let mut grpc_gravity = GravityMsgClient::connect(COSMOS_NODE_GRPC.as_str())
    .await
    .expect("Can't create a channel");
    let msg = grpc_gravity.register_account(msg_register_account_counter_chain)
    .await
    .expect("Could not create interchain account for gravity");
    info!("Sent MsgTransfer with response {:?}", msg);

    delay_for(Duration::from_secs(10)).await;
    return Ok("".to_string())
}


pub async fn get_interchain_account(
    contact: &Contact,
    key: CosmosPrivateKey,
    qc: GravityQueryClient<Channel>,
    timeout: Option<Duration>,
    connection_id: String,
) -> Result<String,CosmosGrpcError> { 

    let mut qc = qc
    ;
    let timeout = match timeout {
        Some(t) => t,
        None => OPERATION_TIMEOUT,
    };

    let start = Instant::now();
    while Instant::now() - start < timeout {
        let account = qc
            .interchain_account_from_address(
                QueryInterchainAccountFromAddressRequest { 
                    owner: key.to_address(&contact.get_prefix()).unwrap().to_string(),
                    connection_id: connection_id.clone(), 
                }
            )
            .await;
        if account.is_err() {
            delay_for(Duration::from_secs(5)).await;
            continue;
        }
        let account = account.unwrap().into_inner().interchain_account_address;
        return Ok(account);
    }
    Err(CosmosGrpcError::BadResponse("No such connection".to_string()))
}