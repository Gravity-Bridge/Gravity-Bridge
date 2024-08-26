pub mod register_orchestrator_address;

use crate::args::RecoverFundsOpts;
use crate::client::cosmos_to_eth::cosmos_to_eth;
use crate::utils::TIMEOUT;
use crate::{
    args::{SetEthereumKeyOpts, SetOrchestratorKeyOpts},
    config::{config_exists, load_keys, save_keys},
};
use cosmos_gravity::utils::get_reasonable_send_to_eth_fee;
use deep_space::{Coin, CosmosPrivateKey, PrivateKey};
use gravity_utils::connection_prep::create_rpc_connections;
use std::{path::Path, process::exit};

pub fn show_keys(home_dir: &Path, prefix: &str) {
    if !config_exists(home_dir) {
        error!("Please run `gbt init` before running this command!");
        exit(1);
    }
    let keys = load_keys(home_dir);
    match keys.orchestrator_phrase {
        Some(v) => {
            let key = CosmosPrivateKey::from_phrase(&v, "")
                .expect("Failed to decode key in keyfile. Did you edit it manually?");
            let address = key.to_address(prefix).unwrap();
            info!("Your Orchestrator key, {}", address);
        }
        None => info!("You do not have an Orchestrator key set"),
    }
    match keys.ethereum_key {
        Some(v) => {
            let address = v.to_address();
            info!("Your Ethereum key, {}", address);
        }
        None => info!("You do not have an Ethereum key set"),
    }
}

pub fn set_eth_key(home_dir: &Path, opts: SetEthereumKeyOpts) {
    if !config_exists(home_dir) {
        error!("Please run `gbt init` before running this command!");
        exit(1);
    }
    let mut keys = load_keys(home_dir);
    keys.ethereum_key = Some(opts.key);
    save_keys(home_dir, keys);
    info!("Successfully updated Ethereum Key")
}

pub fn set_orchestrator_key(home_dir: &Path, opts: SetOrchestratorKeyOpts) {
    if !config_exists(home_dir) {
        error!("Please run `gbt init` before running this command!");
        exit(1);
    }
    let res = CosmosPrivateKey::from_phrase(&opts.phrase, "");
    if let Err(e) = res {
        error!("Invalid Cosmos mnemonic phrase {} {:?}", opts.phrase, e);
        exit(1);
    }
    let mut keys = load_keys(home_dir);
    keys.orchestrator_phrase = Some(opts.phrase);
    save_keys(home_dir, keys);
    info!("Successfully updated Orchestrator Key")
}

pub async fn recover_funds(args: RecoverFundsOpts, address_prefix: String) {
    let connections = create_rpc_connections(
        address_prefix.clone(),
        Some(args.cosmos_grpc.clone()),
        None,
        TIMEOUT,
    )
    .await;
    let contact = connections.contact.unwrap();
    let grpc = connections.grpc.unwrap();

    if args.send_on_cosmos && !args.send_to_eth {
        if args.eth_bridge_fee.is_some() {
            error!("Unexpected --eth-bridge-fee with --send-on-cosmos");
            exit(1);
        }
        if args.eth_destination.is_some() {
            error!("Unexpected --eth-destination with --send-on-cosmos");
            exit(1);
        }
        if args.cosmos_destination.is_none() {
            error!("You must provide --cosmos-destination when using --send-on-cosmos!");
            exit(1);
        }
        let cosmos_destination = args.cosmos_destination.unwrap();
        if cosmos_destination.get_prefix() != address_prefix {
            error!("The provided destination address ({}) does not have the --address-prefix value ({}), are you sure you're sending to the right address?", cosmos_destination, address_prefix);
            exit(1);
        }

        let res = contact
            .send_coins(
                args.amount.clone(),
                args.cosmos_fee,
                cosmos_destination,
                Some(TIMEOUT),
                args.ethereum_key,
            )
            .await;
        if res.is_err() {
            error!(
                "Received an error response when sending {} of {} to {}, are you sure that your account has enough funds? Error: {}",
                args.amount.amount.to_string(),
                args.amount.denom,
                cosmos_destination,
                res.err().unwrap(),
            );
            exit(1);
        }
        info!(
            "Sent {} of {} to {}, Gravity Tx ID: {}",
            args.amount.amount,
            args.amount.denom,
            cosmos_destination.to_string(),
            res.unwrap().txhash()
        );
    } else if args.send_to_eth && !args.send_on_cosmos {
        let mut amount = args.amount; // May need to reduce the amount bridged for the chain_fee
        if args.cosmos_destination.is_some() {
            error!("Unexpected --cosmos-destination with --send-to-eth");
            exit(1);
        }

        let sender_address = args.ethereum_key.to_address(&address_prefix).unwrap();
        let cosmos_fee = args.cosmos_fee.unwrap_or_else(|| Coin {
            amount: 0u8.into(),
            denom: "ugraviton".to_string(),
        });
        if args.eth_bridge_fee.is_none() {
            error!("--eth-bridge-fee must be provided with --send-to-eth!");
            exit(1);
        }
        if args.eth_destination.is_none() {
            error!("--eth-destination must be provided with --send-to-eth!");
            exit(1);
        }
        let chain_fee = if args.chain_fee.is_some() {
            let chain_fee = args.chain_fee.unwrap();
            if amount.denom != chain_fee.denom {
                error!("--chain-fee value must be the same denom as the bridge amount!");
                exit(1);
            }
            chain_fee
        } else {
            info!("Calculating a reasonable chain fee to pay to Gravity Bridge stakers...");
            let chain_fee_amount = get_reasonable_send_to_eth_fee(&contact, amount.amount)
                .await
                .map_err(|e| {
                    format!(
                        "Unable to get a reasonable chain fee due to communication error: {}",
                        e
                    )
                })
                .unwrap();
            amount.amount -= chain_fee_amount;
            info!("Chain fee calculated ({}), your amount to bridge has been reduced to {} accordingly!", chain_fee_amount, amount.amount);
            Coin {
                amount: chain_fee_amount,
                denom: amount.denom.clone(),
            }
        };
        cosmos_to_eth(
            &contact,
            grpc,
            args.ethereum_key,
            sender_address,
            amount,
            cosmos_fee,
            chain_fee,
            args.eth_bridge_fee.unwrap(),
            args.eth_destination.unwrap(),
        )
        .await;
    } else {
        error!("You must provide ONE of --send-to-eth OR --send-on-cosmos");
        exit(1);
    }
}
