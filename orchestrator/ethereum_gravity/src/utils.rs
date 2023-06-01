use std::collections::HashMap;
use std::collections::HashSet;

use clarity::abi::encode_call;
use clarity::Address as EthAddress;
use clarity::Uint256;
use clarity::{abi::Token, constants::zero_address};
use deep_space::error::CosmosGrpcError;
use futures::future::join_all;
use gravity_utils::error::GravityError;
use gravity_utils::num_conversion::downcast_uint256;
use gravity_utils::types::erc20::Erc20Token;
use gravity_utils::types::Valset;
use gravity_utils::types::*;
use web30::{client::Web3, jsonrpc::error::Web3Error};

/// Gets the latest validator set nonce
pub async fn get_valset_nonce(
    contract_address: EthAddress,
    caller_address: EthAddress,
    web3: &Web3,
) -> Result<u64, Web3Error> {
    let payload = encode_call("state_lastValsetNonce()", &[]).unwrap();
    let val = web3
        .simulate_transaction(contract_address, 0u8.into(), payload, caller_address, None)
        .await?;
    // the go represents all nonces as u64, there's no
    // reason they should ever overflow without a user
    // submitting millions or tens of millions of dollars
    // worth of transactions. But we properly check and
    // handle that case here.
    let real_num = Uint256::from_be_bytes(&val);
    Ok(downcast_uint256(real_num).expect("Valset nonce overflow! Bridge Halt!"))
}

/// Gets the latest transaction batch nonce
pub async fn get_tx_batch_nonce(
    gravity_contract_address: EthAddress,
    erc20_contract_address: EthAddress,
    caller_address: EthAddress,
    web3: &Web3,
) -> Result<u64, Web3Error> {
    let payload = encode_call("lastBatchNonce(address)", &[erc20_contract_address.into()]).unwrap();
    let val = web3
        .simulate_transaction(
            gravity_contract_address,
            0u8.into(),
            payload,
            caller_address,
            None,
        )
        .await?;
    // the go represents all nonces as u64, there's no
    // reason they should ever overflow without a user
    // submitting millions or tens of millions of dollars
    // worth of transactions. But we properly check and
    // handle that case here.
    let real_num = Uint256::from_be_bytes(&val);
    Ok(downcast_uint256(real_num).expect("TxBatch nonce overflow! Bridge Halt!"))
}

/// Gets the latest transaction batch nonce
pub async fn get_logic_call_nonce(
    gravity_contract_address: EthAddress,
    invalidation_id: Vec<u8>,
    caller_address: EthAddress,
    web3: &Web3,
) -> Result<u64, Web3Error> {
    let payload = encode_call(
        "lastLogicCallNonce(bytes32)",
        &[Token::Bytes(invalidation_id)],
    )
    .unwrap();
    let val = web3
        .simulate_transaction(
            gravity_contract_address,
            0u8.into(),
            payload,
            caller_address,
            None,
        )
        .await?;
    // the go represents all nonces as u64, there's no
    // reason they should ever overflow without a user
    // submitting millions or tens of millions of dollars
    // worth of transactions. But we properly check and
    // handle that case here.
    let real_num = Uint256::from_be_bytes(&val);
    Ok(downcast_uint256(real_num).expect("LogicCall nonce overflow! Bridge Halt!"))
}

/// Gets the latest transaction batch nonce
pub async fn get_event_nonce(
    gravity_contract_address: EthAddress,
    caller_address: EthAddress,
    web3: &Web3,
) -> Result<u64, Web3Error> {
    let payload = encode_call("state_lastEventNonce()", &[]).unwrap();
    let val = web3
        .simulate_transaction(
            gravity_contract_address,
            0u8.into(),
            payload,
            caller_address,
            None,
        )
        .await?;
    // the go represents all nonces as u64, there's no
    // reason they should ever overflow without a user
    // submitting millions or tens of millions of dollars
    // worth of transactions. But we properly check and
    // handle that case here.
    let real_num = Uint256::from_be_bytes(&val);
    Ok(downcast_uint256(real_num).expect("EventNonce nonce overflow! Bridge Halt!"))
}

/// Gets the gravityID
pub async fn get_gravity_id(
    contract_address: EthAddress,
    caller_address: EthAddress,
    web3: &Web3,
) -> Result<String, Web3Error> {
    let payload = encode_call("state_gravityId()", &[]).unwrap();
    let val = web3
        .simulate_transaction(contract_address, 0u8.into(), payload, caller_address, None)
        .await?;
    let gravity_id = String::from_utf8(val);
    match gravity_id {
        Ok(val) => Ok(val),
        Err(e) => Err(Web3Error::BadResponse(e.to_string())),
    }
}
/// This function is used to retrieve the state_gravitySolAddress from
/// GravityERC721.sol. Note that only address that is allowed to call the withdraw
/// function on GravityERC721.sol is the state_gravitySolAddress which is
/// set at contract instantiation
pub async fn get_gravity_sol_address(
    contract_address: EthAddress,
    caller_address: EthAddress,
    web3: &Web3,
) -> Result<EthAddress, Web3Error> {
    let payload = encode_call("state_gravitySolAddress()", &[]).unwrap();
    let val = web3
        .simulate_transaction(contract_address, 0u8.into(), payload, caller_address, None)
        .await?;

    let mut data: [u8; 20] = Default::default();
    data.copy_from_slice(&val[12..]);
    let gravity_sol_address = EthAddress::from_slice(&data);

    match gravity_sol_address {
        Ok(address_response) => Ok(address_response),
        Err(e) => Err(Web3Error::BadResponse(e.to_string())),
    }
}

/// Just a helper struct to represent the cost of actions on Ethereum
#[derive(Debug, Default, Clone)]
pub struct GasCost {
    /// The amount of gas spent
    pub gas: Uint256,
    /// The price of the gas
    pub gas_price: Uint256,
}

impl GasCost {
    /// Gets the total cost in Eth (or other EVM chain native token)
    /// of executing the batch
    pub fn get_total(&self) -> Uint256 {
        self.gas * self.gas_price
    }
}

/// This encodes the solidity struct ValsetArgs from the Gravity
/// contract useful for all three major contract calls
/// struct ValsetArgs {
///     address[] validators;
///     uint256[] powers;
///     uint256 valsetNonce;
///     uint256 rewardAmount;
///     address rewardToken;
// }
pub fn encode_valset_struct(valset: &Valset) -> Token {
    let (addresses, powers) = valset.to_arrays();
    let nonce = valset.nonce;
    let reward_amount = valset.reward_amount;
    // the zero address represents 'no reward' in this case we have replaced it with a 'none'
    // so that it's easy to identify if this validator set has a reward or not. Now that we're
    // going to encode it for the contract call we need return it to the magic value the contract
    // expects.
    let reward_token = valset.reward_token.unwrap_or(zero_address());
    let struct_tokens = &[
        addresses.into(),
        powers.into(),
        nonce.into(),
        reward_amount.into(),
        reward_token.into(),
    ];
    Token::Struct(struct_tokens.to_vec())
}

#[allow(clippy::too_many_arguments)]
/// Collects the needed Gravity.sol balances for EthereumClaim submissions by first learning which
/// contracts to monitor, then collecting all the ethereum heights needed and then finally
/// querying the ERC20 balances required, packing it all up into a Map
pub async fn collect_eth_balances_for_claims(
    web3: &Web3,
    querier_address: EthAddress,
    gravity_contract: EthAddress,
    monitored_erc20s: Vec<EthAddress>,
    deposits: &[SendToCosmosEvent],
    withdraws: &[TransactionBatchExecutedEvent],
    erc20_deploys: &[Erc20DeployedEvent],
    logic_calls: &[LogicCallExecutedEvent],
    valsets: &[ValsetUpdatedEvent],
) -> Result<HashMap<Uint256, Vec<Erc20Token>>, GravityError> {
    let heights =
        get_heights_from_eth_claims(deposits, withdraws, erc20_deploys, logic_calls, valsets).await;
    if heights.is_empty()
        && !(withdraws.is_empty()
            && erc20_deploys.is_empty()
            && logic_calls.is_empty()
            && valsets.is_empty())
    {
        return Err(GravityError::CosmosGrpcError(CosmosGrpcError::BadInput(
            "Invalid claims to collect balances for!".to_string(),
        )));
    }
    info!("Collecting Eth balances at heights {:?}", heights);
    collect_eth_balances_at_heights(
        web3,
        querier_address,
        gravity_contract,
        &monitored_erc20s,
        &heights,
    )
    .await
}

/// Collects the block_height value from each of the input *Event collections
pub async fn get_heights_from_eth_claims(
    deposits: &[SendToCosmosEvent],
    withdraws: &[TransactionBatchExecutedEvent],
    erc20_deploys: &[Erc20DeployedEvent],
    logic_calls: &[LogicCallExecutedEvent],
    valsets: &[ValsetUpdatedEvent],
) -> Vec<Uint256> {
    let mut heights = HashSet::new();
    for d in deposits {
        heights.insert(d.block_height);
    }
    for w in withdraws {
        heights.insert(w.block_height);
    }
    for e in erc20_deploys {
        heights.insert(e.block_height);
    }
    for l in logic_calls {
        heights.insert(l.block_height);
    }
    for v in valsets {
        heights.insert(v.block_height);
    }

    heights.into_iter().collect::<Vec<Uint256>>()
}

/// Fetches the balances of the Gravity.sol contract balance of each erc20 at each ethereum height provided
/// Does not populate the result for a height if any eth balance could not be obtained
pub async fn collect_eth_balances_at_heights(
    web3: &Web3,
    querier_address: EthAddress,
    gravity_contract: EthAddress,
    erc20s: &[EthAddress],
    heights: &[Uint256],
) -> Result<HashMap<Uint256, Vec<Erc20Token>>, GravityError> {
    let mut balances_by_height = HashMap::new();
    for h in heights {
        let bals =
            collect_eth_balances_at_height(web3, querier_address, gravity_contract, erc20s, *h)
                .await;
        if bals.is_err() {
            info!(
                "Could not query gravity eth balances at height {}: {:?}",
                h.to_string(),
                bals.unwrap_err(),
            );
            continue;
        }
        balances_by_height.insert(*h, bals.unwrap());
    }
    if balances_by_height.is_empty() {
        return Err(GravityError::EthereumRestError(Web3Error::BadResponse(
            "Unable to collect ERC20 balances by height".to_string(),
        )));
    }
    Ok(balances_by_height)
}

/// Fetches the balances of the Gravity.sol contract at the provided ethereum block height
/// Returns an error if any of the underlying queries return an error
pub async fn collect_eth_balances_at_height(
    web3: &Web3,
    querier_address: EthAddress,
    gravity_contract: EthAddress,
    erc20s: &[EthAddress],
    height: Uint256,
) -> Result<Vec<Erc20Token>, GravityError> {
    let mut futs = vec![];
    for e in erc20s {
        futs.push(web3.get_erc20_balance_at_height_as_address(
            Some(querier_address),
            *e,
            gravity_contract,
            Some(height),
        ));
    }
    let res = join_all(futs).await;
    let mut results = vec![];
    // Order of res is preserved, so we can assign the erc20 by index
    for (i, r) in res.into_iter().enumerate() {
        let erc20 = erc20s[i];
        results.push(Erc20Token {
            amount: r?,
            token_contract_address: erc20,
        });
    }

    Ok(results)
}
