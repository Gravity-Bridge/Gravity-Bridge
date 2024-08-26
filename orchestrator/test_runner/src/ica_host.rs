use std::str::FromStr;
/// Tests basic interchain accounts functionality
use std::time::{Duration, Instant};

use cosmos_gravity::send::MSG_SEND_TO_ETH_TYPE_URL;
use cosmos_gravity::utils::get_reasonable_send_to_eth_fee;
use deep_space::client::send::TransactionResponse;
use deep_space::error::CosmosGrpcError;
use deep_space::utils::encode_any;
use deep_space::{Address, Coin, Contact, CosmosPrivateKey, Msg, PrivateKey};
use gravity_proto::cosmos_sdk_proto::cosmos::params::v1beta1::ParamChange;
use gravity_proto::cosmos_sdk_proto::ibc::applications::interchain_accounts::controller::v1::QueryInterchainAccountRequest;
use gravity_proto::cosmos_sdk_proto::ibc::applications::interchain_accounts::controller::v1::QueryParamsRequest as ControllerQueryParamsRequest;
use gravity_proto::cosmos_sdk_proto::ibc::applications::interchain_accounts::host::v1::QueryParamsRequest as HostQueryParamsRequest;
use gravity_proto::gravity::{MsgSendToEth, QueryDenomToErc20Request};
use gravity_proto::gravity_test::gaia::icaauth::v1::{MsgRegisterAccount, MsgSubmitTx};
use gravity_utils::num_conversion::one_atom;
use num256::Uint256;

use tokio::time::sleep;
use tonic::transport::Channel;
use web30::client::Web3;
use gravity_proto::cosmos_sdk_proto::cosmos::params::v1beta1::ParameterChangeProposal;
use gravity_proto::cosmos_sdk_proto::ibc::core::channel::v1::query_client::QueryClient as IbcChannelQueryClient;
use gravity_proto::cosmos_sdk_proto::ibc::applications::interchain_accounts::controller::v1::query_client::QueryClient as ICAControllerQueryClient;
use gravity_proto::cosmos_sdk_proto::ibc::applications::interchain_accounts::host::v1::query_client::QueryClient as ICAHostQueryClient;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use clarity::Address as EthAddress;

use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::happy_path_v2::deploy_cosmos_representing_erc20_and_check_adoption;
use crate::ibc_auto_forward::get_channel;
use crate::utils::{footoken_metadata, get_erc20_balance_safe, vote_yes_on_proposals};
use crate::{get_fee, IBC_STAKING_TOKEN, OPERATION_TIMEOUT, STAKING_TOKEN, TOTAL_TIMEOUT};
use crate::{
    get_ibc_chain_id,
    utils::{create_default_test_config, start_orchestrators, ValidatorKeys},
    COSMOS_NODE_GRPC, IBC_ADDRESS_PREFIX, IBC_NODE_GRPC,
};

pub const MSG_REGISTER_ACCOUNT_TYPE_URL: &str = "/gaia.icaauth.v1.MsgRegisterAccount";
pub const MSG_SUBMIT_TX_TYPE_URL: &str = "/gaia.icaauth.v1.MsgSubmitTx";

// ---------------------------------------TEST FUNCTIONS---------------------------------------

/// Runs the "happy-path" functionality of the Interchain Accounts (ICA) Host Module on Gravity Bridge Chain:
/// 1. Enable Host on Gravity and Controller on the IBC test chain
/// 2. Register an Interchain Account controlled by the IBC test chain and fund it with footoken
/// 3. Deploy an ERC20 for footoken on Ethereum
/// 4. Submit a MsgSendToEth to the ICA Controller module
pub async fn ica_host_happy_path(
    web30: &Web3,
    grpc_client: GravityQueryClient<Channel>,
    gravity_contact: &Contact,
    ibc_contact: &Contact,
    keys: Vec<ValidatorKeys>,
    ibc_keys: Vec<CosmosPrivateKey>,
    gravity_address: EthAddress,
) {
    let mut grpc_client = grpc_client;
    let no_relay_market_config = create_default_test_config();
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;

    let gravity_channel_qc = IbcChannelQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect channel query client");
    let ica_controller_qc = ICAControllerQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Cound not connect ica controller query client");

    // Wait for the ibc channel to be created and find the channel ids
    let channel_id_timeout = Duration::from_secs(60 * 5);
    let gravity_channel = get_channel(
        gravity_channel_qc,
        get_ibc_chain_id(),
        Some(channel_id_timeout),
    )
    .await
    .expect("Could not find gravity-test-1 channel");
    let gravity_connection_id = gravity_channel.connection_hops[0].clone();

    info!("\n\n!!!!!!!!!! Start ICA Host Happy Path Test !!!!!!!!!!\n\n");
    enable_ica_host(gravity_contact, &keys).await;
    enable_ica_controller(ibc_contact, &keys).await;

    let ibc_fee = Coin {
        amount: 1u8.into(),
        denom: IBC_STAKING_TOKEN.to_string(),
    };
    let zero_fee = Coin {
        amount: 0u8.into(),
        denom: STAKING_TOKEN.to_string(),
    };

    let ica_owner = ibc_keys[0];
    let ica_owner_addr = ica_owner.to_address(&IBC_ADDRESS_PREFIX).unwrap();
    let ica_addr: String = get_or_register_ica(
        ibc_contact,
        ica_controller_qc.clone(),
        ica_owner,
        ica_owner_addr.to_string(),
        gravity_connection_id.clone(),
        ibc_fee.clone(),
    )
    .await
    .expect("Could not get/register interchain account");
    let ica_address = Address::from_bech32(ica_addr).expect("invalid interchain account address?");

    info!("Funding interchain account");
    let fund_amt = Coin {
        amount: one_atom(),
        denom: STAKING_TOKEN.to_string(),
    };
    gravity_contact
        .send_coins(
            fund_amt,
            Some(zero_fee),
            ica_address,
            Some(OPERATION_TIMEOUT),
            keys[0].validator_key,
        )
        .await
        .expect("Failed to fund ICA");

    let footoken = footoken_metadata(gravity_contact).await;
    let footoken_deployed = grpc_client
        .denom_to_erc20(QueryDenomToErc20Request {
            denom: footoken.base.clone(),
        })
        .await;
    let erc20_contract = match footoken_deployed {
        Ok(res) => EthAddress::from_str(&res.into_inner().erc20)
            .expect("invalid erc20 returned from grpc query"),
        Err(_) => {
            deploy_cosmos_representing_erc20_and_check_adoption(
                gravity_address,
                web30,
                Some(keys.clone()),
                &mut grpc_client,
                false,
                footoken.clone(),
            )
            .await
        }
    };

    let token_to_send_to_eth = footoken.base;
    let amount_to_bridge: Uint256 = one_atom();
    let chain_fee: Uint256 = 500u64.into(); // A typical chain fee is 2 basis points, this gives us a bit of wiggle room
    let send_to_user_coin = Coin {
        denom: token_to_send_to_eth.clone(),
        amount: amount_to_bridge + chain_fee + one_atom(),
    };
    let send_to_eth_coin = Coin {
        denom: token_to_send_to_eth.clone(),
        amount: amount_to_bridge,
    };
    let chain_fee_coin = Coin {
        denom: token_to_send_to_eth.clone(),
        amount: chain_fee,
    };

    // send the user some footoken
    gravity_contact
        .send_coins(
            send_to_user_coin.clone(),
            Some(get_fee(None)),
            ica_address,
            Some(TOTAL_TIMEOUT),
            keys[0].validator_key,
        )
        .await
        .unwrap();

    let simple_fee = get_fee(Some(token_to_send_to_eth.clone()));
    send_to_eth_via_ica_and_confirm(
        web30,
        ibc_contact,
        gravity_contact,
        ica_address,
        ica_owner,
        ica_owner_addr,
        gravity_connection_id.clone(),
        keys[2].eth_key.to_address(),
        send_to_eth_coin,
        simple_fee.clone(),
        Some(chain_fee_coin),
        erc20_contract,
    )
    .await;

    info!("Successful ICA Host Happy Path Test");
}

// ---------------------------------------HELPER FUNCTIONS---------------------------------------

/// Submits a MsgRegisterAccount to x/icaauth to create an account over `connection_id` for `owner`
pub async fn register_interchain_account(
    contact: &Contact,
    owner_key: impl PrivateKey,
    owner: String,
    connection_id: String,
    fee: Coin,
) -> Result<TransactionResponse, CosmosGrpcError> {
    let register = MsgRegisterAccount {
        owner,
        connection_id,
        version: String::new(),
    };
    let register_msg = Msg::new(MSG_REGISTER_ACCOUNT_TYPE_URL.to_string(), register);
    contact
        .send_message(
            &[register_msg],
            None,
            &[fee],
            Some(OPERATION_TIMEOUT),
            None,
            owner_key,
        )
        .await
}

/// Queries x/icaauth for `owner`'s interchain account over `connection_id`
pub async fn get_interchain_account_address(
    ica_controller_qc: ICAControllerQueryClient<Channel>,
    owner: String,
    connection_id: String,
    timeout: Option<Duration>,
) -> Result<String, CosmosGrpcError> {
    let timeout = timeout.unwrap_or(OPERATION_TIMEOUT);
    let start = Instant::now();
    let mut ica_controller_qc = ica_controller_qc;
    while Instant::now() - start < timeout {
        let res = ica_controller_qc
            .interchain_account(QueryInterchainAccountRequest {
                owner: owner.clone(),
                connection_id: connection_id.clone(),
            })
            .await
            .map(|r| r.into_inner().address)
            .map_err(|e| CosmosGrpcError::BadResponse(e.to_string()));
        if res.is_ok() {
            return res;
        }
        sleep(Duration::from_secs(5)).await;
    }
    Err(CosmosGrpcError::BadResponse(format!(
        "Failed to get account after {timeout:?}"
    )))
}

/// Either locates an already registered interchain account or creates one for the given `ctrlr`, over connection `ctrl_to_host_conn_id`
pub async fn get_or_register_ica(
    ctrl_contact: &Contact,
    ctrl_qc: ICAControllerQueryClient<Channel>,
    ctrlr_key: impl PrivateKey,
    ctrlr_addr: String,
    ctrl_to_host_conn_id: String,
    fee: Coin,
) -> Result<String, CosmosGrpcError> {
    info!("Finding/Making ICA for {ctrlr_addr} on conneciton {ctrl_to_host_conn_id}");
    let ica_addr: String;
    let ica_already_exists = get_interchain_account_address(
        ctrl_qc.clone(),
        ctrlr_addr.to_string(),
        ctrl_to_host_conn_id.clone(),
        None,
    )
    .await;
    let ica = if let Ok(ica_addr) = ica_already_exists {
        info!("Interchain account {ica_addr} already registered");
        ica_addr
    } else {
        let register_res = register_interchain_account(
            ctrl_contact,
            ctrlr_key.clone(),
            ctrlr_addr.clone(),
            ctrl_to_host_conn_id.clone(),
            fee.clone(),
        )
        .await?;
        info!("Registered Interchain Account: {}", register_res.raw_log());

        ica_addr = get_interchain_account_address(
            ctrl_qc.clone(),
            ctrlr_addr,
            ctrl_to_host_conn_id.clone(),
            Some(TOTAL_TIMEOUT),
        )
        .await?;
        info!("Discovered interchain account with address {ica_addr:?}");
        ica_addr
    };
    Ok(ica)
}

/// Creates and ratifies a ParameterChangeProposal to enable the ICA Host module and allow all messages
/// Note: Skips governance if the host module is already enabled
pub async fn enable_ica_host(
    contact: &Contact, // Src chain's deep_space client
    keys: &[ValidatorKeys],
) {
    let mut host_qc = ICAHostQueryClient::connect(contact.get_url())
        .await
        .expect("Unable to connect to ica host query client");
    let host_params = host_qc
        .params(HostQueryParamsRequest {})
        .await
        .unwrap()
        .into_inner()
        .params
        .expect("No ica host params returned?");
    if host_params.host_enabled
        && host_params
            .allow_messages
            .first()
            .map(|m| m == "*")
            .unwrap_or(false)
    {
        info!("ICA Host already enabled, skipping governance vote");
        return;
    } else {
        info!("Host params are {host_params:?}: Enabling ICA Host via governance, will set AllowMessages to [\"*\"]");
    }

    let deposit = Coin {
        amount: one_atom() * 100u8.into(),
        denom: STAKING_TOKEN.clone(),
    };
    let fee = Coin {
        amount: 0u8.into(),
        denom: STAKING_TOKEN.clone(),
    };
    let res = contact
        .submit_parameter_change_proposal(
            ParameterChangeProposal {
                title: "Enable ICA Host".to_string(),
                description: "Enable ICA Host".to_string(),
                changes: vec![
                    // subspace defined at ibc-go/modules/apps/27-interchain-accounts/host/types/keys.go
                    // keys defined at     ibc-go/modules/apps/27-interchain-accounts/host/types/params.go
                    ParamChange {
                        subspace: "icahost".to_string(),
                        key: "HostEnabled".to_string(),
                        value: "true".to_string(),
                    },
                    ParamChange {
                        subspace: "icahost".to_string(),
                        key: "AllowMessages".to_string(),
                        value: "[\"*\"]".to_string(),
                    },
                ],
            },
            deposit,
            fee,
            keys[0].validator_key,
            Some(OPERATION_TIMEOUT),
        )
        .await;
    vote_yes_on_proposals(contact, keys, None).await;
    wait_for_proposals_to_execute(contact).await;
    trace!("Gov proposal executed with {:?}", res);
}

/// Creates and ratifies a ParameterChangeProposal to enable the ICA Controller module
pub async fn enable_ica_controller(
    contact: &Contact, // Src chain's deep_space client
    keys: &[ValidatorKeys],
) {
    let mut controller_qc = ICAControllerQueryClient::connect(contact.get_url())
        .await
        .expect("Unable to connect to ica controller query client");
    let controller_params = controller_qc
        .params(ControllerQueryParamsRequest {})
        .await
        .unwrap()
        .into_inner()
        .params
        .expect("No ica controller params returned?");
    if controller_params.controller_enabled {
        info!("ICA Controller already enabled, skipping governance vote");
        return;
    } else {
        info!("Enabling ICA Controller via governance");
    }

    let deposit = Coin {
        amount: one_atom() * 100u8.into(),
        denom: STAKING_TOKEN.clone(),
    };
    let fee = Coin {
        amount: 0u8.into(),
        denom: STAKING_TOKEN.clone(),
    };
    let res = contact
        .submit_parameter_change_proposal(
            ParameterChangeProposal {
                title: "Enable ICA Controller".to_string(),
                description: "Enable ICA Controller".to_string(),
                changes: vec![
                    // subspace defined at ibc-go/modules/apps/27-interchain-accounts/controller/types/keys.go
                    // keys defined at     ibc-go/modules/apps/27-interchain-accounts/controller/types/params.go
                    ParamChange {
                        subspace: "icacontroller".to_string(),
                        key: "icacontroller".to_string(),
                        value: "true".to_string(),
                    },
                ],
            },
            deposit,
            fee,
            keys[0].validator_key,
            Some(OPERATION_TIMEOUT),
        )
        .await;
    vote_yes_on_proposals(contact, keys, None).await;
    wait_for_proposals_to_execute(contact).await;
    trace!("Gov proposal executed with {:?}", res);
}

/// Very similar to `send_to_eth_and_confirm()`, but submits the MsgSendToEth to the ICA Controller chain
#[allow(clippy::too_many_arguments)]
pub async fn send_to_eth_via_ica_and_confirm(
    web30: &Web3,
    controller_contact: &Contact, // Contact for the ICA Controller chain
    gravity_contact: &Contact,    // Contact for Gravity chain
    grav_ica: Address,            // The address of the ICA on Gravity sending tokens to Ethereum
    ctrl_owner: impl PrivateKey,  // The key for the ICA owner account on the ICA Controller chain
    ctrl_owner_addr: Address, // The address for the ICA owner account on the ICA Controller chain
    ctrl_to_grav_conn_id: String, // The connection id (e.g. connection-0) on the controller chain which connects to Gravity chain
    eth_receiver: EthAddress,     // The Eth address which should receive the funds
    send_to_eth_coin: Coin,       // The funds to send to Ethereum
    bridge_fee_coin: Coin,        // The amount to pay a relayer for bridging the funds
    cosmos_chain_fee_coin: Option<Coin>, // If Gravity's MinChainFeeBasisPoints param is set, the amount needed to fulfil that fee req
    erc20_contract: EthAddress,          // The ERC20, used for balance change verification
) -> bool {
    let starting_balance = get_erc20_balance_safe(erc20_contract, web30, eth_receiver)
        .await
        .unwrap();
    let amount_to_bridge = send_to_eth_coin.amount;
    let res = send_to_eth_via_ica(
        controller_contact,
        gravity_contact,
        grav_ica,
        ctrl_owner,
        ctrl_owner_addr,
        ctrl_to_grav_conn_id,
        eth_receiver,
        send_to_eth_coin,
        bridge_fee_coin,
        cosmos_chain_fee_coin,
    )
    .await
    .unwrap();
    info!("Send to eth res {:?}", res);
    info!("Locked up {} to send to Cosmos", amount_to_bridge);

    info!("Waiting for batch to be signed and relayed to Ethereum");

    let start = Instant::now();
    // overly complicated retry logic allows us to handle the possibility that gas prices change between blocks
    // and cause any individual request to fail.

    while Instant::now() - start < TOTAL_TIMEOUT {
        let new_balance = get_erc20_balance_safe(erc20_contract, web30, eth_receiver).await;
        // only keep trying if our error is gas related
        if new_balance.is_err() {
            continue;
        }
        let balance = new_balance.unwrap();
        if balance - starting_balance == amount_to_bridge {
            info!("Successfully bridged {} to Ethereum!", amount_to_bridge);
            assert!(balance == amount_to_bridge);
            return true;
        } else if balance - starting_balance != 0u8.into() {
            error!("Expected {} but got {} instead", amount_to_bridge, balance);
            return false;
        }
        sleep(Duration::from_secs(1)).await;
    }
    error!("Timed out waiting for ethereum balance");
    false
}

/// Very similar to `send_to_eth()` but the MsgSendToEth is submitted to the ICA Controller chain
#[allow(clippy::too_many_arguments)]
pub async fn send_to_eth_via_ica(
    ctrl_contact: &Contact,       // Contact for the ICA Controller chain
    gravity_contact: &Contact,    // Contact for Gravity chain
    ica_address: Address,         // The address of the ICA on Gravity sending tokens to Ethereum
    owner_key: impl PrivateKey,   // The key for the ICA owner account on the ICA Controller chain
    owner_addr: Address, // The address for the ICA owner account on the ICA Controller chain
    ctrl_to_host_conn_id: String, // The connection id (e.g. connection-0) on the controller chain which connects to Gravity chain
    destination: EthAddress,      // The Eth address which should receive the funds
    amount: Coin,                 // The funds to send
    bridge_fee: Coin,             // The amount to pay a relayer for bridging the funds
    chain_fee: Option<Coin>, // If Gravity's MinChainFeeBasisPoints param is set, the amount needed to fulfil that fee req
) -> Result<TransactionResponse, CosmosGrpcError> {
    if amount.denom != bridge_fee.denom {
        return Err(CosmosGrpcError::BadInput(format!(
            "{} {} is an invalid denom set for SendToEth you must pay ethereum fees in the same token your sending",
            amount.denom, bridge_fee.denom,
        )));
    }
    let chain_fee = match chain_fee {
        Some(fee) => fee,
        None => Coin {
            amount: get_reasonable_send_to_eth_fee(gravity_contact, amount.amount)
                .await
                .expect("Unable to get reasonable SendToEth fee"),
            denom: amount.denom.clone(),
        },
    };
    if amount.denom != chain_fee.denom {
        return Err(CosmosGrpcError::BadInput(format!(
            "{} {} is an invalid denom set for SendToEth you must pay chain fees in the same token your sending",
            amount.denom, chain_fee.denom,
        )));
    }
    let balances = gravity_contact.get_balances(ica_address).await.unwrap();
    let mut found = false;
    for balance in balances {
        if balance.denom == amount.denom {
            let total_amount = amount.amount + (bridge_fee.amount + chain_fee.amount);
            if balance.amount < total_amount {
                return Err(CosmosGrpcError::BadInput(format!(
                    "Insufficient balance of {} to send {}, only have {}",
                    amount.denom, total_amount, balance.amount,
                )));
            }
            found = true;
        }
    }
    if !found {
        return Err(CosmosGrpcError::BadInput(format!(
            "No balance of {} to send",
            amount.denom,
        )));
    }

    let msg_send_to_eth = MsgSendToEth {
        sender: ica_address.to_string(),
        eth_dest: destination.to_string(),
        amount: Some(amount.into()),
        bridge_fee: Some(bridge_fee.into()),
        chain_fee: Some(chain_fee.into()),
    };
    info!(
        "Sending to Ethereum with MsgSendToEth: {:?}",
        msg_send_to_eth
    );
    let msg = encode_any(msg_send_to_eth, MSG_SEND_TO_ETH_TYPE_URL);

    let ica_submit = MsgSubmitTx {
        connection_id: ctrl_to_host_conn_id,
        owner: owner_addr.to_string(),
        msgs: vec![msg],
    };
    info!("Submitting MsgSubmitTx: {ica_submit:?}");
    let ica_msg = Msg::new(MSG_SUBMIT_TX_TYPE_URL, ica_submit);
    let ctrl_fee = Coin {
        amount: 100u8.into(),
        denom: IBC_STAKING_TOKEN.to_string(),
    };
    ctrl_contact
        .send_message(
            &[ica_msg],
            None,
            &[ctrl_fee],
            Some(OPERATION_TIMEOUT),
            None,
            owner_key,
        )
        .await
}
