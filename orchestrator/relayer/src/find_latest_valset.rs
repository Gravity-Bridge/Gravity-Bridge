use clarity::{Address, Uint256};
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_utils::types::event_signatures::*;
use gravity_utils::types::{EthereumEvent, ValsetUpdatedEvent};
use gravity_utils::{error::GravityError, types::Valset};
use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use tonic::transport::Channel;
use web30::client::Web3;

// TODO: using leveldb
lazy_static! {
    // cache evm_chain_prefix => (scan_block,Valset)
    static ref LATEST_VALSET_INFO: Arc<RwLock<HashMap<String, (Uint256,Option<Valset>)>>> =
        Arc::new(RwLock::new(HashMap::new()));
}

fn get_latest_valset_info(evm_chain_prefix: &str) -> Option<(Uint256, Option<Valset>)> {
    LATEST_VALSET_INFO
        .read()
        .unwrap()
        .get(evm_chain_prefix)
        .cloned()
}

fn set_latest_valset_info(evm_chain_prefix: &str, info: (Uint256, Option<Valset>)) {
    let mut lock = LATEST_VALSET_INFO.write().unwrap();
    lock.insert(evm_chain_prefix.to_string(), info);
}

/// This function finds the latest valset on the Gravity contract by looking back through the event
/// history and finding the most recent ValsetUpdatedEvent. Most of the time this will be very fast
/// as the latest update will be in recent blockchain history and the search moves from the present
/// backwards in time. In the case that the validator set has not been updated for a very long time
/// this will take longer.
pub async fn find_latest_valset(
    grpc_client: &mut GravityQueryClient<Channel>,
    evm_chain_prefix: &str,
    gravity_contract_address: Address,
    web3: &Web3,
) -> Result<Valset, GravityError> {
    const BLOCKS_TO_SEARCH: u128 = 5_000u128;
    let latest_block = web3.eth_block_number().await?;
    let mut current_block: Uint256 = latest_block.clone();

    let (previous_block, mut latest_eth_valset) =
        get_latest_valset_info(evm_chain_prefix).unwrap_or((0u8.into(), None));

    while current_block.clone() > previous_block {
        trace!(
            "About to submit a Valset or Batch looking back into the history to find the last Valset Update, on block {}",
            current_block
        );
        let end_search = if current_block.clone() < BLOCKS_TO_SEARCH.into() {
            0u8.into()
        } else {
            current_block.clone() - BLOCKS_TO_SEARCH.into()
        };
        let mut all_valset_events = match web3
            .check_for_events(
                end_search.clone(),
                Some(current_block.clone()),
                vec![gravity_contract_address],
                vec![VALSET_UPDATED_EVENT_SIG],
            )
            .await
        {
            Ok(events) => events,
            Err(_) => {
                warn!(
                    "Failed to get events for block range {} - {}, repeating...",
                    end_search, current_block
                );
                continue;
            }
        };
        // by default the lowest found valset goes first, we want the highest.
        all_valset_events.reverse();

        trace!("Found events {:?}", all_valset_events);

        // we take only the first event if we find any at all.
        if !all_valset_events.is_empty() {
            let event = &all_valset_events[0];
            match ValsetUpdatedEvent::from_log(event) {
                Ok(event) => {
                    // update latest_eth_valset
                    latest_eth_valset = Some(Valset {
                        nonce: event.valset_nonce,
                        members: event.members,
                        reward_amount: event.reward_amount,
                        reward_token: event.reward_token,
                    });

                    // cache latest_eth_valset and current_block
                    set_latest_valset_info(
                        evm_chain_prefix,
                        (current_block, latest_eth_valset.clone()),
                    );

                    // now break from loop
                    break;
                }
                Err(e) => error!("Got valset event that we can't parse {}", e),
            }
        }
        current_block = end_search;
    }

    // return cached valset
    if let Some(latest_eth_valset) = latest_eth_valset {
        // just for warning
        let cosmos_chain_valset = cosmos_gravity::query::get_valset(
            grpc_client,
            evm_chain_prefix,
            latest_eth_valset.nonce,
        )
        .await?;
        check_if_valsets_differ(cosmos_chain_valset, &latest_eth_valset);
        return Ok(latest_eth_valset);
    }

    panic!("Could not find the last validator set for contract {}, probably not a valid Gravity contract!", gravity_contract_address)
}

/// This function exists to provide a warning if Cosmos and Ethereum have different validator sets
/// for a given nonce. In the mundane version of this warning the validator sets disagree on sorting order
/// which can happen if some relayer uses an unstable sort, or in a case of a mild griefing attack.
/// The Gravity contract validates signatures in order of highest to lowest power. That way it can exit
/// the loop early once a vote has enough power, if a relayer were to submit things in the reverse order
/// they could grief users of the contract into paying more in gas.
/// The other (and far worse) way a disagreement here could occur is if validators are colluding to steal
/// funds from the Gravity contract and have submitted a highjacking update. If slashing for off Cosmos chain
/// Ethereum signatures is implemented you would put that handler here.
fn check_if_valsets_differ(cosmos_valset: Option<Valset>, ethereum_valset: &Valset) {
    if cosmos_valset.is_none() && ethereum_valset.nonce == 0 {
        // bootstrapping case
        return;
    } else if cosmos_valset.is_none() {
        error!("Cosmos does not have a valset for nonce {} but that is the one on the Ethereum chain! Possible bridge highjacking!", ethereum_valset.nonce);
        return;
    }
    let cosmos_valset = cosmos_valset.unwrap();
    if cosmos_valset != *ethereum_valset {
        // if this is not true then we have a logic error on the Cosmos chain
        // or with our Ethereum search
        assert_eq!(cosmos_valset.nonce, ethereum_valset.nonce);

        let mut c_valset = cosmos_valset.members;
        let mut e_valset = ethereum_valset.members.clone();
        c_valset.sort();
        e_valset.sort();
        if c_valset == e_valset {
            info!(
                "Sorting disagreement between Cosmos and Ethereum on Valset nonce {}",
                ethereum_valset.nonce
            );
        } else {
            info!("Validator sets for nonce {} Cosmos and Ethereum differ. Possible bridge highjacking!", ethereum_valset.nonce)
        }
    }
}
