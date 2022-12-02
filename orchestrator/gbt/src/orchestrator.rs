use crate::args::OrchestratorOpts;
use crate::config::config_exists;
use crate::config::load_keys;
use crate::utils::print_relaying_explanation;
use clarity::constants::zero_address;
use cosmos_gravity::query::get_gravity_params;
use deep_space::{CosmosPrivateKey, PrivateKey};
use gravity_utils::connection_prep::{
    check_delegate_addresses, check_for_eth, wait_for_cosmos_node_ready,
};
use gravity_utils::connection_prep::{check_for_fee, create_rpc_connections};
use gravity_utils::types::BatchRequestMode;
use gravity_utils::types::GravityBridgeToolsConfig;
use metrics_exporter::metrics_server;
use orchestrator::main_loop::orchestrator_main_loop;
use orchestrator::main_loop::{ETH_ORACLE_LOOP_SPEED, ETH_SIGNER_LOOP_SPEED};
use std::cmp::min;
use std::path::Path;
use std::process::exit;
use std::time::Duration;

pub async fn orchestrator(
    args: OrchestratorOpts,
    address_prefix: String,
    home_dir: &Path,
    config: GravityBridgeToolsConfig,
) {
    let fee = args.fees;
    let cosmos_grpc = args.cosmos_grpc;
    let ethereum_rpc = args.ethereum_rpc;
    let ethereum_key = args.ethereum_key;
    let cosmos_key = args.cosmos_phrase;

    let cosmos_key = if let Some(k) = cosmos_key {
        k
    } else {
        let mut k = None;
        if config_exists(home_dir) {
            let keys = load_keys(home_dir);
            if let Some(stored_key) = keys.orchestrator_phrase {
                k = Some(CosmosPrivateKey::from_phrase(&stored_key, "").unwrap())
            }
        }
        if k.is_none() {
            error!("You must specify an Orchestrator key phrase!");
            error!("To set an already registered key use 'gbt keys set-orchestrator-key --phrase \"your phrase\"`");
            error!("To run from the command line, with no key storage use 'gbt orchestrator --cosmos-phrase \"your phrase\"' ");
            error!("If you have not already generated a key 'gbt keys register-orchestrator-address' will generate one for you");
            exit(1);
        }
        k.unwrap()
    };
    let ethereum_key = if let Some(k) = ethereum_key {
        k
    } else {
        let mut k = None;
        if config_exists(home_dir) {
            let keys = load_keys(home_dir);
            if let Some(stored_key) = keys.ethereum_key {
                k = Some(stored_key)
            }
        }
        if k.is_none() {
            error!("You must specify an Ethereum key!");
            error!("To set an already registered key use 'gbt keys set-ethereum-key -key \"eth private key\"`");
            error!("To run from the command line, with no key storage use 'gbt orchestrator --ethereum-key your key' ");
            error!("If you have not already generated a key 'gbt keys register-orchestrator-address' will generate one for you");
            exit(1);
        }
        k.unwrap()
    };

    let timeout = min(
        min(ETH_SIGNER_LOOP_SPEED, ETH_ORACLE_LOOP_SPEED),
        Duration::from_secs(config.relayer.relayer_loop_speed),
    );

    trace!("Probing RPC connections");
    // probe all rpc connections and see if they are valid
    let connections = create_rpc_connections(
        address_prefix,
        Some(cosmos_grpc),
        Some(ethereum_rpc),
        timeout,
    )
    .await;

    let mut grpc = connections.grpc.clone().unwrap();
    let contact = connections.contact.clone().unwrap();
    let web3 = connections.web3.clone().unwrap();

    let public_eth_key = ethereum_key.to_address();
    let public_cosmos_key = cosmos_key.to_address(&contact.get_prefix()).unwrap();
    info!("Starting Gravity Validator companion binary Relayer + Oracle + Eth Signer");
    info!(
        "Ethereum Address: {} Cosmos Address {}",
        public_eth_key, public_cosmos_key
    );

    // check if the cosmos node is syncing, if so wait for it
    // we can't move any steps above this because they may fail on an incorrect
    // historic chain state while syncing occurs
    wait_for_cosmos_node_ready(&contact).await;

    // check if the delegate addresses are correctly configured
    check_delegate_addresses(
        &mut grpc,
        public_eth_key,
        public_cosmos_key,
        &contact.get_prefix(),
    )
    .await;

    // check if we actually have the promised balance of tokens to pay fees
    check_for_fee(&fee, public_cosmos_key, &contact).await;
    check_for_eth(public_eth_key, &web3).await;

    // get the gravity parameters
    let params = get_gravity_params(&mut grpc)
        .await
        .expect("Failed to get Gravity Bridge module parameters!");

    // get the gravity contract address, if not provided
    let contract_address = if let Some(c) = args.gravity_contract_address {
        c
    } else {
        let c = params.bridge_ethereum_address.parse();
        match c {
            Ok(v) => {
                if v == zero_address() {
                    error!("The Gravity address is not yet set as a chain parameter! You must specify --gravity-contract-address");
                    exit(1);
                }
                c.unwrap()
            }
            Err(_) => {
                error!("The Gravity address is not yet set as a chain parameter! You must specify --gravity-contract-address");
                exit(1);
            }
        }
    };

    if config.orchestrator.relayer_enabled {
        // setup and explain relayer settings
        if config.relayer.batch_request_mode != BatchRequestMode::None {
            let public_cosmos_key = cosmos_key.to_address(&contact.get_prefix()).unwrap();
            check_for_fee(&fee, public_cosmos_key, &contact).await;
            print_relaying_explanation(&config.relayer, true)
        } else {
            print_relaying_explanation(&config.relayer, false)
        }
    }

    // Start monitiring if enabled on config.toml
    if config.metrics.metrics_enabled {
        metrics_server(&config.metrics);
    };

    orchestrator_main_loop(
        cosmos_key,
        ethereum_key,
        connections.web3.unwrap(),
        connections.contact.unwrap(),
        connections.grpc.unwrap(),
        contract_address,
        params.gravity_id,
        fee,
        config,
    )
    .await;
}
