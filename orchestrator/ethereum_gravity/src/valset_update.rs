use crate::message_signatures::encode_valset_confirm_hashed;
use crate::utils::{encode_valset_struct, get_valset_nonce, GasCost};
use clarity::PrivateKey as EthPrivateKey;
use clarity::{Address as EthAddress, Uint256};
use gravity_utils::error::GravityError;
use gravity_utils::types::*;
use std::{cmp::min, time::Duration};
use web30::{client::Web3, types::TransactionRequest};

/// this function generates an appropriate Ethereum transaction
/// to submit the provided validator set and signatures.
#[allow(clippy::too_many_arguments)]
pub async fn send_eth_valset_update(
    new_valset: Valset,
    old_valset: Valset,
    confirms: &[ValsetConfirmResponse],
    web3: &Web3,
    timeout: Duration,
    gravity_contract_address: EthAddress,
    gravity_id: String,
    our_eth_key: EthPrivateKey,
) -> Result<(), GravityError> {
    let old_nonce = old_valset.nonce;
    let new_nonce = new_valset.nonce;
    assert!(new_nonce > old_nonce);
    let eth_address = our_eth_key.to_address();
    info!(
        "Ordering signatures and submitting validator set {} -> {} update to Ethereum",
        old_nonce, new_nonce
    );
    let before_nonce = get_valset_nonce(gravity_contract_address, eth_address, web3).await?;
    if before_nonce != old_nonce {
        info!(
            "Someone else updated the valset to {}, exiting early",
            before_nonce
        );
        return Ok(());
    }

    let payload = encode_valset_update_payload(new_valset, old_valset, confirms, gravity_id)?;

    let tx = web3
        .send_transaction(
            gravity_contract_address,
            payload,
            0u32.into(),
            our_eth_key,
            vec![],
        )
        .await?;
    info!("Sent valset update with txid {:#066x}", tx);

    web3.wait_for_transaction(tx, timeout, None).await?;

    let last_nonce = get_valset_nonce(gravity_contract_address, eth_address, web3).await?;
    if last_nonce != new_nonce {
        error!(
            "Current nonce is {} expected to update to nonce {}",
            last_nonce, new_nonce
        );
    } else {
        info!(
            "Successfully updated Valset with new Nonce {:?}",
            last_nonce
        );
    }
    Ok(())
}

/// Returns the cost in Eth of sending this valset update, because of the way eip1559 fee
/// computation works the minimum allowed gas price will trend up or down by as much
/// as 12.5% per block. In order to prevent race conditions we pad our estimate by 20%
/// if the gas price has in fact gone down we'll be refunded. But we must bake
/// this uncertainty into our cost estimates
pub async fn estimate_valset_cost(
    new_valset: &Valset,
    old_valset: &Valset,
    confirms: &[ValsetConfirmResponse],
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
        .expect("No chain id response from eth node?");
    let gas_limit = min((u64::MAX - 1).into(), our_balance);
    let gas_price = web3.eth_gas_price().await?;
    // increase the value by 20% without using floating point multiplication
    let gas_price = gas_price + (gas_price / 5u8.into());
    let zero: Uint256 = 0u8.into();
    let val = web3
        .eth_estimate_gas(TransactionRequest::Eip1559 {
            chain_id: Some(chain_id.into()),
            access_list: None,
            from: our_eth_address,
            to: gravity_contract_address,
            nonce: Some(our_nonce.into()),
            max_fee_per_gas: Some(gas_price.into()),
            max_priority_fee_per_gas: None,
            gas: Some(gas_limit.into()),
            value: Some(zero.into()),
            data: Some(
                encode_valset_update_payload(
                    new_valset.clone(),
                    old_valset.clone(),
                    confirms,
                    gravity_id,
                )?
                .into(),
            ),
        })
        .await?;

    Ok(GasCost {
        gas: val,
        gas_price,
    })
}

/// Encodes the payload bytes for the validator set update call, useful for
/// estimating the cost of submitting a validator set
pub fn encode_valset_update_payload(
    new_valset: Valset,
    old_valset: Valset,
    confirms: &[ValsetConfirmResponse],
    gravity_id: String,
) -> Result<Vec<u8>, GravityError> {
    let new_valset_token = encode_valset_struct(&new_valset);
    let old_valset_token = encode_valset_struct(&old_valset);

    // remember the signatures are over the new valset and therefore this is the value we must encode
    // the old valset exists only as a hash in the ethereum store
    let hash = encode_valset_confirm_hashed(gravity_id, new_valset);
    // we need to use the old valset here because our signatures need to match the current
    // members of the validator set in the contract.
    let sig_data = old_valset.order_sigs(&hash, confirms)?;
    let sig_arrays = to_arrays(sig_data);

    // Solidity function signature
    // function updateValset(
    // // The new version of the validator set encoded as a ValsetArgsStruct
    // address[] memory _newValidators,
    // uint256[] memory _newPowers,
    // uint256 _newValsetNonce,
    // uint256 _newRewardAmount,
    // address _newRewardToken,
    //
    // // The current validators that approve the change encoded as a ValsetArgsStruct
    // // note that these rewards where *already* issued but we must pass it back in for the hash
    // address[] memory _currentValidators,
    // uint256[] memory _currentPowers,
    // uint256 _currentValsetNonce,
    // uint256 _currentRewardAmount,
    // address _currentRewardToken,
    //
    // // These are arrays of the parts of the current validator's signatures
    // Signature[] _sigs,
    let tokens = &[new_valset_token, old_valset_token, sig_arrays.sigs];
    let payload = clarity::abi::encode_call("updateValset((address[],uint256[],uint256,uint256,address),(address[],uint256[],uint256,uint256,address),(uint8,bytes32,bytes32)[])",
    tokens).unwrap();

    Ok(payload)
}
