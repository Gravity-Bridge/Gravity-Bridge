use std::convert::TryFrom;

use crate::error::GravityError;
use clarity::Address as EthAddress;
use clarity::Uint256;

#[derive(Serialize, Deserialize, Debug, Default, Clone, Eq, PartialEq, Hash)]
pub struct Erc20Token {
    pub amount: Uint256,
    #[serde(rename = "contract")]
    pub token_contract_address: EthAddress,
}

impl TryFrom<gravity_proto::gravity::Erc20Token> for Erc20Token {
    type Error = GravityError;
    fn try_from(input: gravity_proto::gravity::Erc20Token) -> Result<Erc20Token, GravityError> {
        Ok(Erc20Token {
            amount: input.amount.parse()?,
            token_contract_address: input.contract.parse()?,
        })
    }
}
#[allow(clippy::from_over_into)]
impl Into<gravity_proto::gravity::Erc20Token> for &Erc20Token {
    fn into(self) -> gravity_proto::gravity::Erc20Token {
        gravity_proto::gravity::Erc20Token {
            amount: self.amount.to_string(),
            contract: self.token_contract_address.to_string(),
        }
    }
}

#[allow(clippy::from_over_into)]
impl Into<gravity_proto::gravity::Erc20Token> for Erc20Token {
    fn into(self) -> gravity_proto::gravity::Erc20Token {
        gravity_proto::gravity::Erc20Token {
            amount: self.amount.to_string(),
            contract: self.token_contract_address.to_string(),
        }
    }
}

// First order by token address, then split ties by amount
impl PartialOrd for Erc20Token {
    fn partial_cmp(&self, other: &Self) -> Option<std::cmp::Ordering> {
        match self
            .token_contract_address
            .partial_cmp(&other.token_contract_address)
        {
            Some(core::cmp::Ordering::Equal) => self.amount.partial_cmp(&other.amount),
            ord => ord,
        }
    }
}

// First order by token address, then split ties by amount
impl Ord for Erc20Token {
    fn cmp(&self, other: &Self) -> std::cmp::Ordering {
        match self
            .token_contract_address
            .cmp(&other.token_contract_address)
        {
            std::cmp::Ordering::Equal => self.amount.cmp(&other.amount),
            ord => ord,
        }
    }
}
