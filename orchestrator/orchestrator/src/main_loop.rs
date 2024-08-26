//! This file contains the main loops for two distinct functions that just happen to reside int his same binary for ease of use. The Ethereum Signer and the Ethereum Oracle are both roles in Gravity
//! that can only be run by a validator. This single binary the 'Orchestrator' runs not only these two rules but also the untrusted role of a relayer, that does not need any permissions and has it's
//! own crate and binary so that anyone may run it.

use crate::{ethereum_event_watcher::check_for_events, oracle_resync::get_last_checked_block};
use clarity::PrivateKey as EthPrivateKey;
use clarity::{address::Address as EthAddress, Uint256};
use cosmos_gravity::query::get_gravity_params;
use cosmos_gravity::{
    query::{
        get_oldest_unsigned_logic_calls, get_oldest_unsigned_transaction_batches,
        get_oldest_unsigned_valsets,
    },
    send::{send_batch_confirm, send_logic_call_confirm, send_valset_confirms},
    utils::get_last_event_nonce_with_retry,
};
use deep_space::client::send::TransactionResponse;
use deep_space::error::CosmosGrpcError;
use deep_space::Contact;
use deep_space::{client::ChainStatus, utils::FeeInfo};
use deep_space::{
    coin::Coin,
    private_key::{CosmosPrivateKey, PrivateKey},
};
use futures::future::{join, join3};
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_utils::types::GravityBridgeToolsConfig;
use metrics_exporter::{metrics_errors_counter, metrics_latest, metrics_warnings_counter};
use num_traits::ToPrimitive;
use relayer::main_loop::all_relayer_loops;
use std::cmp::min;
use std::process::exit;
use std::time::Duration;
use std::time::Instant;
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use web30::client::Web3;

/// The execution speed governing all loops in this file
/// which is to say all loops started by Orchestrator main
/// loop except the relayer loop
pub const ETH_SIGNER_LOOP_SPEED: Duration = Duration::from_secs(11);
pub const ETH_ORACLE_LOOP_SPEED: Duration = Duration::from_secs(13);
/// Run the oracle loop slower while waiting for the merge
pub const ETH_ORACLE_WAITING_SPEED: Duration = Duration::from_secs(90);

/// This loop combines the three major roles required to make
/// up the 'Orchestrator', all three of these are async loops
/// meaning they will occupy the same thread, but since they do
/// very little actual cpu bound work and spend the vast majority
/// of all execution time sleeping this shouldn't be an issue at all.
#[allow(clippy::too_many_arguments)]
pub async fn orchestrator_main_loop(
    cosmos_key: CosmosPrivateKey,
    ethereum_key: EthPrivateKey,
    web3: Web3,
    contact: Contact,
    grpc_client: GravityQueryClient<Channel>,
    gravity_contract_address: EthAddress,
    gravity_id: String,
    user_fee_amount: Coin,
    config: GravityBridgeToolsConfig,
) {
    let fee = user_fee_amount;

    if config.orchestrator.check_eth_rpc {
        test_eth_connection(web3.clone()).await;
    }

    let a = eth_oracle_main_loop(
        cosmos_key,
        web3.clone(),
        contact.clone(),
        grpc_client.clone(),
        gravity_contract_address,
        fee.clone(),
    );
    let b = eth_signer_main_loop(
        cosmos_key,
        ethereum_key,
        contact.clone(),
        grpc_client.clone(),
        fee.clone(),
    );
    let c = all_relayer_loops(
        Some(cosmos_key),
        ethereum_key,
        web3.clone(),
        contact.clone(),
        grpc_client.clone(),
        gravity_contract_address,
        gravity_id,
        Some(fee.clone()),
        config.relayer,
    );

    // if the relayer is not enabled we just don't start the relayer_main_loop or ibc_auto_forward_loop futures
    if config.orchestrator.relayer_enabled {
        join3(a, b, c).await;
    } else {
        join(a, b).await;
    }
}

const DELAY: Duration = Duration::from_secs(5);

/// Checks forever that the Ethereum RPC returns different results when querying for "finalized" vs "latest" blocks, as this is
/// a critical feature
pub async fn test_eth_connection(web3: Web3) {
    loop {
        let (finalized_res, latest_res) = join(
            web3.eth_get_finalized_block_full(),
            web3.eth_get_latest_block_full(),
        )
        .await;

        match (latest_res, finalized_res) {
            (Ok(latest), Ok(finalized)) => {
                if latest.number < finalized.number
                    || finalized.number + 32u8.into() > latest.number
                {
                    panic!(
                        "Ethereum RPC returned an invalid 'finalized' block, expecting at least a 32 block difference but found latest block ({}) and 'finalized' block ({})",
                        latest.number, finalized.number,
                    );
                }
                info!("Ethereum RPC has returned an acceptable 'finalized' block ({}) behind the latest block ({}), starting the orchestrator!", finalized.number, latest.number);
                return;
            }
            (_, _) => {
                warn!(
                    "Could not connect to Ethereum RPC, delaying {} seconds before trying again.",
                    DELAY.as_secs()
                );
                delay_for(DELAY).await;
                continue;
            }
        }
    }
}

/// This function is responsible for making sure that Ethereum events are retrieved from the Ethereum blockchain
/// and ferried over to Cosmos where they will be used to issue tokens or process batches.
pub async fn eth_oracle_main_loop(
    cosmos_key: CosmosPrivateKey,
    web3: Web3,
    contact: Contact,
    grpc_client: GravityQueryClient<Channel>,
    gravity_contract_address: EthAddress,
    fee: Coin,
) {
    let our_cosmos_address = cosmos_key.to_address(&contact.get_prefix()).unwrap();
    let long_timeout_web30 = Web3::new(&web3.get_url(), Duration::from_secs(120));
    let mut last_checked_block: Uint256 = get_last_checked_block(
        grpc_client.clone(),
        our_cosmos_address,
        contact.get_prefix(),
        gravity_contract_address,
        &long_timeout_web30,
    )
    .await;

    // In case of governance vote to unhalt bridge, need to replay old events. Keep track of the
    // last checked event nonce to detect when this happens
    let mut last_checked_event: Uint256 = 0u8.into();
    info!("Oracle resync complete, Oracle now operational");
    let mut grpc_client = grpc_client;

    loop {
        let loop_start = Instant::now();

        let latest_eth_block = web3.eth_block_number().await;
        let latest_cosmos_block = contact.get_chain_status().await;

        match (&latest_eth_block, latest_cosmos_block) {
            (Ok(latest_eth_block), Ok(ChainStatus::Moving { block_height })) => {
                trace!(
                    "Latest Eth block {} Latest Cosmos block {}",
                    latest_eth_block,
                    block_height,
                );

                metrics_latest(block_height, "latest_cosmos_block");
                // Converting into u64
                metrics_latest(latest_eth_block.to_u64().unwrap(), "latest_eth_block");
            }
            (Ok(_latest_eth_block), Ok(ChainStatus::Syncing)) => {
                warn!("Cosmos node syncing, Eth oracle paused");
                metrics_warnings_counter(2, "Cosmos node syncing");
                delay_for(DELAY).await;
                continue;
            }
            (Ok(_latest_eth_block), Ok(ChainStatus::WaitingToStart)) => {
                warn!("Cosmos node syncing waiting for chain start, Eth oracle paused");
                metrics_warnings_counter(2, "Cosmos node syncing waiting for chain start");
                delay_for(DELAY).await;
                continue;
            }
            (Ok(_), Err(_)) => {
                warn!("Could not contact Cosmos grpc, trying again");
                metrics_warnings_counter(2, "Could not contact Cosmos grpc");
                delay_for(DELAY).await;
                continue;
            }
            (Err(_), Ok(_)) => {
                warn!("Could not contact Eth node, trying again");
                metrics_warnings_counter(1, "Could not contact Eth node");
                delay_for(DELAY).await;
                continue;
            }
            (Err(_), Err(_)) => {
                error!("Could not reach Ethereum or Cosmos rpc!");

                metrics_errors_counter(0, "Could not reach Ethereum or Cosmos rpc");

                delay_for(DELAY).await;
                continue;
            }
        }

        // if the governance vote reset last event nonce sent by validator to some lower value, we can detect this
        // by comparing last_event_nonce retrieved from the chain with last_checked_event saved by the orchestrator
        // in order to reset last_checked_block and last_checked_event and continue from that point
        let last_event_nonce: Uint256 = get_last_event_nonce_with_retry(
            &mut grpc_client,
            our_cosmos_address,
            contact.get_prefix().clone(),
        )
        .await
        .into();

        if last_event_nonce < last_checked_event {
            // validator went back in history
            info!("Governance unhalt vote must have happened, resetting the block to check!");
            last_checked_event = last_event_nonce;
            last_checked_block = get_last_checked_block(
                grpc_client.clone(),
                our_cosmos_address,
                contact.get_prefix(),
                gravity_contract_address,
                &web3,
            )
            .await;
        }

        // Relays events from Ethereum -> Cosmos
        match check_for_events(
            &web3,
            &contact,
            &mut grpc_client,
            gravity_contract_address,
            cosmos_key,
            fee.clone(),
            last_checked_block,
        )
        .await
        {
            Ok(nonces) => {
                // If the governance happened while check_for_events() was executing and there were no new event nonces,
                // nonces.event_nonce would return lower value than last_checked_event. We want to keep last_checked_event
                // value so it could be used in the next iteration to check if we should return to the
                // earlier block and continue from that point. CheckedNonces is accurate unless a governance vote happens.
                last_checked_block = nonces.block_number;
                if nonces.event_nonce > last_checked_event {
                    last_checked_event = nonces.event_nonce;
                }
                metrics_latest(
                    last_checked_event.to_string().parse().unwrap(),
                    "last_checked_event",
                );
            }
            Err(e) => {
                error!("Failed to get events for block range, Check your Eth node and Cosmos gRPC {:?}", e);
                metrics_errors_counter(0, "Failed to get events for block range");
            }
        }

        // a bit of logic that tires to keep things running every LOOP_SPEED seconds exactly
        // this is not required for any specific reason. In fact we expect and plan for
        // the timing being off significantly
        let elapsed = Instant::now() - loop_start;
        if elapsed < ETH_ORACLE_LOOP_SPEED {
            delay_for(ETH_ORACLE_LOOP_SPEED - elapsed).await;
        }
    }
}

/// The eth_signer simply signs off on any batches or validator sets provided by the validator
/// since these are provided directly by a trusted Cosmsos node they can simply be assumed to be
/// valid and signed off on.
pub async fn eth_signer_main_loop(
    cosmos_key: CosmosPrivateKey,
    ethereum_key: EthPrivateKey,
    contact: Contact,
    grpc_client: GravityQueryClient<Channel>,
    fee: Coin,
) {
    let our_cosmos_address = cosmos_key.to_address(&contact.get_prefix()).unwrap();
    let mut grpc_client = grpc_client;

    loop {
        let loop_start = Instant::now();

        // repeatedly refreshing the parameters here maintains loop correctness
        // if the gravity_id is changed or slashing windows are changed. Neither of these
        // is very probable
        let params = match get_gravity_params(&mut grpc_client).await {
            Ok(p) => p,
            Err(e) => {
                error!("Failed to get Gravity parameters with {} correct your Cosmos gRPC connection immediately, you are risking slashing",e);
                metrics_errors_counter(2, "Failed to get Gravity parameters correct your Cosmos gRPC connection immediately, you are risking slashing");
                continue;
            }
        };
        let blocks_until_slashing = min(
            min(params.signed_valsets_window, params.signed_batches_window),
            params.signed_logic_calls_window,
        );
        let gravity_id = params.gravity_id;

        let latest_cosmos_block = contact.get_chain_status().await;
        match latest_cosmos_block {
            Ok(ChainStatus::Moving { block_height }) => {
                trace!("Latest Cosmos block {}", block_height,);
            }
            Ok(ChainStatus::Syncing) => {
                warn!("Cosmos node syncing, Eth signer paused");
                warn!("If this operation will take more than {} blocks of time you must find another node to submit signatures or risk slashing", blocks_until_slashing);
                metrics_warnings_counter(2, "Cosmos node syncing, Eth signer paused");
                metrics_latest(blocks_until_slashing, "blocks_until_slashing");
                delay_for(DELAY).await;
                continue;
            }
            Ok(ChainStatus::WaitingToStart) => {
                warn!("Cosmos node syncing waiting for chain start, Eth signer paused");
                metrics_warnings_counter(
                    2,
                    "Cosmos node syncing waiting for chain start, Eth signer paused",
                );
                delay_for(DELAY).await;
                continue;
            }
            Err(_) => {
                error!("Could not reach Cosmos rpc! You must correct this or you risk being slashed in {} blocks", blocks_until_slashing);
                delay_for(DELAY).await;
                metrics_latest(blocks_until_slashing, "blocks_until_slashing");
                metrics_errors_counter(
                    2,
                    "Could not reach Cosmos rpc! You must correct this or you risk being slashed",
                );
                continue;
            }
        }

        // sign the last unsigned valsets
        match get_oldest_unsigned_valsets(
            &mut grpc_client,
            our_cosmos_address,
            contact.get_prefix(),
        )
        .await
        {
            Ok(valsets) => {
                if valsets.is_empty() {
                    trace!("No validator sets to sign, node is caught up!")
                } else {
                    info!(
                        "Sending {} valset confirms starting with nonce {}",
                        valsets.len(),
                        valsets[0].nonce
                    );
                    let res = send_valset_confirms(
                        &contact,
                        ethereum_key,
                        fee.clone(),
                        valsets,
                        cosmos_key,
                        gravity_id.clone(),
                    )
                    .await;
                    trace!("Valset confirm result is {:?}", res);
                    check_for_fee_error(res, &fee);
                }
            }
            Err(e) => trace!(
                "Failed to get unsigned valsets, check your Cosmos gRPC {:?}",
                e
            ),
        }

        // sign the last unsigned batch, TODO check if we already have signed this
        match get_oldest_unsigned_transaction_batches(
            &mut grpc_client,
            our_cosmos_address,
            contact.get_prefix(),
        )
        .await
        {
            Ok(last_unsigned_batches) => {
                if last_unsigned_batches.is_empty() {
                    trace!("No unsigned batch sets to sign, node is caught up!")
                } else {
                    info!(
                        "Sending {} batch confirms starting with nonce {}",
                        last_unsigned_batches.len(),
                        last_unsigned_batches[0].nonce
                    );

                    let res = send_batch_confirm(
                        &contact,
                        ethereum_key,
                        fee.clone(),
                        last_unsigned_batches,
                        cosmos_key,
                        gravity_id.clone(),
                    )
                    .await;
                    trace!("Batch confirm result is {:?}", res);
                    check_for_fee_error(res, &fee);
                }
            }
            Err(e) => trace!(
                "Failed to get unsigned Batches, check your Cosmos gRPC {:?}",
                e
            ),
        }

        match get_oldest_unsigned_logic_calls(
            &mut grpc_client,
            our_cosmos_address,
            contact.get_prefix(),
        )
        .await
        {
            Ok(last_unsigned_calls) => {
                if last_unsigned_calls.is_empty() {
                    trace!("No unsigned call sets to sign, node is caught up!")
                } else {
                    info!(
                        "Sending {} logic call confirms starting with nonce {}",
                        last_unsigned_calls.len(),
                        last_unsigned_calls[0].invalidation_nonce
                    );
                    let res = send_logic_call_confirm(
                        &contact,
                        ethereum_key,
                        fee.clone(),
                        last_unsigned_calls,
                        cosmos_key,
                        gravity_id.clone(),
                    )
                    .await;
                    trace!("call confirm result is {:?}", res);
                    check_for_fee_error(res, &fee);
                }
            }
            Err(e) => info!(
                "Failed to get unsigned Logic Calls, check your Cosmos gRPC {:?}",
                e
            ),
        }

        // a bit of logic that tires to keep things running every LOOP_SPEED seconds exactly
        // this is not required for any specific reason. In fact we expect and plan for
        // the timing being off significantly
        let elapsed = Instant::now() - loop_start;
        if elapsed < ETH_SIGNER_LOOP_SPEED {
            delay_for(ETH_SIGNER_LOOP_SPEED - elapsed).await;
        }
    }
}

/// Checks for fee errors on our confirm submission transactions, a failure here
/// can be fatal and cause slashing so we want to warn the user and exit. There is
/// no point in running if we can't perform our most important function
fn check_for_fee_error(res: Result<TransactionResponse, CosmosGrpcError>, fee: &Coin) {
    if let Err(CosmosGrpcError::InsufficientFees { fee_info }) = res {
        match fee_info {
            FeeInfo::InsufficientFees { min_fees } => {
                error!(
                    "Your specified fee value {} is too small please use at least {}",
                    fee,
                    Coin::display_list(&min_fees)
                );
                error!("Correct fee argument immediately! You will be slashed within a few hours if you fail to do so");
                exit(1);
            }
            FeeInfo::InsufficientGas { .. } => {
                panic!("Hardcoded gas amounts insufficient!");
            }
        }
    }
}
