use super::*;
use crate::error::GravityError;
use clarity::Signature as EthSignature;
use clarity::{utils::hex_str_to_bytes, Address as EthAddress};
use deep_space::Address as CosmosAddress;

/// the response we get when querying for a valset confirmation
#[derive(Serialize, Deserialize, Debug, Default, Clone)]
pub struct LogicCall {
    /// funds sent to the Logic contract for it's use
    pub transfers: Vec<Erc20Token>,
    /// individual token payments made on Ethereum to the relayer
    pub fees: Vec<Erc20Token>,
    pub logic_contract_address: EthAddress,
    pub payload: Vec<u8>,
    pub timeout: u64,
    pub invalidation_id: Vec<u8>,
    pub invalidation_nonce: u64,
}

/// the response we get when querying for a logic call confirmation
#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct LogicCallConfirmResponse {
    pub invalidation_id: Vec<u8>,
    pub invalidation_nonce: u64,
    pub ethereum_signer: EthAddress,
    pub orchestrator: CosmosAddress,
    pub eth_signature: EthSignature,
}

impl Confirm for LogicCallConfirmResponse {
    fn get_eth_address(&self) -> EthAddress {
        self.ethereum_signer
    }
    fn get_signature(&self) -> EthSignature {
        self.eth_signature.clone()
    }
}

impl TryFrom<gravity_proto::gravity::MsgConfirmLogicCall> for LogicCallConfirmResponse {
    type Error = GravityError;
    fn try_from(
        input: gravity_proto::gravity::MsgConfirmLogicCall,
    ) -> Result<LogicCallConfirmResponse, GravityError> {
        Ok(LogicCallConfirmResponse {
            invalidation_id: hex_str_to_bytes(&input.invalidation_id).unwrap(),
            invalidation_nonce: input.invalidation_nonce,
            orchestrator: input.orchestrator.parse()?,
            ethereum_signer: input.eth_signer.parse()?,
            eth_signature: input.signature.parse()?,
        })
    }
}

impl TryFrom<gravity_proto::gravity::OutgoingLogicCall> for LogicCall {
    type Error = GravityError;
    fn try_from(
        input: gravity_proto::gravity::OutgoingLogicCall,
    ) -> Result<LogicCall, GravityError> {
        let mut transfers: Vec<Erc20Token> = Vec::new();
        let mut fees: Vec<Erc20Token> = Vec::new();
        for transfer in input.transfers {
            transfers.push(Erc20Token::try_from(transfer)?)
        }
        for fee in input.fees {
            fees.push(Erc20Token::try_from(fee)?)
        }
        if transfers.is_empty() || fees.is_empty() {
            return Err(GravityError::InvalidBridgeStateError(
                "Logic call containing no transfers!".to_string(),
            ));
        }

        Ok(LogicCall {
            transfers,
            fees,
            logic_contract_address: input.logic_contract_address.parse()?,
            payload: input.payload,
            timeout: input.timeout,
            invalidation_id: input.invalidation_id,
            invalidation_nonce: input.invalidation_nonce,
        })
    }
}

#[allow(clippy::from_over_into)]
impl Into<gravity_proto::gravity::OutgoingLogicCall> for &LogicCall {
    fn into(self) -> gravity_proto::gravity::OutgoingLogicCall {
        gravity_proto::gravity::OutgoingLogicCall {
            transfers: self.transfers.iter().map(|t| t.into()).collect(),
            fees: self.fees.iter().map(|t| t.into()).collect(),
            logic_contract_address: self.logic_contract_address.to_string(),
            payload: self.payload.clone(),
            timeout: self.timeout,
            invalidation_id: self.invalidation_id.clone(),
            invalidation_nonce: self.invalidation_nonce,
            // note the dummy value, this is not used in signatures
            // so it's not required for our evidence based slashing implementation
            cosmos_block_created: 0,
        }
    }
}

#[allow(clippy::from_over_into)]
impl Into<gravity_proto::gravity::OutgoingLogicCall> for LogicCall {
    fn into(self) -> gravity_proto::gravity::OutgoingLogicCall {
        let r = &self;
        r.into()
    }
}
