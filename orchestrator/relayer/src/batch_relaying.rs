use crate::main_loop::ETH_SUBMIT_WAIT_TIME;
use clarity::PrivateKey as EthPrivateKey;
use clarity::Uint256;
use cosmos_gravity::query::get_latest_transaction_batches;
use cosmos_gravity::query::get_transaction_batch_signatures;
use ethereum_gravity::message_signatures::encode_tx_batch_confirm_hashed;
use ethereum_gravity::submit_batch::send_eth_transaction_batch;
use ethereum_gravity::utils::get_tx_batch_nonce;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_utils::num_conversion::print_eth;
use gravity_utils::num_conversion::print_gwei;
use gravity_utils::prices::get_weth_price_with_retries;
use gravity_utils::types::BatchRelayingMode;
use gravity_utils::types::WhitelistToken;
use gravity_utils::types::{BatchConfirmResponse, RelayerConfig, TransactionBatch, Valset};
use std::collections::HashMap;
use tonic::transport::Channel;
use web30::client::Web3;
use web30::EthAddress;

#[derive(Debug, Clone)]
struct SubmittableBatch {
    batch: TransactionBatch,
    sigs: Vec<BatchConfirmResponse>,
}

#[allow(clippy::too_many_arguments)]
/// This function relays batches from Cosmos to Ethereum. First we request
/// the latest transaction batches, which is a list of the latest 100 batches
/// of all types. From there we determine which batches are valid to submit as
/// far as signatures and then make requests to Ethereum to determine which are
/// valid to submit given the current chain state. From there we simulate a submission
/// and if that succeeds and we like the gas cost we complete the relaying process and
/// actually submit the data to Ethereum
pub async fn relay_batches(
    // the validator set currently in the contract on Ethereum
    current_valset: Valset,
    ethereum_key: EthPrivateKey,
    web3: &Web3,
    grpc_client: &mut GravityQueryClient<Channel>,
    evm_chain_prefix: &str,
    gravity_contract_address: EthAddress,
    gravity_id: String,
    config: RelayerConfig,
) {
    let possible_batches = get_batches_and_signatures(
        current_valset.clone(),
        grpc_client,
        evm_chain_prefix,
        gravity_id.clone(),
    )
    .await;

    trace!("possible batches {:?}", possible_batches);

    submit_batches(
        current_valset,
        ethereum_key,
        web3,
        gravity_contract_address,
        gravity_id,
        possible_batches,
        config,
    )
    .await;
}

/// This function retrieves the latest batches from the Cosmos module and then
/// iterates through the signatures for each batch, determining if they are ready
/// to submit. It is possible for a batch to not have valid signatures for two reasons
/// one is that not enough signatures have been collected yet from the validators two is
/// that the batch is old enough that the signatures do not reflect the current validator
/// set on Ethereum. In both the later and the former case the correct solution is to wait
/// through timeouts, new signatures, or a later valid batch being submitted old batches will
/// always be resolved.
async fn get_batches_and_signatures(
    current_valset: Valset,
    grpc_client: &mut GravityQueryClient<Channel>,
    evm_chain_prefix: &str,
    gravity_id: String,
) -> HashMap<EthAddress, Vec<SubmittableBatch>> {
    let latest_batches =
        if let Ok(lb) = get_latest_transaction_batches(grpc_client, evm_chain_prefix).await {
            lb
        } else {
            return HashMap::new();
        };
    trace!("Latest batches {:?}", latest_batches);

    let mut possible_batches = HashMap::new();
    for batch in latest_batches {
        let sigs = get_transaction_batch_signatures(
            grpc_client,
            evm_chain_prefix,
            batch.nonce,
            batch.token_contract,
        )
        .await;
        trace!("Got sigs {:?}", sigs);
        if let Ok(sigs) = sigs {
            // this checks that the signatures for the batch are actually possible to submit to the chain
            let hash = encode_tx_batch_confirm_hashed(gravity_id.clone(), batch.clone());
            if current_valset.order_sigs(&hash, &sigs).is_ok() {
                // we've found a valid batch, add it to the list for it's token type
                possible_batches
                    .entry(batch.token_contract)
                    .or_insert_with(Vec::new);

                let list = possible_batches.get_mut(&batch.token_contract).unwrap();
                list.push(SubmittableBatch { batch, sigs });
            } else {
                warn!(
                    "Batch {}/{} can not be submitted yet, waiting for more signatures",
                    batch.token_contract, batch.nonce
                );
            }
        } else {
            error!(
                "could not get signatures for {}:{} with {:?}",
                batch.token_contract, batch.nonce, sigs
            );
        }
    }
    // reverse the list so that it is oldest first, we want to submit
    // older batches so that we don't invalidate newer batches
    for (_key, value) in possible_batches.iter_mut() {
        value.reverse();
    }
    possible_batches
}

// Determines whether or not submitting `batch` will be profitable given the estimated `cost`
// and the current exchange rate available on uniswap
async fn should_relay_batch(
    web3: &Web3,
    batch: &TransactionBatch,
    cost: Uint256,
    pubkey: EthAddress,
    config: &BatchRelayingMode,
) -> (bool, Option<Uint256>) {
    // skip price request below in the trivial case, couldn't really
    // figure the code duplication / extra network IO balance otherwise
    if let BatchRelayingMode::EveryBatch = config {
        return (true, None);
    }

    let batch_reward_amount = batch.total_fee.amount.clone();
    let batch_reward_token = batch.total_fee.token_contract_address;
    // gets the price of the provided amount of the provided token in weth
    let price = get_weth_price_with_retries(
        pubkey,
        batch_reward_token,
        batch_reward_amount.clone(),
        web3,
    )
    .await;

    match config {
        BatchRelayingMode::EveryBatch | BatchRelayingMode::Altruistic => (true, None),
        BatchRelayingMode::ProfitableOnly { margin } => {
            let cost_with_margin = get_cost_with_margin(cost, *margin);

            // we need to see how much WETH we can get for the reward token amount,
            // and compare that value to the gas cost times the margin
            match price {
                Ok(price) => (price > cost_with_margin, Some(price)),
                Err(e) => {
                    info!(
                        "Unable to determine swap price of token {} for WETH \n
                it may just not be on Uniswap - Will not be relaying batch {:?}",
                        batch_reward_token, e
                    );
                    (false, None)
                }
            }
        }
        BatchRelayingMode::ProfitableWithWhitelist { margin, whitelist } => {
            let cost_with_margin = get_cost_with_margin(cost, *margin);
            // we need to see how much WETH we can get for the reward token amount,
            // and compare that value to the gas cost times the margin
            match (price, get_whitelist_price(batch.token_contract, whitelist)) {
                // config specifies this tokens price
                (_, Some((whitelist_price, decimals))) => {
                    let one = Uint256::from(1u8);
                    let one = one.pow(decimals as u32).to_string().parse().unwrap();
                    let reward_amount_in_weth = (whitelist_price / one) * batch_reward_amount;
                    (
                        reward_amount_in_weth > cost_with_margin,
                        Some(reward_amount_in_weth),
                    )
                }
                // we got the price in uniswap
                (Ok(price), None) => (price > cost_with_margin, Some(price)),
                (Err(e), _) => {
                    info!(
                        "Unable to determine swap price of token {} for WETH \n
                it may just not be on Uniswap - Will not be relaying batch {:?}",
                        batch_reward_token, e
                    );
                    (false, None)
                }
            }
        }
    }
}

/// Takes a token price whitelist, gets the amount of weth per token we have manually valued it at
fn get_whitelist_price(erc20: EthAddress, whitelist: &[WhitelistToken]) -> Option<(Uint256, u8)> {
    for i in whitelist {
        if i.token == erc20 {
            return Some((i.price.clone(), i.decimals));
        }
    }
    None
}

/// bakes the margin into the cost to provide an easy value to compare against
pub fn get_cost_with_margin(cost: Uint256, margin: f32) -> Uint256 {
    let cost_as_float: f32 = cost.to_string().parse().unwrap();
    let cost_with_margin = cost_as_float * margin;
    (cost_with_margin as u128).into()
}

#[allow(clippy::too_many_arguments)]
/// Attempts to submit batches with valid signatures, checking the state
/// of the Ethereum chain to ensure that it is valid to submit a given batch
/// more specifically that the correctly signed batch has not timed out or already
/// been submitted. The goal of this function is to submit batches in chronological order
/// of their creation, submitting batches newest first will invalidate old batches and is
/// less efficient if those old batches are profitable.
/// This function estimates the cost of submitting a batch before actually submitting it
/// to Ethereum, if it is determined that the ETH cost to submit is too high the batch will
/// be skipped and a later, more profitable, batch may be submitted.
/// Keep in mind that many other relayers are making this same computation and some may have
/// different standards for their profit margin, therefore there may be a race not only to
/// submit individual batches but also batches in different orders
async fn submit_batches(
    current_valset: Valset,
    ethereum_key: EthPrivateKey,
    web3: &Web3,
    gravity_contract_address: EthAddress,
    gravity_id: String,
    possible_batches: HashMap<EthAddress, Vec<SubmittableBatch>>,
    config: RelayerConfig,
) {
    let our_ethereum_address = ethereum_key.to_address();
    let ethereum_block_height = if let Ok(bn) = web3.eth_block_number().await {
        bn
    } else {
        warn!("Failed to get eth block height, is your eth node working?");
        return;
    };

    // requests data from Ethereum only once per token type, this is valid because we are
    // iterating from oldest to newest, so submitting a batch earlier in the loop won't
    // ever invalidate submitting a batch later in the loop. Another relayer could always
    // do that though.
    for (token_type, possible_batches) in possible_batches {
        let erc20_contract = token_type;
        let latest_ethereum_batch = get_tx_batch_nonce(
            gravity_contract_address,
            erc20_contract,
            our_ethereum_address,
            web3,
        )
        .await;
        if latest_ethereum_batch.is_err() {
            error!(
                "Failed to get latest Ethereum batch with {:?}",
                latest_ethereum_batch
            );
            return;
        }
        let latest_ethereum_batch = latest_ethereum_batch.unwrap();

        for batch in possible_batches {
            let oldest_signed_batch = batch.batch;
            let oldest_signatures = batch.sigs;

            let timeout_height: Uint256 = oldest_signed_batch.batch_timeout.into();
            if timeout_height < ethereum_block_height {
                warn!(
                    "Batch {}/{} has timed out and can not be submitted",
                    oldest_signed_batch.nonce, oldest_signed_batch.token_contract
                );
                continue;
            }

            let latest_cosmos_batch_nonce = oldest_signed_batch.clone().nonce;
            if latest_cosmos_batch_nonce > latest_ethereum_batch {
                let cost = ethereum_gravity::submit_batch::estimate_tx_batch_cost(
                    current_valset.clone(),
                    oldest_signed_batch.clone(),
                    &oldest_signatures,
                    web3,
                    gravity_contract_address,
                    gravity_id.clone(),
                    ethereum_key.to_address(),
                )
                .await;
                if cost.is_err() {
                    error!("Batch cost estimate failed with {:?}", cost);
                    continue;
                }
                let cost = cost.unwrap();

                info!(
                    "We have detected a batch to relay. This batch is estimated to cost {} Gas @ {} gwei / {:.4} ETH to submit",
                    cost.gas.clone(),
                    print_gwei(cost.gas_price.clone()),
                    print_eth(cost.get_total())
                );
                oldest_signed_batch
                    .display_with_eth_info(our_ethereum_address, web3)
                    .await;

                let (should_relay, reward_in_weth) = should_relay_batch(
                    web3,
                    &oldest_signed_batch,
                    cost.get_total(),
                    our_ethereum_address,
                    &config.batch_relaying_mode,
                )
                .await;

                if should_relay {
                    info!(
                        "Attempting to relay batch {}/{} Expected Cost: {:} Reward: {:?}",
                        oldest_signed_batch.token_contract,
                        oldest_signed_batch.nonce,
                        print_eth(cost.get_total()),
                        reward_in_weth.clone().map(print_eth),
                    );
                    let res = send_eth_transaction_batch(
                        current_valset.clone(),
                        oldest_signed_batch,
                        &oldest_signatures,
                        web3,
                        ETH_SUBMIT_WAIT_TIME,
                        gravity_contract_address,
                        gravity_id.clone(),
                        ethereum_key,
                    )
                    .await;
                    if res.is_err() {
                        info!("Batch submission failed with {:?}", res);
                    }
                } else {
                    info!(
                        "Not relaying batch {}/{} due to it not being profitable. Cost: {}, Reward: {:?}",
                        oldest_signed_batch.token_contract, oldest_signed_batch.nonce, print_eth(cost.get_total()), reward_in_weth.clone().map(print_eth),
                    );
                }
            }
        }
    }
}
