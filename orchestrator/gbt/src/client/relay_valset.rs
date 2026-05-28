use crate::args::RelayValsetOpts;
use crate::utils::TIMEOUT;
use cosmos_gravity::query::{
    get_all_valset_confirms, get_gravity_params, get_latest_valsets, get_valset,
};
use ethereum_gravity::message_signatures::encode_valset_confirm_hashed;
use ethereum_gravity::valset_update::send_eth_valset_update;
use gravity_proto::gravity::v1::query_client::QueryClient as GravityQueryClient;
use gravity_utils::connection_prep::{check_for_eth, create_rpc_connections};
use gravity_utils::error::GravityError;
use gravity_utils::types::{Valset, ValsetConfirmResponse};
use relayer::find_latest_valset::find_latest_valset;
use std::process::exit;
use tonic::transport::Channel;

/// One-off command that finds the most recent submittable validator set update
/// and relays it to the Gravity contract on Ethereum, printing helpful logs and exiting.
pub async fn relay_valset(args: RelayValsetOpts, address_prefix: String) {
    let grpc_url = args.cosmos_grpc;
    let ethereum_rpc = args.ethereum_rpc;
    let ethereum_key = args.ethereum_key;

    let connections =
        create_rpc_connections(address_prefix, Some(grpc_url), Some(ethereum_rpc), TIMEOUT).await;
    let web3 = connections.web3.unwrap();
    let mut grpc = connections.grpc.unwrap();

    let ethereum_public_key = ethereum_key.to_address();
    check_for_eth(ethereum_public_key, &web3).await;

    let params = get_gravity_params(&mut grpc).await.unwrap();
    let gravity_id = params.gravity_id;

    let gravity_contract_address = if let Some(c) = args.gravity_contract_address {
        c
    } else {
        let c = params.bridge_ethereum_address.parse();
        if c.is_err() {
            error!("The Gravity address is not yet set as a chain parameter! You must specify --gravity-contract-address");
            exit(1);
        }
        c.unwrap()
    };

    // Find the validator set currently deployed in the Gravity contract on Ethereum.
    // We need this to determine which Cosmos validator set is valid to submit next.
    info!("Searching Ethereum for the validator set currently in the Gravity contract");
    let current_valset = match find_latest_valset(&mut grpc, gravity_contract_address, &web3).await
    {
        Ok(v) => v,
        Err(e) => {
            error!("Could not get the current validator set from Ethereum! {e:?}");
            exit(1);
        }
    };
    info!(
        "The validator set currently in the Gravity contract is nonce {}",
        current_valset.nonce
    );

    // Find the latest validator set nonce on Cosmos
    let latest_valsets = get_latest_valsets(&mut grpc).await;
    let latest_cosmos_valset_nonce = match latest_valsets {
        Ok(valsets) => match valsets.iter().map(|v| v.nonce).max() {
            Some(n) => n,
            None => {
                info!("No validator sets have been created on Cosmos yet, nothing to relay");
                exit(0);
            }
        },
        Err(e) => {
            error!("Failed to query the latest validator sets from Cosmos! {e:?}");
            exit(1);
        }
    };
    info!("The latest validator set nonce on Cosmos is {latest_cosmos_valset_nonce}");

    if latest_cosmos_valset_nonce <= current_valset.nonce {
        info!(
            "The validator set in the Gravity contract (nonce {}) is already up to date, nothing to relay",
            current_valset.nonce
        );
        exit(0);
    }

    // Find the latest validator set that can actually be submitted given the constraints of
    // the validator set currently in the bridge. Voting power may have changed enough that we
    // need to play back an older intermediate valset before the latest can be submitted.
    let (valset_to_relay, confirms) = match find_latest_valid_valset(
        latest_cosmos_valset_nonce,
        &current_valset,
        &mut grpc,
        &gravity_id,
    )
    .await
    {
        Ok(v) => v,
        Err(GravityError::ValsetUpToDate) => {
            info!("The validator set is already up to date, nothing to relay");
            exit(0);
        }
        Err(e) => {
            error!("We were unable to find a valid validator set update to submit! {e:?}");
            exit(1);
        }
    };

    info!(
        "Found submittable validator set update {} -> {}, attempting to relay it to Ethereum",
        current_valset.nonce, valset_to_relay.nonce
    );

    let res = send_eth_valset_update(
        valset_to_relay.clone(),
        current_valset,
        &confirms,
        &web3,
        TIMEOUT,
        gravity_contract_address,
        gravity_id,
        ethereum_key,
    )
    .await;

    match res {
        Ok(_) => {
            info!(
                "Validator set update to nonce {} was successfully relayed! Check Etherscan and your wallet",
                valset_to_relay.nonce
            );
        }
        Err(e) => {
            error!("Validator set update relaying has failed {e:?}");
            exit(1);
        }
    }
}

/// Locates the latest valid validator set which can be moved to Ethereum given the validator set
/// currently in the bridge. Iterates backwards from the latest Cosmos nonce until it finds a
/// validator set whose confirmations can be ordered against the current on-chain validator set.
async fn find_latest_valid_valset(
    latest_nonce_on_cosmos: u64,
    current_valset: &Valset,
    grpc_client: &mut GravityQueryClient<Channel>,
    gravity_id: &str,
) -> Result<(Valset, Vec<ValsetConfirmResponse>), GravityError> {
    let mut latest_nonce = latest_nonce_on_cosmos;
    let mut latest_confirms = None;
    let mut latest_valset = None;
    // this is used to display the state of the last validator set to fail signature checks
    let mut last_error = None;
    while latest_nonce > current_valset.nonce {
        let valset = get_valset(grpc_client, latest_nonce).await;
        if let Ok(Some(valset)) = valset {
            assert_eq!(valset.nonce, latest_nonce);
            let confirms = get_all_valset_confirms(grpc_client, latest_nonce).await;
            if let Ok(confirms) = confirms {
                for confirm in confirms.iter() {
                    assert_eq!(valset.nonce, confirm.nonce);
                }
                let hash = encode_valset_confirm_hashed(gravity_id.to_string(), valset.clone());
                // ordering signatures against the current on-chain valset confirms that this
                // validator set update can actually be submitted to the bridge right now
                let res = current_valset.order_sigs(&hash, &confirms, false);
                if res.is_ok() {
                    if !valset.enough_power() {
                        warn!("Validator set {} can not be executed, power is too low to pass following measures. How was this generated?", valset.nonce);
                    } else {
                        latest_confirms = Some(confirms);
                        latest_valset = Some(valset);
                        break;
                    }
                } else if let Err(e) = res {
                    last_error = Some(e);
                }
            }
        }

        latest_nonce -= 1
    }

    if let (Some(v), Some(c)) = (latest_valset, latest_confirms) {
        Ok((v, c))
    } else if let Some(e) = last_error {
        Err(e)
    } else {
        Err(GravityError::ValsetUpToDate)
    }
}
