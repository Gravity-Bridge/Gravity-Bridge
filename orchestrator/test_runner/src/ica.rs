use crate::utils::*;
use crate::OPERATION_TIMEOUT;
use crate::{
    COSMOS_NODE_GRPC, IBC_NODE_GRPC, IBC_ADDRESS_PREFIX,
};
use deep_space::error::CosmosGrpcError;
use deep_space::PrivateKey;
use deep_space::private_key::CosmosPrivateKey;
use deep_space::{Msg,Contact};
use gravity_proto::cosmos_sdk_proto::ibc::core::connection::v1::query_client::QueryClient as ConnectionQueryClient;
use gravity_proto::cosmos_sdk_proto::ibc::core::connection::v1::QueryConnectionsRequest;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use std::time::Instant;
use std::time::Duration;
use cosmos_gravity::send::TIMEOUT;
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use gravity_proto::gravity::{
    QueryInterchainAccountFromAddressRequest, MsgRegisterAccount,
};
use gravity_utils::connection_prep::create_rpc_connections;


pub const MSG_REGISTER_INTERCHAIN_ACCOUNT_URL: &str = "/icaauth.v1.MsgRegisterAccount";

/// Test Interchain accounts host / controller. Create , Send , Delegate
/// Plan is 
/// 1. get connection_id , counterparty_connection_id:  <fn get_connection_id>
/// 2. register interchain account gravity -> ibc: <fn create_interchain_account>
/// 3. check account registered: <fn get_interchain_account>
/// 4. send some stake tokens
/// 5. delegate 
/// 6. repeat 1-5 for ibc -> gravity
pub async fn ica_test(
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    ibc_keys: Vec<CosmosPrivateKey>,
) {
    // Create connection query clients
    let gravity_connection_qc = ConnectionQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect channel query client");
    let ibc_connection_qc = ConnectionQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect counterparty channel query client");
    
    // Wait for the connections to be created and find the connection ids
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
    info!(
        "Found valid connections: connection_id: {} counterparty_connection_id {}", 
        connection_id,
        counterparty_connection_id,
    );
    info!("Waiting 100 seconds for ConOpenConfirm before account create");
    delay_for(Duration::from_secs(100)).await;
    // create GRPC contact for counterparty chain
    let connections =
    create_rpc_connections(IBC_ADDRESS_PREFIX.clone(), Some(IBC_NODE_GRPC.to_string()), None, TIMEOUT).await;
    let counter_chain_contact = connections.contact.unwrap();

    //create gravity, counterparty interchain accounts
    let ok = create_interchain_account(
        contact,
        &counter_chain_contact,
        keys.clone(),
        ibc_keys.clone(),
        connection_id.clone(),
        counterparty_connection_id,
    )
    .await;
    if !ok.is_err() {
        info!("Accounts registered");
        let timeout = Duration::from_secs(60 * 5);

        // gravity query client
        let qc = GravityQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect channel query client");

        // send tx
        let grav_account = get_interchain_account(
            contact,
            keys[0].validator_key,
            qc,
            Some(timeout),
            connection_id.clone(),
        )
        .await
        .expect("Account for gravity not created or something went wrong");

        // counterparty query client
        let ccqc = GravityQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect counterparty channel query client");

        // send tx
        let counterchain_account = get_interchain_account(
            &counter_chain_contact,
            ibc_keys[0],
            ccqc,
            Some(timeout),
            connection_id.clone(),
        )
        .await
        .expect("Account for counterparty chain not created or something went wrong");

        info!("gravity interchain account: {} , counterchain interchain account: {}",
        grav_account, 
        counterchain_account,
        )
    }

}

// Get connection for both chains
pub async fn get_connection_id(
    ibc_connection_qc: ConnectionQueryClient<Channel>,
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

// Create interchain accounts
pub async fn create_interchain_account(
    contact: &Contact,
    counter_chain_contact: &Contact,
    keys: Vec<ValidatorKeys>,
    ibc_keys: Vec<CosmosPrivateKey>,
    connection_id: String,
    counterparty_connection_id: String,
) -> Result<String,CosmosGrpcError> { 
    
    // chain chain register
    let msg_register_account = MsgRegisterAccount {
        owner: keys[0].validator_key.to_address(&contact.get_prefix()).unwrap().to_string(),
        connection_id,
        version: "".to_string(),
    };
    info!("Submitting MsgRegisterAccount to gravity chain {:?}", msg_register_account);

    let msg_register_account = Msg::new(MSG_REGISTER_INTERCHAIN_ACCOUNT_URL, msg_register_account);
    let send_res = contact
        .send_message(
            &[msg_register_account],
            Some("Test Creating interchain account".to_string()),
            &[],
            Some(OPERATION_TIMEOUT),
            keys[0].validator_key,
        )
        .await;
    info!("Sent MsgRegisterAccount with response {:?}", send_res);

    // counterparty chain register
    let msg_register_account_counter_chain = MsgRegisterAccount {
        owner: ibc_keys[0].to_address(&counter_chain_contact.get_prefix()).unwrap().to_string(),
        connection_id: counterparty_connection_id,
        version: "".to_string(),
    };
    info!("Submitting MsgRegisterAccount to counterparty chain {:?}", msg_register_account_counter_chain);

    let msg_register_account_counter_chain = Msg::new(MSG_REGISTER_INTERCHAIN_ACCOUNT_URL, msg_register_account_counter_chain);
    let send_res = counter_chain_contact
        .send_message(
            &[msg_register_account_counter_chain],
            Some("Test Creating interchain account".to_string()),
            &[],
            Some(OPERATION_TIMEOUT),
            ibc_keys[0],
        )
        .await;
    info!("Sent MsgRegisterAccount with response {:?}", send_res);
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
        info!("{:?}",account);
        if account.is_err() {
            delay_for(Duration::from_secs(5)).await;
            continue;
        }

        let account = account.unwrap().into_inner().interchain_account_address;
        return Ok(account);
    }
    Err(CosmosGrpcError::BadResponse("Can't get interchain account".to_string()))
}