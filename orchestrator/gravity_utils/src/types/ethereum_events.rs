//! This file parses the Gravity contract ethereum events. Note that there is no Ethereum ABI unpacking implementation. Instead each event
//! is parsed directly from it's binary representation. This is technical debt within this implementation. It's quite easy to parse any
//! individual event manually but a generic decoder can be quite challenging to implement. A proper implementation would probably closely
//! mirror Serde and perhaps even become a serde crate for Ethereum ABI decoding
//! For now reference the ABI encoding document here https://docs.soliditylang.org/en/v0.8.3/abi-spec.html

// TODO this file needs static assertions that prevent it from compiling on 16 bit systems.
// we assume a system bit width of at least 32

use super::ValsetMember;
use crate::error::GravityError;
use crate::num_conversion::downcast_uint256;
use clarity::constants::zero_address;
use clarity::Address as EthAddress;
use deep_space::utils::bytes_to_hex_str;
use deep_space::{Address as CosmosAddress, Address, Msg};
use gravity_proto::gravity::{
    MsgBatchSendToEthClaim, MsgErc20DeployedClaim, MsgLogicCallExecutedClaim, MsgSendToCosmosClaim,
    MsgValsetUpdatedClaim,
};
use num256::Uint256;
use std::unimplemented;
use web30::types::Log;

// gravity msg type urls
pub const MSG_BATCH_SEND_TO_ETH_TYPE_URL: &str = "/gravity.v1.MsgBatchSendToEthClaim";
pub const MSG_SEND_TO_COSMOS_CLAIM_TYPE_URL: &str = "/gravity.v1.MsgSendToCosmosClaim";
pub const MSG_ERC20_DEPLOYED_CLAIM_TYPE_URL: &str = "/gravity.v1.MsgERC20DeployedClaim";
pub const MSG_LOGIC_CALL_EXECUTED_CLAIM_TYPE_URL: &str = "/gravity.v1.MsgLogicCallExecutedClaim";
pub const MSG_VALSET_UPDATED_CLAIM_TYPE_URL: &str = "/gravity.v1.MsgValsetUpdatedClaim";

/// Used to limit the length of variable length user provided inputs like
/// ERC20 names and deposit destination strings
const ONE_MEGABYTE: usize = 1000usize.pow(3);

/// A type of event which must be sent to Gravity by Orchestrators in a claim, parsed from the
/// ethereum logs
pub trait EthereumEvent
where
    Self: Sized,
{
    fn get_block_height(&self) -> u64;
    fn get_event_nonce(&self) -> u64;
    /// Parses an event out of an Ethereum Log
    fn from_log(input: &Log) -> Result<Self, GravityError>;
    /// Parses multiple events of the same type out of an Ethereum Log
    fn from_logs(input: &[Log]) -> Result<Vec<Self>, GravityError>;
    /// Pares down a list of events to ones after the given `event_nonce`
    fn filter_by_event_nonce(event_nonce: u64, input: &[Self]) -> Vec<Self>;
    /// If the event with the given `event_nonce` is in `input`, returns the block that occurred on
    fn get_block_for_nonce(event_nonce: u64, input: &[Self]) -> Option<Uint256>;
    /// Creates the associated Msg for the given claim, e.g. ValsetUpdated -> MsgValsetUpdatedClaim
    fn to_claim_msg(self, orchestrator: Address) -> Msg;
}

/// A parsed struct representing the Ethereum event fired by the Gravity contract
/// when the validator set is updated.
#[derive(Serialize, Deserialize, Debug, Default, Clone, Eq, PartialEq, Hash)]
pub struct ValsetUpdatedEvent {
    pub valset_nonce: u64,
    pub event_nonce: u64,
    pub block_height: Uint256,
    pub reward_amount: Uint256,
    pub reward_token: Option<EthAddress>,
    pub members: Vec<ValsetMember>,
}

/// special return struct just for the data bytes components
#[derive(Serialize, Deserialize, Debug, Default, Clone, Eq, PartialEq, Hash)]
struct ValsetDataBytes {
    pub event_nonce: u64,
    pub reward_amount: Uint256,
    pub reward_token: Option<EthAddress>,
    pub members: Vec<ValsetMember>,
}

impl ValsetUpdatedEvent {
    /// Decodes the data bytes of a valset log event, separated for easy testing
    fn decode_data_bytes(input: &[u8]) -> Result<ValsetDataBytes, GravityError> {
        if input.len() < 6 * 32 {
            return Err(GravityError::InvalidEventLogError(
                "too short for ValsetUpdatedEventData".to_string(),
            ));
        }
        // first index is the event nonce, then the reward token, amount, following two have event data we don't
        // care about, sixth index contains the length of the eth address array

        // event nonce
        let index_start = 0;
        let index_end = index_start + 32;
        let nonce_data = &input[index_start..index_end];
        let event_nonce = Uint256::from_be_bytes(nonce_data);
        if event_nonce > u64::MAX.into() {
            return Err(GravityError::InvalidEventLogError(
                "Nonce overflow, probably incorrect parsing".to_string(),
            ));
        }
        let event_nonce: u64 = event_nonce.to_string().parse().unwrap();

        // reward amount
        let index_start = 32;
        let index_end = index_start + 32;
        let reward_amount_data = &input[index_start..index_end];
        let reward_amount = Uint256::from_be_bytes(reward_amount_data);

        // reward token
        let index_start = 2 * 32;
        let index_end = index_start + 32;
        let reward_token_data = &input[index_start..index_end];
        // addresses are 12 bytes shorter than the 32 byte field they are stored in
        let reward_token = EthAddress::from_slice(&reward_token_data[12..]);
        if let Err(e) = reward_token {
            return Err(GravityError::InvalidEventLogError(format!(
                "Bad reward address, must be incorrect parsing {:?}",
                e
            )));
        }
        let reward_token = reward_token.unwrap();
        // zero address represents no reward, so we replace it here with a none
        // for ease of checking in the future
        let reward_token = if reward_token == zero_address() {
            None
        } else {
            Some(reward_token)
        };

        // below this we are parsing the dynamic data from the 6th index on
        let index_start = 5 * 32;
        let index_end = index_start + 32;
        let eth_addresses_offset = index_end;
        if input.len() < eth_addresses_offset {
            return Err(GravityError::InvalidEventLogError(
                "too short for dynamic data".to_string(),
            ));
        }

        let len_eth_addresses = Uint256::from_be_bytes(&input[index_start..index_end]);
        if len_eth_addresses > usize::MAX.into() {
            return Err(GravityError::InvalidEventLogError(
                "Ethereum array len overflow, probably incorrect parsing".to_string(),
            ));
        }
        let len_eth_addresses: usize = len_eth_addresses.to_string().parse().unwrap();
        let index_start = (6 + len_eth_addresses) * 32;
        let index_end = index_start + 32;
        let powers_offset = index_end;
        if input.len() < powers_offset {
            return Err(GravityError::InvalidEventLogError(
                "too short for dynamic data".to_string(),
            ));
        }

        let len_powers = Uint256::from_be_bytes(&input[index_start..index_end]);
        if len_powers > usize::MAX.into() {
            return Err(GravityError::InvalidEventLogError(
                "Powers array len overflow, probably incorrect parsing".to_string(),
            ));
        }
        let len_powers: usize = len_eth_addresses.to_string().parse().unwrap();
        if len_powers != len_eth_addresses {
            return Err(GravityError::InvalidEventLogError(
                "Array len mismatch, probably incorrect parsing".to_string(),
            ));
        }

        let mut validators = Vec::new();
        for i in 0..len_eth_addresses {
            let power_start = (i * 32) + powers_offset;
            let power_end = power_start + 32;
            let address_start = (i * 32) + eth_addresses_offset;
            let address_end = address_start + 32;

            if input.len() < address_end || input.len() < power_end {
                return Err(GravityError::InvalidEventLogError(
                    "too short for dynamic data".to_string(),
                ));
            }

            let power = Uint256::from_be_bytes(&input[power_start..power_end]);
            // an eth address at 20 bytes is 12 bytes shorter than the Uint256 it's stored in.
            let eth_address = EthAddress::from_slice(&input[address_start + 12..address_end]);
            if eth_address.is_err() {
                return Err(GravityError::InvalidEventLogError(
                    "Ethereum Address parsing error, probably incorrect parsing".to_string(),
                ));
            }
            let eth_address = eth_address.unwrap();
            if power > u64::MAX.into() {
                return Err(GravityError::InvalidEventLogError(
                    "Power greater than u64::MAX, probably incorrect parsing".to_string(),
                ));
            }
            let power: u64 = power.to_string().parse().unwrap();
            validators.push(ValsetMember { power, eth_address })
        }
        let mut check = validators.clone();
        check.sort();
        check.reverse();
        // if the validator set is not sorted we're in a bad spot
        if validators != check {
            trace!(
                "Someone submitted an unsorted validator set, this means all updates will fail until someone feeds in this unsorted value by hand {:?} instead of {:?}",
                validators, check
            );
        }

        Ok(ValsetDataBytes {
            event_nonce,
            members: validators,
            reward_amount,
            reward_token,
        })
    }
}

impl EthereumEvent for ValsetUpdatedEvent {
    fn get_block_height(&self) -> u64 {
        downcast_uint256(self.block_height).unwrap()
    }

    fn get_event_nonce(&self) -> u64 {
        self.event_nonce
    }

    /// This function is not an abi compatible bytes parser, but it's actually
    /// not hard at all to extract data like this by hand.
    fn from_log(input: &Log) -> Result<ValsetUpdatedEvent, GravityError> {
        // we have one indexed event so we should find two indexes, one the event itself
        // and one the indexed nonce
        if input.topics.get(1).is_none() {
            return Err(GravityError::InvalidEventLogError(
                "Too few topics".to_string(),
            ));
        }
        let valset_nonce_data = &input.topics[1];
        let valset_nonce = Uint256::from_be_bytes(valset_nonce_data);
        if valset_nonce > u64::MAX.into() {
            return Err(GravityError::InvalidEventLogError(
                "Nonce overflow, probably incorrect parsing".to_string(),
            ));
        }
        let valset_nonce: u64 = valset_nonce.to_string().parse().unwrap();

        let block_height = if let Some(bn) = input.block_number {
            if bn > u64::MAX.into() {
                return Err(GravityError::InvalidEventLogError(
                    "Event nonce overflow! probably incorrect parsing".to_string(),
                ));
            } else {
                bn
            }
        } else {
            return Err(GravityError::InvalidEventLogError(
                "Log does not have block number, we only search logs already in blocks?"
                    .to_string(),
            ));
        };

        let decoded_bytes = Self::decode_data_bytes(&input.data)?;

        Ok(ValsetUpdatedEvent {
            valset_nonce,
            event_nonce: decoded_bytes.event_nonce,
            block_height,
            reward_amount: decoded_bytes.reward_amount,
            reward_token: decoded_bytes.reward_token,
            members: decoded_bytes.members,
        })
    }

    fn from_logs(input: &[Log]) -> Result<Vec<ValsetUpdatedEvent>, GravityError> {
        let mut res = Vec::new();
        for item in input {
            res.push(ValsetUpdatedEvent::from_log(item)?);
        }
        Ok(res)
    }
    /// returns all values in the array with event nonces greater
    /// than the provided value
    fn filter_by_event_nonce(event_nonce: u64, input: &[Self]) -> Vec<Self> {
        let mut ret = Vec::new();
        for item in input {
            if item.event_nonce > event_nonce {
                ret.push(item.clone())
            }
        }
        ret
    }
    // gets the Ethereum block for the given nonce
    fn get_block_for_nonce(event_nonce: u64, input: &[Self]) -> Option<Uint256> {
        for item in input {
            if item.event_nonce == event_nonce {
                return Some(item.block_height);
            }
        }
        None
    }

    fn to_claim_msg(self, orchestrator: Address) -> Msg {
        let claim = MsgValsetUpdatedClaim {
            event_nonce: self.event_nonce,
            valset_nonce: self.valset_nonce,
            eth_block_height: self.get_block_height(),
            members: self.members.iter().map(|v| v.into()).collect(),
            reward_amount: self.reward_amount.to_string(),
            reward_token: self.reward_token.unwrap_or(zero_address()).to_string(),
            orchestrator: orchestrator.to_string(),
        };
        Msg::new(MSG_VALSET_UPDATED_CLAIM_TYPE_URL, claim)
    }
}

/// A parsed struct representing the Ethereum event fired by the Gravity contract when
/// a transaction batch is executed.
#[derive(Serialize, Deserialize, Debug, Default, Clone, Eq, PartialEq, Hash)]
pub struct TransactionBatchExecutedEvent {
    /// the nonce attached to the transaction batch that follows
    /// it throughout it's lifecycle
    pub batch_nonce: u64,
    /// The block height this event occurred at
    pub block_height: Uint256,
    /// The ERC20 token contract address for the batch executed, since batches are uniform
    /// in token type there is only one
    pub erc20: EthAddress,
    /// the event nonce representing a unique ordering of events coming out
    /// of the Gravity solidity contract. Ensuring that these events can only be played
    /// back in order
    pub event_nonce: u64,
}

impl EthereumEvent for TransactionBatchExecutedEvent {
    fn get_block_height(&self) -> u64 {
        downcast_uint256(self.block_height).unwrap()
    }

    fn get_event_nonce(&self) -> u64 {
        self.event_nonce
    }

    fn from_log(input: &Log) -> Result<TransactionBatchExecutedEvent, GravityError> {
        if let (Some(batch_nonce_data), Some(erc20_data)) =
            (input.topics.get(1), input.topics.get(2))
        {
            let batch_nonce = Uint256::from_be_bytes(batch_nonce_data);
            let erc20 = EthAddress::from_slice(&erc20_data[12..32])?;
            let event_nonce = Uint256::from_be_bytes(&input.data);
            let block_height = if let Some(bn) = input.block_number {
                if bn > u64::MAX.into() {
                    return Err(GravityError::InvalidEventLogError(
                        "Block height overflow! probably incorrect parsing".to_string(),
                    ));
                } else {
                    bn
                }
            } else {
                return Err(GravityError::InvalidEventLogError(
                    "Log does not have block number, we only search logs already in blocks?"
                        .to_string(),
                ));
            };
            if event_nonce > u64::MAX.into()
                || batch_nonce > u64::MAX.into()
                || block_height > u64::MAX.into()
            {
                Err(GravityError::InvalidEventLogError(
                    "Event nonce overflow, probably incorrect parsing".to_string(),
                ))
            } else {
                let batch_nonce: u64 = batch_nonce.to_string().parse().unwrap();
                let event_nonce: u64 = event_nonce.to_string().parse().unwrap();
                Ok(TransactionBatchExecutedEvent {
                    batch_nonce,
                    block_height,
                    erc20,
                    event_nonce,
                })
            }
        } else {
            Err(GravityError::InvalidEventLogError(
                "Too few topics".to_string(),
            ))
        }
    }
    fn from_logs(input: &[Log]) -> Result<Vec<TransactionBatchExecutedEvent>, GravityError> {
        let mut res = Vec::new();
        for item in input {
            res.push(TransactionBatchExecutedEvent::from_log(item)?);
        }
        Ok(res)
    }
    /// returns all values in the array with event nonces greater
    /// than the provided value
    fn filter_by_event_nonce(event_nonce: u64, input: &[Self]) -> Vec<Self> {
        let mut ret = Vec::new();
        for item in input {
            if item.event_nonce > event_nonce {
                ret.push(item.clone())
            }
        }
        ret
    }

    // gets the Ethereum block for the given nonce
    fn get_block_for_nonce(event_nonce: u64, input: &[Self]) -> Option<Uint256> {
        for item in input {
            if item.event_nonce == event_nonce {
                return Some(item.block_height);
            }
        }
        None
    }

    fn to_claim_msg(self, orchestrator: Address) -> Msg {
        let claim = MsgBatchSendToEthClaim {
            event_nonce: self.event_nonce,
            eth_block_height: self.get_block_height(),
            token_contract: self.erc20.to_string(),
            batch_nonce: self.batch_nonce,
            orchestrator: orchestrator.to_string(),
        };
        Msg::new(MSG_BATCH_SEND_TO_ETH_TYPE_URL, claim)
    }
}

/// A parsed struct representing the Ethereum event fired when someone makes a deposit
/// on the Gravity contract
#[derive(Serialize, Deserialize, Debug, Clone, Eq, PartialEq, Hash)]
pub struct SendToCosmosEvent {
    /// The token contract address for the deposit
    pub erc20: EthAddress,
    /// The Ethereum Sender
    pub sender: EthAddress,
    /// The Cosmos destination, this is a raw value from the Ethereum contract
    /// and therefore could be provided by an attacker. If the string is valid
    /// utf-8 it will be included here, if it is invalid utf8 we will provide
    /// an empty string. Values over 1mb of text are not permitted and will also
    /// be presented as empty
    pub destination: String,
    /// the validated destination is the destination string parsed and interpreted
    /// as a valid Bech32 Cosmos address, if this is not possible the value is none
    pub validated_destination: Option<CosmosAddress>,
    /// The amount of the erc20 token that is being sent
    pub amount: Uint256,
    /// The transaction's nonce, used to make sure there can be no accidental duplication
    pub event_nonce: u64,
    /// The block height this event occurred at
    pub block_height: Uint256,
}

/// struct for holding the data encoded fields
/// of a send to Cosmos event for unit testing
#[derive(Eq, PartialEq, Debug)]
struct SendToCosmosEventData {
    /// The Cosmos destination, None for an invalid deposit address
    pub destination: String,
    /// The amount of the erc20 token that is being sent
    pub amount: Uint256,
    /// The transaction's nonce, used to make sure there can be no accidental duplication
    pub event_nonce: Uint256,
}

impl SendToCosmosEvent {
    fn decode_data_bytes(data: &[u8]) -> Result<SendToCosmosEventData, GravityError> {
        if data.len() < 4 * 32 {
            return Err(GravityError::InvalidEventLogError(
                "too short for SendToCosmosEventData".to_string(),
            ));
        }

        let amount = Uint256::from_be_bytes(&data[32..64]);
        let event_nonce = Uint256::from_be_bytes(&data[64..96]);

        // discard words three and four which contain the data type and length
        let destination_str_len_start = 3 * 32;
        let destination_str_len_end = 4 * 32;
        let destination_str_len =
            Uint256::from_be_bytes(&data[destination_str_len_start..destination_str_len_end]);

        if destination_str_len > u32::MAX.into() {
            return Err(GravityError::InvalidEventLogError(
                "denom length overflow, probably incorrect parsing".to_string(),
            ));
        }
        let destination_str_len: usize = destination_str_len.to_string().parse().unwrap();

        let destination_str_start = 4 * 32;
        let destination_str_end = destination_str_start + destination_str_len;

        if data.len() < destination_str_end {
            return Err(GravityError::InvalidEventLogError(
                "Incorrect length for dynamic data".to_string(),
            ));
        }

        let destination = &data[destination_str_start..destination_str_end];

        let dest = String::from_utf8(destination.to_vec());
        if dest.is_err() {
            if destination.len() < 1000 {
                warn!("Event nonce {} sends tokens to {} which is invalid utf-8, these funds will be allocated to the community pool", event_nonce, bytes_to_hex_str(destination));
            } else {
                warn!("Event nonce {} sends tokens to a destination that is invalid utf-8, these funds will be allocated to the community pool", event_nonce);
            }
            return Ok(SendToCosmosEventData {
                destination: String::new(),
                event_nonce,
                amount,
            });
        }
        // whitespace can not be a valid part of a bech32 address, so we can safely trim it
        let dest = dest.unwrap().trim().to_string();

        if dest.as_bytes().len() > ONE_MEGABYTE {
            warn!("Event nonce {} sends tokens to a destination that exceeds the length limit, these funds will be allocated to the community pool", event_nonce);
            Ok(SendToCosmosEventData {
                destination: String::new(),
                event_nonce,
                amount,
            })
        } else {
            Ok(SendToCosmosEventData {
                destination: dest,
                event_nonce,
                amount,
            })
        }
    }
}
impl EthereumEvent for SendToCosmosEvent {
    fn get_block_height(&self) -> u64 {
        downcast_uint256(self.block_height).unwrap()
    }

    fn get_event_nonce(&self) -> u64 {
        self.event_nonce
    }

    fn from_log(input: &Log) -> Result<SendToCosmosEvent, GravityError> {
        let topics = (input.topics.get(1), input.topics.get(2));
        if let (Some(erc20_data), Some(sender_data)) = topics {
            let erc20 = EthAddress::from_slice(&erc20_data[12..32])?;
            let sender = EthAddress::from_slice(&sender_data[12..32])?;
            let block_height = if let Some(bn) = input.block_number {
                if bn > u64::MAX.into() {
                    return Err(GravityError::InvalidEventLogError(
                        "Block height overflow! probably incorrect parsing".to_string(),
                    ));
                } else {
                    bn
                }
            } else {
                return Err(GravityError::InvalidEventLogError(
                    "Log does not have block number, we only search logs already in blocks?"
                        .to_string(),
                ));
            };

            let data = SendToCosmosEvent::decode_data_bytes(&input.data)?;
            if data.event_nonce > u64::MAX.into() || block_height > u64::MAX.into() {
                Err(GravityError::InvalidEventLogError(
                    "Event nonce overflow, probably incorrect parsing".to_string(),
                ))
            } else {
                let event_nonce: u64 = data.event_nonce.to_string().parse().unwrap();
                let validated_destination = match data.destination.parse() {
                    Ok(v) => Some(v),
                    Err(_) => {
                        if data.destination.len() < 1000 {
                            warn!("Event nonce {} sends tokens to {} which is invalid bech32, these funds will be allocated to the community pool", event_nonce, data.destination);
                        } else {
                            warn!("Event nonce {} sends tokens to a destination which is invalid bech32, these funds will be allocated to the community pool", event_nonce);
                        }
                        None
                    }
                };
                Ok(SendToCosmosEvent {
                    erc20,
                    sender,
                    destination: data.destination,
                    validated_destination,
                    amount: data.amount,
                    event_nonce,
                    block_height,
                })
            }
        } else {
            Err(GravityError::InvalidEventLogError(
                "Too few topics".to_string(),
            ))
        }
    }

    fn from_logs(input: &[Log]) -> Result<Vec<SendToCosmosEvent>, GravityError> {
        let mut res = Vec::new();
        for item in input {
            res.push(SendToCosmosEvent::from_log(item)?);
        }
        Ok(res)
    }
    /// returns all values in the array with event nonces greater
    /// than the provided value
    fn filter_by_event_nonce(event_nonce: u64, input: &[Self]) -> Vec<Self> {
        let mut ret = Vec::new();
        for item in input {
            if item.event_nonce > event_nonce {
                ret.push(item.clone())
            }
        }
        ret
    }

    // gets the Ethereum block for the given nonce
    fn get_block_for_nonce(event_nonce: u64, input: &[Self]) -> Option<Uint256> {
        for item in input {
            if item.event_nonce == event_nonce {
                return Some(item.block_height);
            }
        }
        None
    }

    fn to_claim_msg(self, orchestrator: Address) -> Msg {
        let claim = MsgSendToCosmosClaim {
            event_nonce: self.event_nonce,
            eth_block_height: self.get_block_height(),
            token_contract: self.erc20.to_string(),
            amount: self.amount.to_string(),
            cosmos_receiver: self.destination,
            ethereum_sender: self.sender.to_string(),
            orchestrator: orchestrator.to_string(),
        };
        Msg::new(MSG_SEND_TO_COSMOS_CLAIM_TYPE_URL, claim)
    }
}

/// A parsed struct representing the Ethereum event fired when someone uses the Gravity
/// contract to deploy a new ERC20 contract representing a Cosmos asset
#[derive(Serialize, Deserialize, Debug, Default, Clone, Eq, PartialEq, Hash)]
pub struct Erc20DeployedEvent {
    /// The denom on the Cosmos chain this contract is intended to represent
    pub cosmos_denom: String,
    /// The ERC20 address of the deployed contract, this may or may not be adopted
    /// by the Cosmos chain as the contract for this asset
    pub erc20_address: EthAddress,
    /// The name of the token in the ERC20 contract, should match the Cosmos denom
    /// but it is up to the Cosmos module to check that
    pub name: String,
    /// The symbol for the token in the ERC20 contract
    pub symbol: String,
    /// The number of decimals required to represent the smallest unit of this token
    pub decimals: u8,
    pub event_nonce: u64,
    pub block_height: Uint256,
}

/// struct for holding the data encoded fields
/// of a Erc20DeployedEvent for unit testing
#[derive(Eq, PartialEq, Debug)]
struct Erc20DeployedEventData {
    /// The denom on the Cosmos chain this contract is intended to represent
    pub cosmos_denom: String,
    /// The name of the token in the ERC20 contract, should match the Cosmos denom
    /// but it is up to the Cosmos module to check that
    pub name: String,
    /// The symbol for the token in the ERC20 contract
    pub symbol: String,
    /// The number of decimals required to represent the smallest unit of this token
    pub decimals: u8,
    pub event_nonce: u64,
}
impl Erc20DeployedEvent {
    fn decode_data_bytes(data: &[u8]) -> Result<Erc20DeployedEventData, GravityError> {
        if data.len() < 6 * 32 {
            return Err(GravityError::InvalidEventLogError(
                "too short for Erc20DeployedEventData".to_string(),
            ));
        }

        // discard index 2 as it only contains type data
        let index_start = 3 * 32;
        let index_end = index_start + 32;

        let decimals = Uint256::from_be_bytes(&data[index_start..index_end]);
        if decimals > u8::MAX.into() {
            return Err(GravityError::InvalidEventLogError(
                "Decimals overflow, probably incorrect parsing".to_string(),
            ));
        }
        let decimals: u8 = decimals.to_string().parse().unwrap();

        let index_start = 4 * 32;
        let index_end = index_start + 32;
        let nonce = Uint256::from_be_bytes(&data[index_start..index_end]);
        if nonce > u64::MAX.into() {
            return Err(GravityError::InvalidEventLogError(
                "Nonce overflow, probably incorrect parsing".to_string(),
            ));
        }
        let event_nonce: u64 = nonce.to_string().parse().unwrap();

        let index_start = 5 * 32;
        let index_end = index_start + 32;
        let denom_len = Uint256::from_be_bytes(&data[index_start..index_end]);
        // it's not probable that we have 4+ gigabytes of event data
        if denom_len > u32::MAX.into() {
            return Err(GravityError::InvalidEventLogError(
                "denom length overflow, probably incorrect parsing".to_string(),
            ));
        }
        let denom_len: usize = denom_len.to_string().parse().unwrap();
        let index_start = 6 * 32;
        let index_end = index_start + denom_len;
        let denom = String::from_utf8(data[index_start..index_end].to_vec());
        trace!("Denom {:?}", denom);
        if denom.is_err() {
            warn!("Deployed ERC20 has invalid utf8, will not be adopted");
            // we must return a dummy event in order to finish processing
            // otherwise we halt the oracle
            return Ok(Erc20DeployedEventData {
                cosmos_denom: String::new(),
                name: String::new(),
                symbol: String::new(),
                decimals: 0,
                event_nonce,
            });
        }
        let denom = denom.unwrap();
        if denom.len() > ONE_MEGABYTE {
            warn!("Deployed ERC20 is too large! will not be adopted");
            // we must return a dummy event in order to finish processing
            // otherwise we halt the oracle
            return Ok(Erc20DeployedEventData {
                cosmos_denom: String::new(),
                name: String::new(),
                symbol: String::new(),
                decimals: 0,
                event_nonce,
            });
        }

        // beyond this point we are parsing strings placed
        // after a variable length string and we will need to compute offsets

        // this trick computes the next 32 byte (256 bit) word index, then multiplies by
        // 32 to get the bytes offset, this is required since we have dynamic length types but
        // the next entry always starts on a round 32 byte word.
        let index_start = ((index_end + 31) / 32) * 32;
        let index_end = index_start + 32;

        if data.len() < index_end {
            return Err(GravityError::InvalidEventLogError(
                "Erc20DeployedEvent dynamic data too short".to_string(),
            ));
        }

        let erc20_name_len = Uint256::from_be_bytes(&data[index_start..index_end]);
        // it's not probable that we have 4+ gigabytes of event data
        if erc20_name_len > u32::MAX.into() {
            return Err(GravityError::InvalidEventLogError(
                "ERC20 Name length overflow, probably incorrect parsing".to_string(),
            ));
        }
        let erc20_name_len: usize = erc20_name_len.to_string().parse().unwrap();
        let index_start = index_end;
        let index_end = index_start + erc20_name_len;

        if data.len() < index_end {
            return Err(GravityError::InvalidEventLogError(
                "Erc20DeployedEvent dynamic data too short".to_string(),
            ));
        }

        let erc20_name = String::from_utf8(data[index_start..index_end].to_vec());
        if erc20_name.is_err() {
            warn!("Deployed ERC20 has invalid utf8, will not be adopted");
            // we must return a dummy event in order to finish processing
            // otherwise we halt the oracle
            return Ok(Erc20DeployedEventData {
                cosmos_denom: String::new(),
                name: String::new(),
                symbol: String::new(),
                decimals: 0,
                event_nonce,
            });
        }
        trace!("ERC20 Name {:?}", erc20_name);
        let erc20_name = erc20_name.unwrap();
        if erc20_name.len() > ONE_MEGABYTE {
            warn!("Deployed ERC20 is too large! will not be adopted");
            // we must return a dummy event in order to finish processing
            // otherwise we halt the oracle
            return Ok(Erc20DeployedEventData {
                cosmos_denom: String::new(),
                name: String::new(),
                symbol: String::new(),
                decimals: 0,
                event_nonce,
            });
        }

        let index_start = ((index_end + 31) / 32) * 32;
        let index_end = index_start + 32;

        if data.len() < index_end {
            return Err(GravityError::InvalidEventLogError(
                "Erc20DeployedEvent dynamic data too short".to_string(),
            ));
        }

        let symbol_len = Uint256::from_be_bytes(&data[index_start..index_end]);
        // it's not probable that we have 4+ gigabytes of event data
        if symbol_len > u32::MAX.into() {
            return Err(GravityError::InvalidEventLogError(
                "Symbol length overflow, probably incorrect parsing".to_string(),
            ));
        }
        let symbol_len: usize = symbol_len.to_string().parse().unwrap();
        let index_start = index_end;
        let index_end = index_start + symbol_len;

        if data.len() < index_end {
            return Err(GravityError::InvalidEventLogError(
                "Erc20DeployedEvent dynamic data too short".to_string(),
            ));
        }

        let symbol = String::from_utf8(data[index_start..index_end].to_vec());
        trace!("Symbol {:?}", symbol);
        if symbol.is_err() {
            warn!("Deployed ERC20 has invalid utf8, will not be adopted");
            // we must return a dummy event in order to finish processing
            // otherwise we halt the oracle
            return Ok(Erc20DeployedEventData {
                cosmos_denom: String::new(),
                name: String::new(),
                symbol: String::new(),
                decimals: 0,
                event_nonce,
            });
        }
        let symbol = symbol.unwrap();
        if symbol.len() > ONE_MEGABYTE {
            warn!("Deployed ERC20 is too large! will not be adopted");
            // we must return a dummy event in order to finish processing
            // otherwise we halt the oracle
            return Ok(Erc20DeployedEventData {
                cosmos_denom: String::new(),
                name: String::new(),
                symbol: String::new(),
                decimals: 0,
                event_nonce,
            });
        }

        Ok(Erc20DeployedEventData {
            cosmos_denom: denom,
            name: erc20_name,
            symbol,
            decimals,
            event_nonce,
        })
    }
}

impl EthereumEvent for Erc20DeployedEvent {
    fn get_block_height(&self) -> u64 {
        downcast_uint256(self.block_height).unwrap()
    }

    fn get_event_nonce(&self) -> u64 {
        self.event_nonce
    }

    fn from_log(input: &Log) -> Result<Erc20DeployedEvent, GravityError> {
        let token_contract = input.topics.get(1);
        if let Some(new_token_contract_data) = token_contract {
            let erc20 = EthAddress::from_slice(&new_token_contract_data[12..32])?;

            let block_height = if let Some(bn) = input.block_number {
                if bn > u64::MAX.into() {
                    return Err(GravityError::InvalidEventLogError(
                        "Event nonce overflow! probably incorrect parsing".to_string(),
                    ));
                } else {
                    bn
                }
            } else {
                return Err(GravityError::InvalidEventLogError(
                    "Log does not have block number, we only search logs already in blocks?"
                        .to_string(),
                ));
            };

            let data = Erc20DeployedEvent::decode_data_bytes(&input.data)?;

            Ok(Erc20DeployedEvent {
                cosmos_denom: data.cosmos_denom,
                name: data.name,
                decimals: data.decimals,
                event_nonce: data.event_nonce,
                erc20_address: erc20,
                symbol: data.symbol,
                block_height,
            })
        } else {
            Err(GravityError::InvalidEventLogError(
                "Too few topics".to_string(),
            ))
        }
    }

    fn from_logs(input: &[Log]) -> Result<Vec<Erc20DeployedEvent>, GravityError> {
        let mut res = Vec::new();
        for item in input {
            res.push(Erc20DeployedEvent::from_log(item)?);
        }
        Ok(res)
    }
    /// returns all values in the array with event nonces greater
    /// than the provided value
    fn filter_by_event_nonce(event_nonce: u64, input: &[Self]) -> Vec<Self> {
        let mut ret = Vec::new();
        for item in input {
            if item.event_nonce > event_nonce {
                ret.push(item.clone())
            }
        }
        ret
    }

    // gets the Ethereum block for the given nonce
    fn get_block_for_nonce(event_nonce: u64, input: &[Self]) -> Option<Uint256> {
        for item in input {
            if item.event_nonce == event_nonce {
                return Some(item.block_height);
            }
        }
        None
    }

    fn to_claim_msg(self, orchestrator: Address) -> Msg {
        let claim = MsgErc20DeployedClaim {
            event_nonce: self.event_nonce,
            eth_block_height: self.get_block_height(),
            cosmos_denom: self.cosmos_denom,
            token_contract: self.erc20_address.to_string(),
            name: self.name,
            symbol: self.symbol,
            decimals: self.decimals as u64,
            orchestrator: orchestrator.to_string(),
        };
        Msg::new(MSG_ERC20_DEPLOYED_CLAIM_TYPE_URL, claim)
    }
}
/// A parsed struct representing the Ethereum event fired when someone uses the Gravity
/// contract to deploy a new ERC20 contract representing a Cosmos asset
#[derive(Serialize, Deserialize, Debug, Default, Clone, Eq, PartialEq, Hash)]
pub struct LogicCallExecutedEvent {
    pub invalidation_id: Vec<u8>,
    pub invalidation_nonce: u64,
    pub return_data: Vec<u8>,
    pub event_nonce: u64,
    pub block_height: Uint256,
}

impl EthereumEvent for LogicCallExecutedEvent {
    fn get_block_height(&self) -> u64 {
        downcast_uint256(self.block_height).unwrap()
    }

    fn get_event_nonce(&self) -> u64 {
        self.event_nonce
    }

    fn from_log(_input: &Log) -> Result<LogicCallExecutedEvent, GravityError> {
        unimplemented!()
    }
    fn from_logs(input: &[Log]) -> Result<Vec<LogicCallExecutedEvent>, GravityError> {
        let mut res = Vec::new();
        for item in input {
            res.push(LogicCallExecutedEvent::from_log(item)?);
        }
        Ok(res)
    }
    /// returns all values in the array with event nonces greater
    /// than the provided value
    fn filter_by_event_nonce(event_nonce: u64, input: &[Self]) -> Vec<Self> {
        let mut ret = Vec::new();
        for item in input {
            if item.event_nonce > event_nonce {
                ret.push(item.clone())
            }
        }
        ret
    }

    // gets the Ethereum block for the given nonce
    fn get_block_for_nonce(event_nonce: u64, input: &[Self]) -> Option<Uint256> {
        for item in input {
            if item.event_nonce == event_nonce {
                return Some(item.block_height);
            }
        }
        None
    }

    fn to_claim_msg(self, orchestrator: Address) -> Msg {
        let claim = MsgLogicCallExecutedClaim {
            event_nonce: self.event_nonce,
            eth_block_height: self.get_block_height(),
            invalidation_id: self.invalidation_id,
            invalidation_nonce: self.invalidation_nonce,
            orchestrator: orchestrator.to_string(),
        };
        Msg::new(MSG_LOGIC_CALL_EXECUTED_CLAIM_TYPE_URL, claim)
    }
}

/// Function used for debug printing hex dumps
/// of ethereum events with each uint256 on a new
/// line
fn _debug_print_data(input: &[u8]) {
    let count = input.len() / 32;
    println!("data hex dump");
    for i in 0..count {
        println!("0x{}", bytes_to_hex_str(&input[(i * 32)..((i * 32) + 32)]))
    }
    println!("end dump");
}

#[cfg(test)]
mod tests {
    use super::*;
    use clarity::utils::hex_str_to_bytes;
    use rand::distributions::Distribution;
    use rand::distributions::Uniform;
    use rand::prelude::ThreadRng;
    use rand::thread_rng;
    use rand::Rng;
    use std::time::Duration;
    use std::time::Instant;

    /// Five minutes fuzzing by default
    const FUZZ_TIME: Duration = Duration::from_secs(30);

    fn get_fuzz_bytes(rng: &mut ThreadRng) -> Vec<u8> {
        let range = Uniform::from(1..200_000);
        let size: usize = range.sample(rng);
        let event_bytes: Vec<u8> = (0..size)
            .map(|_| {
                let val: u8 = rng.gen();
                val
            })
            .collect();
        event_bytes
    }

    #[test]
    fn test_valset_decode() {
        let event = "0x0000000000000000000000000000000000000000000000000000000000000001\
                          000000000000000000000000000000000000000000000000000000000000000000\
                          000000000000000000000000000000000000000000000000000000000000000000\
                          0000000000000000000000000000000000000000000000000000000000a0000000\
                          000000000000000000000000000000000000000000000000000000012000000000\
                          000000000000000000000000000000000000000000000000000000030000000000\
                          000000000000001bb537aa56ffc7d608793baffc6c9c7de3c4f270000000000000\
                          000000000000906313229cfb30959b39a5946099e4526625cbd400000000000000\
                          00000000009f49c7617b72b5784f482bd728d26eba354a0b390000000000000000\
                          000000000000000000000000000000000000000000000003000000000000000000\
                          000000000000000000000000000000000000005555555500000000000000000000\
                          000000000000000000000000000000000000555555550000000000000000000000\
                          000000000000000000000000000000000055555555";
        let event_bytes = hex_str_to_bytes(event).unwrap();

        let correct = ValsetDataBytes {
            event_nonce: 1u8.into(),
            reward_amount: 0u8.into(),
            reward_token: None,
            members: vec![
                ValsetMember {
                    eth_address: "0x1bb537Aa56fFc7D608793BAFFC6c9C7De3c4F270"
                        .parse()
                        .unwrap(),
                    power: 1431655765,
                },
                ValsetMember {
                    eth_address: "0x906313229CFB30959b39A5946099e4526625CBD4"
                        .parse()
                        .unwrap(),
                    power: 1431655765,
                },
                ValsetMember {
                    eth_address: "0x9F49C7617b72b5784F482Bd728d26EbA354a0B39"
                        .parse()
                        .unwrap(),

                    power: 1431655765,
                },
            ],
        };
        let res = ValsetUpdatedEvent::decode_data_bytes(&event_bytes).unwrap();
        assert_eq!(correct, res);
    }

    #[test]
    fn test_send_to_cosmos_decode() {
        let event = "0x0000000000000000000000000000000000000000000000000000000000000060\
        0000000000000000000000000000000000000000000000000000000000000064\
        0000000000000000000000000000000000000000000000000000000000000002\
        000000000000000000000000000000000000000000000000000000000000002f\
        67726176697479313139347a613679766737646a7a33633676716c63787a7877\
        636a6b617a397264717332656739700000000000000000000000000000000000";
        let event_bytes = hex_str_to_bytes(event).unwrap();

        let correct = SendToCosmosEventData {
            destination: "gravity1194za6yvg7djz3c6vqlcxzxwcjkaz9rdqs2eg9p".to_string(),
            amount: 100u8.into(),
            event_nonce: 2u8.into(),
        };
        let res = SendToCosmosEvent::decode_data_bytes(&event_bytes).unwrap();
        assert_eq!(correct, res);
    }

    #[test]
    fn fuzz_send_to_cosmos_decode() {
        let start = Instant::now();
        let mut rng = thread_rng();
        while Instant::now() - start < FUZZ_TIME {
            let event_bytes = get_fuzz_bytes(&mut rng);

            let res = SendToCosmosEvent::decode_data_bytes(&event_bytes);
            match res {
                Ok(_) => println!("Got valid output, this should happen very rarely!"),
                Err(_e) => {}
            }
        }
    }

    #[test]
    fn fuzz_valset_updated_event_decode() {
        let start = Instant::now();
        let mut rng = thread_rng();
        while Instant::now() - start < FUZZ_TIME {
            let event_bytes = get_fuzz_bytes(&mut rng);

            let res = ValsetUpdatedEvent::decode_data_bytes(&event_bytes);
            match res {
                Ok(_) => println!("Got valid output, this should happen very rarely!"),
                Err(_e) => {}
            }
        }
    }

    #[test]
    fn fuzz_erc20_deployed_event_decode() {
        let start = Instant::now();
        let mut rng = thread_rng();
        while Instant::now() - start < FUZZ_TIME {
            let event_bytes = get_fuzz_bytes(&mut rng);

            let res = Erc20DeployedEvent::decode_data_bytes(&event_bytes);
            match res {
                Ok(_) => println!("Got valid output, this should happen very rarely!"),
                Err(_e) => {}
            }
        }
    }
}
