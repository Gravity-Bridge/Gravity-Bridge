use crate::utils::*;
use crate::OPERATION_TIMEOUT;
use crate::{
    COSMOS_NODE_GRPC, IBC_NODE_GRPC, IBC_ADDRESS_PREFIX, STAKING_TOKEN, ADDRESS_PREFIX,
};
use deep_space::error::CosmosGrpcError;
use deep_space::PrivateKey;
use deep_space::private_key::CosmosPrivateKey;
use deep_space::{Msg,Contact};
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::QueryAllBalancesRequest;
use gravity_proto::cosmos_sdk_proto::ibc::core::connection::v1::query_client::QueryClient as ConnectionQueryClient;
use gravity_proto::cosmos_sdk_proto::ibc::core::connection::v1::QueryConnectionsRequest;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::MsgSend;
use gravity_proto::cosmos_sdk_proto::cosmos::staking::v1beta1::MsgDelegate;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::query_client::QueryClient as CosmosQueryClient;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::{
    QueryInterchainAccountFromAddressRequest, MsgRegisterAccount, MsgSubmitTx,
};
use gravity_proto::cosmos_sdk_proto::cosmos::base::v1beta1::Coin;
use gravity_utils::connection_prep::create_rpc_connections;
use std::time::Instant;
use std::time::Duration;
use cosmos_gravity::send::TIMEOUT;
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use anyhow::{Context, Result};
use prost::Message;
use prost_types::Any;

pub const MSG_REGISTER_INTERCHAIN_ACCOUNT_URL: &str = "/icaauth.v1.MsgRegisterAccount";
pub const MSG_SEND_TOKENS_URL: &str = "/cosmos.bank.v1beta1.MsgSend";
pub const MSG_SUBMIT_TX_URL: &str = "/icaauth.v1.MsgSubmitTx";
pub const STAKIN_DELEGATE_TYPE_URL: &str = "/cosmos.staking.v1beta1.MsgDelegate";

// Trait to serialize/deserialize types to and from `prost_types::Any`
pub trait AnyConvert: Sized {
    // Deserialize value from `prost_types::Any`
    fn from_any(value: &Any) -> Result<Self>;

    // Serialize value to `prost_types::Any`
    fn to_any(&self) -> Result<Any>;
}

// Encodes a message into protobuf
pub fn proto_encode<M: Message>(message: &M) -> Result<Vec<u8>> {
    let mut buf = Vec::with_capacity(message.encoded_len());
    message
        .encode(&mut buf)
        .context("unable to encode protobuf message")?;
    Ok(buf)
}
    
impl AnyConvert for MsgDelegate {
        fn from_any(value: &::prost_types::Any) -> ::anyhow::Result<Self> {
            ::anyhow::ensure!(
            value.type_url == STAKIN_DELEGATE_TYPE_URL,
            "invalid type url for `Any` type: expected `{}` and found `{}`",
            STAKIN_DELEGATE_TYPE_URL,
            value.type_url
        );

        <Self as ::prost::Message>::decode(value.value.as_slice()).map_err(Into::into)
    }

    fn to_any(&self) -> ::anyhow::Result<::prost_types::Any> {
        Ok(::prost_types::Any {
            type_url: STAKIN_DELEGATE_TYPE_URL.to_owned(),
            value: proto_encode(self)?,
        })
    }
}

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
        counterparty_connection_id.clone(),
    )
    .await;
    if ok.is_err() {}

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
    );
    
    // send in gravity chain
    let ok = send_tokens_to_interchain_account(
        contact,
        keys[0].validator_key,
        counterchain_account.clone(),
    )
    .await;
    if ok.is_err() {
        error!("Gravity chain response error {:?}",ok.err())
    };
    info!("Tokens sent!");

    // send in counterparty chain
    let ok = send_tokens_to_interchain_account(
        &counter_chain_contact,
        ibc_keys[0],
        grav_account.clone(),
    )
    .await;
    if ok.is_err() {
        error!("Counterparty chain response error {:?}",ok.err())
    };
    info!("Tokens sent!");
    
    info!("Waith 25 seconds and check balances..");
    delay_for(Duration::from_secs(25)).await;
    
    let cosmos_qc = CosmosQueryClient::connect(COSMOS_NODE_GRPC.as_str())
    .await
    .expect("Could not connect channel query client");
    let cosmos_ccqc = CosmosQueryClient::connect(IBC_NODE_GRPC.as_str())
    .await
    .expect("Could not connect channel query client");
    let gravity_interchain_account_balance = get_interchain_account_balance(
        grav_account.clone(),
        cosmos_ccqc,
        Some(timeout),
    ).await
    .expect("Error on balance check");
    let counterchain_interchain_account_balance = get_interchain_account_balance(
        counterchain_account.clone(),
        cosmos_qc,
        Some(timeout),
    )
    .await
    .expect("Error on balance check");
    if counterchain_interchain_account_balance == gravity_interchain_account_balance {
        info!("balances: {} and {}",
        gravity_interchain_account_balance,
        counterchain_interchain_account_balance)
    };

    info!("Delegating from interchain accounts:");
    let delegeted_from_gravity = delegate_from_interchain_account(
        contact,
        ibc_keys[0],
        keys[0].validator_key,
        grav_account,
        connection_id, 
        IBC_ADDRESS_PREFIX.to_string(),
    )
    .await
    .expect("Can't delegate");
    info!("{:?}",delegeted_from_gravity );
    let delegeted_from_counter_chain = delegate_from_interchain_account(
        &counter_chain_contact,
        keys[0].validator_key,
        ibc_keys[0],
        counterchain_account,
        counterparty_connection_id, 
        ADDRESS_PREFIX.to_string(),
    )
    .await
    .expect("Can't delegate");
    info!("{:?}",delegeted_from_counter_chain );

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

// get interchain account address
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

// send tokens to interchain account
pub async fn send_tokens_to_interchain_account(
    contact: &Contact,
    key: CosmosPrivateKey,
    receiver: String,
) -> Result<String,CosmosGrpcError> { 

    let coin = Coin {
        denom:STAKING_TOKEN.clone(),
        amount: "1000".to_string() ,
    };
    let mut coin_vec = Vec::new();
    coin_vec.push(coin);
    let send_tokens = MsgSend {
        from_address: key.to_address(&contact.get_prefix()).unwrap().to_string(),
        to_address: receiver,
        amount: coin_vec,
    };
    info!("Submitting MsgSend , gravity chain {:?}", send_tokens);

    let send_tokens = Msg::new(MSG_SEND_TOKENS_URL, send_tokens);
    let send_res = contact
        .send_message(
            &[send_tokens],
            Some("Test Creating interchain account".to_string()),
            &[],
            Some(OPERATION_TIMEOUT),
            key,
        )
        .await;
    info!("Sent MsgSend with response {:?}", send_res);
    return Ok("".to_string())

}

// get balance
pub async fn get_interchain_account_balance(
    account: String,
    qc: CosmosQueryClient<Channel>,
    timeout: Option<Duration>,
) -> Result<String,CosmosGrpcError> { 

    let mut qc = qc
    ;
    let timeout = match timeout {
        Some(t) => t,
        None => OPERATION_TIMEOUT,
    };

    let start = Instant::now();
    while Instant::now() - start < timeout {
        let balance = qc
            .all_balances(
                QueryAllBalancesRequest { 
                    address: account.clone(),
                    pagination: None, 
                }
            )
            .await;
        info!("{:?}",account);
        if balance.is_err() {
            delay_for(Duration::from_secs(5)).await;
            continue;
        }
        let balance = balance.unwrap().into_inner().balances;
        for b in balance {
            return Ok(b.amount);
        }    
    }
    Err(CosmosGrpcError::BadResponse("Can't get interchain account".to_string()))
}

// Create interchain accounts
pub async fn delegate_from_interchain_account(
    contact: &Contact,
    validator_key: CosmosPrivateKey,
    delegator_key: CosmosPrivateKey,
    delegator_address: String,
    connection_id: String,
    prefix: String
) -> Result<String,CosmosGrpcError> { 
    
    let coin = Coin {
        denom:STAKING_TOKEN.clone(),
        amount: "999".to_string() ,
    };

    let valoper_prefix = prefix + "valoper";

    let msg = MsgDelegate {
        delegator_address,
        validator_address: validator_key
        .to_address(&valoper_prefix)
        .unwrap().
        to_string(),
        amount: Some(coin),
    };

    let msg = msg.to_any().unwrap();

    // delegate 
    let msg_delegate = MsgSubmitTx {
        owner: delegator_key.to_address(&contact.get_prefix()).unwrap().to_string(),
        connection_id,
        msg: Some(msg),
    };
    info!("Submitting MsgSubmitTx to gravity chain {:?}", msg_delegate);

    let msg_delegate = Msg::new(MSG_SUBMIT_TX_URL, msg_delegate);
    let send_res = contact
        .send_message(
            &[msg_delegate],
            Some("Test delegating from interchain account".to_string()),
            &[],
            Some(OPERATION_TIMEOUT),
            delegator_key,
        )
        .await;
    info!("Sent MsgSubmitTx with response {:?}", send_res);

    delay_for(Duration::from_secs(10)).await;
    return Ok("".to_string())
}
