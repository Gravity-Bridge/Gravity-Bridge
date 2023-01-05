/// Attestation is an aggregate of `claims` that eventually becomes `observed` by
/// all orchestrators
/// EVENT_NONCE:
/// EventNonce a nonce provided by the gravity contract that is unique per event fired
/// These event nonces must be relayed in order. This is a correctness issue,
/// if relaying out of order transaction replay attacks become possible
/// OBSERVED:
/// Observed indicates that >67% of validators have attested to the event,
/// and that the event should be executed by the gravity state machine
///
/// The actual content of the claims is passed in with the transaction making the claim
/// and then passed through the call stack alongside the attestation while it is processed
/// the key in which the attestation is stored is keyed on the exact details of the claim
/// but there is no reason to store those exact details becuause the next message sender
/// will kindly provide you with them.
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Attestation {
    #[prost(bool, tag = "1")]
    pub observed: bool,
    #[prost(string, repeated, tag = "2")]
    pub votes: ::prost::alloc::vec::Vec<::prost::alloc::string::String>,
    #[prost(uint64, tag = "3")]
    pub height: u64,
    #[prost(message, optional, tag = "4")]
    pub claim: ::core::option::Option<::prost_types::Any>,
}
/// ERC20Token unique identifier for an Ethereum ERC20 token.
/// CONTRACT:
/// The contract address on ETH of the token, this could be a Cosmos
/// originated token, if so it will be the ERC20 address of the representation
/// (note: developers should look up the token symbol using the address on ETH to display for UI)
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Erc20Token {
    #[prost(string, tag = "1")]
    pub contract: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub amount: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventObservation {
    #[prost(string, tag = "1")]
    pub attestation_type: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub bridge_contract: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub bridge_chain_id: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub attestation_id: ::prost::alloc::string::String,
    #[prost(string, tag = "5")]
    pub nonce: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventInvalidSendToCosmosReceiver {
    #[prost(string, tag = "1")]
    pub amount: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub nonce: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub token: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub sender: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventSendToCosmos {
    #[prost(string, tag = "1")]
    pub amount: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub nonce: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub token: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventSendToCosmosLocal {
    #[prost(string, tag = "1")]
    pub nonce: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub receiver: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub token: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub amount: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventSendToCosmosPendingIbcAutoForward {
    #[prost(string, tag = "1")]
    pub nonce: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub receiver: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub token: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub amount: ::prost::alloc::string::String,
    #[prost(string, tag = "5")]
    pub channel: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventSendToCosmosExecutedIbcAutoForward {
    #[prost(string, tag = "1")]
    pub nonce: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub receiver: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub token: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub amount: ::prost::alloc::string::String,
    #[prost(string, tag = "5")]
    pub channel: ::prost::alloc::string::String,
    #[prost(string, tag = "6")]
    pub timeout_time: ::prost::alloc::string::String,
    #[prost(string, tag = "7")]
    pub timeout_height: ::prost::alloc::string::String,
}
/// ClaimType is the cosmos type of an event from the counterpart chain that can
/// be handled
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord, ::prost::Enumeration)]
#[repr(i32)]
pub enum ClaimType {
    /// An unspecified claim type
    Unspecified = 0,
    /// A claim for a SendToCosmos transaction
    SendToCosmos = 1,
    /// A claim for when batches are relayed
    BatchSendToEth = 2,
    /// A claim for when an erc20 contract has been deployed
    Erc20Deployed = 3,
    /// A claim for when a logic call has been executed
    LogicCallExecuted = 4,
    /// A claim for when a valset update has happened
    ValsetUpdated = 5,
}
impl ClaimType {
    /// String value of the enum field names used in the ProtoBuf definition.
    ///
    /// The values are not transformed in any way and thus are considered stable
    /// (if the ProtoBuf definition does not change) and safe for programmatic use.
    pub fn as_str_name(&self) -> &'static str {
        match self {
            ClaimType::Unspecified => "CLAIM_TYPE_UNSPECIFIED",
            ClaimType::SendToCosmos => "CLAIM_TYPE_SEND_TO_COSMOS",
            ClaimType::BatchSendToEth => "CLAIM_TYPE_BATCH_SEND_TO_ETH",
            ClaimType::Erc20Deployed => "CLAIM_TYPE_ERC20_DEPLOYED",
            ClaimType::LogicCallExecuted => "CLAIM_TYPE_LOGIC_CALL_EXECUTED",
            ClaimType::ValsetUpdated => "CLAIM_TYPE_VALSET_UPDATED",
        }
    }
    /// Creates an enum from field names used in the ProtoBuf definition.
    pub fn from_str_name(value: &str) -> ::core::option::Option<Self> {
        match value {
            "CLAIM_TYPE_UNSPECIFIED" => Some(Self::Unspecified),
            "CLAIM_TYPE_SEND_TO_COSMOS" => Some(Self::SendToCosmos),
            "CLAIM_TYPE_BATCH_SEND_TO_ETH" => Some(Self::BatchSendToEth),
            "CLAIM_TYPE_ERC20_DEPLOYED" => Some(Self::Erc20Deployed),
            "CLAIM_TYPE_LOGIC_CALL_EXECUTED" => Some(Self::LogicCallExecuted),
            "CLAIM_TYPE_VALSET_UPDATED" => Some(Self::ValsetUpdated),
            _ => None,
        }
    }
}
/// IDSet represents a set of IDs
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct IdSet {
    #[prost(uint64, repeated, tag = "1")]
    pub ids: ::prost::alloc::vec::Vec<u64>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct BatchFees {
    #[prost(string, tag = "1")]
    pub token: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub total_fees: ::prost::alloc::string::String,
    #[prost(uint64, tag = "3")]
    pub tx_count: u64,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventWithdrawalReceived {
    #[prost(string, tag = "1")]
    pub bridge_contract: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub bridge_chain_id: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub outgoing_tx_id: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub nonce: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventWithdrawCanceled {
    #[prost(string, tag = "1")]
    pub sender: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub tx_id: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub bridge_contract: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub bridge_chain_id: ::prost::alloc::string::String,
}
/// OutgoingTxBatch represents a batch of transactions going from gravity to ETH
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct OutgoingTxBatch {
    #[prost(uint64, tag = "1")]
    pub batch_nonce: u64,
    #[prost(uint64, tag = "2")]
    pub batch_timeout: u64,
    #[prost(message, repeated, tag = "3")]
    pub transactions: ::prost::alloc::vec::Vec<OutgoingTransferTx>,
    #[prost(string, tag = "4")]
    pub token_contract: ::prost::alloc::string::String,
    #[prost(uint64, tag = "5")]
    pub cosmos_block_created: u64,
}
/// OutgoingTransferTx represents an individual send from gravity to ETH
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct OutgoingTransferTx {
    #[prost(uint64, tag = "1")]
    pub id: u64,
    #[prost(string, tag = "2")]
    pub sender: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub dest_address: ::prost::alloc::string::String,
    #[prost(message, optional, tag = "4")]
    pub erc20_token: ::core::option::Option<Erc20Token>,
    #[prost(message, optional, tag = "5")]
    pub erc20_fee: ::core::option::Option<Erc20Token>,
}
/// OutgoingLogicCall represents an individual logic call from gravity to ETH
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct OutgoingLogicCall {
    #[prost(message, repeated, tag = "1")]
    pub transfers: ::prost::alloc::vec::Vec<Erc20Token>,
    #[prost(message, repeated, tag = "2")]
    pub fees: ::prost::alloc::vec::Vec<Erc20Token>,
    #[prost(string, tag = "3")]
    pub logic_contract_address: ::prost::alloc::string::String,
    #[prost(bytes = "vec", tag = "4")]
    pub payload: ::prost::alloc::vec::Vec<u8>,
    #[prost(uint64, tag = "5")]
    pub timeout: u64,
    #[prost(bytes = "vec", tag = "6")]
    pub invalidation_id: ::prost::alloc::vec::Vec<u8>,
    #[prost(uint64, tag = "7")]
    pub invalidation_nonce: u64,
    #[prost(uint64, tag = "8")]
    pub cosmos_block_created: u64,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventOutgoingBatchCanceled {
    #[prost(string, tag = "1")]
    pub bridge_contract: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub bridge_chain_id: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub batch_id: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub nonce: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventOutgoingBatch {
    #[prost(string, tag = "1")]
    pub bridge_contract: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub bridge_chain_id: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub batch_id: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub nonce: ::prost::alloc::string::String,
}
/// BridgeValidator represents a validator's ETH address and its power
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct BridgeValidator {
    #[prost(uint64, tag = "1")]
    pub power: u64,
    #[prost(string, tag = "2")]
    pub ethereum_address: ::prost::alloc::string::String,
}
/// Valset is the Ethereum Bridge Multsig Set, each gravity validator also
/// maintains an ETH key to sign messages, these are used to check signatures on
/// ETH because of the significant gas savings
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Valset {
    #[prost(uint64, tag = "1")]
    pub nonce: u64,
    #[prost(message, repeated, tag = "2")]
    pub members: ::prost::alloc::vec::Vec<BridgeValidator>,
    #[prost(uint64, tag = "3")]
    pub height: u64,
    #[prost(string, tag = "4")]
    pub reward_amount: ::prost::alloc::string::String,
    /// the reward token in it's Ethereum hex address representation
    #[prost(string, tag = "5")]
    pub reward_token: ::prost::alloc::string::String,
}
/// LastObservedEthereumBlockHeight stores the last observed
/// Ethereum block height along with the Cosmos block height that
/// it was observed at. These two numbers can be used to project
/// outward and always produce batches with timeouts in the future
/// even if no Ethereum block height has been relayed for a long time
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct LastObservedEthereumBlockHeight {
    #[prost(uint64, tag = "1")]
    pub cosmos_block_height: u64,
    #[prost(uint64, tag = "2")]
    pub ethereum_block_height: u64,
}
/// This records the relationship between an ERC20 token and the denom
/// of the corresponding Cosmos originated asset
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Erc20ToDenom {
    #[prost(string, tag = "1")]
    pub erc20: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub denom: ::prost::alloc::string::String,
}
/// UnhaltBridgeProposal defines a custom governance proposal useful for
/// restoring the bridge after a oracle disagreement. Once this proposal is
/// passed bridge state will roll back events to the nonce provided in
/// target_nonce if and only if those events have not yet been observed (executed
/// on the Cosmos chain). This allows for easy handling of cases where for
/// example an Ethereum hardfork has occured and more than 1/3 of the vlaidtor
/// set disagrees with the rest. Normally this would require a chain halt, manual
/// genesis editing and restar to resolve with this feature a governance proposal
/// can be used instead
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct UnhaltBridgeProposal {
    #[prost(string, tag = "1")]
    pub title: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub description: ::prost::alloc::string::String,
    #[prost(uint64, tag = "4")]
    pub target_nonce: u64,
    #[prost(string, tag = "5")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
/// AirdropProposal defines a custom governance proposal type that allows an
/// airdrop to occur in a decentralized fashion. A list of destination addresses
/// and an amount per airdrop recipient is provided. The funds for this airdrop
/// are removed from the Community Pool, if the community pool does not have
/// sufficient funding to perform the airdrop to all provided recipients nothing
/// will occur
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct AirdropProposal {
    #[prost(string, tag = "1")]
    pub title: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub description: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub denom: ::prost::alloc::string::String,
    #[prost(bytes = "vec", tag = "4")]
    pub recipients: ::prost::alloc::vec::Vec<u8>,
    #[prost(uint64, repeated, tag = "5")]
    pub amounts: ::prost::alloc::vec::Vec<u64>,
}
/// IBCMetadataProposal defines a custom governance proposal type that allows
/// governance to set the metadata for an IBC token, this will allow Gravity to
/// deploy an ERC20 representing this token on Ethereum Name: the token name
/// Symbol: the token symbol
/// Description: the token description, not sent to ETH at all, only used on
/// Cosmos Display: the token display name (only used on Cosmos to decide ERC20
/// Decimals) Deicmals: the decimals for the display unit ibc_denom is the denom
/// of the token in question on this chain
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct IbcMetadataProposal {
    #[prost(string, tag = "1")]
    pub title: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub description: ::prost::alloc::string::String,
    #[prost(message, optional, tag = "3")]
    pub metadata: ::core::option::Option<cosmos_sdk_proto::cosmos::bank::v1beta1::Metadata>,
    #[prost(string, tag = "4")]
    pub ibc_denom: ::prost::alloc::string::String,
    #[prost(string, tag = "5")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
/// AddEvmChainProposal
/// this types allows users to add new EVM chain through gov proposal
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct AddEvmChainProposal {
    #[prost(string, tag = "1")]
    pub title: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub description: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub evm_chain_name: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
    #[prost(uint64, tag = "5")]
    pub evm_chain_net_version: u64,
}
/// PendingIbcAutoForward represents a SendToCosmos transaction with a foreign CosmosReceiver which will be added to the
/// PendingIbcAutoForward queue in attestation_handler and sent over IBC on some submission of a MsgExecuteIbcAutoForwards
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct PendingIbcAutoForward {
    /// the destination address. sdk.AccAddress does not preserve foreign prefixes
    #[prost(string, tag = "1")]
    pub foreign_receiver: ::prost::alloc::string::String,
    /// the token sent from ethereum to the ibc-enabled chain over `IbcChannel`
    #[prost(message, optional, tag = "2")]
    pub token: ::core::option::Option<cosmos_sdk_proto::cosmos::base::v1beta1::Coin>,
    /// the IBC channel to send `Amount` over via ibc-transfer module
    #[prost(string, tag = "3")]
    pub ibc_channel: ::prost::alloc::string::String,
    /// the EventNonce from the MsgSendToCosmosClaim, used for ordering the queue
    #[prost(uint64, tag = "4")]
    pub event_nonce: u64,
}
/// MsgSetOrchestratorAddress
/// this message allows validators to delegate their voting responsibilities
/// to a given key. This key is then used as an optional authentication method
/// for sigining oracle claims
/// VALIDATOR
/// The validator field is a cosmosvaloper1... string (i.e. sdk.ValAddress)
/// that references a validator in the active set
/// ORCHESTRATOR
/// The orchestrator field is a cosmos1... string  (i.e. sdk.AccAddress) that
/// references the key that is being delegated to
/// ETH_ADDRESS
/// This is a hex encoded 0x Ethereum public key that will be used by this
/// validator on Ethereum
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgSetOrchestratorAddress {
    #[prost(string, tag = "1")]
    pub validator: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub orchestrator: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub eth_address: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgSetOrchestratorAddressResponse {}
/// MsgValsetConfirm
/// this is the message sent by the validators when they wish to submit their
/// signatures over the validator set at a given block height. A validator must
/// first call MsgSetEthAddress to set their Ethereum address to be used for
/// signing. Then someone (anyone) must make a ValsetRequest, the request is
/// essentially a messaging mechanism to determine which block all validators
/// should submit signatures over. Finally validators sign the validator set,
/// powers, and Ethereum addresses of the entire validator set at the height of a
/// ValsetRequest and submit that signature with this message.
///
/// If a sufficient number of validators (66% of voting power) (A) have set
/// Ethereum addresses and (B) submit ValsetConfirm messages with their
/// signatures it is then possible for anyone to view these signatures in the
/// chain store and submit them to Ethereum to update the validator set
/// -------------
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgValsetConfirm {
    #[prost(uint64, tag = "1")]
    pub nonce: u64,
    #[prost(string, tag = "2")]
    pub orchestrator: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub eth_address: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub signature: ::prost::alloc::string::String,
    #[prost(string, tag = "5")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgValsetConfirmResponse {}
/// MsgSendToEth
/// This is the message that a user calls when they want to bridge an asset
/// it will later be removed when it is included in a batch and successfully
/// submitted tokens are removed from the users balance immediately
/// -------------
/// AMOUNT:
/// the coin to send across the bridge, note the restriction that this is a
/// single coin not a set of coins that is normal in other Cosmos messages
/// BRIDGE_FEE:
/// the fee paid for the bridge, distinct from the fee paid to the chain to
/// actually send this message in the first place. So a successful send has
/// two layers of fees for the user
/// CHAIN_FEE:
/// the fee paid to the chain for handling the request, which must be a
/// certain percentage of the AMOUNT, as determined by governance.
/// This Msg will be rejected if CHAIN_FEE is insufficient.
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgSendToEth {
    #[prost(string, tag = "1")]
    pub sender: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub eth_dest: ::prost::alloc::string::String,
    #[prost(message, optional, tag = "3")]
    pub amount: ::core::option::Option<cosmos_sdk_proto::cosmos::base::v1beta1::Coin>,
    #[prost(message, optional, tag = "4")]
    pub bridge_fee: ::core::option::Option<cosmos_sdk_proto::cosmos::base::v1beta1::Coin>,
    #[prost(message, optional, tag = "5")]
    pub chain_fee: ::core::option::Option<cosmos_sdk_proto::cosmos::base::v1beta1::Coin>,
    #[prost(string, tag = "6")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgSendToEthResponse {}
/// MsgRequestBatch
/// this is a message anyone can send that requests a batch of transactions to
/// send across the bridge be created for whatever block height this message is
/// included in. This acts as a coordination point, the handler for this message
/// looks at the AddToOutgoingPool tx's in the store and generates a batch, also
/// available in the store tied to this message. The validators then grab this
/// batch, sign it, submit the signatures with a MsgConfirmBatch before a relayer
/// can finally submit the batch
/// -------------
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgRequestBatch {
    #[prost(string, tag = "1")]
    pub sender: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub denom: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgRequestBatchResponse {}
/// MsgConfirmBatch
/// When validators observe a MsgRequestBatch they form a batch by ordering
/// transactions currently in the txqueue in order of highest to lowest fee,
/// cutting off when the batch either reaches a hardcoded maximum size (to be
/// decided, probably around 100) or when transactions stop being profitable
/// (TODO determine this without nondeterminism) This message includes the batch
/// as well as an Ethereum signature over this batch by the validator
/// -------------
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgConfirmBatch {
    #[prost(uint64, tag = "1")]
    pub nonce: u64,
    #[prost(string, tag = "2")]
    pub token_contract: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub eth_signer: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub orchestrator: ::prost::alloc::string::String,
    #[prost(string, tag = "5")]
    pub signature: ::prost::alloc::string::String,
    #[prost(string, tag = "6")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgConfirmBatchResponse {}
/// MsgConfirmLogicCall
/// When validators observe a MsgRequestBatch they form a batch by ordering
/// transactions currently in the txqueue in order of highest to lowest fee,
/// cutting off when the batch either reaches a hardcoded maximum size (to be
/// decided, probably around 100) or when transactions stop being profitable
/// (TODO determine this without nondeterminism) This message includes the batch
/// as well as an Ethereum signature over this batch by the validator
/// -------------
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgConfirmLogicCall {
    #[prost(string, tag = "1")]
    pub invalidation_id: ::prost::alloc::string::String,
    #[prost(uint64, tag = "2")]
    pub invalidation_nonce: u64,
    #[prost(string, tag = "3")]
    pub eth_signer: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub orchestrator: ::prost::alloc::string::String,
    #[prost(string, tag = "5")]
    pub signature: ::prost::alloc::string::String,
    #[prost(string, tag = "6")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgConfirmLogicCallResponse {}
/// MsgSendToCosmosClaim
/// When more than 66% of the active validator set has
/// claimed to have seen the deposit enter the ethereum blockchain coins are
/// issued to the Cosmos address in question
/// -------------
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgSendToCosmosClaim {
    #[prost(uint64, tag = "1")]
    pub event_nonce: u64,
    #[prost(uint64, tag = "2")]
    pub eth_block_height: u64,
    #[prost(string, tag = "3")]
    pub token_contract: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub amount: ::prost::alloc::string::String,
    #[prost(string, tag = "5")]
    pub ethereum_sender: ::prost::alloc::string::String,
    #[prost(string, tag = "6")]
    pub cosmos_receiver: ::prost::alloc::string::String,
    #[prost(string, tag = "7")]
    pub orchestrator: ::prost::alloc::string::String,
    #[prost(string, tag = "8")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgSendToCosmosClaimResponse {}
/// MsgExecuteIbcAutoForwards
/// Prompts the forwarding of Pending IBC Auto-Forwards in the queue
/// The Pending forwards will be executed in order of their original
/// SendToCosmos.EventNonce The funds in the queue will be sent to a local
/// gravity-prefixed address if IBC transfer is not possible
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgExecuteIbcAutoForwards {
    /// How many queued forwards to clear, be careful about gas limits
    #[prost(uint64, tag = "1")]
    pub forwards_to_clear: u64,
    /// This message's sender
    #[prost(string, tag = "2")]
    pub executor: ::prost::alloc::string::String,
    /// auto forward for a specific chain
    #[prost(string, tag = "3")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgExecuteIbcAutoForwardsResponse {}
/// BatchSendToEthClaim claims that a batch of send to eth
/// operations on the bridge contract was executed.
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgBatchSendToEthClaim {
    #[prost(uint64, tag = "1")]
    pub event_nonce: u64,
    #[prost(uint64, tag = "2")]
    pub eth_block_height: u64,
    #[prost(uint64, tag = "3")]
    pub batch_nonce: u64,
    #[prost(string, tag = "4")]
    pub token_contract: ::prost::alloc::string::String,
    #[prost(string, tag = "5")]
    pub orchestrator: ::prost::alloc::string::String,
    #[prost(string, tag = "6")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgBatchSendToEthClaimResponse {}
/// ERC20DeployedClaim allows the Cosmos module
/// to learn about an ERC20 that someone deployed
/// to represent a Cosmos asset
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgErc20DeployedClaim {
    #[prost(uint64, tag = "1")]
    pub event_nonce: u64,
    #[prost(uint64, tag = "2")]
    pub eth_block_height: u64,
    #[prost(string, tag = "3")]
    pub cosmos_denom: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub token_contract: ::prost::alloc::string::String,
    #[prost(string, tag = "5")]
    pub name: ::prost::alloc::string::String,
    #[prost(string, tag = "6")]
    pub symbol: ::prost::alloc::string::String,
    #[prost(uint64, tag = "7")]
    pub decimals: u64,
    #[prost(string, tag = "8")]
    pub orchestrator: ::prost::alloc::string::String,
    #[prost(string, tag = "9")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgErc20DeployedClaimResponse {}
/// This informs the Cosmos module that a logic
/// call has been executed
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgLogicCallExecutedClaim {
    #[prost(uint64, tag = "1")]
    pub event_nonce: u64,
    #[prost(uint64, tag = "2")]
    pub eth_block_height: u64,
    #[prost(bytes = "vec", tag = "3")]
    pub invalidation_id: ::prost::alloc::vec::Vec<u8>,
    #[prost(uint64, tag = "4")]
    pub invalidation_nonce: u64,
    #[prost(string, tag = "5")]
    pub orchestrator: ::prost::alloc::string::String,
    #[prost(string, tag = "6")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgLogicCallExecutedClaimResponse {}
/// This informs the Cosmos module that a validator
/// set has been updated.
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgValsetUpdatedClaim {
    #[prost(uint64, tag = "1")]
    pub event_nonce: u64,
    #[prost(uint64, tag = "2")]
    pub valset_nonce: u64,
    #[prost(uint64, tag = "3")]
    pub eth_block_height: u64,
    #[prost(message, repeated, tag = "4")]
    pub members: ::prost::alloc::vec::Vec<BridgeValidator>,
    #[prost(string, tag = "5")]
    pub reward_amount: ::prost::alloc::string::String,
    #[prost(string, tag = "6")]
    pub reward_token: ::prost::alloc::string::String,
    #[prost(string, tag = "7")]
    pub orchestrator: ::prost::alloc::string::String,
    #[prost(string, tag = "8")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgValsetUpdatedClaimResponse {}
/// This call allows the sender (and only the sender)
/// to cancel a given MsgSendToEth and recieve a refund
/// of the tokens
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgCancelSendToEth {
    #[prost(uint64, tag = "1")]
    pub transaction_id: u64,
    #[prost(string, tag = "2")]
    pub sender: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgCancelSendToEthResponse {}
/// This call allows anyone to submit evidence that a
/// validator has signed a valset, batch, or logic call that never
/// existed on the Cosmos chain.
/// Subject contains the batch, valset, or logic call.
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgSubmitBadSignatureEvidence {
    #[prost(message, optional, tag = "1")]
    pub subject: ::core::option::Option<::prost_types::Any>,
    #[prost(string, tag = "2")]
    pub signature: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub sender: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgSubmitBadSignatureEvidenceResponse {}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventSetOperatorAddress {
    #[prost(string, tag = "1")]
    pub message: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub address: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventValsetConfirmKey {
    #[prost(string, tag = "1")]
    pub message: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub key: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventBatchCreated {
    #[prost(string, tag = "1")]
    pub message: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub batch_nonce: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventBatchConfirmKey {
    #[prost(string, tag = "1")]
    pub message: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub batch_confirm_key: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventBatchSendToEthClaim {
    #[prost(string, tag = "1")]
    pub nonce: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventClaim {
    #[prost(string, tag = "1")]
    pub message: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub claim_hash: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub attestation_id: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventBadSignatureEvidence {
    #[prost(string, tag = "1")]
    pub message: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub bad_eth_signature: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub bad_eth_signature_subject: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventErc20DeployedClaim {
    #[prost(string, tag = "1")]
    pub token: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub nonce: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventValsetUpdatedClaim {
    #[prost(string, tag = "1")]
    pub nonce: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventMultisigUpdateRequest {
    #[prost(string, tag = "1")]
    pub bridge_contract: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub bridge_chain_id: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub multisig_id: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub nonce: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventOutgoingLogicCallCanceled {
    #[prost(string, tag = "1")]
    pub logic_call_invalidation_id: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub logic_call_invalidation_nonce: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventSignatureSlashing {
    #[prost(string, tag = "1")]
    pub r#type: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub address: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EventOutgoingTxId {
    #[prost(string, tag = "1")]
    pub message: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub tx_id: ::prost::alloc::string::String,
}
/// Generated client implementations.
pub mod msg_client {
    #![allow(unused_variables, dead_code, missing_docs, clippy::let_unit_value)]
    use tonic::codegen::*;
    use tonic::codegen::http::Uri;
    /// Msg defines the state transitions possible within gravity
    #[derive(Debug, Clone)]
    pub struct MsgClient<T> {
        inner: tonic::client::Grpc<T>,
    }
    impl MsgClient<tonic::transport::Channel> {
        /// Attempt to create a new client by connecting to a given endpoint.
        pub async fn connect<D>(dst: D) -> Result<Self, tonic::transport::Error>
        where
            D: std::convert::TryInto<tonic::transport::Endpoint>,
            D::Error: Into<StdError>,
        {
            let conn = tonic::transport::Endpoint::new(dst)?.connect().await?;
            Ok(Self::new(conn))
        }
    }
    impl<T> MsgClient<T>
    where
        T: tonic::client::GrpcService<tonic::body::BoxBody>,
        T::Error: Into<StdError>,
        T::ResponseBody: Body<Data = Bytes> + Send + 'static,
        <T::ResponseBody as Body>::Error: Into<StdError> + Send,
    {
        pub fn new(inner: T) -> Self {
            let inner = tonic::client::Grpc::new(inner);
            Self { inner }
        }
        pub fn with_origin(inner: T, origin: Uri) -> Self {
            let inner = tonic::client::Grpc::with_origin(inner, origin);
            Self { inner }
        }
        pub fn with_interceptor<F>(
            inner: T,
            interceptor: F,
        ) -> MsgClient<InterceptedService<T, F>>
        where
            F: tonic::service::Interceptor,
            T::ResponseBody: Default,
            T: tonic::codegen::Service<
                http::Request<tonic::body::BoxBody>,
                Response = http::Response<
                    <T as tonic::client::GrpcService<tonic::body::BoxBody>>::ResponseBody,
                >,
            >,
            <T as tonic::codegen::Service<
                http::Request<tonic::body::BoxBody>,
            >>::Error: Into<StdError> + Send + Sync,
        {
            MsgClient::new(InterceptedService::new(inner, interceptor))
        }
        /// Compress requests with the given encoding.
        ///
        /// This requires the server to support it otherwise it might respond with an
        /// error.
        #[must_use]
        pub fn send_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.send_compressed(encoding);
            self
        }
        /// Enable decompressing responses.
        #[must_use]
        pub fn accept_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.accept_compressed(encoding);
            self
        }
        pub async fn valset_confirm(
            &mut self,
            request: impl tonic::IntoRequest<super::MsgValsetConfirm>,
        ) -> Result<tonic::Response<super::MsgValsetConfirmResponse>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Msg/ValsetConfirm",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn send_to_eth(
            &mut self,
            request: impl tonic::IntoRequest<super::MsgSendToEth>,
        ) -> Result<tonic::Response<super::MsgSendToEthResponse>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static("/gravity.v1.Msg/SendToEth");
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn request_batch(
            &mut self,
            request: impl tonic::IntoRequest<super::MsgRequestBatch>,
        ) -> Result<tonic::Response<super::MsgRequestBatchResponse>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Msg/RequestBatch",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn confirm_batch(
            &mut self,
            request: impl tonic::IntoRequest<super::MsgConfirmBatch>,
        ) -> Result<tonic::Response<super::MsgConfirmBatchResponse>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Msg/ConfirmBatch",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn confirm_logic_call(
            &mut self,
            request: impl tonic::IntoRequest<super::MsgConfirmLogicCall>,
        ) -> Result<tonic::Response<super::MsgConfirmLogicCallResponse>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Msg/ConfirmLogicCall",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn send_to_cosmos_claim(
            &mut self,
            request: impl tonic::IntoRequest<super::MsgSendToCosmosClaim>,
        ) -> Result<
            tonic::Response<super::MsgSendToCosmosClaimResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Msg/SendToCosmosClaim",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn execute_ibc_auto_forwards(
            &mut self,
            request: impl tonic::IntoRequest<super::MsgExecuteIbcAutoForwards>,
        ) -> Result<
            tonic::Response<super::MsgExecuteIbcAutoForwardsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Msg/ExecuteIbcAutoForwards",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn batch_send_to_eth_claim(
            &mut self,
            request: impl tonic::IntoRequest<super::MsgBatchSendToEthClaim>,
        ) -> Result<
            tonic::Response<super::MsgBatchSendToEthClaimResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Msg/BatchSendToEthClaim",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn valset_update_claim(
            &mut self,
            request: impl tonic::IntoRequest<super::MsgValsetUpdatedClaim>,
        ) -> Result<
            tonic::Response<super::MsgValsetUpdatedClaimResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Msg/ValsetUpdateClaim",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn erc20_deployed_claim(
            &mut self,
            request: impl tonic::IntoRequest<super::MsgErc20DeployedClaim>,
        ) -> Result<
            tonic::Response<super::MsgErc20DeployedClaimResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Msg/ERC20DeployedClaim",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn logic_call_executed_claim(
            &mut self,
            request: impl tonic::IntoRequest<super::MsgLogicCallExecutedClaim>,
        ) -> Result<
            tonic::Response<super::MsgLogicCallExecutedClaimResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Msg/LogicCallExecutedClaim",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn set_orchestrator_address(
            &mut self,
            request: impl tonic::IntoRequest<super::MsgSetOrchestratorAddress>,
        ) -> Result<
            tonic::Response<super::MsgSetOrchestratorAddressResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Msg/SetOrchestratorAddress",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn cancel_send_to_eth(
            &mut self,
            request: impl tonic::IntoRequest<super::MsgCancelSendToEth>,
        ) -> Result<tonic::Response<super::MsgCancelSendToEthResponse>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Msg/CancelSendToEth",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn submit_bad_signature_evidence(
            &mut self,
            request: impl tonic::IntoRequest<super::MsgSubmitBadSignatureEvidence>,
        ) -> Result<
            tonic::Response<super::MsgSubmitBadSignatureEvidenceResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Msg/SubmitBadSignatureEvidence",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
    }
}
/// The slashing fractions for the various gravity related slashing conditions. The first three
/// refer to not submitting a particular message, the third for submitting a different claim
/// for the same Ethereum event
///
/// unbond_slashing_valsets_window
///
/// The unbond slashing valsets window is used to determine how many blocks after starting to unbond
/// a validator needs to continue signing blocks. The goal of this paramater is that when a validator leaves
/// the set, if their leaving creates enough change in the validator set to justify an update they will sign
/// a validator set update for the Ethereum bridge that does not include themselves. Allowing us to remove them
/// from the Ethereum bridge and replace them with the new set gracefully.
///
/// valset_reward
///
/// These parameters allow for the bridge oracle to resolve a fork on the Ethereum chain without halting
/// the chain. Once set reset bridge state will roll back events to the nonce provided in reset_bridge_nonce
/// if and only if those events have not yet been observed (executed on the Cosmos chain). This allows for easy
/// handling of cases where for example an Ethereum hardfork has occured and more than 1/3 of the vlaidtor set
/// disagrees with the rest. Normally this would require a chain halt, manual genesis editing and restar to resolve
/// with this feature a governance proposal can be used instead
///
/// bridge_active
///
/// This boolean flag can be used by governance to temporarily halt the bridge due to a vulnerability or other issue
/// In this context halting the bridge means prevent the execution of any oracle events from Ethereum and preventing
/// the creation of new batches that may be relayed to Ethereum.
/// This does not prevent the creation of validator sets
/// or slashing for not submitting validator set signatures as either of these might allow key signers to leave the validator
/// set and steal funds on Ethereum without consequence.
/// The practical outcome of this flag being set to 'false' is that deposits from Ethereum will not show up and withdraws from
/// Cosmos will not execute on Ethereum.
///
/// min_chain_fee_basis_points
///
/// The minimum SendToEth `chain_fee` amount, in terms of basis points. e.g. 10% fee = 1000, and 0.02% fee = 2
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Params {
    #[prost(string, tag = "1")]
    pub gravity_id: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub contract_source_hash: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub bridge_ethereum_address: ::prost::alloc::string::String,
    #[prost(uint64, tag = "5")]
    pub bridge_chain_id: u64,
    #[prost(uint64, tag = "6")]
    pub signed_valsets_window: u64,
    #[prost(uint64, tag = "7")]
    pub signed_batches_window: u64,
    #[prost(uint64, tag = "8")]
    pub signed_logic_calls_window: u64,
    #[prost(uint64, tag = "9")]
    pub target_batch_timeout: u64,
    #[prost(uint64, tag = "10")]
    pub average_block_time: u64,
    #[prost(uint64, tag = "11")]
    pub average_ethereum_block_time: u64,
    #[prost(bytes = "vec", tag = "12")]
    pub slash_fraction_valset: ::prost::alloc::vec::Vec<u8>,
    #[prost(bytes = "vec", tag = "13")]
    pub slash_fraction_batch: ::prost::alloc::vec::Vec<u8>,
    #[prost(bytes = "vec", tag = "14")]
    pub slash_fraction_logic_call: ::prost::alloc::vec::Vec<u8>,
    #[prost(uint64, tag = "15")]
    pub unbond_slashing_valsets_window: u64,
    #[prost(bytes = "vec", tag = "16")]
    pub slash_fraction_bad_eth_signature: ::prost::alloc::vec::Vec<u8>,
    #[prost(message, optional, tag = "17")]
    pub valset_reward: ::core::option::Option<cosmos_sdk_proto::cosmos::base::v1beta1::Coin>,
    #[prost(bool, tag = "18")]
    pub bridge_active: bool,
    /// addresses on this blacklist are forbidden from depositing or withdrawing
    /// from Ethereum to the bridge
    #[prost(string, repeated, tag = "19")]
    pub ethereum_blacklist: ::prost::alloc::vec::Vec<::prost::alloc::string::String>,
    #[prost(uint64, tag = "20")]
    pub min_chain_fee_basis_points: u64,
}
/// GenesisState struct, containing all persistant data required by the Gravity module
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct GenesisState {
    #[prost(message, optional, tag = "1")]
    pub params: ::core::option::Option<Params>,
    #[prost(message, repeated, tag = "2")]
    pub evm_chains: ::prost::alloc::vec::Vec<EvmChainData>,
}
/// EvmChainData struct, containing all persistant data per EVM chain required by the Gravity module
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EvmChainData {
    #[prost(message, optional, tag = "1")]
    pub evm_chain: ::core::option::Option<EvmChain>,
    #[prost(message, optional, tag = "2")]
    pub gravity_nonces: ::core::option::Option<GravityNonces>,
    #[prost(message, repeated, tag = "3")]
    pub valsets: ::prost::alloc::vec::Vec<Valset>,
    #[prost(message, repeated, tag = "4")]
    pub valset_confirms: ::prost::alloc::vec::Vec<MsgValsetConfirm>,
    #[prost(message, repeated, tag = "5")]
    pub batches: ::prost::alloc::vec::Vec<OutgoingTxBatch>,
    #[prost(message, repeated, tag = "6")]
    pub batch_confirms: ::prost::alloc::vec::Vec<MsgConfirmBatch>,
    #[prost(message, repeated, tag = "7")]
    pub logic_calls: ::prost::alloc::vec::Vec<OutgoingLogicCall>,
    #[prost(message, repeated, tag = "8")]
    pub logic_call_confirms: ::prost::alloc::vec::Vec<MsgConfirmLogicCall>,
    #[prost(message, repeated, tag = "9")]
    pub attestations: ::prost::alloc::vec::Vec<Attestation>,
    #[prost(message, repeated, tag = "10")]
    pub delegate_keys: ::prost::alloc::vec::Vec<MsgSetOrchestratorAddress>,
    #[prost(message, repeated, tag = "11")]
    pub erc20_to_denoms: ::prost::alloc::vec::Vec<Erc20ToDenom>,
    #[prost(message, repeated, tag = "12")]
    pub unbatched_transfers: ::prost::alloc::vec::Vec<OutgoingTransferTx>,
    #[prost(message, repeated, tag = "13")]
    pub pending_ibc_auto_forwards: ::prost::alloc::vec::Vec<PendingIbcAutoForward>,
}
/// EvmChain struct contains EVM chain specific data
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EvmChain {
    #[prost(string, tag = "1")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub evm_chain_name: ::prost::alloc::string::String,
    #[prost(uint64, tag = "3")]
    pub evm_chain_net_version: u64,
}
/// GravityCounters contains the many noces and counters required to maintain the bridge state in the genesis
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct GravityNonces {
    /// the nonce of the last generated validator set
    #[prost(uint64, tag = "1")]
    pub latest_valset_nonce: u64,
    /// the last observed Gravity.sol contract event nonce
    #[prost(uint64, tag = "2")]
    pub last_observed_nonce: u64,
    /// the last valset nonce we have slashed, to prevent double slashing
    #[prost(uint64, tag = "3")]
    pub last_slashed_valset_nonce: u64,
    /// the last batch Cosmos chain block that batch slashing has completed for
    /// there is an individual batch nonce for each token type so this removes
    /// the need to store them all
    #[prost(uint64, tag = "4")]
    pub last_slashed_batch_block: u64,
    /// the last cosmos block that logic call slashing has completed for
    #[prost(uint64, tag = "5")]
    pub last_slashed_logic_call_block: u64,
    /// the last transaction id from the Gravity TX pool, this prevents ID
    /// duplication during chain upgrades
    #[prost(uint64, tag = "6")]
    pub last_tx_pool_id: u64,
    /// the last batch id from the Gravity batch pool, this prevents ID duplication
    /// during chain upgrades
    #[prost(uint64, tag = "7")]
    pub last_batch_id: u64,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryParamsRequest {}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryParamsResponse {
    #[prost(message, optional, tag = "1")]
    pub params: ::core::option::Option<Params>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryCurrentValsetRequest {
    #[prost(string, tag = "1")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryCurrentValsetResponse {
    #[prost(message, optional, tag = "1")]
    pub valset: ::core::option::Option<Valset>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryValsetRequestRequest {
    #[prost(uint64, tag = "1")]
    pub nonce: u64,
    #[prost(string, tag = "2")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryValsetRequestResponse {
    #[prost(message, optional, tag = "1")]
    pub valset: ::core::option::Option<Valset>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryValsetConfirmRequest {
    #[prost(uint64, tag = "1")]
    pub nonce: u64,
    #[prost(string, tag = "2")]
    pub address: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryValsetConfirmResponse {
    #[prost(message, optional, tag = "1")]
    pub confirm: ::core::option::Option<MsgValsetConfirm>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryValsetConfirmsByNonceRequest {
    #[prost(uint64, tag = "1")]
    pub nonce: u64,
    #[prost(string, tag = "2")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryValsetConfirmsByNonceResponse {
    #[prost(message, repeated, tag = "1")]
    pub confirms: ::prost::alloc::vec::Vec<MsgValsetConfirm>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastValsetRequestsRequest {
    #[prost(string, tag = "1")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastValsetRequestsResponse {
    #[prost(message, repeated, tag = "1")]
    pub valsets: ::prost::alloc::vec::Vec<Valset>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastPendingValsetRequestByAddrRequest {
    #[prost(string, tag = "1")]
    pub address: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastPendingValsetRequestByAddrResponse {
    #[prost(message, repeated, tag = "1")]
    pub valsets: ::prost::alloc::vec::Vec<Valset>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryBatchFeeRequest {
    #[prost(string, tag = "1")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryBatchFeeResponse {
    #[prost(message, repeated, tag = "1")]
    pub batch_fees: ::prost::alloc::vec::Vec<BatchFees>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastPendingBatchRequestByAddrRequest {
    #[prost(string, tag = "1")]
    pub address: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastPendingBatchRequestByAddrResponse {
    #[prost(message, repeated, tag = "1")]
    pub batch: ::prost::alloc::vec::Vec<OutgoingTxBatch>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastPendingLogicCallByAddrRequest {
    #[prost(string, tag = "1")]
    pub address: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastPendingLogicCallByAddrResponse {
    #[prost(message, repeated, tag = "1")]
    pub call: ::prost::alloc::vec::Vec<OutgoingLogicCall>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryOutgoingTxBatchesRequest {
    #[prost(string, tag = "1")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryOutgoingTxBatchesResponse {
    #[prost(message, repeated, tag = "1")]
    pub batches: ::prost::alloc::vec::Vec<OutgoingTxBatch>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryOutgoingLogicCallsRequest {
    #[prost(string, tag = "1")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryOutgoingLogicCallsResponse {
    #[prost(message, repeated, tag = "1")]
    pub calls: ::prost::alloc::vec::Vec<OutgoingLogicCall>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryBatchRequestByNonceRequest {
    #[prost(uint64, tag = "1")]
    pub nonce: u64,
    #[prost(string, tag = "2")]
    pub contract_address: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryBatchRequestByNonceResponse {
    #[prost(message, optional, tag = "1")]
    pub batch: ::core::option::Option<OutgoingTxBatch>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryBatchConfirmsRequest {
    #[prost(uint64, tag = "1")]
    pub nonce: u64,
    #[prost(string, tag = "2")]
    pub contract_address: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryBatchConfirmsResponse {
    #[prost(message, repeated, tag = "1")]
    pub confirms: ::prost::alloc::vec::Vec<MsgConfirmBatch>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLogicConfirmsRequest {
    #[prost(bytes = "vec", tag = "1")]
    pub invalidation_id: ::prost::alloc::vec::Vec<u8>,
    #[prost(uint64, tag = "2")]
    pub invalidation_nonce: u64,
    #[prost(string, tag = "3")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLogicConfirmsResponse {
    #[prost(message, repeated, tag = "1")]
    pub confirms: ::prost::alloc::vec::Vec<MsgConfirmLogicCall>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastEventNonceByAddrRequest {
    #[prost(string, tag = "1")]
    pub address: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastEventNonceByAddrResponse {
    #[prost(uint64, tag = "1")]
    pub event_nonce: u64,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryErc20ToDenomRequest {
    #[prost(string, tag = "1")]
    pub erc20: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryErc20ToDenomResponse {
    #[prost(string, tag = "1")]
    pub denom: ::prost::alloc::string::String,
    #[prost(bool, tag = "2")]
    pub cosmos_originated: bool,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryDenomToErc20Request {
    #[prost(string, tag = "1")]
    pub denom: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryDenomToErc20Response {
    #[prost(string, tag = "1")]
    pub erc20: ::prost::alloc::string::String,
    #[prost(bool, tag = "2")]
    pub cosmos_originated: bool,
}
/// QueryLastObservedEthBlockRequest defines the request for getting the height
/// of the last applied Ethereum Event on the bridge. This is expected to lag the
/// actual Ethereum block height significantly due to 1. Ethereum Finality and
///   2. Consensus mirroring the state on Ethereum
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastObservedEthBlockRequest {
    /// indicates whether to search for store data using the old Gravity v1 key
    /// "LastObservedEthereumBlockHeightKey" Note that queries before the Mercury
    /// upgrade at height 1282013 must set this to true
    #[prost(bool, tag = "1")]
    pub use_v1_key: bool,
    /// new version query by evm chain prefix
    #[prost(string, tag = "2")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastObservedEthBlockResponse {
    /// a response of 0 indicates that no Ethereum events have been observed, and
    /// thus the bridge is inactive
    #[prost(uint64, tag = "1")]
    pub block: u64,
}
/// QueryLastObservedEthNonceRequest defines the request for getting the event
/// nonce of the last applied Ethereum Event on the bridge. Note that this is
/// likely to lag the last executed event a little due to 1. Ethereum Finality
/// and 2. Consensus mirroring the Ethereum state
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastObservedEthNonceRequest {
    /// indicates whether to search for store data using the old Gravity v1 key
    /// "LastObservedEventNonceKey" Note that queries before the Mercury upgrade at
    /// height 1282013 must set this to true
    #[prost(bool, tag = "1")]
    pub use_v1_key: bool,
    /// new version query by evm chain prefix
    #[prost(string, tag = "2")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastObservedEthNonceResponse {
    /// a response of 0 indicates that no Ethereum events have been observed, and
    /// thus the bridge is inactive
    #[prost(uint64, tag = "1")]
    pub nonce: u64,
}
/// QueryAttestationsRequest defines the request structure for getting recent
/// attestations with optional query parameters. By default, a limited set of
/// recent attestations will be returned, defined by 'limit'. These attestations
/// can be ordered ascending or descending by nonce, that defaults to ascending.
/// Filtering criteria may also be provided, including nonce, claim type, and
/// height. Note, that an attestation will be returned if it matches ANY of the
/// filter query parameters provided.
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryAttestationsRequest {
    /// limit defines how many attestations to limit in the response.
    #[prost(uint64, tag = "1")]
    pub limit: u64,
    /// order_by provides ordering of atteststions by nonce in the response. Either
    /// 'asc' or 'desc' can be provided. If no value is provided, it defaults to
    /// 'asc'.
    #[prost(string, tag = "2")]
    pub order_by: ::prost::alloc::string::String,
    /// claim_type allows filtering attestations by Ethereum claim type.
    #[prost(string, tag = "3")]
    pub claim_type: ::prost::alloc::string::String,
    /// nonce allows filtering attestations by Ethereum claim nonce.
    #[prost(uint64, tag = "4")]
    pub nonce: u64,
    /// height allows filtering attestations by Ethereum claim height.
    #[prost(uint64, tag = "5")]
    pub height: u64,
    /// indicates whether to search for store data using the old Gravity v1 key
    /// "OracleAttestationKey" Note that queries before the Mercury upgrade at
    /// height 1282013 must set this to true
    #[prost(bool, tag = "6")]
    pub use_v1_key: bool,
    #[prost(string, tag = "7")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryAttestationsResponse {
    #[prost(message, repeated, tag = "1")]
    pub attestations: ::prost::alloc::vec::Vec<Attestation>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryDelegateKeysByValidatorAddress {
    #[prost(string, tag = "1")]
    pub validator_address: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryDelegateKeysByValidatorAddressResponse {
    #[prost(string, tag = "1")]
    pub eth_address: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub orchestrator_address: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryDelegateKeysByEthAddress {
    #[prost(string, tag = "1")]
    pub eth_address: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryDelegateKeysByEthAddressResponse {
    #[prost(string, tag = "1")]
    pub validator_address: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub orchestrator_address: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryDelegateKeysByOrchestratorAddress {
    #[prost(string, tag = "1")]
    pub orchestrator_address: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryDelegateKeysByOrchestratorAddressResponse {
    #[prost(string, tag = "1")]
    pub validator_address: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub eth_address: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryPendingSendToEth {
    #[prost(string, tag = "1")]
    pub sender_address: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryPendingSendToEthResponse {
    #[prost(message, repeated, tag = "1")]
    pub transfers_in_batches: ::prost::alloc::vec::Vec<OutgoingTransferTx>,
    #[prost(message, repeated, tag = "2")]
    pub unbatched_transfers: ::prost::alloc::vec::Vec<OutgoingTransferTx>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryPendingIbcAutoForwards {
    /// limit defines the number of pending forwards to return, in order of their
    /// SendToCosmos.EventNonce
    #[prost(uint64, tag = "1")]
    pub limit: u64,
    #[prost(string, tag = "2")]
    pub evm_chain_prefix: ::prost::alloc::string::String,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryPendingIbcAutoForwardsResponse {
    #[prost(message, repeated, tag = "1")]
    pub pending_ibc_auto_forwards: ::prost::alloc::vec::Vec<PendingIbcAutoForward>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryListEvmChains {
    /// limit defines the number of pending forwards to return, in order of their
    /// SendToCosmos.EventNonce
    #[prost(uint64, tag = "1")]
    pub limit: u64,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryListEvmChainsResponse {
    #[prost(message, repeated, tag = "1")]
    pub evm_chains: ::prost::alloc::vec::Vec<EvmChain>,
}
/// Generated client implementations.
pub mod query_client {
    #![allow(unused_variables, dead_code, missing_docs, clippy::let_unit_value)]
    use tonic::codegen::*;
    use tonic::codegen::http::Uri;
    /// Query defines the gRPC querier service
    #[derive(Debug, Clone)]
    pub struct QueryClient<T> {
        inner: tonic::client::Grpc<T>,
    }
    impl QueryClient<tonic::transport::Channel> {
        /// Attempt to create a new client by connecting to a given endpoint.
        pub async fn connect<D>(dst: D) -> Result<Self, tonic::transport::Error>
        where
            D: std::convert::TryInto<tonic::transport::Endpoint>,
            D::Error: Into<StdError>,
        {
            let conn = tonic::transport::Endpoint::new(dst)?.connect().await?;
            Ok(Self::new(conn))
        }
    }
    impl<T> QueryClient<T>
    where
        T: tonic::client::GrpcService<tonic::body::BoxBody>,
        T::Error: Into<StdError>,
        T::ResponseBody: Body<Data = Bytes> + Send + 'static,
        <T::ResponseBody as Body>::Error: Into<StdError> + Send,
    {
        pub fn new(inner: T) -> Self {
            let inner = tonic::client::Grpc::new(inner);
            Self { inner }
        }
        pub fn with_origin(inner: T, origin: Uri) -> Self {
            let inner = tonic::client::Grpc::with_origin(inner, origin);
            Self { inner }
        }
        pub fn with_interceptor<F>(
            inner: T,
            interceptor: F,
        ) -> QueryClient<InterceptedService<T, F>>
        where
            F: tonic::service::Interceptor,
            T::ResponseBody: Default,
            T: tonic::codegen::Service<
                http::Request<tonic::body::BoxBody>,
                Response = http::Response<
                    <T as tonic::client::GrpcService<tonic::body::BoxBody>>::ResponseBody,
                >,
            >,
            <T as tonic::codegen::Service<
                http::Request<tonic::body::BoxBody>,
            >>::Error: Into<StdError> + Send + Sync,
        {
            QueryClient::new(InterceptedService::new(inner, interceptor))
        }
        /// Compress requests with the given encoding.
        ///
        /// This requires the server to support it otherwise it might respond with an
        /// error.
        #[must_use]
        pub fn send_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.send_compressed(encoding);
            self
        }
        /// Enable decompressing responses.
        #[must_use]
        pub fn accept_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.accept_compressed(encoding);
            self
        }
        /// Deployments queries deployments
        pub async fn params(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryParamsRequest>,
        ) -> Result<tonic::Response<super::QueryParamsResponse>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static("/gravity.v1.Query/Params");
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn current_valset(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryCurrentValsetRequest>,
        ) -> Result<tonic::Response<super::QueryCurrentValsetResponse>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/CurrentValset",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn valset_request(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryValsetRequestRequest>,
        ) -> Result<tonic::Response<super::QueryValsetRequestResponse>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/ValsetRequest",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn valset_confirm(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryValsetConfirmRequest>,
        ) -> Result<tonic::Response<super::QueryValsetConfirmResponse>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/ValsetConfirm",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn valset_confirms_by_nonce(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryValsetConfirmsByNonceRequest>,
        ) -> Result<
            tonic::Response<super::QueryValsetConfirmsByNonceResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/ValsetConfirmsByNonce",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn last_valset_requests(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryLastValsetRequestsRequest>,
        ) -> Result<
            tonic::Response<super::QueryLastValsetRequestsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/LastValsetRequests",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn last_pending_valset_request_by_addr(
            &mut self,
            request: impl tonic::IntoRequest<
                super::QueryLastPendingValsetRequestByAddrRequest,
            >,
        ) -> Result<
            tonic::Response<super::QueryLastPendingValsetRequestByAddrResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/LastPendingValsetRequestByAddr",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn last_pending_batch_request_by_addr(
            &mut self,
            request: impl tonic::IntoRequest<
                super::QueryLastPendingBatchRequestByAddrRequest,
            >,
        ) -> Result<
            tonic::Response<super::QueryLastPendingBatchRequestByAddrResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/LastPendingBatchRequestByAddr",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn last_pending_logic_call_by_addr(
            &mut self,
            request: impl tonic::IntoRequest<
                super::QueryLastPendingLogicCallByAddrRequest,
            >,
        ) -> Result<
            tonic::Response<super::QueryLastPendingLogicCallByAddrResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/LastPendingLogicCallByAddr",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn last_event_nonce_by_addr(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryLastEventNonceByAddrRequest>,
        ) -> Result<
            tonic::Response<super::QueryLastEventNonceByAddrResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/LastEventNonceByAddr",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn batch_fees(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryBatchFeeRequest>,
        ) -> Result<tonic::Response<super::QueryBatchFeeResponse>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/BatchFees",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn outgoing_tx_batches(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryOutgoingTxBatchesRequest>,
        ) -> Result<
            tonic::Response<super::QueryOutgoingTxBatchesResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/OutgoingTxBatches",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn outgoing_logic_calls(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryOutgoingLogicCallsRequest>,
        ) -> Result<
            tonic::Response<super::QueryOutgoingLogicCallsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/OutgoingLogicCalls",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn batch_request_by_nonce(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryBatchRequestByNonceRequest>,
        ) -> Result<
            tonic::Response<super::QueryBatchRequestByNonceResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/BatchRequestByNonce",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn batch_confirms(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryBatchConfirmsRequest>,
        ) -> Result<tonic::Response<super::QueryBatchConfirmsResponse>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/BatchConfirms",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn logic_confirms(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryLogicConfirmsRequest>,
        ) -> Result<tonic::Response<super::QueryLogicConfirmsResponse>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/LogicConfirms",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn erc20_to_denom(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryErc20ToDenomRequest>,
        ) -> Result<tonic::Response<super::QueryErc20ToDenomResponse>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/ERC20ToDenom",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn denom_to_erc20(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryDenomToErc20Request>,
        ) -> Result<tonic::Response<super::QueryDenomToErc20Response>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/DenomToERC20",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn get_last_observed_eth_block(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryLastObservedEthBlockRequest>,
        ) -> Result<
            tonic::Response<super::QueryLastObservedEthBlockResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/GetLastObservedEthBlock",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn get_last_observed_eth_nonce(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryLastObservedEthNonceRequest>,
        ) -> Result<
            tonic::Response<super::QueryLastObservedEthNonceResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/GetLastObservedEthNonce",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn get_attestations(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryAttestationsRequest>,
        ) -> Result<tonic::Response<super::QueryAttestationsResponse>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/GetAttestations",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn get_delegate_key_by_validator(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryDelegateKeysByValidatorAddress>,
        ) -> Result<
            tonic::Response<super::QueryDelegateKeysByValidatorAddressResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/GetDelegateKeyByValidator",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn get_delegate_key_by_eth(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryDelegateKeysByEthAddress>,
        ) -> Result<
            tonic::Response<super::QueryDelegateKeysByEthAddressResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/GetDelegateKeyByEth",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn get_delegate_key_by_orchestrator(
            &mut self,
            request: impl tonic::IntoRequest<
                super::QueryDelegateKeysByOrchestratorAddress,
            >,
        ) -> Result<
            tonic::Response<super::QueryDelegateKeysByOrchestratorAddressResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/GetDelegateKeyByOrchestrator",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn get_pending_send_to_eth(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryPendingSendToEth>,
        ) -> Result<
            tonic::Response<super::QueryPendingSendToEthResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/GetPendingSendToEth",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn get_pending_ibc_auto_forwards(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryPendingIbcAutoForwards>,
        ) -> Result<
            tonic::Response<super::QueryPendingIbcAutoForwardsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/GetPendingIbcAutoForwards",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        pub async fn get_list_evm_chains(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryListEvmChains>,
        ) -> Result<tonic::Response<super::QueryListEvmChainsResponse>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::new(
                        tonic::Code::Unknown,
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/gravity.v1.Query/GetListEvmChains",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
    }
}
/// SignType defines messages that have been signed by an orchestrator
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord, ::prost::Enumeration)]
#[repr(i32)]
pub enum SignType {
    /// An unspecified type
    Unspecified = 0,
    /// A type for multi-sig updates
    OrchestratorSignedMultiSigUpdate = 1,
    /// A type for batches
    OrchestratorSignedWithdrawBatch = 2,
}
impl SignType {
    /// String value of the enum field names used in the ProtoBuf definition.
    ///
    /// The values are not transformed in any way and thus are considered stable
    /// (if the ProtoBuf definition does not change) and safe for programmatic use.
    pub fn as_str_name(&self) -> &'static str {
        match self {
            SignType::Unspecified => "SIGN_TYPE_UNSPECIFIED",
            SignType::OrchestratorSignedMultiSigUpdate => {
                "SIGN_TYPE_ORCHESTRATOR_SIGNED_MULTI_SIG_UPDATE"
            }
            SignType::OrchestratorSignedWithdrawBatch => {
                "SIGN_TYPE_ORCHESTRATOR_SIGNED_WITHDRAW_BATCH"
            }
        }
    }
    /// Creates an enum from field names used in the ProtoBuf definition.
    pub fn from_str_name(value: &str) -> ::core::option::Option<Self> {
        match value {
            "SIGN_TYPE_UNSPECIFIED" => Some(Self::Unspecified),
            "SIGN_TYPE_ORCHESTRATOR_SIGNED_MULTI_SIG_UPDATE" => {
                Some(Self::OrchestratorSignedMultiSigUpdate)
            }
            "SIGN_TYPE_ORCHESTRATOR_SIGNED_WITHDRAW_BATCH" => {
                Some(Self::OrchestratorSignedWithdrawBatch)
            }
            _ => None,
        }
    }
}
