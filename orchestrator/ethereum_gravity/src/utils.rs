use std::time::Duration;

use clarity::abi::encode_call;
use clarity::PrivateKey;

use clarity::Uint256;
use clarity::{abi::Token, constants::zero_address};
use gravity_utils::error::GravityError;
use gravity_utils::num_conversion::downcast_uint256;
use gravity_utils::types::*;

use web30::types::SendTxOption;
use web30::{client::Web3, jsonrpc::error::Web3Error, EthAddress};

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
        self.gas.clone() * self.gas_price.clone()
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
    let reward_amount = valset.reward_amount.clone();
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

pub async fn send_transaction(
    web3: &Web3,
    contract: EthAddress,
    selector: &str,
    tokens: &[Token],
    sender_secret: PrivateKey,
    wait_timeout: Option<Duration>,
    options: Vec<SendTxOption>,
) -> Result<Uint256, GravityError> {
    // extract method name for logging
    let method_name = &selector[..selector.find('(').unwrap_or(selector.len())];

    let sender_address = sender_secret.to_address();
    let tx_hash = web3
        .send_transaction(
            contract,
            selector,
            &tokens,
            0u32.into(),
            sender_address,
            sender_secret,
            options,
        )
        .await?;

    info!("Call {} with txid {:#066x}", method_name, tx_hash);

    if let Some(timeout) = wait_timeout {
        web3.wait_for_transaction(tx_hash.clone(), timeout, None)
            .await?;
    }

    Ok(tx_hash)
}

#[cfg(test)]
mod test {

    use std::str::FromStr;
    use web30::{EthAddress, TronAddress};

    #[test]
    fn address() {
        let evm_addr = EthAddress::from_str("0xf2846a1E4dAFaeA38C1660a618277d67605bd2B5").unwrap();
        let tron_addr: TronAddress = evm_addr.into();

        let evm_addr_again: EthAddress = tron_addr.into();
        assert_eq!(evm_addr, evm_addr_again);
    }

    #[test]
    fn gas_multiplier() {
        let estimated = 12f64;
        let fee_limit = (estimated * 1.3f64).round() as u64;
        assert_eq!(fee_limit, 16u64);
    }

    #[test]
    fn encode_tokens() {
        let evm_addr = EthAddress::from_str("0xf2846a1E4dAFaeA38C1660a618277d67605bd2B5").unwrap();
        let tokens = vec![clarity::abi::Token::Address(evm_addr)];
        let encoded1 = clarity::abi::encode_tokens(&tokens);

        let base58_addr = TronAddress::from_str("TY5X9ocQACH9YGAyiK3WUxLcLw3t2ethnc").unwrap();
        let tokens = vec![ethabi::Token::Address(base58_addr.into())];
        let encoded2 = ethabi::encode(&tokens);

        assert_eq!(encoded1, encoded2);
    }
}
