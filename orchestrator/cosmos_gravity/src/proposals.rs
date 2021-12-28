//! This file handles submitting and querying governance proposals custom to Gravity bridge

use deep_space::error::AddressError;
use deep_space::error::CosmosGrpcError;
use deep_space::utils::encode_any;
use deep_space::Address;
use deep_space::Coin;
use deep_space::Contact;
use deep_space::PrivateKey;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::DenomUnit;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::Metadata;
use gravity_proto::cosmos_sdk_proto::cosmos::base::abci::v1beta1::TxResponse;
use gravity_proto::cosmos_sdk_proto::cosmos::params::v1beta1::ParamChange;
use gravity_proto::cosmos_sdk_proto::cosmos::params::v1beta1::ParameterChangeProposal;
use gravity_proto::gravity::AirdropProposal as AirdropProposalMsg;
use gravity_proto::gravity::IbcMetadataProposal;
use gravity_proto::gravity::UnhaltBridgeProposal;
use serde::Deserialize;
use serde::Serialize;
use std::convert::TryFrom;
use std::time::Duration;

pub const AIRDROP_PROPOSAL_TYPE_URL: &str = "/gravity.v1.AirdropProposal";
pub const UNHALT_BRIDGE_PROPOSAL_TYPE_URL: &str = "/gravity.v1.UnhaltBridgeProposal";
pub const PARAMETER_CHANGE_PROPOSAL_TYPE_URL: &str =
    "/cosmos.params.v1beta1.ParameterChangeProposal";
pub const IBC_METADATA_PROPOSAL_TYPE_URL: &str = "/gravity.v1.IBCMetadataProposal";

/// The proposal.json representation for the airdrop proposal
#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct AirdropProposalJson {
    pub title: String,
    pub denom: String,
    pub description: String,
    pub amounts: Vec<u64>,
    pub recipients: Vec<Address>,
}

/// The proposal.json representation for the airdrop proposal
#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct AirdropProposalJsonUnparsed {
    pub title: String,
    pub denom: String,
    pub description: String,
    pub amounts: Vec<u64>,
    pub recipients: Vec<String>,
}
impl TryFrom<AirdropProposalJsonUnparsed> for AirdropProposalJson {
    type Error = AddressError;

    fn try_from(value: AirdropProposalJsonUnparsed) -> Result<Self, Self::Error> {
        let mut parsed = Vec::new();
        for i in value.recipients {
            let a: Address = i.parse()?;
            parsed.push(a);
        }
        Ok(AirdropProposalJson {
            title: value.title,
            denom: value.denom,
            description: value.description,
            amounts: value.amounts,
            recipients: parsed,
        })
    }
}

/// Encodes and submits an airdrop proposal provided the json file
pub async fn submit_airdrop_proposal(
    proposal: AirdropProposalJson,
    deposit: Coin,
    fee: Coin,
    contact: &Contact,
    key: PrivateKey,
    wait_timeout: Option<Duration>,
) -> Result<TxResponse, CosmosGrpcError> {
    let mut byte_recipients = Vec::new();
    for r in proposal.recipients {
        byte_recipients.extend_from_slice(r.as_bytes())
    }

    let proposal_content = AirdropProposalMsg {
        title: proposal.title,
        description: proposal.description,
        denom: proposal.denom,
        amounts: proposal.amounts,
        recipients: byte_recipients,
    };

    // encode as a generic proposal
    let any = encode_any(proposal_content, AIRDROP_PROPOSAL_TYPE_URL.to_string());

    contact
        .create_gov_proposal(any, deposit, fee, key, wait_timeout)
        .await
}

/// The proposal.json representation for pausing/unpausing the bridge easily
#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct UnhaltBridgeProposalJson {
    pub title: String,
    pub description: String,
    pub target_nonce: u64,
}
impl From<UnhaltBridgeProposalJson> for UnhaltBridgeProposal {
    fn from(v: UnhaltBridgeProposalJson) -> Self {
        UnhaltBridgeProposal {
            title: v.title,
            description: v.description,
            target_nonce: v.target_nonce,
        }
    }
}

/// Encodes and submits a proposal to reset the bridge oracle to a specific nonce that has
/// not yet been observed
pub async fn submit_unhalt_bridge_proposal(
    proposal: UnhaltBridgeProposal,
    deposit: Coin,
    fee: Coin,
    contact: &Contact,
    key: PrivateKey,
    wait_timeout: Option<Duration>,
) -> Result<TxResponse, CosmosGrpcError> {
    // encode as a generic proposal
    let any = encode_any(proposal, UNHALT_BRIDGE_PROPOSAL_TYPE_URL.to_string());
    contact
        .create_gov_proposal(any, deposit, fee, key, wait_timeout)
        .await
}

/// The proposal.json representation for pausing/unpausing the bridge easily
#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct PauseBridgeProposalJson {
    pub title: String,
    pub description: String,
    pub paused: bool,
}

/// Submit a parameter change proposal to temporarily halt some operations of the bridge
pub async fn submit_pause_bridge_proposal(
    proposal: PauseBridgeProposalJson,
    deposit: Coin,
    fee: Coin,
    contact: &Contact,
    key: PrivateKey,
    wait_timeout: Option<Duration>,
) -> Result<TxResponse, CosmosGrpcError> {
    let mut params_to_change = Vec::new();
    let halt = ParamChange {
        subspace: "gravity".to_string(),
        key: "BridgeActive".to_string(),
        value: format!("{}", proposal.paused),
    };
    params_to_change.push(halt);
    let proposal = ParameterChangeProposal {
        title: proposal.title,
        description: proposal.description,
        changes: params_to_change,
    };
    submit_parameter_change_proposal(proposal, deposit, fee, contact, key, wait_timeout).await
}

/// Encodes and submits a proposal change bridge parameters, should maybe be in deep_space
pub async fn submit_parameter_change_proposal(
    proposal: ParameterChangeProposal,
    deposit: Coin,
    fee: Coin,
    contact: &Contact,
    key: PrivateKey,
    wait_timeout: Option<Duration>,
) -> Result<TxResponse, CosmosGrpcError> {
    // encode as a generic proposal
    let any = encode_any(proposal, PARAMETER_CHANGE_PROPOSAL_TYPE_URL.to_string());
    contact
        .create_gov_proposal(any, deposit, fee, key, wait_timeout)
        .await
}

// local types for which we can implement serialize/deserialize
// for json work
#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct IbcMetadataProposalJson {
    title: String,
    description: String,
    metadata: MetadataJson,
    ibc_denom: String,
}
impl From<IbcMetadataProposalJson> for IbcMetadataProposal {
    fn from(v: IbcMetadataProposalJson) -> Self {
        IbcMetadataProposal {
            title: v.title,
            description: v.description,
            ibc_denom: v.ibc_denom,
            metadata: Some(v.metadata.into()),
        }
    }
}
#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct MetadataJson {
    pub description: String,
    pub denom_units: Vec<DenomUnitJson>,
    pub base: String,
    pub display: String,
    pub name: String,
    pub symbol: String,
}
impl From<MetadataJson> for Metadata {
    fn from(v: MetadataJson) -> Self {
        Metadata {
            description: v.description,
            denom_units: v.denom_units.into_iter().map(|a| a.into()).collect(),
            base: v.base,
            display: v.display,
            name: v.name,
            symbol: v.symbol,
        }
    }
}
#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct DenomUnitJson {
    pub denom: String,
    pub exponent: u32,
    pub aliases: Vec<String>,
}
impl From<DenomUnitJson> for DenomUnit {
    fn from(v: DenomUnitJson) -> Self {
        DenomUnit {
            denom: v.denom,
            exponent: v.exponent,
            aliases: v.aliases,
        }
    }
}

pub async fn submit_ibc_metadata_proposal(
    proposal: IbcMetadataProposal,
    deposit: Coin,
    fee: Coin,
    contact: &Contact,
    key: PrivateKey,
    wait_timeout: Option<Duration>,
) -> Result<TxResponse, CosmosGrpcError> {
    // encode as a generic proposal
    let any = encode_any(proposal, IBC_METADATA_PROPOSAL_TYPE_URL.to_string());
    contact
        .create_gov_proposal(any, deposit, fee, key, wait_timeout)
        .await
}
