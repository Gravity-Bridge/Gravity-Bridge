use clarity::{Address, Uint256};
use cosmos_gravity::utils::get_last_event_nonce_with_retry;
use deep_space::address::Address as CosmosAddress;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_utils::get_with_retry::RETRY_TIME;
use gravity_utils::types::event_signatures::*;
use gravity_utils::types::{
    Erc20DeployedEvent, LogicCallExecutedEvent, SendToCosmosEvent, TransactionBatchExecutedEvent,
    ValsetUpdatedEvent,
};
use metrics_exporter::metrics_errors_counter;
use relayer::find_latest_valset::convert_block_to_search;
use std::collections::HashMap;
use web30::{ContractEvent, Web3Event};
// use std::env;
// use std::ops::Sub;
// use std::str::FromStr;
use std::sync::{Arc, RwLock};
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use web30::client::Web3;

use crate::ethereum_event_watcher::get_latest_safe_block;

lazy_static! {
    // cache evm_chain_prefix => (scan_block,Valset)
    static ref LAST_CHECKED_BLOCK_INFO: Arc<RwLock<HashMap<String, (Uint256,Option<Uint256>)>>> =
        Arc::new(RwLock::new(HashMap::new()));
}

pub fn get_last_checked_block_info(evm_chain_prefix: &str) -> Option<(Uint256, Option<Uint256>)> {
    LAST_CHECKED_BLOCK_INFO
        .read()
        .unwrap()
        .get(evm_chain_prefix)
        .cloned()
}

pub fn set_last_checked_block_info(evm_chain_prefix: &str, info: (Uint256, Option<Uint256>)) {
    let mut lock = LAST_CHECKED_BLOCK_INFO.write().unwrap();
    lock.insert(evm_chain_prefix.to_string(), info);
}

fn batch_events_callback(
    last_event_nonce: Uint256,
    _our_cosmos_address: CosmosAddress,
    web3_event: &Web3Event,
) -> Option<Uint256> {
    match TransactionBatchExecutedEvent::from_events(web3_event) {
        Ok(batches) => {
            if let Some(batch) = batches
                .iter()
                .find(|&b| upcast(b.event_nonce) == last_event_nonce)
            {
                trace!(
                    "{} batch event nonce {} last event nonce",
                    batch.event_nonce,
                    last_event_nonce
                );

                return Some(batch.block_height);
            }
        }
        Err(e) => {
            error!("Got batch event that we can't parse {}", e);
            metrics_errors_counter(1, "Got batch event that we can't parse");
        }
    }

    None
}

fn send_to_cosmos_events_callback(
    last_event_nonce: Uint256,
    _our_cosmos_address: CosmosAddress,
    web3_event: &Web3Event,
) -> Option<Uint256> {
    match SendToCosmosEvent::from_events(web3_event) {
        Ok(sends) => {
            if let Some(send) = sends
                .iter()
                .find(|&b| upcast(b.event_nonce) == last_event_nonce)
            {
                trace!(
                    "{} send event nonce {} last event nonce",
                    send.event_nonce,
                    last_event_nonce
                );
                return Some(send.block_height);
            }
        }
        Err(e) => {
            error!("Got SendToCosmos event that we can't parse {}", e);
            metrics_errors_counter(3, "Got SendToCosmos event that we can't parse");
        }
    }

    None
}

fn erc20_deployed_events_callback(
    last_event_nonce: Uint256,
    _our_cosmos_address: CosmosAddress,
    web3_event: &Web3Event,
) -> Option<Uint256> {
    match Erc20DeployedEvent::from_events(web3_event) {
        Ok(deploys) => {
            if let Some(deploy) = deploys
                .iter()
                .find(|&b| upcast(b.event_nonce) == last_event_nonce)
            {
                trace!(
                    "{} deploy event nonce {} last event nonce",
                    deploy.event_nonce,
                    last_event_nonce
                );
                return Some(deploy.block_height);
            }
        }
        Err(e) => {
            error!("Got ERC20Deployed event that we can't parse {}", e);
            metrics_errors_counter(3, "Got ERC20Deployed event that we can't parse");
        }
    }

    None
}

fn logic_call_executed_events_callback(
    last_event_nonce: Uint256,
    _our_cosmos_address: CosmosAddress,
    web3_event: &Web3Event,
) -> Option<Uint256> {
    match LogicCallExecutedEvent::from_events(web3_event) {
        Ok(calls) => {
            if let Some(call) = calls
                .iter()
                .find(|&b| upcast(b.event_nonce) == last_event_nonce)
            {
                trace!(
                    "{} LogicCall event nonce {} last event nonce",
                    call.event_nonce,
                    last_event_nonce
                );
                return Some(call.block_height);
            }
        }
        Err(e) => {
            error!("Got ERC20Deployed event that we can't parse {}", e);
            metrics_errors_counter(3, "Got ERC20Deployed event that we can't parse");
        }
    }

    None
}

// this reverse solves a very specific bug, we use the properties of the first valsets for edgecase
// handling here, but events come in chronological order, so if we don't reverse the iterator
// we will encounter the first validator sets first and exit early and incorrectly.
// note that reversing everything won't actually get you that much of a performance gain
// because this only involves events within the searching block range.
fn valset_events_callback(
    last_event_nonce: Uint256,
    our_cosmos_address: CosmosAddress,
    web3_event: &Web3Event,
) -> Option<Uint256> {
    match ValsetUpdatedEvent::from_events(web3_event) {
        Ok(valsets) => {
            for valset in valsets.iter().rev() {
                // if we've found this event it is the first possible event from the contract
                // no other events can come before it, therefore either there's been a parsing error
                // or no events have been submitted on this chain yet.
                let bootstrapping = valset.valset_nonce == 0 && last_event_nonce == 1u8.into();
                // our last event was a valset update event, treat as normal case
                let common_case = upcast(valset.event_nonce) == last_event_nonce;
                trace!(
                    "{} valset event nonce {} last event nonce",
                    valset.event_nonce,
                    last_event_nonce
                );
                if common_case || bootstrapping {
                    return Some(valset.block_height);
                }
                // if we're looking for a later event nonce and we find the deployment of the contract
                // we must have failed to parse the event we're looking for. The oracle can not start
                else if valset.valset_nonce == 0 && last_event_nonce > 1u8.into() {
                    panic!("Could not find the last event relayed by {}, Last Event nonce is {} but no event matching that could be found!", our_cosmos_address, last_event_nonce)
                }
            }
        }
        Err(e) => {
            error!("Got valset event that we can't parse {}", e);
            metrics_errors_counter(3, "Got valset event that we can't parse");
        }
    }

    None
}

/// This function retrieves the last event nonce this oracle has relayed to Cosmos
/// it then uses the Ethereum indexes to determine what block the last entry
pub async fn get_last_checked_block(
    grpc_client: GravityQueryClient<Channel>,
    evm_chain_prefix: &str,
    our_cosmos_address: CosmosAddress,
    prefix: String,
    gravity_contract_address: Address,
    web3: &Web3,
) -> Uint256 {
    let mut grpc_client = grpc_client;

    let latest_block = get_latest_safe_block(web3).await;
    let mut last_event_nonce: Uint256 = get_last_event_nonce_with_retry(
        &mut grpc_client,
        our_cosmos_address,
        prefix,
        evm_chain_prefix.to_string(),
    )
    .await
    .into();

    // zero indicates this oracle has never submitted an event before since there is no
    // zero event nonce (it's pre-incremented in the solidity contract) we have to go
    // and look for event nonce one.
    if last_event_nonce == 0u8.into() {
        last_event_nonce = 1u8.into();
    }

    let block_to_search = convert_block_to_search();

    let mut current_block: Uint256 = latest_block.clone();

    let (previous_block, mut prev_checked_block) =
        get_last_checked_block_info(evm_chain_prefix).unwrap_or((0u8.into(), None));

    while current_block.clone() > previous_block {
        info!(
            "Oracle is resyncing, looking back into the history to find our last event nonce {}, on block {}",
            last_event_nonce, current_block
        );
        let end_search = if current_block.clone() < block_to_search.into() {
            0u8.into()
        } else {
            Uint256::max(
                current_block.clone() - block_to_search.into(),
                previous_block.clone(),
            )
        };

        let callback_chains: Vec<(
            &str,
            fn(Uint256, deep_space::Address, &Web3Event) -> Option<Uint256>,
        )> = vec![
            (TRANSACTION_BATCH_EXECUTED_EVENT_SIG, batch_events_callback),
            (SENT_TO_COSMOS_EVENT_SIG, send_to_cosmos_events_callback),
            (ERC20_DEPLOYED_EVENT_SIG, erc20_deployed_events_callback),
            (LOGIC_CALL_EVENT_SIG, logic_call_executed_events_callback),
            // valset update events have one special property
            // that is useful to us in this handler a valset update event for nonce 0 is emitted
            // in the contract constructor meaning once you find that event you can exit the search
            // with confidence that you have not missed any events without searching the entire blockchain
            // history
            (VALSET_UPDATED_EVENT_SIG, valset_events_callback),
        ];
        let mut ind = 0;
        let mut found_block = false;
        while ind < callback_chains.len() {
            // get events log
            let (event_sig, callback) = callback_chains[ind];
            let web3_event = match web3
                .check_for_event(
                    end_search.clone(),
                    Some(current_block.clone()),
                    gravity_contract_address,
                    event_sig,
                )
                .await
            {
                Err(e) => {
                    if e.to_string().contains("non contiguous event nonce") {
                        // reduce last_block scanned to retry to find checked block with new nonce
                        set_last_checked_block_info(evm_chain_prefix, (Uint256::from(0u128), None))
                    }
                    error!("Failed to get blockchain events while resyncing, is your Eth node working? If you see only one of these it's fine",);
                    delay_for(RETRY_TIME).await;
                    metrics_errors_counter(1, "Failed to get blockchain events while resyncing");
                    continue;
                }
                Ok(events) => events,
            };
            // look for and return the block number of the event last seen on the Cosmos chain
            // then we will play events from that block (including that block, just in case
            // there is more than one event there) onwards. We use valset nonce 0 as an indicator
            // of what block the contract was deployed on.
            if let Some(block) = callback(last_event_nonce.clone(), our_cosmos_address, &web3_event)
            {
                // found last_checked_block
                found_block = true;
                prev_checked_block = Some(block);
                break;
            }

            ind += 1;
        }

        if found_block {
            break;
        }

        current_block = end_search;
    }

    // return cached valset
    if let Some(last_checked_block) = prev_checked_block.clone() {
        // cache latest_eth_valset and current_block
        set_last_checked_block_info(evm_chain_prefix, (latest_block, prev_checked_block));
        return last_checked_block;
    }

    // we should exit above when we find the zero valset, if we have the wrong contract address through we could be at it a while as we go over
    // the entire history to 'prove' it.
    panic!("You have reached the end of block history without finding the Gravity contract deploy event! You must have the wrong contract address!");
}

fn upcast(input: u64) -> Uint256 {
    input.into()
}
