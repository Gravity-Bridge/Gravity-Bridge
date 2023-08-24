//! This file handles the automatic request of batches, see the documentation on batch creation
//! https://github.com/Gravity-Bridge/Gravity-Bridge/blob/main/spec/batch-creation-spec.md
//! By having batches requested by relayers instead of created automatically the chain can outsource
//! the significant work of checking if a batch is profitable before creating it

use std::time::Duration;

use crate::altruistic::get_acceptable_gas_price;
use clarity::Address as EthAddress;
use clarity::Uint256;
use cosmos_gravity::query::get_erc20_to_denom;
use cosmos_gravity::query::get_pending_batch_fees;
use cosmos_gravity::send::send_request_batch;
use deep_space::{Coin, Contact, PrivateKey};
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_utils::num_conversion::print_eth;
use gravity_utils::prices::get_weth_price_with_retries;
use gravity_utils::types::BatchRequestMode;
use gravity_utils::types::RelayerConfig;
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use web30::client::Web3;

pub async fn request_batches(
    contact: &Contact,
    web30: &Web3,
    grpc_client: &mut GravityQueryClient<Channel>,
    config: &RelayerConfig,
    eth_address: EthAddress,
    private_key: impl PrivateKey,
    request_fee: Coin,
) {
    // this actually works either way but sending a tx with zero as the fee
    // value seems strange
    let request_fee = if request_fee.amount == 0u8.into() {
        None
    } else {
        Some(request_fee)
    };
    // TODO: this is a heuristic that needs to be dialed in
    // it's not easy to really estimate the actual cost of a batch
    // before we have an eth tx to simulate it with, so we're just
    // assuming a base batch starts at 200k gas
    const BATCH_GAS: u128 = 200_000;
    // get the gas price once
    let eth_gas_price = web30.eth_gas_price().await;
    if let Err(e) = eth_gas_price {
        warn!("Could not get gas price for auto batch request {:?}", e);
        return;
    }
    let eth_gas_price = eth_gas_price.unwrap();

    let batch_fees = get_pending_batch_fees(grpc_client).await;
    if let Err(e) = batch_fees {
        warn!("Failed to get batch fees with {:?}", e);
        return;
    }
    let batch_fees = batch_fees.unwrap();
    if batch_fees.batch_fees.is_empty() {
        debug!("No pending batches to request: Empty batch fees response")
    }

    let mut batch_requested = false;
    for fee in batch_fees.batch_fees {
        let total_fee: Uint256 = fee.total_fees.parse().unwrap();
        let token: EthAddress = fee.token.parse().unwrap();
        let denom = get_erc20_to_denom(grpc_client, token).await;
        if let Err(e) = denom {
            error!(
                "Failed to lookup erc20 {} for batch with {:?}",
                fee.token, e
            );
            continue;
        }
        let denom = denom.unwrap().denom;

        match config.batch_request_mode {
            BatchRequestMode::ProfitableOnly => {
                let weth_cost_estimate = eth_gas_price * BATCH_GAS.into();
                match get_weth_price_with_retries(eth_address, token, total_fee, web30).await {
                    Ok(price) => {
                        if price > weth_cost_estimate {
                            info!(
                                "Requesting batch for {} because it is likely to be profitable: Cost: {} Reward: {}",
                                fee.token, print_eth(weth_cost_estimate), print_eth(price)
                            );
                            let res = send_request_batch(
                                private_key.clone(),
                                denom,
                                request_fee.clone(),
                                contact,
                            )
                            .await;
                            if let Err(e) = res {
                                if e.to_string().contains("would not be more profitable") {
                                    info!("Batch would not have been more profitable, no new batch created");
                                } else {
                                    warn!("Failed to request batch with {:?}", e);
                                }
                            } else {
                                batch_requested = true;
                            }
                        } else {
                            trace!("Did not request unprofitable batch");
                        }
                    }
                    Err(e) => warn!("Failed to get price for token {} with {:?}", fee.token, e),
                }
            }
            BatchRequestMode::Altruistic => {
                let ideal_gas =
                    get_acceptable_gas_price(config.altruistic_acceptable_gas_price_percentage);
                let should_request_altruistic = if let Some(good_price) = ideal_gas {
                    eth_gas_price <= good_price
                } else {
                    false
                };

                if should_request_altruistic {
                    info!(
                        "Requesting batch for {} because gas prices ({}) are good",
                        fee.token, eth_gas_price,
                    );
                    let res = send_request_batch(
                        private_key.clone(),
                        denom,
                        request_fee.clone(),
                        contact,
                    )
                    .await;
                    if let Err(e) = res {
                        warn!("Failed to request batch with {:?}", e);
                    } else {
                        batch_requested = true;
                    }
                }
            }
            BatchRequestMode::EveryBatch => {
                info!("Requesting batch for {}", fee.token);
                let res =
                    send_request_batch(private_key.clone(), denom, request_fee.clone(), contact)
                        .await;
                if let Err(e) = res {
                    warn!("Failed to request batch with {:?}", e);
                } else {
                    batch_requested = true;
                }
            }
            BatchRequestMode::None => {}
        }
    }

    // Delay the main loop a bit before attempting to relay batches so that the batch request can be processed
    if batch_requested {
        delay_for(Duration::from_secs(config.batch_request_relay_offset)).await;
    }
}
