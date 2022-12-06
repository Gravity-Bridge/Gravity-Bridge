use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::happy_path::test_valset_update;
use crate::happy_path_v2::deploy_cosmos_representing_erc20_and_check_adoption;
use crate::ibc_auto_forward::{get_channel_id, setup_gravity_auto_forwards};
use crate::{
    create_default_test_config, footoken_metadata, get_ibc_chain_id, one_eth, start_orchestrators,
    vote_yes_on_proposals, CosmosAddress, EthPrivateKey, GravityQueryClient, ValidatorKeys,
    ADDRESS_PREFIX, COSMOS_NODE_GRPC, ETH_NODE, IBC_ADDRESS_PREFIX, MINER_ADDRESS,
    OPERATION_TIMEOUT, STAKING_TOKEN,
};
use actix_rt::System;
use clarity::Address as EthAddress;
use cosmos_gravity::proposals::{
    submit_set_monitored_token_addresses_proposal, SetMonitoredTokenAddressesProposal,
};
use cosmos_gravity::query::get_gravity_params;
use cosmos_gravity::send::send_to_eth;
use cosmos_gravity::utils::get_gravity_monitored_erc20s;
use crossbeam_channel::Sender;
use deep_space::{Coin, Contact, CosmosPrivateKey, PrivateKey};
use ethereum_gravity::send_to_cosmos::send_to_cosmos;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::Metadata;
use gravity_proto::cosmos_sdk_proto::ibc::core::channel::v1::query_client::QueryClient as IbcChannelQueryClient;
use gravity_proto::gravity::query_client::QueryClient;

use gravity_utils::types::{
    BatchRelayingMode, BatchRequestMode, RelayerConfig, ValsetRelayingMode,
};
use num256::Uint256;
use relayer::main_loop::{delay_until_next_iteration, single_relayer_iteration};
use std::ops::Mul;
use std::thread;
use std::time::{Duration, Instant};
use tokio::time::sleep;
use tonic::transport::Channel;
use web30::client::Web3;

const FOOTOKEN_ALLOCATION: u64 = 100u64; // Validators will have 100 FOO to spend

pub async fn cross_bridge_balance_test(
    web30: &Web3,
    grpc: QueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    _ibc_keys: Vec<CosmosPrivateKey>,
    gravity_address: EthAddress,
    erc20_addresses: Vec<EthAddress>,
) {
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
    ) = setup(web30, contact, grpc.clone(), keys.clone(), gravity_address).await;
    let mut monitored_erc20s = erc20_addresses.clone();
    monitored_erc20s.push(footoken_erc20);

    create_send_to_eth_activity(
        web30,
        grpc.clone(),
        contact,
        gravity_address,
        params.gravity_id.clone(),
        single_iteration_relayer_config.clone(),
        monitored_erc20s.clone(),
        footoken_metadata.base,
        validator_cosmos_keys.clone(),
        validator_eth_addrs.clone(),
        validator_eth_keys[0],
        validator_cosmos_keys[0],
    )
    .await;

    create_send_to_cosmos_activity(
        web30,
        grpc.clone(),
        contact,
        params.gravity_id.clone(),
        single_iteration_relayer_config.clone(),
        keys.clone(),
        validator_cosmos_keys.clone(),
        validator_eth_keys.clone(),
        gravity_address,
        monitored_erc20s.clone(),
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
    valset_relaying_only_config.altruistic_batch_relaying_samples_delay = 10000000u64;
    let relayer_halter = haltable_relayer(
        validator_eth_keys[0],
        Some(validator_cosmos_keys[0]),
        Some(relayer_fee),
        contact,
        &grpc,
        gravity_address,
        &params.gravity_id,
        &no_relayer_config.relayer, // haltable relayer ignores the relayer enabled flag
        true,
    );
    test_valset_update(web30, contact, &mut grpc, &keys, gravity_address).await;
    test_valset_update(web30, contact, &mut grpc, &keys, gravity_address).await;
    test_valset_update(web30, contact, &mut grpc, &keys, gravity_address).await;

    relayer_halter
        .send(true)
        .expect("Relayer could not be halted!"); // Halt the relayer

    ////////////// SECOND //////////////

    // Try to mess up the balances by sending to Gravity.sol + Gravity Module (SHOULD NOT HALT)
    // Try to send to the gravity module, which is not permitted
    // Send some tokens to the gravity_address, which should have no effect on the

    ////////////// THIRD //////////////
    // Submit a false claim with all validators where at least one ERC20 balance is lower than it
    // should be (SHOULD HALT)
    // MANUALLY INSPECT THAT THE OTHER NODES ALSO HALTED
}

pub async fn setup(
    web30: &Web3,
    contact: &Contact,
    grpc: GravityQueryClient<Channel>,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
) -> (
    Vec<CosmosPrivateKey>, // Vec<Validator Cosmos Keys>
    Vec<EthPrivateKey>,    // Vec<Validator Eth Keys>
    Vec<EthAddress>,       // Vec<Validator Eth Addresses>
    Metadata,              // Footoken
    EthAddress,            // New ERC20 deployed
) {
    let mut grpc = grpc;
    let ibc_metadata = footoken_metadata(contact).await;

    // Deploy an ERC20 for Cosmos-originated IBC auto-forwards + donations to Gravity.sol
    // This call does not depend on an active relayer
    let erc20_contract = deploy_cosmos_representing_erc20_and_check_adoption(
        gravity_address,
        web30,
        None, // Already started the orchestrators with custom config
        &mut grpc,
        false,
        ibc_metadata.clone(),
    )
    .await;

    // Give the validators' Eth accts some footoken erc20
    let mut validator_cosmos_keys = vec![];
    let mut validator_eth_keys = vec![];
    let mut validator_eth_addrs = vec![];
    let _ = keys
        .into_iter()
        .map(|k| {
            validator_cosmos_keys.push(k.validator_key);
            validator_eth_keys.push(k.eth_key);
            validator_eth_addrs.push(k.eth_key.to_address());
        })
        .collect::<Vec<()>>();
    let coin_to_send = Coin {
        denom: ibc_metadata.base.clone(),
        amount: one_eth().mul(FOOTOKEN_ALLOCATION.into()),
    };
    let fee_coin = Coin {
        denom: ibc_metadata.base.clone(),
        amount: 1u8.into(),
    };
    send_tokens_to_eth(
        contact,
        validator_cosmos_keys.clone(),
        validator_eth_addrs.clone(),
        coin_to_send.clone(),
        fee_coin.clone(),
    )
    .await;
    send_tokens_to_eth(
        contact,
        vec![validator_cosmos_keys[0]],
        vec![(*MINER_ADDRESS)],
        coin_to_send.clone(),
        fee_coin.clone(),
    )
    .await;

    (
        validator_cosmos_keys,
        validator_eth_keys,
        validator_eth_addrs,
        ibc_metadata,
        erc20_contract,
    )
}

#[allow(clippy::too_many_arguments)]
pub async fn create_send_to_cosmos_activity(
    web30: &Web3,
    grpc: QueryClient<Channel>,
    contact: &Contact,
    gravity_id: String,
    relayer_config: RelayerConfig,
    keys: Vec<ValidatorKeys>,
    validator_cosmos_keys: Vec<CosmosPrivateKey>,
    validator_eth_keys: Vec<EthPrivateKey>,
    gravity_address: EthAddress,
    erc20_addresses: Vec<EthAddress>,
) {
    let fee_coin = Coin {
        denom: (*STAKING_TOKEN).clone(),
        amount: 0u8.into(),
    };
    // Run the relayer so that the batches can make their way to Ethereum
    for _ in 0..5 {
        single_relayer_iteration(
            validator_eth_keys[0],
            Some(validator_cosmos_keys[0]),
            Some(fee_coin.clone()),
            contact,
            web30,
            &grpc,
            gravity_address,
            &gravity_id,
            &relayer_config,
            true,
        )
        .await;
        sleep(Duration::from_secs(5)).await;
    }

    submit_and_pass_set_monitored_token_addresses_proposal(
        contact,
        keys.clone(),
        erc20_addresses.clone(),
    )
    .await;

    // Send tokens to Gravity addresses:
    let mut sends: Vec<SendToCosmosArgs> = vec![];
    for erc20 in &erc20_addresses {
        sends.push(SendToCosmosArgs {
            sender: validator_eth_keys[0],
            dest: validator_cosmos_keys[0]
                .to_address(&ADDRESS_PREFIX)
                .unwrap(),
            amount: one_eth(),
            contract: *erc20,
        });
    }

    send_erc20s_to_cosmos(web30, gravity_address, sends).await;

    // Create pending IBC Auto forwards which will stay pending
    let gravity_channel_qc = IbcChannelQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect channel query client");
    let gravity_channel_id = get_channel_id(
        gravity_channel_qc,
        get_ibc_chain_id(),
        Some(OPERATION_TIMEOUT),
    )
    .await
    .expect("Could not find gravity-test-1 channel");
    setup_gravity_auto_forwards(
        contact,
        (*IBC_ADDRESS_PREFIX).clone(),
        gravity_channel_id.clone(),
        validator_cosmos_keys[0],
        &keys,
    )
    .await;
    let foreign_receiver = validator_cosmos_keys[0]
        .to_address(&IBC_ADDRESS_PREFIX)
        .unwrap();
    let mut sends: Vec<SendToCosmosArgs> = vec![];
    for erc20 in erc20_addresses {
        sends.push(SendToCosmosArgs {
            sender: validator_eth_keys[0],
            dest: foreign_receiver,
            amount: one_eth(),
            contract: erc20,
        });
    }
    send_erc20s_to_cosmos(web30, gravity_address, sends).await;
}

/// Sends `coin_to_send` from `sender_keys[i]` to `reciever_addrs[i]` for all i, paying `fee_coin` to bridge
pub async fn send_tokens_to_eth(
    contact: &Contact,
    sender_keys: Vec<CosmosPrivateKey>,
    receiver_addrs: Vec<EthAddress>,
    coin_to_send: Coin,
    fee_coin: Coin,
) {
    for (i, sender) in sender_keys.into_iter().enumerate() {
        let sender_addr = sender.to_address(&ADDRESS_PREFIX).unwrap().to_string();
        let receiver = receiver_addrs
            .get(i)
            .expect("Make sure senders and receivers have the same length!");
        let res = send_to_eth(
            sender,
            *receiver,
            coin_to_send.clone(),
            fee_coin.clone(),       // Bridge fee
            Some(fee_coin.clone()), // Chain fee
            fee_coin.clone(),       // Cosmos Tx fee
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

pub async fn submit_and_pass_set_monitored_token_addresses_proposal(
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    monitored_erc20s: Vec<EthAddress>,
) {
    let res = submit_set_monitored_token_addresses_proposal(
        SetMonitoredTokenAddressesProposal {
            title: "Set MonitoredTokenAddresses".to_string(),
            description: "Setting MonitoredTokenAddresses to the test ERC20s".to_string(),
            monitored_addresses: monitored_erc20s.clone(),
        },
        Coin {
            // deposit
            amount: one_eth().mul(1000u64.into()),
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

    vote_yes_on_proposals(contact, &keys, None).await;
    wait_for_proposals_to_execute(contact).await;
    trace!("Gov proposal executed with {:?}", res);

    let actual_erc20s = get_gravity_monitored_erc20s(contact)
        .await
        .expect("Could not obtain MonitoredTokenAddresses!");

    assert_eq!(monitored_erc20s, actual_erc20s);
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
    ibc_denom: String,
    validator_cosmos_keys: Vec<CosmosPrivateKey>,
    validator_eth_addrs: Vec<EthAddress>,
    relayer_eth_key: EthPrivateKey,
    relayer_cosmos_key: CosmosPrivateKey,
) {
    let mut denoms: Vec<String> = erc20_addresses
        .into_iter()
        .map(|e| format!("{}{}", "gravity", e))
        .collect();
    denoms.push(ibc_denom);
    for denom in denoms {
        let coin_to_send = Coin {
            amount: 1u8.into(),
            denom: denom.to_string(),
        };
        let fee_coin = coin_to_send.clone();
        send_tokens_to_eth(
            contact,
            validator_cosmos_keys.clone(),
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

/// Runs a single_relayer_iteration() every `relayer_loop_speed` seconds in a thread until `true`
/// is sent on the returned channel Sender
#[allow(clippy::too_many_arguments)]
pub fn haltable_relayer(
    relayer_eth_key: EthPrivateKey,
    relayer_cosmos_key: Option<CosmosPrivateKey>,
    cosmos_fee: Option<Coin>,
    contact: &Contact,
    grpc: &GravityQueryClient<Channel>,
    gravity_address: EthAddress,
    gravity_id: &str,
    relayer_config: &RelayerConfig,
    should_relay_altruistic: bool,
) -> Sender<bool> {
    let contact = contact.clone();
    let grpc = grpc.clone();
    let gravity_id = gravity_id.to_string();
    let relayer_config = relayer_config.clone();
    let (tx, rx) = crossbeam_channel::bounded::<bool>(1);
    thread::spawn(move || {
        let web30 = Web3::new(ETH_NODE.as_str(), OPERATION_TIMEOUT);
        let rx = rx.clone();
        let system = System::new();
        loop {
            let loop_start = Instant::now();
            let recv = rx.recv();
            if let Ok(true) = recv {
                return;
            }
            let fut = single_relayer_iteration(
                relayer_eth_key,
                relayer_cosmos_key,
                cosmos_fee.clone(),
                &contact,
                &web30,
                &grpc,
                gravity_address,
                &gravity_id,
                &relayer_config,
                should_relay_altruistic,
            );
            system.block_on(fut);
            let fut = delay_until_next_iteration(loop_start, relayer_config.relayer_loop_speed);
            system.block_on(fut);
        }
    });
    tx
}
