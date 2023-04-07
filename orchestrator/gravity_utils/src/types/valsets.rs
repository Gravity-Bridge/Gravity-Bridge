use super::*;
use crate::error::GravityError;
use clarity::constants::zero_address;
use clarity::Address as EthAddress;
use clarity::Signature as EthSignature;
use deep_space::error::CosmosGrpcError;
use deep_space::Address as CosmosAddress;
use std::convert::TryFrom;
use std::fmt::Debug;
use std::{
    cmp::Ordering,
    collections::{HashMap, HashSet},
    fmt,
};

/// The total power in the Gravity bridge is normalized to 2^32 every
/// time a validator set is created. So this is the total power
pub const TOTAL_GRAVITY_POWER: u64 = 2u64.pow(32);
/// The total power in the Gravity bridge is normalized to 2^32 every
/// time a validator set is created. This is a mirror of constant_powerThreshold
/// in Gravity.sol
pub const GRAVITY_POWER_TO_PASS: u64 = 2863311530;

/// takes in an amount of power in the gravity bridge, returns a percentage of total
/// use this for printing values to users only, for accurate calculations use GRAVITY_POWER_TO_PASS
fn gravity_power_to_percent(input: u64) -> f32 {
    (input as f32 / TOTAL_GRAVITY_POWER as f32) * 100f32
}
/// This trait implements an overarching interface for signature confirmations
/// so that they can all use the same method to order signatures
pub trait Confirm {
    fn get_eth_address(&self) -> EthAddress;
    fn get_signature(&self) -> EthSignature;
}

pub fn get_hashmap<T: Confirm + Clone>(input: &[T]) -> HashMap<EthAddress, T> {
    let mut out = HashMap::new();
    for i in input.iter() {
        out.insert(i.get_eth_address(), i.clone());
    }
    out
}

/// Used to encapsulate signature ordering across all different confirm
/// messages
#[derive(Debug, Clone)]
struct SignatureStatus {
    ordered_signatures: Vec<GravitySignature>,
    power_of_good_sigs: u64,
    power_of_nonvoters: u64,
    number_of_nonvoters: usize,
    num_validators: usize,
}

/// the response we get when querying for a valset confirmation
#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct ValsetConfirmResponse {
    pub orchestrator: CosmosAddress,
    pub eth_address: EthAddress,
    pub nonce: u64,
    pub eth_signature: EthSignature,
}

impl Confirm for ValsetConfirmResponse {
    fn get_eth_address(&self) -> EthAddress {
        self.eth_address
    }
    fn get_signature(&self) -> EthSignature {
        self.eth_signature.clone()
    }
}

/// a list of validators, powers, and eth addresses at a given block height
#[derive(Serialize, Deserialize, Debug, Default, Clone, PartialEq, Eq)]
pub struct Valset {
    /// The strictly increasing nonce value used to prevent
    /// validator set update replay
    pub nonce: u64,
    /// Members of the validator set, these are the Ethereum keys of
    /// the Cosmos validator set at the time of creation along with
    /// normalized powers for them
    pub members: Vec<ValsetMember>,
    /// An optional reward to be issued to the Relayer on Ethereum
    /// TODO make Option<Uint256>
    pub reward_amount: Uint256,
    /// An optional reward to be issued to the Relayer on Ethereum
    pub reward_token: Option<EthAddress>,
}

impl Valset {
    /// If this is false the validator set is invalid, as it does not
    /// have enough validators with enough voting power to ever pass
    /// anything on the bridge
    pub fn enough_power(&self) -> bool {
        self.get_total_power() > GRAVITY_POWER_TO_PASS
    }

    /// Gets the total voting power in this validator set
    /// must be greater than GRAVITY_POWER_TO_PASS to be valid
    pub fn get_total_power(&self) -> u64 {
        let mut total = 0;
        for val in self.members.iter() {
            total += val.power;
        }
        total
    }

    /// Gets the Gravity bridge voting power of an individual validator
    pub fn get_bridge_validator_power(&self, address: EthAddress) -> Result<u64, CosmosGrpcError> {
        for val in self.members.iter() {
            if val.eth_address == address {
                return Ok(val.power);
            }
        }
        Err(CosmosGrpcError::BadInput("Not found!".to_string()))
    }

    /// combines the provided signatures with the valset ensuring that ordering and signature data is correct
    /// Note the 'correct' ordering is the *same* ordering as the validator set members in 'self'. In some cases
    /// this will be sorted, in others it will be improperly sorted but must be maintained so that the signatures
    /// are accepted on the Ethereum chain, which requires the submitted addresses to match whatever the previously
    /// submitted ordering was and the signatures must be in parallel arrays to reduce shuffling.
    fn get_signature_status<T: Confirm + Clone + Debug>(
        &self,
        signed_message: &[u8],
        signatures: &[T],
    ) -> Result<SignatureStatus, GravityError> {
        if signatures.is_empty() {
            return Err(GravityError::InsufficientVotingPowerToPass(
                "No signatures!".to_string(),
            ));
        }

        let mut out = Vec::new();
        let signatures_hashmap: HashMap<EthAddress, T> = get_hashmap(signatures);
        let mut power_of_good_sigs = 0;
        let mut power_of_nonvoters = 0;
        let mut number_of_nonvoters = 0;
        for member in self.members.iter() {
            if let Some(sig) = signatures_hashmap.get(&member.eth_address) {
                assert_eq!(sig.get_eth_address(), member.eth_address);
                assert!(sig.get_signature().is_valid());
                let recover_key = sig.get_signature().recover(signed_message).unwrap();
                if recover_key == sig.get_eth_address() {
                    out.push(GravitySignature {
                        power: member.power,
                        eth_address: sig.get_eth_address(),
                        v: sig.get_signature().v,
                        r: sig.get_signature().r,
                        s: sig.get_signature().s,
                    });
                    power_of_good_sigs += member.power;
                } else {
                    // the go code verifies signatures, if we ever see this it means
                    // that something has gone horribly wrong with our parsing or ordering
                    // in the orchestrator
                    // this has been observed to occur spuriously, not currently root caused
                    let err = format!(
                        "Found invalid signature from {} {:?}",
                        sig.get_eth_address(),
                        sig.get_signature()
                    );
                    error!("{}", err);
                    return Err(GravityError::InvalidBridgeStateError(err));
                }
            } else {
                out.push(GravitySignature {
                    power: member.power,
                    eth_address: member.eth_address,
                    v: 0u8.into(),
                    r: 0u8.into(),
                    s: 0u8.into(),
                });
                power_of_nonvoters += member.power;
                number_of_nonvoters += 1;
            }
        }

        let num_validators = self.members.len();
        Ok(SignatureStatus {
            ordered_signatures: out,
            power_of_good_sigs,
            power_of_nonvoters,
            num_validators,
            number_of_nonvoters,
        })
    }

    pub fn order_sigs<T: Confirm + Clone + Debug>(
        &self,
        signed_message: &[u8],
        signatures: &[T],
    ) -> Result<Vec<GravitySignature>, GravityError> {
        let status = self.get_signature_status(signed_message, signatures)?;
        // now that we have collected the signatures we can determine if the measure has the votes to pass
        // and error early if it does not, otherwise the user will pay fees for a transaction that will
        // just throw
        if status.power_of_good_sigs <= GRAVITY_POWER_TO_PASS {
            let message = format!(
                "
                has {}/{} or {:.2}% power voting! Can not execute on Ethereum!
                {}/{} validators have unset Ethereum keys representing {}/{} or {:.2}% of the power required
                This valset probably just needs to accumulate signatures for a moment.",
                status.power_of_good_sigs,
                TOTAL_GRAVITY_POWER,
                gravity_power_to_percent(status.power_of_good_sigs),
                status.number_of_nonvoters,
                status.num_validators,
                status.power_of_nonvoters,
                TOTAL_GRAVITY_POWER,
                gravity_power_to_percent(status.power_of_nonvoters),
            );
            Err(GravityError::InsufficientVotingPowerToPass(message))
        } else {
            Ok(status.ordered_signatures)
        }
    }

    /// A utility function to provide a HashMap of members for easy lookups
    pub fn to_hashmap(&self) -> HashMap<EthAddress, u64> {
        let mut res = HashMap::new();
        for item in self.members.iter() {
            res.insert(item.eth_address, item.power);
        }
        res
    }

    /// A utility function to provide a HashSet of members for union operations
    pub fn to_hashset(&self) -> HashSet<EthAddress> {
        let mut res = HashSet::new();
        for item in self.members.iter() {
            res.insert(item.eth_address);
        }
        res
    }

    pub fn to_arrays(&self) -> (Vec<EthAddress>, Vec<u64>) {
        let mut addresses = Vec::new();
        let mut powers = Vec::new();
        for i in self.members.iter() {
            addresses.push(i.eth_address);
            powers.push(i.power)
        }
        (addresses, powers)
    }

    /// This function takes the current valset and compares it to a provided one
    /// returning a percentage difference in their power allocation. This is a very
    /// important function as it's used to decide when the validator sets are updated
    /// on the Ethereum chain and when new validator sets are requested on the Cosmos
    /// side. In theory an error here, if unnoticed for long enough, could allow funds
    /// to be stolen from the bridge without the validators in question still having stake
    /// to lose.
    /// Returned value must be less than or equal to two
    pub fn power_diff(&self, other: &Valset) -> f32 {
        let mut total_power_diff = 0u64;
        let a = self.to_hashmap();
        let b = other.to_hashmap();
        let a_map = self.to_hashset();
        let b_map = other.to_hashset();
        // items in A and B, we go through these and compute the absolute value of the
        // difference in power and sum it.
        let intersection = a_map.intersection(&b_map);
        // items in A but not in B or vice versa, since we're just trying to compute the difference
        // we can simply sum all of these up.
        let symmetric_difference = a_map.symmetric_difference(&b_map);
        for item in symmetric_difference {
            let mut power = None;
            if let Some(val) = a.get(item) {
                power = Some(val);
            } else if let Some(val) = b.get(item) {
                power = Some(val);
            }
            // impossible for this to panic without a failure in the logic
            // of the symmetric difference function
            let power = power.unwrap();
            total_power_diff += power;
        }
        for item in intersection {
            // can't panic since there must be an entry for both.
            let power_a = a[item];
            let power_b = b[item];
            if power_a > power_b {
                total_power_diff += power_a - power_b;
            } else {
                total_power_diff += power_b - power_a;
            }
        }

        (total_power_diff as f32) / (u32::MAX as f32)
    }
}

impl From<gravity_proto::gravity::Valset> for Valset {
    fn from(input: gravity_proto::gravity::Valset) -> Self {
        (&input).into()
    }
}

impl From<&gravity_proto::gravity::Valset> for Valset {
    fn from(input: &gravity_proto::gravity::Valset) -> Self {
        let parsed_reward_token = input.reward_token.parse().unwrap();
        let reward_token = if parsed_reward_token == zero_address() {
            None
        } else {
            Some(parsed_reward_token)
        };
        Valset {
            nonce: input.nonce,
            members: input.members.iter().map(|i| i.into()).collect(),
            reward_amount: input.reward_amount.parse().unwrap(),
            reward_token,
        }
    }
}

/// a list of validators, powers, and eth addresses at a given block height
#[derive(Serialize, Deserialize, Debug, Default, Clone, Eq, PartialEq, Hash)]
pub struct ValsetMember {
    // ord sorts on the first member first, so this produces the correct sorting
    pub power: u64,
    pub eth_address: EthAddress,
}

impl Ord for ValsetMember {
    // Alex wrote the Go sorting implementation for validator
    // sets as Greatest to Least, now this isn't the convention
    // for any standard sorting implementation and Rust doesn't
    // really like it when you implement sort yourself. It prefers
    // Ord. So here we implement Ord with the Eth address sorting
    // reversed, since they are also sorted greatest to least in
    // the Cosmos module. Then we can call .sort and .reverse and get
    // the same sorting as the Cosmos module.
    fn cmp(&self, other: &Self) -> Ordering {
        if self.power != other.power {
            self.power.cmp(&other.power)
        } else {
            self.eth_address.cmp(&other.eth_address).reverse()
        }
    }
}

impl PartialOrd for ValsetMember {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

impl ValsetMember {
    pub fn display_vec(input: &[ValsetMember]) -> String {
        let mut out = String::new();
        for val in input.iter() {
            out += &val.to_string()
        }
        out
    }
}

impl fmt::Display for ValsetMember {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "Address: {} Power: {}", self.eth_address, self.power)
    }
}

impl From<gravity_proto::gravity::BridgeValidator> for ValsetMember {
    fn from(input: gravity_proto::gravity::BridgeValidator) -> Self {
        let eth_address = match input.ethereum_address.parse() {
            Ok(e) => e,
            Err(_) => panic!("Validator set with empty address!"),
        };
        ValsetMember {
            power: input.power,
            eth_address,
        }
    }
}

impl From<&gravity_proto::gravity::BridgeValidator> for ValsetMember {
    fn from(input: &gravity_proto::gravity::BridgeValidator) -> Self {
        let eth_address = match input.ethereum_address.parse() {
            Ok(e) => e,
            Err(_) => panic!("Validator set with empty address!"),
        };
        ValsetMember {
            power: input.power,
            eth_address,
        }
    }
}

impl From<&ValsetMember> for gravity_proto::gravity::BridgeValidator {
    fn from(input: &ValsetMember) -> gravity_proto::gravity::BridgeValidator {
        let ethereum_address = input.eth_address.to_string();
        gravity_proto::gravity::BridgeValidator {
            power: input.power,
            ethereum_address,
        }
    }
}

impl TryFrom<gravity_proto::gravity::MsgValsetConfirm> for ValsetConfirmResponse {
    type Error = GravityError;
    fn try_from(input: gravity_proto::gravity::MsgValsetConfirm) -> Result<Self, GravityError> {
        ValsetConfirmResponse::try_from(&input)
    }
}

impl TryFrom<&gravity_proto::gravity::MsgValsetConfirm> for ValsetConfirmResponse {
    type Error = GravityError;
    fn try_from(input: &gravity_proto::gravity::MsgValsetConfirm) -> Result<Self, GravityError> {
        Ok(ValsetConfirmResponse {
            orchestrator: input.orchestrator.parse()?,
            eth_address: input.eth_address.parse()?,
            nonce: input.nonce,
            eth_signature: input.signature.parse()?,
        })
    }
}

#[allow(clippy::from_over_into)]
impl Into<gravity_proto::gravity::Valset> for &Valset {
    fn into(self) -> gravity_proto::gravity::Valset {
        gravity_proto::gravity::Valset {
            nonce: self.nonce,
            // note our use of a dummy value here, this value is not used for signatures
            // so it has no effect on evidence based slashing
            height: 0,
            members: self.members.iter().map(|v| v.into()).collect(),
            reward_amount: self.reward_amount.to_string(),
            reward_token: self.reward_token.unwrap_or(zero_address()).to_string(),
        }
    }
}

#[allow(clippy::from_over_into)]
impl Into<gravity_proto::gravity::Valset> for Valset {
    fn into(self) -> gravity_proto::gravity::Valset {
        let r = &self;
        r.into()
    }
}

#[allow(clippy::from_over_into)]
impl Into<gravity_proto::gravity::BridgeValidator> for ValsetMember {
    fn into(self) -> gravity_proto::gravity::BridgeValidator {
        let r = &self;
        r.into()
    }
}
