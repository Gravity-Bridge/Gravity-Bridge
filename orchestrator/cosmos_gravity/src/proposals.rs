//! This file handles submitting and querying governance proposals custom to Gravity bridge

use deep_space::client::send::TransactionResponse;
use deep_space::error::AddressError;
use deep_space::error::CosmosGrpcError;
use deep_space::utils::encode_any;
use deep_space::Address;
use deep_space::Coin;
use deep_space::Contact;
use deep_space::PrivateKey;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::DenomUnit;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::Metadata;
use gravity_proto::cosmos_sdk_proto::cosmos::params::v1beta1::ParamChange;
use gravity_proto::cosmos_sdk_proto::cosmos::params::v1beta1::ParameterChangeProposal;
use gravity_proto::cosmos_sdk_proto::cosmos::upgrade::v1beta1::SoftwareUpgradeProposal;
use gravity_proto::gravity::AirdropProposal as AirdropProposalMsg;
use gravity_proto::gravity::IbcMetadataProposal;
use gravity_proto::gravity::UnhaltBridgeProposal;
use serde::Deserialize;
use serde::Serialize;
use std::convert::TryFrom;
use std::time::Duration;

// gravity proposals
pub const AIRDROP_PROPOSAL_TYPE_URL: &str = "/gravity.v1.AirdropProposal";
pub const UNHALT_BRIDGE_PROPOSAL_TYPE_URL: &str = "/gravity.v1.UnhaltBridgeProposal";
pub const IBC_METADATA_PROPOSAL_TYPE_URL: &str = "/gravity.v1.IBCMetadataProposal";

// cosmos-sdk proposals
pub const PARAMETER_CHANGE_PROPOSAL_TYPE_URL: &str =
    "/cosmos.params.v1beta1.ParameterChangeProposal";
pub const SOFTWARE_UPGRADE_PROPOSAL_TYPE_URL: &str =
    "/cosmos.upgrade.v1beta1.SoftwareUpgradeProposal";

// bech32ibc proposals
pub const UPDATE_HRP_IBC_CHANNEL_PROPOSAL: &str =
    "/bech32ibc.bech32ibc.v1beta1.UpdateHrpIbcChannelProposal";

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
    key: impl PrivateKey,
    wait_timeout: Option<Duration>,
) -> Result<TransactionResponse, CosmosGrpcError> {
    let mut byte_recipients = Vec::new();
    for r in proposal.recipients {
        byte_recipients.extend_from_slice(r.get_bytes())
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
    key: impl PrivateKey,
    wait_timeout: Option<Duration>,
) -> Result<TransactionResponse, CosmosGrpcError> {
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
    key: impl PrivateKey,
    wait_timeout: Option<Duration>,
) -> Result<TransactionResponse, CosmosGrpcError> {
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
    key: impl PrivateKey,
    wait_timeout: Option<Duration>,
) -> Result<TransactionResponse, CosmosGrpcError> {
    // encode as a generic proposal
    let any = encode_any(proposal, PARAMETER_CHANGE_PROPOSAL_TYPE_URL.to_string());
    contact
        .create_gov_proposal(any, deposit, fee, key, wait_timeout)
        .await
}

/// Encodes and submits a proposal to upgrade chain software, should maybe be in deep_space (sorry)
pub async fn submit_upgrade_proposal(
    proposal: SoftwareUpgradeProposal,
    deposit: Coin,
    fee: Coin,
    contact: &Contact,
    key: impl PrivateKey,
    wait_timeout: Option<Duration>,
) -> Result<TransactionResponse, CosmosGrpcError> {
    // encode as a generic proposal
    let any = encode_any(proposal, SOFTWARE_UPGRADE_PROPOSAL_TYPE_URL.to_string());
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
            uri: String::new(),
            uri_hash: String::new(),
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
    key: impl PrivateKey,
    wait_timeout: Option<Duration>,
) -> Result<TransactionResponse, CosmosGrpcError> {
    // encode as a generic proposal
    let any = encode_any(proposal, IBC_METADATA_PROPOSAL_TYPE_URL.to_string());
    contact
        .create_gov_proposal(any, deposit, fee, key, wait_timeout)
        .await
}

/// The proposal.json representation for setting the MinChainFeeBasisPoints parameter
#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct SendToEthFeesProposalJson {
    pub title: String,
    pub description: String,
    pub min_chain_fee_basis_points: u64,
}

/// Submit a parameter change proposal to set the MinChainFeeBasisPoints parameter
pub async fn submit_send_to_eth_fees_proposal(
    proposal: SendToEthFeesProposalJson,
    deposit: Coin,
    fee: Coin,
    contact: &Contact,
    key: impl PrivateKey,
    wait_timeout: Option<Duration>,
) -> Result<TransactionResponse, CosmosGrpcError> {
    let mut params_to_change = Vec::new();
    let set_fees = ParamChange {
        subspace: "gravity".to_string(),
        key: "MinChainFeeBasisPoints".to_string(),
        value: format!("\"{}\"", proposal.min_chain_fee_basis_points),
    };
    params_to_change.push(set_fees);
    let proposal = ParameterChangeProposal {
        title: proposal.title,
        description: proposal.description,
        changes: params_to_change,
    };
    submit_parameter_change_proposal(proposal, deposit, fee, contact, key, wait_timeout).await
}

/// The proposal.json representation for setting any and all of the Auction module params
#[derive(Serialize, Deserialize, Debug, Default, Clone)]
pub struct AuctionParamsProposalJson {
    pub title: String,
    pub description: String,

    pub auction_length: Option<u64>,
    pub min_bid_fee: Option<u64>,
    pub non_auctionable_tokens: Option<Vec<String>>,
    pub burn_winning_bids: Option<bool>,
    pub enabled: Option<bool>,
}

/// Submit a parameter change proposal to set the auction module's params
pub async fn submit_auction_params_proposal(
    proposal: AuctionParamsProposalJson,
    deposit: Coin,
    fee: Coin,
    contact: &Contact,
    key: impl PrivateKey,
    wait_timeout: Option<Duration>,
) -> Result<TransactionResponse, CosmosGrpcError> {
    let mut params_to_change = Vec::new();
    if let Some(val) = proposal.auction_length {
        let param = ParamChange {
            subspace: "auction".to_string(),
            key: "AuctionLength".to_string(),
            value: format!("\"{}\"", val),
        };
        params_to_change.push(param);
    }
    if let Some(val) = proposal.min_bid_fee {
        let param = ParamChange {
            subspace: "auction".to_string(),
            key: "MinBidFee".to_string(),
            value: format!("\"{}\"", val),
        };
        params_to_change.push(param);
    }
    if let Some(val) = proposal.non_auctionable_tokens {
        let json_value = serde_json::to_string(&val).unwrap();
        let param = ParamChange {
            subspace: "auction".to_string(),
            key: "NonAuctionableTokens".to_string(),
            value: json_value,
        };
        params_to_change.push(param);
    }
    if let Some(val) = proposal.burn_winning_bids {
        let param = ParamChange {
            subspace: "auction".to_string(),
            key: "BurnWinningBids".to_string(),
            value: format!("{}", val),
        };
        params_to_change.push(param);
    }
    if let Some(val) = proposal.enabled {
        let param = ParamChange {
            subspace: "auction".to_string(),
            key: "Enabled".to_string(),
            value: format!("{}", val),
        };
        params_to_change.push(param);
    }
    let proposal = ParameterChangeProposal {
        title: proposal.title,
        description: proposal.description,
        changes: params_to_change,
    };
    info!("Submitting auction params proposal:\n{:?}", proposal);
    submit_parameter_change_proposal(proposal, deposit, fee, contact, key, wait_timeout).await
}
