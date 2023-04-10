use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::happy_path::{test_erc20_deposit_panic, test_erc20_deposit_result, test_valset_update};
use crate::happy_path_v2::deploy_cosmos_representing_erc20_and_check_adoption;
use crate::ibc_auto_forward::{
    get_channel_id, get_ibc_balance, ibc_transfer, setup_gravity_auto_forwards, test_ibc_transfer,
};
use crate::ibc_metadata::submit_and_pass_ibc_metadata_proposal;
use crate::utils::{get_user_key, send_one_eth, vote_yes_on_proposals};
use crate::{
    create_default_test_config, footoken_metadata, get_gravity_chain_id, get_ibc_chain_id, one_eth,
    start_orchestrators, CosmosAddress, EthPrivateKey, GravityQueryClient, ValidatorKeys,
    ADDRESS_PREFIX, COSMOS_NODE_GRPC, GRAVITY_MODULE_ADDRESS, IBC_ADDRESS_PREFIX, IBC_NODE_GRPC,
    MINER_ADDRESS, MINER_PRIVATE_KEY, OPERATION_TIMEOUT, STAKING_TOKEN,
};
use clarity::utils::display_uint256_as_address;
use clarity::Address as EthAddress;
use cosmos_gravity::proposals::{
    submit_monitored_erc20s_proposal, MonitoredErc20TokensProposalJson,
};
use cosmos_gravity::query::get_gravity_params;
use cosmos_gravity::send::send_to_eth;
use deep_space::error::CosmosGrpcError;
use deep_space::{Coin, Contact, CosmosPrivateKey, PrivateKey};
use ethereum_gravity::send_to_cosmos::send_to_cosmos;
use futures::future::join;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::DenomUnit;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::{
    v1beta1::query_client::QueryClient as BankQueryClient, v1beta1::Metadata,
};
use gravity_proto::cosmos_sdk_proto::ibc::applications::transfer::v1::query_client::QueryClient as IbcTransferQueryClient;
use gravity_proto::cosmos_sdk_proto::ibc::core::channel::v1::query_client::QueryClient as IbcChannelQueryClient;
use gravity_proto::gravity::query_client::QueryClient;
use gravity_proto::gravity::QueryMonitoredErc20Addresses;
use gravity_utils::num_conversion::one_atom;
use gravity_utils::types::{
    BatchRelayingMode, BatchRequestMode, RelayerConfig, ValsetRelayingMode,
};
use num256::Uint256;
use relayer::main_loop::single_relayer_iteration;
use std::ops::Mul;
use std::time::Duration;
use tokio::time::sleep;
use tonic::transport::Channel;
use web30::client::Web3;
use web30::jsonrpc::error::Web3Error;
use web30::types::SendTxOption;

const FOOTOKEN_ALLOCATION: u64 = 100u64; // Validators will have 100 FOO to spend

/// Creates bridge activity, sets the MonitoredTokenAddresses with the 3 deployed ERC20's + footoken
/// then tries several ways of halting the chain before finally verifying that an incorrect balance
/// successfully halts the chain
///
/// Note: Not idempotent - will fail on successive runs
#[allow(clippy::too_many_arguments)]
pub async fn cross_bridge_balance_test(
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
    let ibc_bank_qc = BankQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect ibc bank query client");
    let ibc_transfer_qc = IbcTransferQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect ibc ibc-transfer query client");
    let ibc_channel_qc = IbcChannelQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect ibc channel query client");
    let gravity_bank_qc = BankQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect gravity bank query client");
    let gravity_ibc_transfer_qc = IbcTransferQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect gravity ibc-transfer query client");
    let gravity_channel_qc = IbcChannelQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect gravity channel query client");
    let channel_id_timeout = Duration::from_secs(60 * 5);
    let gravity_channel_id = get_channel_id(
        gravity_channel_qc,
        get_ibc_chain_id(),
        Some(channel_id_timeout),
    )
    .await
    .expect("Could not find gravity -> ibc-test-1 channel");
    let ibc_channel_id = get_channel_id(
        ibc_channel_qc,
        get_gravity_chain_id(),
        Some(channel_id_timeout),
    )
    .await
    .expect("Could not find ibc -> gravity-test-1 channel");

    // Disable the relayer so that complicated state can exist without race conditions
    let mut no_relayer_config = create_default_test_config();
    no_relayer_config.orchestrator.relayer_enabled = false;
    let mut single_iteration_relayer_config = no_relayer_config.relayer.clone();
    single_iteration_relayer_config.ibc_auto_forward_loop_speed = u64::MAX; // Prevent IBC auto forward execution

    start_orchestrators(
        keys.clone(),
        gravity_address,
        false,
        no_relayer_config.clone(),
    )
    .await;
    let mut grpc = grpc;
    let params = get_gravity_params(&mut grpc)
        .await
        .expect("Failed to get Gravity Bridge module parameters!");

    ////////////// FIRST //////////////
    // Create a Cosmos-Originated asset, set the MonitoredTokenAddresses governance parameter,
    // create bridge activity, run the happy_path functions + IBC auto-forwards (SHOULD NOT HALT)
    let (
        validator_cosmos_keys,
        validator_eth_keys,
        validator_eth_addrs,
        footoken_metadata,
        footoken_erc20,
        ibc_stake,
        ibc_stake_erc20,
    ) = setup(SetupArgs {
        web30: web30.clone(),
        contact: contact.clone(),
        ibc_contact: ibc_contact.clone(),
        grpc: grpc.clone(),
        relayer_config: no_relayer_config.relayer.clone(),
        keys: keys.clone(),
        ibc_keys: ibc_keys.clone(),
        gravity_address,
        gravity_id: params.gravity_id.clone(),
        erc20_addresses: erc20_addresses.clone(),
        vulnerable_erc20_address,
        gravity_ibc_channel_id: gravity_channel_id.clone(),
        gravity_bank_qc,
        gravity_ibc_transfer_qc,
        ibc_ibc_channel_id: ibc_channel_id.clone(),
        ibc_chain_transfer_qc: ibc_transfer_qc.clone(),
        ibc_chain_bank_qc: ibc_bank_qc.clone(),
    })
    .await;

    info!("\n\n\n CREATING COSMOS -> ETH ACTIVITY \n\n\n");
    create_send_to_eth_activity(
        web30,
        grpc.clone(),
        contact,
        gravity_address,
        params.gravity_id.clone(),
        single_iteration_relayer_config.clone(),
        erc20_addresses.clone(),
        footoken_metadata.base.clone(),
        ibc_stake.clone(),
        validator_cosmos_keys.clone(),
        validator_eth_addrs.clone(),
        validator_eth_keys[0],
        validator_cosmos_keys[0],
    )
    .await;

    // Final setup - set the MonitoredTokenAddreses parameter forcing orchestrators assert bridge balances on every
    // claim
    info!("\n\n\n Setting Monitored ERC20s via governance! \n\n\n");
    let mut monitored_erc20s = erc20_addresses.clone();
    monitored_erc20s.push(footoken_erc20);
    monitored_erc20s.push(ibc_stake_erc20);
    monitored_erc20s.push(vulnerable_erc20_address);

    submit_and_pass_monitored_erc20s_proposal(contact, keys.clone(), monitored_erc20s.clone())
        .await;

    info!("\n\n\n CREATING COSMOS -> ETH ACTIVITY \n\n\n");
    create_send_to_cosmos_activity(
        web30,
        validator_cosmos_keys.clone(),
        validator_eth_keys.clone(),
        gravity_address,
        erc20_addresses.clone(),
        vulnerable_erc20_address,
        footoken_erc20,
    )
    .await;

    let relayer_fee = Coin {
        denom: (*STAKING_TOKEN).clone(),
        amount: 0u8.into(),
    };

    // Relay some valsets to test the current configuration (SHOULD NOT HALT)
    let mut valset_relaying_only_config = no_relayer_config.relayer.clone();
    valset_relaying_only_config.valset_relaying_mode = ValsetRelayingMode::EveryValset;
    valset_relaying_only_config.batch_request_mode = BatchRequestMode::None;
    valset_relaying_only_config.batch_relaying_mode = BatchRelayingMode::Altruistic;
    info!("\n\n\n CREATING VALSETS TO TEST BALANCES\n\n\n");

    create_and_execute_attestations(
        keys.clone(),
        validator_eth_keys[0],
        Some(validator_cosmos_keys[0]),
        Some(relayer_fee.clone()),
        contact,
        web30,
        &grpc,
        gravity_address,
        &params.gravity_id,
        &valset_relaying_only_config,
    )
    .await;

    ////////////// SECOND //////////////
    // Try to mess up the balances by sending to Gravity.sol + Gravity Module (SHOULD NOT HALT)
    // A. Try to send to the gravity module, which is not permitted
    let gravity_module = CosmosAddress::from_bech32(GRAVITY_MODULE_ADDRESS.to_string())
        .expect("Invalid Gravity module address");
    let gravity_expected_balance = contact
        .get_balance(gravity_module, ibc_stake.clone())
        .await
        .expect("Unable to get gravity module foo balance");
    let coin_to_send = Coin {
        denom: ibc_stake.clone(),
        amount: one_atom(),
    };
    info!("\n\n\nAttempting to mess up balances by sending to the gravity module\n\n\n");
    let res = contact
        .send_coins(
            coin_to_send,
            None,
            gravity_module,
            Some(OPERATION_TIMEOUT),
            validator_cosmos_keys[0],
        )
        .await;
    info!(
        "Tried an invalid send to gravity module address, should have failed: {:?}",
        res
    );
    assert!(res.is_err());

    let gravity_updated_balance = contact
        .get_balance(gravity_module, ibc_stake.clone())
        .await
        .expect("Unable to get gravity module foo balance");
    assert_eq!(gravity_expected_balance, gravity_updated_balance);

    // B. Send some tokens to the Gravity.sol address, which should have no effect on the chain
    info!("\n\n\n Attempting to mess up balances by sending to Gravity.sol \n\n\n");
    let res = web30
        .erc20_send(
            one_atom(),
            gravity_address,
            footoken_erc20,
            validator_eth_keys[0],
            Some(OPERATION_TIMEOUT),
            vec![
                SendTxOption::GasPriceMultiplier(2.0),
                SendTxOption::GasLimitMultiplier(2.0),
            ],
        )
        .await
        .expect("Unable to send tokens to Gravity.sol");
    info!(
        "Sent footoken erc20 to gravity bridge contract with res: {:?}",
        res
    );

    // Test that the chain is still functioning by creating new valsets
    create_and_execute_attestations(
        keys.clone(),
        validator_eth_keys[0],
        Some(validator_cosmos_keys[0]),
        Some(relayer_fee.clone()),
        contact,
        web30,
        &grpc,
        gravity_address,
        &params.gravity_id,
        &valset_relaying_only_config,
    )
    .await;

    ////////////// THIRD //////////////
    // Check that the IBC token supply changing does not halt the chain
    let coin_to_send = Coin {
        amount: one_atom(),
        denom: ibc_stake.clone(),
    };

    info!("\n\n\n Ensuring that ibc balance changes do not halt the chain \n\n\n");
    let res = ibc_transfer(
        contact,
        coin_to_send.clone().into(),
        gravity_channel_id,
        None,
        validator_cosmos_keys[0],
        ibc_keys[0].to_address(IBC_ADDRESS_PREFIX.as_str()).unwrap(),
        None,
        Some(Duration::from_secs(60 * 5)),
        Some(coin_to_send),
    )
    .await;
    assert!(
        res.is_ok(),
        "Error trying to send ibc stake back to the ibc chain: {:?}",
        res
    );

    // Test that the chain is still functioning by creating new valsets
    create_and_execute_attestations(
        keys.clone(),
        validator_eth_keys[0],
        Some(validator_cosmos_keys[0]),
        Some(relayer_fee.clone()),
        contact,
        web30,
        &grpc,
        gravity_address,
        &params.gravity_id,
        &valset_relaying_only_config,
    )
    .await;

    ////////////// FOURTH //////////////
    // Check that the orchestrator halts by "stealing" from the bridge using theft_erc20_address.
    // This ERC20 has been setup with an unprotected .transferFrom(from, to, amount) function which
    // allows any sender to forcibly share funds from someone else's account.
    let thief = get_user_key(None);
    let thief_address = thief.eth_address;
    send_one_eth(thief_address, web30).await;
    info!("\n\n\n Simulating theft by exploiting vulnerable ERC20 contract \n\n\n");
    let res = steal_from_bridge(
        web30,
        gravity_address,
        vulnerable_erc20_address,
        thief.eth_key,
        one_atom(),
        validator_eth_addrs[0],
    )
    .await;
    info!("Tried to steal from bridge: {res:?}");
    res.unwrap();

    // The next attestation will populate a balance snapshot, but succeed
    test_erc20_deposit_panic(
        web30,
        contact,
        &mut grpc.clone(),
        thief.cosmos_address,
        gravity_address,
        *erc20_addresses.get(0).unwrap(),
        one_eth(),
        None,
        Some(one_eth()),
    )
    .await;

    // Any subsequent attestations will cause the orchestrator to shut down, so the bridge should be locked
    let res = test_erc20_deposit_result(
        web30,
        contact,
        &mut grpc.clone(),
        thief.cosmos_address,
        gravity_address,
        *erc20_addresses.get(0).unwrap(),
        one_eth(),
        Some(Duration::from_secs(180)),
        Some(one_eth()),
    )
    .await;
    assert!(
        res.is_err(),
        "Expected ERC20 deposit to time out due to orchestrator halt, but result is Ok(())"
    );
    error!("{res:?}");

    info!("Successful Cross Bridge Balances Test!")
}

pub struct SetupArgs {
    pub web30: Web3,
    pub contact: Contact,
    pub ibc_contact: Contact,
    pub grpc: GravityQueryClient<Channel>,
    pub relayer_config: RelayerConfig,
    pub keys: Vec<ValidatorKeys>,
    pub ibc_keys: Vec<CosmosPrivateKey>,
    pub gravity_address: EthAddress,
    pub gravity_id: String,
    pub erc20_addresses: Vec<EthAddress>,
    pub vulnerable_erc20_address: EthAddress,
    pub gravity_ibc_channel_id: String,
    pub gravity_ibc_transfer_qc: IbcTransferQueryClient<Channel>,
    pub gravity_bank_qc: BankQueryClient<Channel>,
    pub ibc_ibc_channel_id: String,
    pub ibc_chain_transfer_qc: IbcTransferQueryClient<Channel>,
    pub ibc_chain_bank_qc: BankQueryClient<Channel>,
}

pub async fn setup(
    args: SetupArgs,
) -> (
    Vec<CosmosPrivateKey>, // Vec<Validator Cosmos Keys>
    Vec<EthPrivateKey>,    // Vec<Validator Eth Keys>
    Vec<EthAddress>,       // Vec<Validator Eth Addresses>
    Metadata,              // Footoken
    EthAddress,            // Footoken ERC20 deployed
    String,                // IBC Token denom on gravity chain
    EthAddress,            // IBC Token ERC20 representation
) {
    let SetupArgs {
        web30,
        contact,
        ibc_contact,
        grpc,
        relayer_config,
        keys,
        ibc_keys,
        gravity_address,
        gravity_id,
        erc20_addresses,
        vulnerable_erc20_address,
        gravity_ibc_channel_id,
        gravity_ibc_transfer_qc,
        gravity_bank_qc,
        ibc_ibc_channel_id,
        ibc_chain_transfer_qc,
        ibc_chain_bank_qc,
    } = args;

    let mut erc20s = erc20_addresses.clone();
    erc20s.push(vulnerable_erc20_address);
    let mut grpc = grpc;
    let footoken_metadata = footoken_metadata(&contact).await;
    let mut validator_cosmos_keys = vec![];
    let mut validator_eth_keys = vec![];
    let mut validator_eth_addrs = vec![];
    keys.clone()
        .into_iter()
        .map(|k| {
            validator_cosmos_keys.push(k.validator_key);
            validator_eth_keys.push(k.eth_key);
            validator_eth_addrs.push(k.eth_key.to_address());
        })
        .for_each(drop); // Exhaust the map, actually adding items to the above Vecs
    let coin_to_send = Coin {
        denom: footoken_metadata.base.clone(),
        amount: one_atom().mul(FOOTOKEN_ALLOCATION.into()),
    };
    let fee_coin = Coin {
        denom: footoken_metadata.base.clone(),
        amount: 1u8.into(),
    };
    info!("\n\n\n SENDING ERC20S FROM ETHEREUM TO VALIDATORS ON COSMOS\n\n\n");
    // Send tokens to Gravity addresses:
    let mut sends: Vec<SendToCosmosArgs> = vec![];
    for erc20 in &erc20s {
        sends.push(SendToCosmosArgs {
            sender: *MINER_PRIVATE_KEY,
            dest: validator_cosmos_keys[0]
                .to_address(&ADDRESS_PREFIX)
                .unwrap(),
            amount: one_eth(),
            contract: *erc20,
        });
    }

    send_erc20s_to_cosmos(&web30, gravity_address, sends).await;

    info!(
        "\n\n\n SENDING ERC20S FROM MINER ({}) TO VALIDATORS ON ETHEREUM\n\n\n",
        MINER_ADDRESS.to_string()
    );
    for eth_addr in &validator_eth_addrs {
        for erc20 in &erc20s {
            info!(
                "Sending 10^18 of {} to {}",
                erc20.to_string(),
                eth_addr.to_string()
            );
            let res = web30
                .erc20_send(
                    one_eth(),
                    *eth_addr,
                    *erc20,
                    *MINER_PRIVATE_KEY,
                    Some(Duration::from_secs(30)),
                    vec![
                        SendTxOption::GasLimitMultiplier(2.0),
                        SendTxOption::GasPriceMultiplier(2.0),
                    ],
                )
                .await;
            info!("Sent tokens to validator with res {:?}", res);
        }
    }

    info!("\n\n\n DEPLOYING FOOTOKEN \n\n\n");
    // Deploy an ERC20 for Cosmos-originated IBC auto-forwards + donations to Gravity.sol
    // This call does not depend on an active relayer
    let footoken_erc20 = deploy_cosmos_representing_erc20_and_check_adoption(
        gravity_address,
        &web30,
        None, // Already started the orchestrators with custom config
        &mut grpc,
        false,
        footoken_metadata.clone(),
    )
    .await;

    // Give the validators' Eth accts some footoken erc20
    info!("\n\n\n SENDING FOOTOKEN FROM COSMOS TO VALIDATORS ON ETHEREUM\n\n\n");
    // This will need a relayer to run
    send_tokens_to_eth(
        &contact,
        validator_cosmos_keys[0],
        validator_eth_addrs.clone(),
        coin_to_send.clone(),
        fee_coin.clone(),
    )
    .await;

    info!("\n\n\n SENDING ERC20S FROM ETHEREUM TO VALIDATORS ON COSMOS\n\n\n");
    // These will make their way to cosmos without an orchestrator
    let mut sends: Vec<SendToCosmosArgs> = vec![];
    for erc20 in &erc20s {
        sends.push(SendToCosmosArgs {
            sender: *MINER_PRIVATE_KEY,
            dest: validator_cosmos_keys[0]
                .to_address(&ADDRESS_PREFIX)
                .unwrap(),
            amount: one_eth(),
            contract: *erc20,
        });
    }
    sends.push(SendToCosmosArgs {
        sender: validator_eth_keys[0],
        dest: validator_cosmos_keys[0]
            .to_address(&ADDRESS_PREFIX)
            .unwrap(),
        amount: one_atom(),
        contract: footoken_erc20,
    });

    send_erc20s_to_cosmos(&web30, gravity_address, sends).await;

    info!("\n\n\n SETTING UP IBC AUTO FORWARDING \n\n\n");
    // Wait for the ibc channel to be created and find the channel ids

    // Test an IBC transfer of 1 stake from gravity-test-1 to ibc-test-1
    let sender = keys[0].validator_key;
    let receiver = ibc_keys[0].to_address(&IBC_ADDRESS_PREFIX).unwrap();
    info!("Testing a regular IBC transfer first");
    assert!(
        test_ibc_transfer(
            &contact,
            ibc_chain_bank_qc.clone(),
            ibc_chain_transfer_qc.clone(),
            sender,
            receiver,
            None,
            None,
            gravity_ibc_channel_id.clone(),
            Duration::from_secs(60 * 5),
        )
        .await,
        "Unable to perform a basic IBC transfer"
    );

    setup_gravity_auto_forwards(
        &contact,
        (*IBC_ADDRESS_PREFIX).clone(),
        gravity_ibc_channel_id.clone(),
        validator_cosmos_keys[0],
        &keys,
    )
    .await;

    info!("\n\n\n Creating an erc20 representation of an ibc token \n\n\n");

    let (ibc_denom, ibc_token) = create_ibc_token_erc20_representation(
        &contact,
        keys,
        &ibc_contact,
        ibc_keys,
        ibc_ibc_channel_id,
        gravity_bank_qc,
        gravity_ibc_transfer_qc,
        gravity_address,
        &web30,
        grpc.clone(),
    )
    .await
    .expect("unable to deploy ibc token erc20");

    // Run the relayer for a bit to clear any pending work
    for _ in 0..10 {
        single_relayer_iteration(
            validator_eth_keys[0],
            Some(validator_cosmos_keys[0]),
            Some(fee_coin.clone()),
            &contact,
            &web30,
            &grpc,
            gravity_address,
            &gravity_id,
            &relayer_config,
            true,
        )
        .await;
        sleep(Duration::from_secs(5)).await;
    }

    (
        validator_cosmos_keys,
        validator_eth_keys,
        validator_eth_addrs,
        footoken_metadata,
        footoken_erc20,
        ibc_denom,
        ibc_token,
    )
}

pub async fn submit_and_pass_monitored_erc20s_proposal(
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    monitored_erc20s: Vec<EthAddress>,
) {
    let monitored_erc20s: Vec<String> = monitored_erc20s
        .into_iter()
        .map(|e| e.to_string())
        .collect();

    let res = submit_monitored_erc20s_proposal(
        MonitoredErc20TokensProposalJson {
            title: "Set MonitoredTokenAddresses".to_string(),
            description: "Setting MonitoredTokenAddresses to the test ERC20s".to_string(),
            tokens: monitored_erc20s.clone(),
        },
        Coin {
            // deposit
            amount: one_atom().mul(1000u64.into()),
            denom: (*STAKING_TOKEN).clone(),
        },
        Coin {
            // fee
            amount: 0u64.into(),
            denom: (*STAKING_TOKEN).clone(),
        },
        contact,
        keys[0].validator_key, // proposer
        Some(OPERATION_TIMEOUT),
    )
    .await;

    info!("Gov proposal executed with {:?}", res);
    vote_yes_on_proposals(contact, &keys, None).await;
    wait_for_proposals_to_execute(contact).await;
    let mut gravity_grpc = GravityQueryClient::connect(contact.get_url())
        .await
        .expect("unable to contact gravity grpc");
    let actual_erc20s = gravity_grpc
        .get_monitored_erc20_addresses(QueryMonitoredErc20Addresses {})
        .await
        .expect("Could not obtain MonitoredTokenAddresses!")
        .into_inner()
        .addresses;

    // Check that all the set ERC20s are as expected
    monitored_erc20s
        .into_iter()
        .zip(actual_erc20s.into_iter()) // Pair the input with the query response
        .map(|(exp, act)| assert_eq!(exp, act)) // Add the check
        .for_each(drop); // Tell rust to actually check every value
}

#[allow(clippy::too_many_arguments)]
pub async fn create_send_to_cosmos_activity(
    web30: &Web3,
    validator_cosmos_keys: Vec<CosmosPrivateKey>,
    validator_eth_keys: Vec<EthPrivateKey>,
    gravity_address: EthAddress,
    erc20_addresses: Vec<EthAddress>,
    vulnerable_erc20_address: EthAddress,
    footoken_erc20: EthAddress,
) {
    info!("\n\n\n SENDING ERC20S FROM ETHEREUM TO VALIDATORS ON COSMOS\n\n\n");
    // Send tokens to Gravity addresses:
    let mut sends: Vec<SendToCosmosArgs> = vec![];
    for erc20 in &erc20_addresses {
        sends.push(SendToCosmosArgs {
            sender: *MINER_PRIVATE_KEY,
            dest: validator_cosmos_keys[0]
                .to_address(&ADDRESS_PREFIX)
                .unwrap(),
            amount: one_atom(),
            contract: *erc20,
        });
    }
    sends.push(SendToCosmosArgs {
        sender: validator_eth_keys[0],
        dest: validator_cosmos_keys[0]
            .to_address(&ADDRESS_PREFIX)
            .unwrap(),
        amount: 10u16.into(),
        contract: footoken_erc20,
    });
    sends.push(SendToCosmosArgs {
        sender: validator_eth_keys[1],
        dest: validator_cosmos_keys[1]
            .to_address(&ADDRESS_PREFIX)
            .unwrap(),
        amount: 10u16.into(),
        contract: vulnerable_erc20_address,
    });

    send_erc20s_to_cosmos(web30, gravity_address, sends).await;

    // Create pending IBC Auto forwards which will stay pending

    let foreign_receiver = validator_cosmos_keys[0]
        .to_address(&IBC_ADDRESS_PREFIX)
        .unwrap();
    let mut sends: Vec<SendToCosmosArgs> = vec![];
    for erc20 in erc20_addresses {
        sends.push(SendToCosmosArgs {
            sender: validator_eth_keys[0],
            dest: foreign_receiver,
            amount: one_atom(),
            contract: erc20,
        });
    }
    info!("\n\n\n SENDING ERC20S BACK TO ETHEREUM \n\n\n");
    send_erc20s_to_cosmos(web30, gravity_address, sends).await;
}

/// Sends `coin_to_send` from `sender_keys[i]` to `reciever_addrs[i]` for all i, paying `fee_coin` to bridge
pub async fn send_tokens_to_eth(
    contact: &Contact,
    sender_key: CosmosPrivateKey,
    receiver_addrs: Vec<EthAddress>,
    coin_to_send: Coin,
    fee_coin: Coin,
) {
    let sender_addr = sender_key
        .to_address(ADDRESS_PREFIX.as_str())
        .expect("Invalid sender!");
    for receiver in receiver_addrs {
        let res = send_to_eth(
            sender_key,
            receiver,
            coin_to_send.clone(),
            fee_coin.clone(),
            None,
            fee_coin.clone(),
            contact,
        )
        .await
        .unwrap();
        info!(
            "Sent {}{} tokens from {} to {}{:?}",
            coin_to_send.amount,
            coin_to_send.denom,
            sender_addr,
            receiver.to_string(),
            res
        );
    }
}

pub struct SendToCosmosArgs {
    sender: EthPrivateKey,
    dest: CosmosAddress,
    amount: Uint256,
    contract: EthAddress,
}

pub async fn send_erc20s_to_cosmos(
    web30: &Web3,
    gravity_contract: EthAddress,
    send_args: Vec<SendToCosmosArgs>,
) {
    for send in send_args {
        info!(
            "Sending {} of {} to {}",
            send.amount.to_string(),
            send.contract.to_string(),
            send.dest.to_string()
        );
        let tx_id = send_to_cosmos(
            send.contract,
            gravity_contract,
            send.amount,
            send.dest,
            send.sender,
            None,
            web30,
            vec![],
        )
        .await
        .expect("Failed to send tokens to Cosmos");
        info!("Send to Cosmos txid: {:#066x}", tx_id);

        let _tx_res = web30
            .wait_for_transaction(tx_id, OPERATION_TIMEOUT, None)
            .await
            .expect("Send to cosmos transaction failed to be included into ethereum side");
    }
}

/// Transfers the native "stake" token on the ibc test chain to the gravity test chain, submits a metadata proposal,
/// and deploys an erc20 on ethereum for the foreign token.
#[allow(clippy::too_many_arguments)]
pub async fn create_ibc_token_erc20_representation(
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    ibc_contact: &Contact,
    ibc_keys: Vec<CosmosPrivateKey>,
    ibc_ibc_channel_id: String,
    gravity_bank_qc: BankQueryClient<Channel>,
    gravity_ibc_transfer_qc: IbcTransferQueryClient<Channel>,
    gravity_contract_address: EthAddress,
    web30: &Web3,
    gravity_qc: GravityQueryClient<Channel>,
) -> Result<(String, EthAddress), CosmosGrpcError> {
    let mut gravity_qc = gravity_qc;
    let foreign_denom: String = "stake".to_string();
    let zero_fee = Coin {
        amount: 0u8.into(),
        denom: foreign_denom.clone(),
    };
    // Each validator holds 10000000000stake, but they have only 9500000000 undelegated
    let working_amount: Uint256 = 9000000000u64.into();

    info!("Transferring foreign stake token to gravity");
    // Obtain some foreign stake token
    let sender = ibc_keys[0];
    let receiver = keys[0]
        .validator_key
        .to_address(ADDRESS_PREFIX.as_str())
        .unwrap();
    assert!(
        test_ibc_transfer(
            ibc_contact,
            gravity_bank_qc.clone(),
            gravity_ibc_transfer_qc.clone(),
            sender,
            receiver,
            Some(
                Coin {
                    amount: working_amount,
                    denom: foreign_denom.clone()
                }
                .into()
            ),
            Some(zero_fee),
            ibc_ibc_channel_id,
            Duration::from_secs(60 * 5),
        )
        .await,
        "Unable to send foreign stake token to gravity address"
    );

    info!("Fetching foreign stake balance on gravity");

    let transferred_token = get_ibc_balance(
        receiver,
        foreign_denom,
        None,
        gravity_bank_qc.clone(),
        gravity_ibc_transfer_qc.clone(),
        None,
    )
    .await
    .expect("Unable to retrieve transferred ibc token balance");

    let ibc_stake = transferred_token.denom.clone();

    // Submit a metadata proposal
    // Base stake token denom units: {"denom": "stake", "exponent": 0}, {"denom": "mstake", "exponent": 6}]}
    let metadata = Metadata {
        description: "ibc chain stake metadata".to_string(),
        denom_units: vec![
            DenomUnit {
                denom: ibc_stake.clone(),
                exponent: 0,
                aliases: vec![],
            },
            DenomUnit {
                denom: "mibcstake".to_string(),
                exponent: 6,
                aliases: vec![],
            },
        ],
        base: ibc_stake.clone(),
        display: "mibcstake".to_string(),
        name: "Ibc Stake".to_string(),
        symbol: "iSTEAK".to_string(),
    };

    info!("Deploying foreign stake ibc metadata proposal");
    submit_and_pass_ibc_metadata_proposal(ibc_stake.clone(), metadata.clone(), contact, &keys)
        .await;

    info!("Deploying representative ERC20 for foreign stake token");
    let erc20 = deploy_cosmos_representing_erc20_and_check_adoption(
        gravity_contract_address,
        web30,
        Some(keys),
        &mut gravity_qc,
        false,
        metadata,
    )
    .await;

    Ok((ibc_stake, erc20))
}

/// Creates several transactions and requests several batches to create in-progress transfers to Eth
#[allow(clippy::too_many_arguments)]
pub async fn create_send_to_eth_activity(
    web30: &Web3,
    grpc: QueryClient<Channel>,
    contact: &Contact,
    gravity_address: EthAddress,
    gravity_id: String,
    relayer_config: RelayerConfig,
    erc20_addresses: Vec<EthAddress>,
    footoken_denom: String,
    ibc_stake_denom: String,
    validator_cosmos_keys: Vec<CosmosPrivateKey>,
    validator_eth_addrs: Vec<EthAddress>,
    relayer_eth_key: EthPrivateKey,
    relayer_cosmos_key: CosmosPrivateKey,
) {
    let denoms: Vec<String> = erc20_addresses
        .into_iter()
        .map(|e| format!("{}{}", "gravity", e))
        .collect();
    info!("\n\n\n SENDING ERC20S TO ETH FOR BATCH CREATION\n\n\n");
    for denom in denoms {
        let coin_to_send = Coin {
            amount: 1u8.into(),
            denom: denom.to_string(),
        };
        let fee_coin = coin_to_send.clone();
        send_tokens_to_eth(
            contact,
            validator_cosmos_keys[0],
            validator_eth_addrs.clone(),
            coin_to_send,
            fee_coin.clone(),
        )
        .await;
    }
    info!("\n\n\n SENDING COSMOS DENOMS TO ETH FOR BATCH CREATION \n\n\n");
    let cosmos_denoms = vec![footoken_denom.clone(), ibc_stake_denom.clone()];
    for denom in cosmos_denoms {
        let coin_to_send = Coin {
            amount: 1u8.into(),
            denom,
        };
        let fee_coin = coin_to_send.clone();
        send_tokens_to_eth(
            contact,
            validator_cosmos_keys[0],
            validator_eth_addrs.clone(),
            coin_to_send,
            fee_coin.clone(),
        )
        .await;
    }
    let cosmos_fee = Coin {
        amount: 0u8.into(),
        denom: (*STAKING_TOKEN).clone(),
    };

    info!("\n\n\n CREATING BATCHES \n\n\n");
    // Trigger batch creation
    let mut config = relayer_config.clone();
    config.batch_request_mode = BatchRequestMode::EveryBatch;
    config.batch_relaying_mode = BatchRelayingMode::Altruistic;
    single_relayer_iteration(
        relayer_eth_key,
        Some(relayer_cosmos_key),
        Some(cosmos_fee),
        contact,
        web30,
        &grpc,
        gravity_address,
        &gravity_id,
        &relayer_config,
        true,
    )
    .await;
}

/// Runs the relayer `n` times, delaying for `iteration_delay` seconds before subsequent calls
///
/// Note: It is not nice to genericize this function to a `run_n_times` method, as working with async
/// closures is still a pain
#[allow(clippy::too_many_arguments)]
pub async fn run_relayer_n_times(
    n: usize,
    iteration_delay: u64,
    ethereum_key: EthPrivateKey,
    cosmos_key: Option<CosmosPrivateKey>,
    cosmos_fee: Option<Coin>,
    contact: &Contact,
    web30: &Web3,
    grpc: &GravityQueryClient<Channel>,
    gravity_contract_address: EthAddress,
    gravity_id: &str,
    relayer_config: &RelayerConfig,
    should_relay_altruistic: bool,
) {
    let delay = Duration::from_secs(iteration_delay);
    for _ in 0..n {
        single_relayer_iteration(
            ethereum_key,
            cosmos_key,
            cosmos_fee.clone(),
            contact,
            web30,
            grpc,
            gravity_contract_address,
            gravity_id,
            relayer_config,
            should_relay_altruistic,
        )
        .await;
        sleep(delay).await;
    }
}

#[allow(clippy::too_many_arguments)]
pub async fn create_and_execute_attestations(
    validator_keys: Vec<ValidatorKeys>,
    relayer_ethereum_key: EthPrivateKey,
    relayer_cosmos_key: Option<CosmosPrivateKey>,
    relayer_fee: Option<Coin>,
    contact: &Contact,
    web30: &Web3,
    grpc: &GravityQueryClient<Channel>,
    gravity_contract_address: EthAddress,
    gravity_id: &str,
    relayer_config: &RelayerConfig,
) {
    let relay_fut = run_relayer_n_times(
        10,
        20,
        relayer_ethereum_key,
        relayer_cosmos_key,
        relayer_fee.clone(),
        contact,
        web30,
        grpc,
        gravity_contract_address,
        gravity_id,
        relayer_config,
        true,
    );
    let mut grpc = grpc.clone();
    let valset_fut = test_valset_update(
        web30,
        contact,
        &mut grpc,
        &validator_keys,
        gravity_contract_address,
    );
    join(relay_fut, valset_fut).await;
}

pub async fn steal_from_bridge(
    web30: &Web3,
    gravity_contract_address: EthAddress,
    vulnerable_erc20_address: EthAddress,
    thief: EthPrivateKey,
    theft_amount: Uint256,
    query_address: EthAddress,
) -> Result<Uint256, Web3Error> {
    let thief_address = thief.to_address();

    let options: Vec<SendTxOption> = vec![SendTxOption::GasLimit(100_000u64.into())];

    let start_bal = web30
        .get_erc20_balance_as_address(
            Some(query_address),
            vulnerable_erc20_address,
            gravity_contract_address,
        )
        .await
        .expect("Unable to get ERC20 balance before theft");

    let tx_hash = web30
        .send_transaction(
            vulnerable_erc20_address,
            clarity::abi::encode_call(
                "steal(address,address,uint256)",
                &[
                    gravity_contract_address.into(),
                    thief_address.into(),
                    theft_amount.into(),
                ],
            )?,
            0u32.into(),
            thief_address,
            thief,
            options,
        )
        .await?;

    sleep(Duration::from_secs(10)).await;

    let end_bal = web30
        .get_erc20_balance_as_address(
            Some(query_address),
            vulnerable_erc20_address,
            gravity_contract_address,
        )
        .await
        .expect("Unable to get ERC20 balance after theft");
    info!(
        "Start bal {start_bal} and end bal {end_bal}: {}",
        display_uint256_as_address(tx_hash)
    );
    assert!(start_bal > end_bal && start_bal - end_bal == theft_amount);
    Ok(tx_hash)
}
