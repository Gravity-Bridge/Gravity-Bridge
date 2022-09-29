use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::happy_path_v2::deploy_cosmos_representing_erc20_and_check_adoption;
use crate::utils::*;
use crate::MINER_ADDRESS;
use crate::OPERATION_TIMEOUT;
use crate::{
    get_fee, ADDRESS_PREFIX, COSMOS_NODE_GRPC, IBC_ADDRESS_PREFIX, IBC_NODE_GRPC, STAKING_TOKEN,
};
use anyhow::{Context, Result};
use clarity::Address as EthAddress;
use clarity::Uint256;
use cosmos_gravity::{send::send_request_batch, send::MSG_SEND_TO_ETH_TYPE_URL, send::TIMEOUT};
use deep_space::error::CosmosGrpcError;
use deep_space::private_key::CosmosPrivateKey;
use deep_space::PrivateKey;
use deep_space::{Contact, Msg};
use ethereum_gravity::utils::get_tx_batch_nonce;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::query_client::QueryClient as CosmosQueryClient;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::{MsgSend, QueryAllBalancesRequest};
use gravity_proto::cosmos_sdk_proto::cosmos::base::v1beta1::Coin;
use gravity_proto::cosmos_sdk_proto::cosmos::params::v1beta1::ParamChange;
use gravity_proto::cosmos_sdk_proto::cosmos::staking::v1beta1::{
    query_client::QueryClient as StakingQueryClient, MsgDelegate, QueryValidatorDelegationsRequest,
};
use gravity_proto::cosmos_sdk_proto::ibc::core::connection::v1::query_client::QueryClient as ConnectionQueryClient;
use gravity_proto::cosmos_sdk_proto::ibc::core::connection::v1::QueryConnectionsRequest;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::{
    MsgRegisterAccount, MsgSendToEth, MsgSubmitTx, QueryInterchainAccountFromAddressRequest,
};
use gravity_utils::connection_prep::create_rpc_connections;
use prost::Message;
use prost_types::Any;
use std::time::Duration;
use std::time::Instant;
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use web30::client::Web3;

pub const MSG_REGISTER_INTERCHAIN_ACCOUNT_URL: &str = "/icaauth.v1.MsgRegisterAccount";
pub const MSG_SEND_TOKENS_URL: &str = "/cosmos.bank.v1beta1.MsgSend";
pub const MSG_SUBMIT_TX_URL: &str = "/icaauth.v1.MsgSubmitTx";
pub const STAKING_DELEGATE_TYPE_URL: &str = "/cosmos.staking.v1beta1.MsgDelegate";

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
            value.type_url == STAKING_DELEGATE_TYPE_URL,
            "invalid type url for `Any` type: expected `{}` and found `{}`",
            STAKING_DELEGATE_TYPE_URL,
            value.type_url
        );

        <Self as ::prost::Message>::decode(value.value.as_slice()).map_err(Into::into)
    }

    fn to_any(&self) -> ::anyhow::Result<::prost_types::Any> {
        Ok(::prost_types::Any {
            type_url: STAKING_DELEGATE_TYPE_URL.to_owned(),
            value: proto_encode(self)?,
        })
    }
}

impl AnyConvert for MsgSendToEth {
    fn from_any(value: &::prost_types::Any) -> ::anyhow::Result<Self> {
        ::anyhow::ensure!(
            value.type_url == MSG_SEND_TO_ETH_TYPE_URL,
            "invalid type url for `Any` type: expected `{}` and found `{}`",
            MSG_SEND_TO_ETH_TYPE_URL,
            value.type_url
        );

        <Self as ::prost::Message>::decode(value.value.as_slice()).map_err(Into::into)
    }

    fn to_any(&self) -> ::anyhow::Result<::prost_types::Any> {
        Ok(::prost_types::Any {
            type_url: MSG_SEND_TO_ETH_TYPE_URL.to_owned(),
            value: proto_encode(self)?,
        })
    }
}

/// Test Interchain accounts host / controller. Create , Send , Delegate
/// Plan is
/// 1. get connection_id , cpc_connection_id:  <fn get_connection_id>
/// 2. register interchain account gravity -> ibc: <fn create_interchain_account>
/// 3. check account registered: <fn get_interchain_account>
/// 4. send some stake tokens: <fn send_tokens_to_interchain_account>, <fn get_interchain_account_balance>
/// 5. delegate <fn delegate_from_interchain_account>, <fn check_delegatinons>
pub async fn ica_test(
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    ibc_keys: Vec<CosmosPrivateKey>,
    gravity_address: EthAddress,
    erc20_address: EthAddress,
    web30: &Web3,
    grpc_client: GravityQueryClient<Channel>,
) {
    let no_relay_market_config = create_default_test_config();
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;

    // Add allow messages
    add_ica_host_allow_messages(contact, &keys).await;
    // Create connection query clients for both chains
    let gravity_connection_qc = ConnectionQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect gravity channel query client");
    let cpc_connection_qc = ConnectionQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect counterparty chain channel query client");

    // Retrieving connections ids. Waiting up to 2 minutes before OnChanOpenConfirm = success
    let connection_id_timeout = Duration::from_secs(60 * 5);
    let connection_id = get_connection_id(gravity_connection_qc, Some(connection_id_timeout))
        .await
        .expect("Could not find gravity-test-1 connection id");
    let cpc_connection_id = get_connection_id(cpc_connection_qc, Some(connection_id_timeout))
        .await
        .expect("Could not find gravity-test-1 counterparty connection id");
    info!(
        "Found valid connections: connection_id: {} cpc_connection_id {}",
        connection_id, cpc_connection_id,
    );
    info!("Waiting 120 seconds for ConOpenConfirm before account create");
    delay_for(Duration::from_secs(120)).await;
    // create GRPC contact for counterparty chain
    let connections = create_rpc_connections(
        IBC_ADDRESS_PREFIX.clone(),
        Some(IBC_NODE_GRPC.to_string()),
        None,
        TIMEOUT,
    )
    .await;
    let cpc_contact = connections.contact.unwrap();

    //create gravity and cpc interchain accounts
    info!("Processng interchain account registration");
    let ok = create_interchain_account(
        contact,
        &cpc_contact,
        keys.clone(),
        ibc_keys.clone(),
        connection_id.clone(),
        cpc_connection_id.clone(),
    )
    .await;
    if ok.is_err() {
        error!("Accounts not registred! {:?}", ok.err())
    }
    info!("Accounts registered");

    // Create gravity qyery clients for both chains
    let timeout = Duration::from_secs(60 * 5);
    let qc = GravityQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect gravity chain channel query client");
    let cpc_qc = GravityQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect counterparty chain channel query client");

    info!("waiting for ACK 60 secs then send TX");
    delay_for(Duration::from_secs(60)).await;
    let gravity_account = get_interchain_account(
        contact,
        keys[0].validator_key,
        qc,
        Some(timeout),
        connection_id.clone(),
    )
    .await
    .expect("Account for gravity not created or something went wrong");

    // send tx
    let cpc_account = get_interchain_account(
        &cpc_contact,
        ibc_keys[0],
        cpc_qc,
        Some(timeout),
        connection_id.clone(),
    )
    .await
    .expect("Account for counterparty chain not created or something went wrong");

    info!(
        "gravity interchain account: {} , counterchain interchain account: {}",
        gravity_account, cpc_account,
    );
    info!("Waiting for TX 30 secs");
    delay_for(Duration::from_secs(30)).await;
    let staking_coin = Coin {
        denom: STAKING_TOKEN.clone(),
        amount: "2000".to_string(),
    };
    // send in gravity chain
    let ok = send_tokens_to_interchain_account(
        contact,
        keys[0].validator_key,
        cpc_account.clone(),
        staking_coin.clone(),
    )
    .await;
    if ok.is_err() {
        error!("Gravity chain response error {:?}", ok.err())
    };
    info!("Tokens sent to counterparty interchain account from gravity regular account !");

    info!("Pause 30 seconds, then send TX");
    delay_for(Duration::from_secs(30)).await;
    // send in counterparty chain
    let ok = send_tokens_to_interchain_account(
        &cpc_contact,
        ibc_keys[0],
        gravity_account.clone(),
        staking_coin.clone(),
    )
    .await;
    if ok.is_err() {
        error!("Counterparty chain response error {:?}", ok.err())
    };
    info!("Tokens sent to gravity interchain account from counterparty regular account!");
    info!("Waiting 60 seconds and check balances..");
    delay_for(Duration::from_secs(60)).await;

    // Create CosmosQueryClient for both chains
    let gravity_cosmos_qc = CosmosQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect gravity chain channel query client");

    let cpc_cosmos_qc = CosmosQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect counterparty chain channel query client");

    info!("Try to get balance");
    let gravity_interchain_account_balance =
        get_interchain_account_balance(gravity_account.clone(), cpc_cosmos_qc, Some(timeout))
            .await
            .expect("Error on balance check");

    info!("Pause 20 seconds, then try to get balance");
    delay_for(Duration::from_secs(20)).await;
    let cpc_interchain_account_balance =
        get_interchain_account_balance(cpc_account.clone(), gravity_cosmos_qc, Some(timeout))
            .await
            .expect("Error on balance check");
    if cpc_interchain_account_balance == gravity_interchain_account_balance {
        info!(
            "balances: {} and {}",
            gravity_interchain_account_balance, cpc_interchain_account_balance
        )
    };

    let coin = Coin {
        denom: STAKING_TOKEN.clone(),
        amount: "999".to_string(),
    };
    let valoper_prefix = ADDRESS_PREFIX.to_string() + "valoper";
    let ibc_valoper_prefix = IBC_ADDRESS_PREFIX.to_string() + "valoper";
    let msg_delegate_from_gravity = prepare_msg_delegate(
        gravity_account.clone(),
        ibc_keys[0]
            .to_address(&ibc_valoper_prefix)
            .unwrap()
            .to_string(),
        coin.clone(),
    );
    let msg_delegate_from_cpc = prepare_msg_delegate(
        cpc_account.clone(),
        keys[0]
            .validator_key
            .to_address(&valoper_prefix)
            .unwrap()
            .to_string(),
        coin.clone(),
    );
    // delegate from interchain account to gravity validator
    info!("Delegating from interchain accounts:");
    let delegeted_from_gravity = submit_tx(
        contact,
        keys[0].validator_key,
        connection_id,
        msg_delegate_from_gravity,
    )
    .await
    .expect("Can't delegate");
    info!("{:?}", delegeted_from_gravity);
    info!("Pause 60 seconds");

    // delegate from interchain account to counterparty validator
    info!("Pause 60 seconds. Then delegate from counterparty chain");
    delay_for(Duration::from_secs(60)).await;
    let delegeted_from_cpc = submit_tx(
        &cpc_contact,
        ibc_keys[0],
        cpc_connection_id.clone(),
        msg_delegate_from_cpc,
    )
    .await
    .expect("Can't delegate");
    info!("{:?}", delegeted_from_cpc);

    info!("Wait 60 seconds then check delegations");
    delay_for(Duration::from_secs(60)).await;

    // Creating StakingQuery clinets for both chains
    let staking_qc = StakingQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect channel query client");
    let cpc_staking_qc = StakingQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect channel query client");

    // check delegation shares
    let amount_delegated_to_gravity_validator = check_delegatinons(
        keys[0].validator_key,
        ADDRESS_PREFIX.to_string(),
        cpc_account.clone(),
        staking_qc,
        Some(timeout),
    )
    .await
    .expect("Not delegated!");
    info!(
        "found delegation! to gravity, shares: {} ",
        amount_delegated_to_gravity_validator
    );
    info!("Pause 30 seconds");
    delay_for(Duration::from_secs(30)).await;

    let amount_delegated_to_counterchain_validator = check_delegatinons(
        ibc_keys[0],
        IBC_ADDRESS_PREFIX.to_string(),
        gravity_account.clone(),
        cpc_staking_qc,
        Some(timeout),
    )
    .await
    .expect("Not delegated!");
    info!(
        "found delegations! to counterchain, shares: {}",
        amount_delegated_to_counterchain_validator
    );

    info!("Send some footoken to interchain account");
    let amount_to_bridge: Uint256 = 1_000_000u64.into();
    let foo_coin = Coin {
        denom: "footoken".to_string(),
        amount: amount_to_bridge.clone().to_string(),
    };
    // send in gravity chain
    let ok = send_tokens_to_interchain_account(
        contact,
        keys[0].validator_key,
        cpc_account.clone(),
        foo_coin.clone(),
    )
    .await;
    if ok.is_err() {
        error!("Gravity chain response error {:?}", ok.err())
    };

    info!("deploy cosmos representing erc20");
    let ibc_metadata = footoken_metadata(contact).await;
    let mut grpc_client = grpc_client;
    let erc20_contract = deploy_cosmos_representing_erc20_and_check_adoption(
        gravity_address,
        web30,
        Some(keys.clone()),
        &mut grpc_client,
        false,
        ibc_metadata.clone(),
    )
    .await;
    let token_to_send_to_eth = ibc_metadata.base.clone();
    let send_to_eth_coin = Coin {
        denom: token_to_send_to_eth.clone(),
        amount: amount_to_bridge.clone().to_string(),
    };

    info!("Prepare and send SendToEth message from counterparty chain");
    let fee = Coin {
        denom: token_to_send_to_eth.to_string(),
        amount: "".to_string(),
    };
    let msg_send_to_eth = prepare_msg_send_to_eth(
        cpc_account.clone(),
        keys[0].eth_key.to_address().to_string(),
        send_to_eth_coin.clone(),
        Some(fee),
    );
    info!("{} {:?}", msg_send_to_eth.type_url, msg_send_to_eth.value);
    let send_to_eth_from_cpc = submit_tx(
        &cpc_contact,
        ibc_keys[0],
        cpc_connection_id.clone(),
        msg_send_to_eth,
    )
    .await
    .expect("Can't send MsgSendToEth");
    info!("{:?}", send_to_eth_from_cpc);
    info!("Wait 60 seconds then request batch");
    delay_for(Duration::from_secs(60)).await;

    let mut current_eth_batch_nonce =
        get_tx_batch_nonce(gravity_address, erc20_address, *MINER_ADDRESS, web30)
            .await
            .expect("Failed to get current eth valset");

    let res = send_request_batch(
        keys[0].orch_key,
        token_to_send_to_eth.clone(),
        Some(get_fee(None)),
        contact,
    )
    .await
    .unwrap();
    info!("batch request response is {:?}", res);
    let starting_batch_nonce = current_eth_batch_nonce;

    let start = Instant::now();
    while starting_batch_nonce == current_eth_batch_nonce {
        info!(
            "Batch is not yet submitted {}>, waiting",
            starting_batch_nonce
        );
        current_eth_batch_nonce =
            get_tx_batch_nonce(gravity_address, erc20_address, *MINER_ADDRESS, web30)
                .await
                .expect("Failed to get current eth tx batch nonce");
        delay_for(Duration::from_secs(4)).await;
        if Instant::now() - start > TIMEOUT {
            panic!("Failed to submit transaction batch set");
        }
    }
    while Instant::now() - start < TIMEOUT {
        let new_balance =
            get_erc20_balance_safe(erc20_contract, web30, keys[0].eth_key.to_address()).await;
        // only keep trying if our error is gas related
        if new_balance.is_err() {
            continue;
        }
        let balance = new_balance.unwrap();
        if balance == amount_to_bridge {
            info!("Successfully bridged {} to Ethereum!", amount_to_bridge);
            assert!(balance == amount_to_bridge.clone());
        } else if balance != 0u8.into() {
            error!("Expected {} but got {} instead", amount_to_bridge, balance);
        }
        delay_for(Duration::from_secs(1)).await;
        if Instant::now() - start > TIMEOUT {
            panic!(
                "Failed to get balance. Expected {} but got {} instead",
                amount_to_bridge, balance
            );
        }
    }
}

// Get connection for both chains
pub async fn get_connection_id(
    cpc_connection_qc: ConnectionQueryClient<Channel>,
    timeout: Option<Duration>,
) -> Result<String, CosmosGrpcError> {
    let mut cpc_connection_qc = cpc_connection_qc;
    let timeout = match timeout {
        Some(t) => t,
        None => OPERATION_TIMEOUT,
    };

    let start = Instant::now();
    while Instant::now() - start < timeout {
        let connections = cpc_connection_qc
            .connections(QueryConnectionsRequest { pagination: None })
            .await;
        if connections.is_err() {
            delay_for(Duration::from_secs(5)).await;
            continue;
        }
        let connections = connections.unwrap().into_inner().connections;
        if let Some(connection) = connections.into_iter().next() {
            return Ok(connection.id);
        }
    }
    Err(CosmosGrpcError::BadResponse(
        "No such connection".to_string(),
    ))
}

// Create interchain accounts
pub async fn create_interchain_account(
    contact: &Contact,
    cpc_contact: &Contact,
    keys: Vec<ValidatorKeys>,
    ibc_keys: Vec<CosmosPrivateKey>,
    connection_id: String,
    cpc_connection_id: String,
) -> Result<String, CosmosGrpcError> {
    // chain chain register
    let msg_register_account = MsgRegisterAccount {
        owner: keys[0]
            .validator_key
            .to_address(&contact.get_prefix())
            .unwrap()
            .to_string(),
        connection_id,
        version: "".to_string(),
    };
    info!(
        "Submitting MsgRegisterAccount to gravity chain {:?}",
        msg_register_account
    );

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
        owner: ibc_keys[0]
            .to_address(&cpc_contact.get_prefix())
            .unwrap()
            .to_string(),
        connection_id: cpc_connection_id,
        version: "".to_string(),
    };
    info!(
        "Submitting MsgRegisterAccount to counterparty chain {:?}",
        msg_register_account_counter_chain
    );

    let msg_register_account_counter_chain = Msg::new(
        MSG_REGISTER_INTERCHAIN_ACCOUNT_URL,
        msg_register_account_counter_chain,
    );
    let send_res = cpc_contact
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
    if send_res.is_err() {
        Err(CosmosGrpcError::BadResponse(
            "Can't create account".to_string(),
        ))
    } else {
        Ok(send_res.unwrap().txhash)
    }
}

// get interchain account address
pub async fn get_interchain_account(
    contact: &Contact,
    key: CosmosPrivateKey,
    qc: GravityQueryClient<Channel>,
    timeout: Option<Duration>,
    connection_id: String,
) -> Result<String, CosmosGrpcError> {
    let mut qc = qc;
    let timeout = match timeout {
        Some(t) => t,
        None => OPERATION_TIMEOUT,
    };

    let start = Instant::now();
    while Instant::now() - start < timeout {
        let account = qc
            .interchain_account_from_address(QueryInterchainAccountFromAddressRequest {
                owner: key.to_address(&contact.get_prefix()).unwrap().to_string(),
                connection_id: connection_id.clone(),
            })
            .await;
        info!("{:?}, waiting...", account);
        if account.is_err() {
            delay_for(Duration::from_secs(5)).await;
            continue;
        }

        let account = account.unwrap().into_inner().interchain_account_address;
        return Ok(account);
    }
    Err(CosmosGrpcError::BadResponse(
        "Can't get interchain account".to_string(),
    ))
}

// send tokens to interchain account
pub async fn send_tokens_to_interchain_account(
    contact: &Contact,
    key: CosmosPrivateKey,
    receiver: String,
    coin: Coin,
) -> Result<String, CosmosGrpcError> {
    let coin_vec = vec![coin];
    let send_tokens = MsgSend {
        from_address: key.to_address(&contact.get_prefix()).unwrap().to_string(),
        to_address: receiver,
        amount: coin_vec,
    };
    info!("Submitting MsgSend {:?}", send_tokens);

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
    if send_res.is_err() {
        Err(CosmosGrpcError::BadResponse(
            "Message not submitted".to_string(),
        ))
    } else {
        Ok(send_res.unwrap().txhash)
    }
}

// get balance
pub async fn get_interchain_account_balance(
    account: String,
    qc: CosmosQueryClient<Channel>,
    timeout: Option<Duration>,
) -> Result<String, CosmosGrpcError> {
    let mut qc = qc;
    let timeout = match timeout {
        Some(t) => t,
        None => OPERATION_TIMEOUT,
    };

    let start = Instant::now();
    while Instant::now() - start < timeout {
        let balance = qc
            .all_balances(QueryAllBalancesRequest {
                address: account.clone(),
                pagination: None,
            })
            .await;
        info!("{:?}", balance);
        if balance.is_err() {
            delay_for(Duration::from_secs(5)).await;
            continue;
        }
        let balance = balance.unwrap().into_inner().balances;
        if let Some(b) = balance.into_iter().next() {
            return Ok(b.amount);
        }
    }
    Err(CosmosGrpcError::BadResponse(
        "Can't get interchain account".to_string(),
    ))
}

// get balance
pub async fn check_delegatinons(
    validator_key: CosmosPrivateKey,
    prefix: String,
    delegator_address: String,
    qc: StakingQueryClient<Channel>,
    timeout: Option<Duration>,
) -> Result<String, CosmosGrpcError> {
    let mut qc = qc;
    let timeout = match timeout {
        Some(t) => t,
        None => OPERATION_TIMEOUT,
    };

    let valoper_prefix = prefix + "valoper";
    info!(
        "valoper address: {}",
        validator_key
            .to_address(&valoper_prefix)
            .unwrap()
            .to_string()
    );
    let start = Instant::now();
    while Instant::now() - start < timeout {
        delay_for(Duration::from_secs(5)).await;
        let delegators = qc
            .validator_delegations(QueryValidatorDelegationsRequest {
                validator_addr: validator_key
                    .to_address(&valoper_prefix)
                    .unwrap()
                    .to_string(),
                pagination: None,
            })
            .await;
        if delegators.is_err() {
            delay_for(Duration::from_secs(5)).await;
            continue;
        }
        let delegators = delegators.unwrap().into_inner().delegation_responses;
        for delegator in delegators {
            if delegator.delegation.clone().unwrap().delegator_address == delegator_address {
                info!("{:?}", delegator);
                delay_for(Duration::from_secs(5)).await;
                return Ok(delegator.delegation.clone().unwrap().shares);
            }
        }
    }

    Err(CosmosGrpcError::BadResponse(
        "Delegator not found:(".to_string(),
    ))
}

/// submits and passes a proposal to add interchainaccounts host allow messages
pub async fn add_ica_host_allow_messages(contact: &Contact, keys: &[ValidatorKeys]) {
    info!("Submitting and passing a proposal to allow all messages for interchainaccounts");
    let mut params_to_change = Vec::new();
    let change = ParamChange {
        subspace: "icahost".to_string(),
        key: "AllowMessages".to_string(),
        value: r#"["*"]"#.to_string(),
    };
    params_to_change.push(change);
    create_parameter_change_proposal(
        contact,
        keys[0].validator_key,
        params_to_change,
        get_fee(None),
    )
    .await;
    vote_yes_on_proposals(contact, keys, None).await;
    wait_for_proposals_to_execute(contact).await;
}

// SendToETH ICA
pub async fn submit_tx(
    contact: &Contact,
    sender_key: CosmosPrivateKey,
    connection_id: String,
    msg: Any,
) -> Result<String, CosmosGrpcError> {
    // send
    let msg_send = MsgSubmitTx {
        owner: sender_key
            .to_address(&contact.get_prefix())
            .unwrap()
            .to_string(),
        connection_id,
        msg: Some(msg),
    };
    info!("Submitting MsgSubmitTx to gravity chain {:?}", msg_send);

    let msg_send = Msg::new(MSG_SUBMIT_TX_URL, msg_send);
    let send_res = contact
        .send_message(
            &[msg_send],
            Some("Test interchain accounts".to_string()),
            &[],
            Some(OPERATION_TIMEOUT),
            sender_key,
        )
        .await;
    info!("Sent MsgSubmitTx with response {:?}", send_res);

    delay_for(Duration::from_secs(10)).await;
    Ok(send_res.unwrap().txhash)
}

pub fn prepare_msg_send_to_eth(
    sender: String,
    eth_dest: String,
    amount: Coin,
    bridge_fee: Option<Coin>,
) -> Any {
    let msg = MsgSendToEth {
        sender,
        eth_dest,
        amount: Some(amount),
        bridge_fee,
    };

    msg.to_any().unwrap()
}

pub fn prepare_msg_delegate(
    delegator_address: String,
    validator_address: String,
    amount: Coin,
) -> Any {
    let msg = MsgDelegate {
        delegator_address,
        validator_address,
        amount: Some(amount),
    };

    msg.to_any().unwrap()
}
