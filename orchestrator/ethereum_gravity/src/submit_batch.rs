use crate::message_signatures::encode_tx_batch_confirm_hashed;
use crate::utils::{encode_valset_struct, get_tx_batch_nonce, GasCost};
use clarity::PrivateKey as EthPrivateKey;
use clarity::{Address as EthAddress, Uint256};
use gravity_utils::error::GravityError;
use gravity_utils::types::*;
use std::{cmp::min, time::Duration};
use web30::{client::Web3, types::TransactionRequest};

/// this function generates an appropriate Ethereum transaction
/// to submit the provided transaction batch
#[allow(clippy::too_many_arguments)]
pub async fn send_eth_transaction_batch(
    current_valset: Valset,
    batch: TransactionBatch,
    confirms: &[BatchConfirmResponse],
    web3: &Web3,
    timeout: Duration,
    gravity_contract_address: EthAddress,
    gravity_id: String,
    our_eth_key: EthPrivateKey,
) -> Result<(), GravityError> {
    let new_batch_nonce = batch.nonce;
    let eth_address = our_eth_key.to_address();
    info!(
        "Ordering signatures and submitting TransactionBatch {}:{} to Ethereum",
        batch.token_contract, new_batch_nonce
    );
    trace!("Batch {:?}", batch);

    let before_nonce = get_tx_batch_nonce(
        gravity_contract_address,
        batch.token_contract,
        eth_address,
        web3,
    )
    .await?;
    let current_block_height = web3.eth_block_number().await?;
    if before_nonce >= new_batch_nonce {
        info!(
            "Someone else updated the batch to {}, exiting early",
            before_nonce
        );
        return Ok(());
    } else if current_block_height > batch.batch_timeout.into() {
        info!(
            "This batch is timed out. timeout block: {} current block: {}, exiting early",
            current_block_height, batch.batch_timeout
        );
        return Ok(());
    }

    let payload = encode_batch_payload(current_valset, &batch, confirms, gravity_id)?;
    let tx = web3
        .send_transaction(
            gravity_contract_address,
            payload,
            0u32.into(),
            our_eth_key,
            vec![],
        )
        .await?;
    info!("Sent batch update with txid {:#066x}", tx);

    web3.wait_for_transaction(tx, timeout, None).await?;

    let last_nonce = get_tx_batch_nonce(
        gravity_contract_address,
        batch.token_contract,
        eth_address,
        web3,
    )
    .await?;
    if last_nonce != new_batch_nonce {
        error!(
            "Current nonce is {} expected to update to nonce {}",
            last_nonce, new_batch_nonce
        );
    } else {
        info!("Successfully updated Batch with new Nonce {:?}", last_nonce);
    }
    Ok(())
}

/// Returns the cost in Eth of sending this batch, because of the way eip1559 fee
/// computation works the minimum allowed gas price will trend up or down by as much
/// as 12.5% per block. In order to prevent race conditions we pad our estimate by 20%
/// if the gas price has in fact gone down we'll be refunded. But we must bake
/// this uncertainty into our cost estimates
pub async fn estimate_tx_batch_cost(
    current_valset: Valset,
    batch: TransactionBatch,
    confirms: &[BatchConfirmResponse],
    web3: &Web3,
    gravity_contract_address: EthAddress,
    gravity_id: String,
    our_eth_address: EthAddress,
) -> Result<GasCost, GravityError> {
    let our_balance = web3.eth_get_balance(our_eth_address).await?;
    let our_nonce = web3.eth_get_transaction_count(our_eth_address).await?;
    let chain_id = web3
        .eth_chainid()
        .await?
        .expect("Failed to get chain id from ETH node?");
    let gas_limit = min((u64::MAX - 1).into(), our_balance);
    let gas_price = web3.eth_gas_price().await?;
    // increase the value by 20% without using floating point multiplication
    let gas_price = gas_price + (gas_price / 5u8.into());
    let zero: Uint256 = 0u8.into();
    let val = web3
        .eth_estimate_gas(TransactionRequest::Eip1559 {
            access_list: None,
            chain_id: Some(chain_id.into()),
            from: our_eth_address,
            to: gravity_contract_address,
            nonce: Some(our_nonce.into()),
            max_fee_per_gas: Some(gas_price.into()),
            max_priority_fee_per_gas: None,
            gas: Some(gas_limit.into()),
            value: Some(zero.into()),
            data: Some(encode_batch_payload(current_valset, &batch, confirms, gravity_id)?.into()),
        })
        .await?;

    Ok(GasCost {
        gas: val,
        gas_price,
    })
}

/// Encodes the batch payload for both estimate_tx_batch_cost and send_eth_transaction_batch
fn encode_batch_payload(
    current_valset: Valset,
    batch: &TransactionBatch,
    confirms: &[BatchConfirmResponse],
    gravity_id: String,
) -> Result<Vec<u8>, GravityError> {
    let current_valset_token = encode_valset_struct(&current_valset);
    let new_batch_nonce = batch.nonce;
    let hash = encode_tx_batch_confirm_hashed(gravity_id, batch.clone());
    let sig_data = current_valset.order_sigs(&hash, confirms)?;
    let sig_arrays = to_arrays(sig_data);
    let (amounts, destinations, fees) = batch.get_checkpoint_values();

    // Solidity function signature
    // function submitBatch(
    // // The validators that approve the batch and new valset encoded as a ValsetArgs struct
    // address[] memory _currentValidators,
    // uint256[] memory _currentPowers,
    // uint256 _currentValsetNonce,
    // uint256 _rewardAmount,
    // address _rewardToken,
    //
    // // These are arrays of the parts of the validators signatures
    // uint8[] memory _v,
    // bytes32[] memory _r,
    // bytes32[] memory _s,
    // // The batch of transactions
    // uint256[] memory _amounts,
    // address[] memory _destinations,
    // uint256[] memory _fees,
    // uint256 _batchNonce,
    // address _tokenContract,
    // uint256 _batchTimeout
    let tokens = &[
        current_valset_token,
        sig_arrays.sigs,
        amounts,
        destinations,
        fees,
        new_batch_nonce.into(),
        batch.token_contract.into(),
        batch.batch_timeout.into(),
    ];
    let payload = clarity::abi::encode_call("submitBatch((address[],uint256[],uint256,uint256,address),(uint8,bytes32,bytes32)[],uint256[],address[],uint256[],uint256,address,uint256)",
    tokens).unwrap();
    trace!("Tokens {:?}", tokens);

    Ok(payload)
}
