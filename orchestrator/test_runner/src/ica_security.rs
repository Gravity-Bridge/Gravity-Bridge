use crate::airdrop_proposal::wait_for_proposals_to_execute;

use crate::utils::*;
use crate::{
    get_fee, ADDRESS_PREFIX, COSMOS_NODE_GRPC, IBC_ADDRESS_PREFIX, IBC_NODE_GRPC, STAKING_TOKEN,
};
use crate::{CosmosAddress, OPERATION_TIMEOUT};
use clarity::Address as EthAddress;
use clarity::Uint256;

use cosmos_gravity::send::TIMEOUT;
use deep_space::client::msgs::MSG_SEND_TYPE_URL;
use deep_space::error::CosmosGrpcError;
use deep_space::private_key::CosmosPrivateKey;
use deep_space::utils::encode_any;
use deep_space::PrivateKey;
use deep_space::{Address, Coin as DSCoin};
use deep_space::{Contact, Msg};

use futures::future::join_all;

use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::MsgSend;
use gravity_proto::cosmos_sdk_proto::cosmos::base::abci::v1beta1::TxResponse;
use gravity_proto::cosmos_sdk_proto::cosmos::base::v1beta1::Coin;
use gravity_proto::cosmos_sdk_proto::cosmos::params::v1beta1::ParamChange;

use gravity_proto::cosmos_sdk_proto::ibc::core::connection::v1::query_client::QueryClient as ConnectionQueryClient;
use gravity_proto::cosmos_sdk_proto::ibc::core::connection::v1::QueryConnectionsRequest;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;

use gravity_proto::icaauth::query_client::QueryClient as IcaQueryClient;
use gravity_proto::icaauth::{
    MsgRegisterAccount, MsgSubmitTx, QueryInterchainAccountFromAddressRequest,
};
use gravity_utils::num_conversion::one_atom;
use itertools::Itertools;

use prost_types::Any;

use std::time::Duration;
use std::time::Instant;
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use web30::client::Web3;

pub const MSG_REGISTER_INTERCHAIN_ACCOUNT_URL: &str = "/icaauth.v1.MsgRegisterAccount";
pub const MSG_SEND_TOKENS_URL: &str = "/cosmos.bank.v1beta1.MsgSend";
pub const MSG_SUBMIT_TX_URL: &str = "/icaauth.v1.MsgSubmitTx";

/// Test Interchain accounts host / controller to make sure that obvious issues are not present
/// 1. Stress test
/// 2. Try to control other users' interchain accounts
/// 3. TODO: Try to control other users' regular accounts via ICA
/// 4. TODO: Submit invalid messages with ICA (no funds to transfer, nonsensical attempts at msg execution)
/// 5. TODO: Submit malformed messages
pub async fn ica_sec_test(
    gravity_contact: &Contact,
    keys: Vec<ValidatorKeys>,
    ibc_keys: Vec<CosmosPrivateKey>,
    _gravity_address: EthAddress,
    _web30: &Web3,
    _grpc_client: GravityQueryClient<Channel>,
    registers_per_iteration: usize,
) {
    let val_priv_keys = get_validator_private_keys(&keys);
    // create GRPC contact for counterparty chain
    let ibc_contact = &Contact::new(&*IBC_NODE_GRPC, TIMEOUT, &*IBC_ADDRESS_PREFIX)
        .expect("Unable to create counterparty Contact");

    // Add allow messages to both chains
    add_ica_host_allow_messages(gravity_contact, &val_priv_keys).await;
    add_ica_host_allow_messages(ibc_contact, &ibc_keys).await;

    // Create connection query clients for both chains
    let gravity_connection_qc = ConnectionQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect gravity channel query client");
    let ibc_connection_qc = ConnectionQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect counterparty chain channel query client");

    // Retrieving connections ids. Waiting up to 2 minutes before OnChanOpenConfirm = success
    let connection_id_timeout = Duration::from_secs(60 * 5);
    let gravity_conn_id = get_connection_id(gravity_connection_qc, Some(connection_id_timeout))
        .await
        .expect("Could not find gravity-test-1 connection id");
    let ibc_conn_id = get_connection_id(ibc_connection_qc, Some(connection_id_timeout))
        .await
        .expect("Could not find gravity-test-1 counterparty connection id");
    info!(
        "Found valid connections: connection_id: {} cpc_connection_id {}",
        gravity_conn_id, ibc_conn_id,
    );
    let _gravity_icaqc = IcaQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect gravity chain channel query client");
    let _ibc_icaqc = IcaQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect counterparty chain channel query client");

    info!("Waiting 20 seconds for ConOpenConfirm before account create");
    delay_for(Duration::from_secs(20)).await;

    // Create connection query clients for both chains
    let gravity_ica_qc = IcaQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect gravity channel query client");
    let ibc_ica_qc = IcaQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect counterparty chain channel query client");

    let (gravity_users, gravity_to_ibc_icas, ibc_users, ibc_to_gravity_icas) =
        create_and_fund_test_accounts(
            gravity_contact,
            gravity_ica_qc,
            gravity_conn_id.clone(),
            val_priv_keys,
            ibc_contact,
            ibc_ica_qc,
            ibc_conn_id.clone(),
            ibc_keys,
            30u64,
            registers_per_iteration,
        )
        .await;

    test_ica_account_theft(
        gravity_contact,
        gravity_conn_id.clone(),
        gravity_users,
        gravity_to_ibc_icas,
        ibc_contact,
        ibc_conn_id.clone(),
        ibc_users,
        ibc_to_gravity_icas,
    )
    .await
    .unwrap();

    info!("ICA Tests Done!")
}

////////////////////////////////// SETUP FUNCTIONS

/// submits and passes a proposal to add interchainaccounts host allow messages
pub async fn add_ica_host_allow_messages(contact: &Contact, keys: &[CosmosPrivateKey]) {
    info!("Submitting and passing a proposal to allow all messages for interchainaccounts");
    let mut params_to_change = Vec::new();
    let change = ParamChange {
        subspace: "icahost".to_string(),
        key: "AllowMessages".to_string(),
        value: r#"["*"]"#.to_string(),
    };
    params_to_change.push(change);
    create_parameter_change_proposal(contact, keys[0], params_to_change, get_fee(None)).await;
    vote_yes_on_proposals(contact, keys, None).await;
    wait_for_proposals_to_execute(contact).await;
}

/// Creates `num_accounts` native accounts on each gravity and ibc-chain, then registers an
/// interchain account for each of those native accounts, funding native accounts with 1 STAKE and
/// interchain accounts with 10 STAKE
pub async fn create_and_fund_test_accounts(
    gravity_contact: &Contact,
    gravity_icaqc: IcaQueryClient<Channel>,
    gravity_conn_id: String,
    gravity_keys: Vec<CosmosPrivateKey>,
    ibc_contact: &Contact,
    ibc_icaqc: IcaQueryClient<Channel>,
    ibc_conn_id: String,
    ibc_keys: Vec<CosmosPrivateKey>,
    num_accounts: u64,
    registers_per_iteration: usize,
) -> (
    Vec<(CosmosPrivateKey, CosmosAddress)>, // gravity users
    Vec<CosmosAddress>, // interchain accounts on ibc-chain owned by gravity users
    Vec<(CosmosPrivateKey, CosmosAddress)>, // ibc users
    Vec<CosmosAddress>, // interchain Accounts on gravity owned by ibc users
) {
    // Generate native chain users to use in stress/security tests
    let gravity_users = get_cosmos_accounts(&*ADDRESS_PREFIX, num_accounts);
    let gravity_accounts = gravity_users
        .clone()
        .into_iter()
        .map(|(k, _a)| k)
        .collect_vec();
    let gravity_addresses = gravity_users
        .clone()
        .into_iter()
        .map(|(_k, a)| a)
        .collect_vec();
    let ibc_users = get_cosmos_accounts(&*IBC_ADDRESS_PREFIX, num_accounts);
    let ibc_accounts: Vec<CosmosPrivateKey> =
        ibc_users.clone().into_iter().map(|(k, _a)| k).collect();
    let ibc_addresses: Vec<Address> = ibc_users.clone().into_iter().map(|(_k, a)| a).collect();

    // Fund the native accounts with 1 STAKE
    let amount = one_atom();
    let denom = &*STAKING_TOKEN;
    fund_users_in_chunks(
        gravity_contact,
        gravity_keys.clone(),
        gravity_addresses.clone(),
        amount.clone(),
        denom.clone(),
    )
    .await;
    fund_users_in_chunks(
        ibc_contact,
        ibc_keys.clone(),
        ibc_addresses.clone(),
        amount.clone(),
        denom.clone(),
    )
    .await;

    // Create gravity and ibc-chain interchain accounts
    info!("Creating interchain accounts on both chains");
    let gravity_icas = create_interchain_accounts_in_chunks(
        gravity_contact,
        gravity_accounts.clone(),
        gravity_conn_id.clone(),
        registers_per_iteration,
    );
    let ibc_icas = create_interchain_accounts_in_chunks(
        ibc_contact,
        ibc_accounts.clone(),
        ibc_conn_id.clone(),
        registers_per_iteration,
    );
    let results = join_all([gravity_icas, ibc_icas]).await;
    for r in results {
        r.map_err(|e| format!("Unable to create interchain accounts in chunks: {:?}", e))
            .unwrap();
    }

    info!("Register Interchain Account requests sent for all users, waiting before querying their status");

    delay_for(Duration::from_secs(60)).await;

    // Next, identify all the interchain accounts which should have been created on both chains
    let timeout = Duration::from_secs(60 * 5);
    let gravity_to_ibc_icas = identify_interchain_accounts(
        gravity_contact,
        gravity_accounts.clone(),
        gravity_icaqc,
        gravity_conn_id.clone(),
        Some(timeout),
    )
    .await;
    let gravity_to_ibc_icas: Vec<Address> = gravity_to_ibc_icas
        .into_iter()
        .map(|r| {
            r.map_err(|e| format!("Failed to create interchain account on ibc-chain: {}", e))
                .unwrap()
        })
        .collect();

    let ibc_to_gravity_icas = identify_interchain_accounts(
        ibc_contact,
        ibc_accounts,
        ibc_icaqc,
        ibc_conn_id.clone(),
        Some(timeout),
    )
    .await;
    let ibc_to_gravity_icas: Vec<Address> = ibc_to_gravity_icas
        .into_iter()
        .map(|r| {
            r.map_err(|e| format!("Failed to create interchain account on gravity: {}", e))
                .unwrap()
        })
        .collect();

    debug!(
        "gravity interchain accounts: {:?} , ibc-chain interchain accounts: {:?}",
        gravity_to_ibc_icas, ibc_to_gravity_icas,
    );

    // Now fund those interchain accounts so that they can act
    let amount: Uint256 = one_atom() * 10u8.into();
    fund_users_in_chunks(
        gravity_contact,
        gravity_keys,
        ibc_to_gravity_icas.clone(),
        amount.clone(),
        denom.clone(),
    )
    .await;
    fund_users_in_chunks(
        ibc_contact,
        ibc_keys,
        gravity_to_ibc_icas.clone(),
        amount.clone(),
        denom.clone(),
    )
    .await;

    // Verify that all native and interchain accounts have the expected funds
    info!("Checking balances");
    let expected_native_balance = &[DSCoin {
        amount: one_atom(),
        denom: (*STAKING_TOKEN).clone(),
    }];
    check_many_balances(gravity_contact, &gravity_addresses, expected_native_balance).await;
    check_many_balances(ibc_contact, &ibc_addresses, expected_native_balance).await;
    let expected_ica_balance = &[DSCoin {
        amount: amount.clone(),
        denom: (*STAKING_TOKEN).clone(),
    }];
    check_many_balances(gravity_contact, &ibc_to_gravity_icas, expected_ica_balance).await;
    check_many_balances(ibc_contact, &gravity_to_ibc_icas, expected_ica_balance).await;

    (
        gravity_users,
        gravity_to_ibc_icas,
        ibc_users,
        ibc_to_gravity_icas,
    )
}

/// Creates a `number` of accounts `(CosmosPrivateKey, CosmosAddress)` with a given `prefix`
pub fn get_cosmos_accounts(prefix: &str, number: u64) -> Vec<(CosmosPrivateKey, CosmosAddress)> {
    let mut accounts = vec![];
    for _i in 0..number {
        accounts.push(get_cosmos_key(prefix));
    }

    accounts
}

/// Funds all the `users` thorugh all the `funders`, attempting to send `amount` of `denom` without draining any funder more than others
pub async fn fund_users_in_chunks(
    contact: &Contact,
    funders: Vec<CosmosPrivateKey>,
    users: Vec<CosmosAddress>,
    amount: Uint256,
    denom: String,
) {
    let num_funders = funders.len();
    let chunk_size = users.len() / num_funders;
    let chunks = users.chunks(chunk_size);

    info!(
        "Funding users: num_funders {} num_users {} chunk_size {} chunks {:?}",
        num_funders,
        users.len(),
        chunk_size,
        chunks
    );

    for (i, chunk) in chunks.enumerate() {
        let funder = funders
            .get(i % num_funders)
            .expect("Not enough funders, how is that?");
        fund_cosmos_accounts(contact, *funder, chunk, amount.clone(), denom.clone()).await;
    }
}

/// Funds `addresses_to_fund` with a given `amount` of `denom`, with funds coming from `funder_key`
pub async fn fund_cosmos_accounts(
    contact: &Contact,
    funder_key: CosmosPrivateKey,
    addresses_to_fund: &[CosmosAddress],
    amount: Uint256,
    denom: String,
) {
    info!(
        "Funding cosmos accounts: addresses_to_fund {:?}",
        addresses_to_fund
    );
    let coin = Coin {
        amount: amount.to_string(),
        denom,
    };
    for address in addresses_to_fund {
        send_tokens_to_account_with_retries(contact, funder_key, address.to_string(), &coin)
            .await
            .unwrap();
    }
}

pub async fn check_many_balances(
    contact: &Contact,
    addresses: &[CosmosAddress],
    expected_coins: &[DSCoin],
) {
    let mut futures = vec![];
    for address in addresses {
        let fut = check_cosmos_balances(contact, *address, expected_coins);
        futures.push(fut);
    }

    join_all(futures).await;
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

/// Creates an interchain account for each user in gravity_users and also ibc_users, avoiding node
/// halts by chunking the users into groups of `chunk_size`
pub async fn create_interchain_accounts_in_chunks(
    contact: &Contact,            // A contact leading to the controller chain
    users: Vec<CosmosPrivateKey>, // Native users who should control ICAs on the host chain
    conn_id: String,              // The controller chain's connection leading to the host chain
    chunk_size: usize,
) -> Result<(), CosmosGrpcError> {
    let chunks = users.chunks(chunk_size).collect_vec();
    for chunk in chunks.clone() {
        let result = create_interchain_accounts(contact, chunk.to_vec(), conn_id.clone()).await;
        result
            .map_err(|e| format!("Failed to create interchain account! {}", e))
            .unwrap();
        delay_for(Duration::from_secs(15)).await;
    }

    Ok(())
}

/// Creates an interchain account controlled by each user in users over connection conn_id
pub async fn create_interchain_accounts(
    contact: &Contact,            // A contact leading to the controller chain
    users: Vec<CosmosPrivateKey>, // Controller users who should control ICAs on the host chain
    conn_id: String,              // The controller connection leading to the host chain
) -> Result<(), CosmosGrpcError> {
    let mut futures = vec![];
    for user in users.clone() {
        let fut = create_interchain_account_with_retries(contact, user, conn_id.clone(), None);
        futures.push(fut);
    }
    let results = join_all(futures)
        .await
        .into_iter()
        .map(|r| {
            r.map_err(|e| format!("Failed to create interchain account! {}", e))
                .unwrap()
        })
        .collect_vec();
    debug!("Interchain account creation results: {:?}", results);

    Ok(())
}

/// Creates an interchain account controlled by `key` living on the chain pointed to by `connection_id`
pub async fn create_interchain_account_with_retries(
    contact: &Contact,     // The controller chain
    key: CosmosPrivateKey, // The controlling account
    connection_id: String, // The controller's connection id corresponding to the host chain
    timeout: Option<Duration>,
) -> Result<String, CosmosGrpcError> {
    let timeout = timeout.unwrap_or(Duration::from_secs(60 * 5));
    let msg_register_account = MsgRegisterAccount {
        owner: key.to_address(&contact.get_prefix()).unwrap().to_string(),
        connection_id,
        version: "".to_string(),
    };
    info!(
        "Submitting MsgRegisterAccount to gravity chain {:?}",
        msg_register_account
    );

    let msg_register_account = Msg::new(MSG_REGISTER_INTERCHAIN_ACCOUNT_URL, msg_register_account);

    let start = Instant::now();
    while Instant::now() - start < timeout {
        let send_res = contact
            .send_message(
                &[msg_register_account.clone()],
                Some("Test Creating interchain account".to_string()),
                &[],
                Some(OPERATION_TIMEOUT),
                key,
            )
            .await;
        info!("Sent MsgRegisterAccount with response {:?}", send_res);
        if send_res.is_err() {
            delay_for(Duration::from_secs(20)).await;
            continue;
        } else {
            return Ok(send_res.unwrap().txhash);
        }
    }
    return Err(CosmosGrpcError::BadResponse(
        "Can't create account".to_string(),
    ));
}

/// Attempts to identify each interchain account owned by members of `owner_keys` via grpc queries
/// the resulting Vec should have owner_keys[i] = i's interchain account
pub async fn identify_interchain_accounts(
    contact: &Contact, // The
    owner_keys: Vec<CosmosPrivateKey>,
    qc: IcaQueryClient<Channel>,
    connection_id: String,
    timeout: Option<Duration>,
) -> Vec<Result<CosmosAddress, CosmosGrpcError>> {
    let mut futures = vec![];
    for owner in owner_keys {
        let fut =
            get_interchain_account(contact, owner, qc.clone(), connection_id.clone(), timeout);
        futures.push(fut);
    }

    join_all(futures).await
}

/// Attempts to identify the interchain account owned by `key` over ibc connection `connection_id`
pub async fn get_interchain_account(
    contact: &Contact,
    key: CosmosPrivateKey,
    qc: IcaQueryClient<Channel>,
    connection_id: String,
    timeout: Option<Duration>,
) -> Result<CosmosAddress, CosmosGrpcError> {
    let mut qc = qc;
    let timeout = match timeout {
        Some(t) => t,
        None => OPERATION_TIMEOUT,
    };
    let owner = key.to_address(&contact.get_prefix()).unwrap().to_string();

    let start = Instant::now();
    while Instant::now() - start < timeout {
        let account = qc
            .interchain_account_from_address(QueryInterchainAccountFromAddressRequest {
                owner: owner.clone(),
                connection_id: connection_id.clone(),
            })
            .await;
        info!(
            "Querying for account owned by {:?}, got account query response {:?}",
            owner, account,
        );
        if account.is_err() {
            info!("Delaying, since the account did not exist");
            delay_for(Duration::from_secs(20)).await;
            continue;
        }

        let account = account.unwrap().into_inner().interchain_account_address;
        let address = CosmosAddress::from_bech32(account)
            .map_err(|e| CosmosGrpcError::BadInput(e.to_string()))?;
        return Ok(address);
    }
    Err(CosmosGrpcError::BadResponse(
        "Can't get interchain account".to_string(),
    ))
}

// send tokens to account
pub async fn send_tokens_to_account_with_retries(
    contact: &Contact,
    key: CosmosPrivateKey,
    receiver: String,
    coin: &Coin,
) -> Result<String, CosmosGrpcError> {
    let coin_vec = vec![coin.clone()];
    let send_tokens = MsgSend {
        from_address: key.to_address(&contact.get_prefix()).unwrap().to_string(),
        to_address: receiver,
        amount: coin_vec,
    };
    info!("Submitting MsgSend {:?}", send_tokens);

    let send_tokens = Msg::new(MSG_SEND_TOKENS_URL, send_tokens);
    let mut retries = 5;
    loop {
        let send_res = contact
            .send_message(
                &[send_tokens.clone()],
                Some("Test Creating interchain account".to_string()),
                &[],
                Some(OPERATION_TIMEOUT),
                key,
            )
            .await;
        info!("Sent MsgSend with response {:?}", send_res);
        if send_res.is_err() {
            if retries > 0 {
                retries -= 1;
                delay_for(Duration::from_secs(10)).await;
                continue;
            }
            return Err(CosmosGrpcError::BadResponse(
                "Message not submitted".to_string(),
            ));
        } else {
            return Ok(send_res.unwrap().txhash);
        }
    }
}

////////////////////////////////// TEST INTERCHAIN ACCOUNT THEFT

/// Attempts to submit messages to a foreign chain and gain control of another interchain account or native account
pub async fn test_ica_account_theft(
    gravity_contact: &Contact,
    gravity_conn_id: String,
    gravity_users: Vec<(CosmosPrivateKey, Address)>,
    gravity_to_ibc_icas: Vec<Address>,
    ibc_contact: &Contact,
    ibc_conn_id: String,
    ibc_users: Vec<(CosmosPrivateKey, Address)>,
    ibc_to_gravity_icas: Vec<Address>,
) -> Result<(), CosmosGrpcError> {
    info!("\n\n\nAttempting theft over interchain accounts!\n\n\n");

    let gravity_theft_receiver: Address = get_cosmos_key(&*ADDRESS_PREFIX).1;
    let ibc_theft_receiver: Address = get_cosmos_key(&*IBC_ADDRESS_PREFIX).1;

    let theft_amount = Coin {
        amount: "10".to_string(),
        denom: (*STAKING_TOKEN).clone(),
    };

    let gravity_to_ibc_theft_attempt = try_ica_steal_tokens(
        gravity_contact,
        gravity_conn_id,
        gravity_users,
        gravity_to_ibc_icas,
        gravity_theft_receiver,
        theft_amount.clone(),
        None,
    );
    let ibc_to_gravity_theft_attempt = try_ica_steal_tokens(
        ibc_contact,
        ibc_conn_id,
        ibc_users,
        ibc_to_gravity_icas,
        ibc_theft_receiver,
        theft_amount.clone(),
        None,
    );

    let res = join_all([gravity_to_ibc_theft_attempt, ibc_to_gravity_theft_attempt]).await;
    for r in res {
        r?;
    }

    info!("\n\n\nTheft attempt failed!\n\n\n");
    Ok(())
}

/// Attempt to steal tokens into one receiver account across every user against all other users
/// In other words, an environment of pure chaos
pub async fn try_ica_steal_tokens(
    contact: &Contact,                       // src chain point of contact
    conn_id: String,                         // src chain ICA connection id
    users: Vec<(CosmosPrivateKey, Address)>, // dst chain sender
    icas: Vec<Address>,                      // dst chain receiver
    receiver: Address,                       // src chain ICA initiator
    coin: Coin,                              // amount to try and send
    ibc_resolution_delay: Option<Duration>,  // how long to wait for IBC relaying to happen
) -> Result<(), CosmosGrpcError> {
    // Ensure the theft deposit address is empty
    let balance = contact.get_balances(receiver).await?;
    assert!(
        balance.is_empty(),
        "Expected theft deposit address to start with no balances!"
    );

    // Each account will be used as a thief to try and steal tokens from all other interchain accounts
    for (i, (thief_key, _thief_address)) in users.into_iter().enumerate() {
        let mut msgs = vec![];
        for (j, victim_address) in icas.clone().into_iter().enumerate() {
            if i == j {
                // Do not try to steal from the thief's own ICA
                continue;
            }

            // Batch the sends for quicker execution
            let msg = format_send_tokens(victim_address, receiver, coin.clone());
            msgs.push(msg);

            if msgs.len() % 15 == 0 {
                let _res =
                    submit_multimsg_tx(contact, thief_key, conn_id.clone(), &msgs.clone()).await;

                msgs.clear();
                msgs.resize(0, Default::default());
            }
        }
        // Clear any missed msgs
        if msgs.len() > 0 {
            let _res = submit_multimsg_tx(contact, thief_key, conn_id.clone(), &msgs);
        }
    }

    let timeout = match ibc_resolution_delay {
        Some(dur) => dur,
        None => Duration::from_secs(240),
    };
    delay_for(timeout).await;

    // Check that the theft failed each time
    let balance = contact.get_balances(receiver).await?;
    assert!(
        balance.is_empty(),
        "!!!Discovered evidence of successful ICA theft!!! {:?}",
        balance
    );

    Ok(())
}

/// Creates an Any encoded x/bank MsgSend of `coins` from `sender` to `receiver`
pub fn format_send_tokens(
    sender: CosmosAddress,   // dst chain sender
    receiver: CosmosAddress, // dst chain receiver
    coin: Coin,              // amount to try and send
) -> Any {
    let msg = MsgSend {
        from_address: sender.to_string(),
        to_address: receiver.to_string(),
        amount: vec![coin.clone()],
    };
    encode_any(msg, MSG_SEND_TYPE_URL)
}

/// Attempts to send tokens from `sender` to `receiver` via ICA connection `conn_id`, signed by `ica_owner`
pub async fn try_ica_send_tokens_with_retries(
    contact: &Contact,           // src chain point of contact
    conn_id: String,             // src chain ICA connection id
    sender: CosmosAddress,       // dst chain sender
    receiver: CosmosAddress,     // dst chain receiver
    ica_owner: CosmosPrivateKey, // src chain ICA initiator
    coin: Coin,                  // amount to try and send
) -> Result<TxResponse, CosmosGrpcError> {
    let any = format_send_tokens(sender, receiver, coin);

    let mut retries = 5;
    loop {
        let res = submit_tx(contact, ica_owner, conn_id.clone(), any.clone()).await;
        match &res {
            Ok(r) => {
                info!(
                    "Successfully sent tx to execute on dst chain: raw_log={:?}",
                    r.raw_log
                )
            }
            Err(e) => {
                if retries > 0 {
                    retries -= 1;
                    delay_for(Duration::from_secs(10)).await;
                    continue;
                }
                error!("Failed to send tx to execute on dst chain! {:?}", e)
            }
        }

        return res;
    }
}

////////////////////////////////// ICA HELPER FUNCTIONS

/// Submits a given (Any encoded) `msg` to `contact` using ICA.
/// The tx will be sent over `connection_id` for execution
pub async fn submit_tx(
    contact: &Contact,
    sender_key: CosmosPrivateKey,
    connection_id: String,
    msg: Any,
) -> Result<TxResponse, CosmosGrpcError> {
    // send
    let msg_send = MsgSubmitTx {
        owner: sender_key
            .to_address(&contact.get_prefix())
            .unwrap()
            .to_string(),
        connection_id,
        msgs: vec![msg],
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

    send_res
}

/// Submits a tx with the given collection of (Any encoded) `msgs` to `contact` using ICA.
/// The tx will be sent over `connection_id` for execution
pub async fn submit_multimsg_tx(
    contact: &Contact,
    sender_key: CosmosPrivateKey,
    connection_id: String,
    msgs: &[Any],
) -> Result<TxResponse, CosmosGrpcError> {
    // send
    let msg_send = MsgSubmitTx {
        owner: sender_key
            .to_address(&contact.get_prefix())
            .unwrap()
            .to_string(),
        connection_id,
        msgs: msgs.to_vec(),
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

    send_res
}
