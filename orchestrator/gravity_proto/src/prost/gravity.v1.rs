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
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Attestation {
    #[prost(bool, tag="1")]
    pub observed: bool,
    #[prost(string, repeated, tag="2")]
    pub votes: ::prost::alloc::vec::Vec<::prost::alloc::string::String>,
    #[prost(uint64, tag="3")]
    pub height: u64,
    #[prost(message, optional, tag="4")]
    pub claim: ::core::option::Option<::prost_types::Any>,
}
/// ERC20Token unique identifier for an Ethereum ERC20 token.
/// CONTRACT:
/// The contract address on ETH of the token, this could be a Cosmos
/// originated token, if so it will be the ERC20 address of the representation
/// (note: developers should look up the token symbol using the address on ETH to display for UI)
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Erc20Token {
    #[prost(string, tag="1")]
    pub contract: ::prost::alloc::string::String,
    #[prost(string, tag="2")]
    pub amount: ::prost::alloc::string::String,
}
// ClaimType is the cosmos type of an event from the counterpart chain that can
// be handled

#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord, ::prost::Enumeration)]
#[repr(i32)]
pub enum ClaimType {
    Unspecified = 0,
    SendToCosmos = 1,
    BatchSendToEth = 2,
    Erc20Deployed = 3,
    LogicCallExecuted = 4,
    ValsetUpdated = 5,
}
/// IDSet represents a set of IDs
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct IdSet {
    #[prost(uint64, repeated, tag="1")]
    pub ids: ::prost::alloc::vec::Vec<u64>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct BatchFees {
    #[prost(string, tag="1")]
    pub token: ::prost::alloc::string::String,
    #[prost(string, tag="2")]
    pub total_fees: ::prost::alloc::string::String,
    #[prost(uint64, tag="3")]
    pub tx_count: u64,
}
/// OutgoingTxBatch represents a batch of transactions going from gravity to ETH
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct OutgoingTxBatch {
    #[prost(uint64, tag="1")]
    pub batch_nonce: u64,
    #[prost(uint64, tag="2")]
    pub batch_timeout: u64,
    #[prost(message, repeated, tag="3")]
    pub transactions: ::prost::alloc::vec::Vec<OutgoingTransferTx>,
    #[prost(string, tag="4")]
    pub token_contract: ::prost::alloc::string::String,
    #[prost(uint64, tag="5")]
    pub block: u64,
}
/// OutgoingTransferTx represents an individual send from gravity to ETH
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct OutgoingTransferTx {
    #[prost(uint64, tag="1")]
    pub id: u64,
    #[prost(string, tag="2")]
    pub sender: ::prost::alloc::string::String,
    #[prost(string, tag="3")]
    pub dest_address: ::prost::alloc::string::String,
    #[prost(message, optional, tag="4")]
    pub erc20_token: ::core::option::Option<Erc20Token>,
    #[prost(message, optional, tag="5")]
    pub erc20_fee: ::core::option::Option<Erc20Token>,
}
/// OutgoingLogicCall represents an individual logic call from gravity to ETH
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct OutgoingLogicCall {
    #[prost(message, repeated, tag="1")]
    pub transfers: ::prost::alloc::vec::Vec<Erc20Token>,
    #[prost(message, repeated, tag="2")]
    pub fees: ::prost::alloc::vec::Vec<Erc20Token>,
    #[prost(string, tag="3")]
    pub logic_contract_address: ::prost::alloc::string::String,
    #[prost(bytes="vec", tag="4")]
    pub payload: ::prost::alloc::vec::Vec<u8>,
    #[prost(uint64, tag="5")]
    pub timeout: u64,
    #[prost(bytes="vec", tag="6")]
    pub invalidation_id: ::prost::alloc::vec::Vec<u8>,
    #[prost(uint64, tag="7")]
    pub invalidation_nonce: u64,
    #[prost(uint64, tag="8")]
    pub block: u64,
}
/// SignType defines messages that have been signed by an orchestrator
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord, ::prost::Enumeration)]
#[repr(i32)]
pub enum SignType {
    Unspecified = 0,
    OrchestratorSignedMultiSigUpdate = 1,
    OrchestratorSignedWithdrawBatch = 2,
}
/// BridgeValidator represents a validator's ETH address and its power
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct BridgeValidator {
    #[prost(uint64, tag="1")]
    pub power: u64,
    #[prost(string, tag="2")]
    pub ethereum_address: ::prost::alloc::string::String,
}
/// Valset is the Ethereum Bridge Multsig Set, each gravity validator also
/// maintains an ETH key to sign messages, these are used to check signatures on
/// ETH because of the significant gas savings
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Valset {
    #[prost(uint64, tag="1")]
    pub nonce: u64,
    #[prost(message, repeated, tag="2")]
    pub members: ::prost::alloc::vec::Vec<BridgeValidator>,
    #[prost(uint64, tag="3")]
    pub height: u64,
    #[prost(string, tag="4")]
    pub reward_amount: ::prost::alloc::string::String,
    /// the reward token in it's Ethereum hex address representation
    #[prost(string, tag="5")]
    pub reward_token: ::prost::alloc::string::String,
}
/// LastObservedEthereumBlockHeight stores the last observed
/// Ethereum block height along with the Cosmos block height that
/// it was observed at. These two numbers can be used to project
/// outward and always produce batches with timeouts in the future
/// even if no Ethereum block height has been relayed for a long time
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct LastObservedEthereumBlockHeight {
    #[prost(uint64, tag="1")]
    pub cosmos_block_height: u64,
    #[prost(uint64, tag="2")]
    pub ethereum_block_height: u64,
}
/// This records the relationship between an ERC20 token and the denom
/// of the corresponding Cosmos originated asset
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Erc20ToDenom {
    #[prost(string, tag="1")]
    pub erc20: ::prost::alloc::string::String,
    #[prost(string, tag="2")]
    pub denom: ::prost::alloc::string::String,
}
/// UnhaltBridgeProposal defines a custom governance proposal useful for restoring
/// the bridge after a oracle disagreement. Once this proposal is passed bridge state will roll back events 
/// to the nonce provided in target_nonce if and only if those events have not yet been observed (executed on the Cosmos chain). This allows for easy
/// handling of cases where for example an Ethereum hardfork has occured and more than 1/3 of the vlaidtor set
/// disagrees with the rest. Normally this would require a chain halt, manual genesis editing and restar to resolve
/// with this feature a governance proposal can be used instead
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct UnhaltBridgeProposal {
    #[prost(string, tag="1")]
    pub title: ::prost::alloc::string::String,
    #[prost(string, tag="2")]
    pub description: ::prost::alloc::string::String,
    #[prost(uint64, tag="4")]
    pub target_nonce: u64,
}
/// AirdropProposal defines a custom governance proposal type that allows an airdrop to occur in a decentralized
/// fashion. A list of destination addresses and an amount per airdrop recipient is provided. The funds for this
/// airdrop are removed from the Community Pool, if the community pool does not have sufficient funding to perform
/// the airdrop to all provided recipients nothing will occur
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct AirdropProposal {
    #[prost(string, tag="1")]
    pub title: ::prost::alloc::string::String,
    #[prost(string, tag="2")]
    pub description: ::prost::alloc::string::String,
    #[prost(string, tag="3")]
    pub denom: ::prost::alloc::string::String,
    #[prost(bytes="vec", tag="4")]
    pub recipients: ::prost::alloc::vec::Vec<u8>,
    #[prost(uint64, repeated, tag="5")]
    pub amounts: ::prost::alloc::vec::Vec<u64>,
}
/// IBCMetadataProposal defines a custom governance proposal type that allows governance to set the
/// metadata for an IBC token, this will allow Gravity to deploy an ERC20 representing this token on
/// Ethereum
/// Name: the token name
/// Symbol: the token symbol
/// Description: the token description, not sent to ETH at all, only used on Cosmos
/// Display: the token display name (only used on Cosmos to decide ERC20 Decimals)
/// Deicmals: the decimals for the display unit
/// ibc_denom is the denom of the token in question on this chain
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct IbcMetadataProposal {
    #[prost(string, tag="1")]
    pub title: ::prost::alloc::string::String,
    #[prost(string, tag="2")]
    pub description: ::prost::alloc::string::String,
    #[prost(message, optional, tag="3")]
    pub metadata: ::core::option::Option<cosmos_sdk_proto::cosmos::bank::v1beta1::Metadata>,
    #[prost(string, tag="4")]
    pub ibc_denom: ::prost::alloc::string::String,
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
/// This is a hex encoded 0x Ethereum public key that will be used by this validator
/// on Ethereum
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgSetOrchestratorAddress {
    #[prost(string, tag="1")]
    pub validator: ::prost::alloc::string::String,
    #[prost(string, tag="2")]
    pub orchestrator: ::prost::alloc::string::String,
    #[prost(string, tag="3")]
    pub eth_address: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgSetOrchestratorAddressResponse {
}
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
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgValsetConfirm {
    #[prost(uint64, tag="1")]
    pub nonce: u64,
    #[prost(string, tag="2")]
    pub orchestrator: ::prost::alloc::string::String,
    #[prost(string, tag="3")]
    pub eth_address: ::prost::alloc::string::String,
    #[prost(string, tag="4")]
    pub signature: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgValsetConfirmResponse {
}
/// MsgSendToEth
/// This is the message that a user calls when they want to bridge an asset
/// it will later be removed when it is included in a batch and successfully
/// submitted tokens are removed from the users balance immediately
/// -------------
/// AMOUNT:
/// the coin to send across the bridge, note the restriction that this is a
/// single coin not a set of coins that is normal in other Cosmos messages
/// FEE:
/// the fee paid for the bridge, distinct from the fee paid to the chain to
/// actually send this message in the first place. So a successful send has
/// two layers of fees for the user
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgSendToEth {
    #[prost(string, tag="1")]
    pub sender: ::prost::alloc::string::String,
    #[prost(string, tag="2")]
    pub eth_dest: ::prost::alloc::string::String,
    #[prost(message, optional, tag="3")]
    pub amount: ::core::option::Option<cosmos_sdk_proto::cosmos::base::v1beta1::Coin>,
    #[prost(message, optional, tag="4")]
    pub bridge_fee: ::core::option::Option<cosmos_sdk_proto::cosmos::base::v1beta1::Coin>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgSendToEthResponse {
}
/// MsgRequestBatch
/// this is a message anyone can send that requests a batch of transactions to
/// send across the bridge be created for whatever block height this message is
/// included in. This acts as a coordination point, the handler for this message
/// looks at the AddToOutgoingPool tx's in the store and generates a batch, also
/// available in the store tied to this message. The validators then grab this
/// batch, sign it, submit the signatures with a MsgConfirmBatch before a relayer
/// can finally submit the batch
/// -------------
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgRequestBatch {
    #[prost(string, tag="1")]
    pub sender: ::prost::alloc::string::String,
    #[prost(string, tag="2")]
    pub denom: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgRequestBatchResponse {
}
/// MsgConfirmBatch
/// When validators observe a MsgRequestBatch they form a batch by ordering
/// transactions currently in the txqueue in order of highest to lowest fee,
/// cutting off when the batch either reaches a hardcoded maximum size (to be
/// decided, probably around 100) or when transactions stop being profitable
/// (TODO determine this without nondeterminism) This message includes the batch
/// as well as an Ethereum signature over this batch by the validator
/// -------------
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgConfirmBatch {
    #[prost(uint64, tag="1")]
    pub nonce: u64,
    #[prost(string, tag="2")]
    pub token_contract: ::prost::alloc::string::String,
    #[prost(string, tag="3")]
    pub eth_signer: ::prost::alloc::string::String,
    #[prost(string, tag="4")]
    pub orchestrator: ::prost::alloc::string::String,
    #[prost(string, tag="5")]
    pub signature: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgConfirmBatchResponse {
}
/// MsgConfirmLogicCall
/// When validators observe a MsgRequestBatch they form a batch by ordering
/// transactions currently in the txqueue in order of highest to lowest fee,
/// cutting off when the batch either reaches a hardcoded maximum size (to be
/// decided, probably around 100) or when transactions stop being profitable
/// (TODO determine this without nondeterminism) This message includes the batch
/// as well as an Ethereum signature over this batch by the validator
/// -------------
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgConfirmLogicCall {
    #[prost(string, tag="1")]
    pub invalidation_id: ::prost::alloc::string::String,
    #[prost(uint64, tag="2")]
    pub invalidation_nonce: u64,
    #[prost(string, tag="3")]
    pub eth_signer: ::prost::alloc::string::String,
    #[prost(string, tag="4")]
    pub orchestrator: ::prost::alloc::string::String,
    #[prost(string, tag="5")]
    pub signature: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgConfirmLogicCallResponse {
}
/// MsgSendToCosmosClaim
/// When more than 66% of the active validator set has
/// claimed to have seen the deposit enter the ethereum blockchain coins are
/// issued to the Cosmos address in question
/// -------------
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgSendToCosmosClaim {
    #[prost(uint64, tag="1")]
    pub event_nonce: u64,
    #[prost(uint64, tag="2")]
    pub block_height: u64,
    #[prost(string, tag="3")]
    pub token_contract: ::prost::alloc::string::String,
    #[prost(string, tag="4")]
    pub amount: ::prost::alloc::string::String,
    #[prost(string, tag="5")]
    pub ethereum_sender: ::prost::alloc::string::String,
    #[prost(string, tag="6")]
    pub cosmos_receiver: ::prost::alloc::string::String,
    #[prost(string, tag="7")]
    pub orchestrator: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgSendToCosmosClaimResponse {
}
/// BatchSendToEthClaim claims that a batch of send to eth
/// operations on the bridge contract was executed.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgBatchSendToEthClaim {
    #[prost(uint64, tag="1")]
    pub event_nonce: u64,
    #[prost(uint64, tag="2")]
    pub block_height: u64,
    #[prost(uint64, tag="3")]
    pub batch_nonce: u64,
    #[prost(string, tag="4")]
    pub token_contract: ::prost::alloc::string::String,
    #[prost(string, tag="5")]
    pub orchestrator: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgBatchSendToEthClaimResponse {
}
/// ERC20DeployedClaim allows the Cosmos module
/// to learn about an ERC20 that someone deployed
/// to represent a Cosmos asset
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgErc20DeployedClaim {
    #[prost(uint64, tag="1")]
    pub event_nonce: u64,
    #[prost(uint64, tag="2")]
    pub block_height: u64,
    #[prost(string, tag="3")]
    pub cosmos_denom: ::prost::alloc::string::String,
    #[prost(string, tag="4")]
    pub token_contract: ::prost::alloc::string::String,
    #[prost(string, tag="5")]
    pub name: ::prost::alloc::string::String,
    #[prost(string, tag="6")]
    pub symbol: ::prost::alloc::string::String,
    #[prost(uint64, tag="7")]
    pub decimals: u64,
    #[prost(string, tag="8")]
    pub orchestrator: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgErc20DeployedClaimResponse {
}
/// This informs the Cosmos module that a logic
/// call has been executed
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgLogicCallExecutedClaim {
    #[prost(uint64, tag="1")]
    pub event_nonce: u64,
    #[prost(uint64, tag="2")]
    pub block_height: u64,
    #[prost(bytes="vec", tag="3")]
    pub invalidation_id: ::prost::alloc::vec::Vec<u8>,
    #[prost(uint64, tag="4")]
    pub invalidation_nonce: u64,
    #[prost(string, tag="5")]
    pub orchestrator: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgLogicCallExecutedClaimResponse {
}
/// This informs the Cosmos module that a validator
/// set has been updated.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgValsetUpdatedClaim {
    #[prost(uint64, tag="1")]
    pub event_nonce: u64,
    #[prost(uint64, tag="2")]
    pub valset_nonce: u64,
    #[prost(uint64, tag="3")]
    pub block_height: u64,
    #[prost(message, repeated, tag="4")]
    pub members: ::prost::alloc::vec::Vec<BridgeValidator>,
    #[prost(string, tag="5")]
    pub reward_amount: ::prost::alloc::string::String,
    #[prost(string, tag="6")]
    pub reward_token: ::prost::alloc::string::String,
    #[prost(string, tag="7")]
    pub orchestrator: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgValsetUpdatedClaimResponse {
}
/// This call allows the sender (and only the sender)
/// to cancel a given MsgSendToEth and recieve a refund
/// of the tokens
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgCancelSendToEth {
    #[prost(uint64, tag="1")]
    pub transaction_id: u64,
    #[prost(string, tag="2")]
    pub sender: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgCancelSendToEthResponse {
}
/// This call allows anyone to submit evidence that a
/// validator has signed a valset, batch, or logic call that never
/// existed on the Cosmos chain. 
/// Subject contains the batch, valset, or logic call.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgSubmitBadSignatureEvidence {
    #[prost(message, optional, tag="1")]
    pub subject: ::core::option::Option<::prost_types::Any>,
    #[prost(string, tag="2")]
    pub signature: ::prost::alloc::string::String,
    #[prost(string, tag="3")]
    pub sender: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MsgSubmitBadSignatureEvidenceResponse {
}
# [doc = r" Generated client implementations."] pub mod msg_client { # ! [allow (unused_variables , dead_code , missing_docs , clippy :: let_unit_value ,)] use tonic :: codegen :: * ; # [doc = " Msg defines the state transitions possible within gravity"] # [derive (Debug , Clone)] pub struct MsgClient < T > { inner : tonic :: client :: Grpc < T > , } impl MsgClient < tonic :: transport :: Channel > { # [doc = r" Attempt to create a new client by connecting to a given endpoint."] pub async fn connect < D > (dst : D) -> Result < Self , tonic :: transport :: Error > where D : std :: convert :: TryInto < tonic :: transport :: Endpoint > , D :: Error : Into < StdError > , { let conn = tonic :: transport :: Endpoint :: new (dst) ? . connect () . await ? ; Ok (Self :: new (conn)) } } impl < T > MsgClient < T > where T : tonic :: client :: GrpcService < tonic :: body :: BoxBody > , T :: ResponseBody : Body + Send + 'static , T :: Error : Into < StdError > , < T :: ResponseBody as Body > :: Error : Into < StdError > + Send , { pub fn new (inner : T) -> Self { let inner = tonic :: client :: Grpc :: new (inner) ; Self { inner } } pub fn with_interceptor < F > (inner : T , interceptor : F) -> MsgClient < InterceptedService < T , F >> where F : tonic :: service :: Interceptor , T : tonic :: codegen :: Service < http :: Request < tonic :: body :: BoxBody > , Response = http :: Response << T as tonic :: client :: GrpcService < tonic :: body :: BoxBody >> :: ResponseBody > > , < T as tonic :: codegen :: Service < http :: Request < tonic :: body :: BoxBody >> > :: Error : Into < StdError > + Send + Sync , { MsgClient :: new (InterceptedService :: new (inner , interceptor)) } # [doc = r" Compress requests with `gzip`."] # [doc = r""] # [doc = r" This requires the server to support it otherwise it might respond with an"] # [doc = r" error."] pub fn send_gzip (mut self) -> Self { self . inner = self . inner . send_gzip () ; self } # [doc = r" Enable decompressing responses with `gzip`."] pub fn accept_gzip (mut self) -> Self { self . inner = self . inner . accept_gzip () ; self } pub async fn valset_confirm (& mut self , request : impl tonic :: IntoRequest < super :: MsgValsetConfirm > ,) -> Result < tonic :: Response < super :: MsgValsetConfirmResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Msg/ValsetConfirm") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn send_to_eth (& mut self , request : impl tonic :: IntoRequest < super :: MsgSendToEth > ,) -> Result < tonic :: Response < super :: MsgSendToEthResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Msg/SendToEth") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn request_batch (& mut self , request : impl tonic :: IntoRequest < super :: MsgRequestBatch > ,) -> Result < tonic :: Response < super :: MsgRequestBatchResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Msg/RequestBatch") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn confirm_batch (& mut self , request : impl tonic :: IntoRequest < super :: MsgConfirmBatch > ,) -> Result < tonic :: Response < super :: MsgConfirmBatchResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Msg/ConfirmBatch") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn confirm_logic_call (& mut self , request : impl tonic :: IntoRequest < super :: MsgConfirmLogicCall > ,) -> Result < tonic :: Response < super :: MsgConfirmLogicCallResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Msg/ConfirmLogicCall") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn send_to_cosmos_claim (& mut self , request : impl tonic :: IntoRequest < super :: MsgSendToCosmosClaim > ,) -> Result < tonic :: Response < super :: MsgSendToCosmosClaimResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Msg/SendToCosmosClaim") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn batch_send_to_eth_claim (& mut self , request : impl tonic :: IntoRequest < super :: MsgBatchSendToEthClaim > ,) -> Result < tonic :: Response < super :: MsgBatchSendToEthClaimResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Msg/BatchSendToEthClaim") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn valset_update_claim (& mut self , request : impl tonic :: IntoRequest < super :: MsgValsetUpdatedClaim > ,) -> Result < tonic :: Response < super :: MsgValsetUpdatedClaimResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Msg/ValsetUpdateClaim") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn erc20_deployed_claim (& mut self , request : impl tonic :: IntoRequest < super :: MsgErc20DeployedClaim > ,) -> Result < tonic :: Response < super :: MsgErc20DeployedClaimResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Msg/ERC20DeployedClaim") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn logic_call_executed_claim (& mut self , request : impl tonic :: IntoRequest < super :: MsgLogicCallExecutedClaim > ,) -> Result < tonic :: Response < super :: MsgLogicCallExecutedClaimResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Msg/LogicCallExecutedClaim") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn set_orchestrator_address (& mut self , request : impl tonic :: IntoRequest < super :: MsgSetOrchestratorAddress > ,) -> Result < tonic :: Response < super :: MsgSetOrchestratorAddressResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Msg/SetOrchestratorAddress") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn cancel_send_to_eth (& mut self , request : impl tonic :: IntoRequest < super :: MsgCancelSendToEth > ,) -> Result < tonic :: Response < super :: MsgCancelSendToEthResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Msg/CancelSendToEth") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn submit_bad_signature_evidence (& mut self , request : impl tonic :: IntoRequest < super :: MsgSubmitBadSignatureEvidence > ,) -> Result < tonic :: Response < super :: MsgSubmitBadSignatureEvidenceResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Msg/SubmitBadSignatureEvidence") ; self . inner . unary (request . into_request () , path , codec) . await } } }// Params represent the Gravity genesis and store parameters
// gravity_id:
// a random 32 byte value to prevent signature reuse, for example if the
// cosmos validators decided to use the same Ethereum keys for another chain
// also running Gravity we would not want it to be possible to play a deposit
// from chain A back on chain B's Gravity. This value IS USED ON ETHEREUM so
// it must be set in your genesis.json before launch and not changed after
// deploying Gravity

// contract_hash:
// the code hash of a known good version of the Gravity contract
// solidity code. This can be used to verify the correct version
// of the contract has been deployed. This is a reference value for
// goernance action only it is never read by any Gravity code

// bridge_ethereum_address:
// is address of the bridge contract on the Ethereum side, this is a
// reference value for governance only and is not actually used by any
// Gravity code

// bridge_chain_id:
// the unique identifier of the Ethereum chain, this is a reference value
// only and is not actually used by any Gravity code

// These reference values may be used by future Gravity client implemetnations
// to allow for saftey features or convenience features like the Gravity address
// in your relayer. A relayer would require a configured Gravity address if
// governance had not set the address on the chain it was relaying for.

// signed_valsets_window
// signed_batches_window
// signed_logiccall_window
// signed_claims_window

// These values represent the time in blocks that a validator has to submit
// a signature for a batch or valset, or to submit a claim for a particular
// attestation nonce. In the case of attestations this clock starts when the
// attestation is created, but only allows for slashing once the event has passed

// target_batch_timeout:

// This is the 'target' value for when batches time out, this is a target becuase
// Ethereum is a probabalistic chain and you can't say for sure what the block
// frequency is ahead of time.

// average_block_time
// average_ethereum_block_time

// These values are the average Cosmos block time and Ethereum block time repsectively
// and they are used to compute what the target batch timeout is. It is important that
// governance updates these in case of any major, prolonged change in the time it takes
// to produce a block

// slash_fraction_valset
// slash_fraction_batch
// slash_fraction_claim
// slash_fraction_conflicting_claim

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
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Params {
    #[prost(string, tag="1")]
    pub gravity_id: ::prost::alloc::string::String,
    #[prost(string, tag="2")]
    pub contract_source_hash: ::prost::alloc::string::String,
    #[prost(string, tag="4")]
    pub bridge_ethereum_address: ::prost::alloc::string::String,
    #[prost(uint64, tag="5")]
    pub bridge_chain_id: u64,
    #[prost(uint64, tag="6")]
    pub signed_valsets_window: u64,
    #[prost(uint64, tag="7")]
    pub signed_batches_window: u64,
    #[prost(uint64, tag="8")]
    pub signed_logic_calls_window: u64,
    #[prost(uint64, tag="9")]
    pub target_batch_timeout: u64,
    #[prost(uint64, tag="10")]
    pub average_block_time: u64,
    #[prost(uint64, tag="11")]
    pub average_ethereum_block_time: u64,
    #[prost(bytes="vec", tag="12")]
    pub slash_fraction_valset: ::prost::alloc::vec::Vec<u8>,
    #[prost(bytes="vec", tag="13")]
    pub slash_fraction_batch: ::prost::alloc::vec::Vec<u8>,
    #[prost(bytes="vec", tag="14")]
    pub slash_fraction_logic_call: ::prost::alloc::vec::Vec<u8>,
    #[prost(uint64, tag="15")]
    pub unbond_slashing_valsets_window: u64,
    #[prost(bytes="vec", tag="16")]
    pub slash_fraction_bad_eth_signature: ::prost::alloc::vec::Vec<u8>,
    #[prost(message, optional, tag="17")]
    pub valset_reward: ::core::option::Option<cosmos_sdk_proto::cosmos::base::v1beta1::Coin>,
    #[prost(bool, tag="18")]
    pub bridge_active: bool,
    /// addresses on this blacklist are forbidden from depositing or withdrawing
    /// from Ethereum to the bridge
    #[prost(string, repeated, tag="19")]
    pub ethereum_blacklist: ::prost::alloc::vec::Vec<::prost::alloc::string::String>,
}
/// GenesisState struct, containing all persistant data required by the Gravity module
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct GenesisState {
    #[prost(message, optional, tag="1")]
    pub params: ::core::option::Option<Params>,
    #[prost(message, optional, tag="2")]
    pub gravity_nonces: ::core::option::Option<GravityNonces>,
    #[prost(message, repeated, tag="3")]
    pub valsets: ::prost::alloc::vec::Vec<Valset>,
    #[prost(message, repeated, tag="4")]
    pub valset_confirms: ::prost::alloc::vec::Vec<MsgValsetConfirm>,
    #[prost(message, repeated, tag="5")]
    pub batches: ::prost::alloc::vec::Vec<OutgoingTxBatch>,
    #[prost(message, repeated, tag="6")]
    pub batch_confirms: ::prost::alloc::vec::Vec<MsgConfirmBatch>,
    #[prost(message, repeated, tag="7")]
    pub logic_calls: ::prost::alloc::vec::Vec<OutgoingLogicCall>,
    #[prost(message, repeated, tag="8")]
    pub logic_call_confirms: ::prost::alloc::vec::Vec<MsgConfirmLogicCall>,
    #[prost(message, repeated, tag="9")]
    pub attestations: ::prost::alloc::vec::Vec<Attestation>,
    #[prost(message, repeated, tag="10")]
    pub delegate_keys: ::prost::alloc::vec::Vec<MsgSetOrchestratorAddress>,
    #[prost(message, repeated, tag="11")]
    pub erc20_to_denoms: ::prost::alloc::vec::Vec<Erc20ToDenom>,
    #[prost(message, repeated, tag="12")]
    pub unbatched_transfers: ::prost::alloc::vec::Vec<OutgoingTransferTx>,
}
/// GravityCounters contains the many noces and counters required to maintain the bridge state in the genesis
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct GravityNonces {
    /// the nonce of the last generated validator set
    #[prost(uint64, tag="1")]
    pub latest_valset_nonce: u64,
    /// the last observed Gravity.sol contract event nonce
    #[prost(uint64, tag="2")]
    pub last_observed_nonce: u64,
    /// the last valset nonce we have slashed, to prevent double slashing
    #[prost(uint64, tag="3")]
    pub last_slashed_valset_nonce: u64,
    /// the last batch Cosmos chain block that batch slashing has completed for
    /// there is an individual batch nonce for each token type so this removes
    /// the need to store them all
    #[prost(uint64, tag="4")]
    pub last_slashed_batch_block: u64,
    /// the last cosmos block that logic call slashing has completed for
    #[prost(uint64, tag="5")]
    pub last_slashed_logic_call_block: u64,
    /// the last transaction id from the Gravity TX pool, this prevents ID
    /// duplication during chain upgrades
    #[prost(uint64, tag="6")]
    pub last_tx_pool_id: u64,
    /// the last batch id from the Gravity batch pool, this prevents ID duplication
    /// during chain upgrades
    #[prost(uint64, tag="7")]
    pub last_batch_id: u64,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryParamsRequest {
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryParamsResponse {
    #[prost(message, optional, tag="1")]
    pub params: ::core::option::Option<Params>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryCurrentValsetRequest {
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryCurrentValsetResponse {
    #[prost(message, optional, tag="1")]
    pub valset: ::core::option::Option<Valset>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryValsetRequestRequest {
    #[prost(uint64, tag="1")]
    pub nonce: u64,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryValsetRequestResponse {
    #[prost(message, optional, tag="1")]
    pub valset: ::core::option::Option<Valset>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryValsetConfirmRequest {
    #[prost(uint64, tag="1")]
    pub nonce: u64,
    #[prost(string, tag="2")]
    pub address: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryValsetConfirmResponse {
    #[prost(message, optional, tag="1")]
    pub confirm: ::core::option::Option<MsgValsetConfirm>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryValsetConfirmsByNonceRequest {
    #[prost(uint64, tag="1")]
    pub nonce: u64,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryValsetConfirmsByNonceResponse {
    #[prost(message, repeated, tag="1")]
    pub confirms: ::prost::alloc::vec::Vec<MsgValsetConfirm>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastValsetRequestsRequest {
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastValsetRequestsResponse {
    #[prost(message, repeated, tag="1")]
    pub valsets: ::prost::alloc::vec::Vec<Valset>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastPendingValsetRequestByAddrRequest {
    #[prost(string, tag="1")]
    pub address: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastPendingValsetRequestByAddrResponse {
    #[prost(message, repeated, tag="1")]
    pub valsets: ::prost::alloc::vec::Vec<Valset>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryBatchFeeRequest {
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryBatchFeeResponse {
    #[prost(message, repeated, tag="1")]
    pub batch_fees: ::prost::alloc::vec::Vec<BatchFees>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastPendingBatchRequestByAddrRequest {
    #[prost(string, tag="1")]
    pub address: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastPendingBatchRequestByAddrResponse {
    #[prost(message, repeated, tag="1")]
    pub batch: ::prost::alloc::vec::Vec<OutgoingTxBatch>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastPendingLogicCallByAddrRequest {
    #[prost(string, tag="1")]
    pub address: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastPendingLogicCallByAddrResponse {
    #[prost(message, repeated, tag="1")]
    pub call: ::prost::alloc::vec::Vec<OutgoingLogicCall>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryOutgoingTxBatchesRequest {
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryOutgoingTxBatchesResponse {
    #[prost(message, repeated, tag="1")]
    pub batches: ::prost::alloc::vec::Vec<OutgoingTxBatch>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryOutgoingLogicCallsRequest {
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryOutgoingLogicCallsResponse {
    #[prost(message, repeated, tag="1")]
    pub calls: ::prost::alloc::vec::Vec<OutgoingLogicCall>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryBatchRequestByNonceRequest {
    #[prost(uint64, tag="1")]
    pub nonce: u64,
    #[prost(string, tag="2")]
    pub contract_address: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryBatchRequestByNonceResponse {
    #[prost(message, optional, tag="1")]
    pub batch: ::core::option::Option<OutgoingTxBatch>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryBatchConfirmsRequest {
    #[prost(uint64, tag="1")]
    pub nonce: u64,
    #[prost(string, tag="2")]
    pub contract_address: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryBatchConfirmsResponse {
    #[prost(message, repeated, tag="1")]
    pub confirms: ::prost::alloc::vec::Vec<MsgConfirmBatch>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLogicConfirmsRequest {
    #[prost(bytes="vec", tag="1")]
    pub invalidation_id: ::prost::alloc::vec::Vec<u8>,
    #[prost(uint64, tag="2")]
    pub invalidation_nonce: u64,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLogicConfirmsResponse {
    #[prost(message, repeated, tag="1")]
    pub confirms: ::prost::alloc::vec::Vec<MsgConfirmLogicCall>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastEventNonceByAddrRequest {
    #[prost(string, tag="1")]
    pub address: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryLastEventNonceByAddrResponse {
    #[prost(uint64, tag="1")]
    pub event_nonce: u64,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryErc20ToDenomRequest {
    #[prost(string, tag="1")]
    pub erc20: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryErc20ToDenomResponse {
    #[prost(string, tag="1")]
    pub denom: ::prost::alloc::string::String,
    #[prost(bool, tag="2")]
    pub cosmos_originated: bool,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryDenomToErc20Request {
    #[prost(string, tag="1")]
    pub denom: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryDenomToErc20Response {
    #[prost(string, tag="1")]
    pub erc20: ::prost::alloc::string::String,
    #[prost(bool, tag="2")]
    pub cosmos_originated: bool,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryAttestationsRequest {
    #[prost(uint64, tag="1")]
    pub limit: u64,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryAttestationsResponse {
    #[prost(message, repeated, tag="1")]
    pub attestations: ::prost::alloc::vec::Vec<Attestation>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryDelegateKeysByValidatorAddress {
    #[prost(string, tag="1")]
    pub validator_address: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryDelegateKeysByValidatorAddressResponse {
    #[prost(string, tag="1")]
    pub eth_address: ::prost::alloc::string::String,
    #[prost(string, tag="2")]
    pub orchestrator_address: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryDelegateKeysByEthAddress {
    #[prost(string, tag="1")]
    pub eth_address: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryDelegateKeysByEthAddressResponse {
    #[prost(string, tag="1")]
    pub validator_address: ::prost::alloc::string::String,
    #[prost(string, tag="2")]
    pub orchestrator_address: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryDelegateKeysByOrchestratorAddress {
    #[prost(string, tag="1")]
    pub orchestrator_address: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryDelegateKeysByOrchestratorAddressResponse {
    #[prost(string, tag="1")]
    pub validator_address: ::prost::alloc::string::String,
    #[prost(string, tag="2")]
    pub eth_address: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryPendingSendToEth {
    #[prost(string, tag="1")]
    pub sender_address: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryPendingSendToEthResponse {
    #[prost(message, repeated, tag="1")]
    pub transfers_in_batches: ::prost::alloc::vec::Vec<OutgoingTransferTx>,
    #[prost(message, repeated, tag="2")]
    pub unbatched_transfers: ::prost::alloc::vec::Vec<OutgoingTransferTx>,
}
# [doc = r" Generated client implementations."] pub mod query_client { # ! [allow (unused_variables , dead_code , missing_docs , clippy :: let_unit_value ,)] use tonic :: codegen :: * ; # [doc = " Query defines the gRPC querier service"] # [derive (Debug , Clone)] pub struct QueryClient < T > { inner : tonic :: client :: Grpc < T > , } impl QueryClient < tonic :: transport :: Channel > { # [doc = r" Attempt to create a new client by connecting to a given endpoint."] pub async fn connect < D > (dst : D) -> Result < Self , tonic :: transport :: Error > where D : std :: convert :: TryInto < tonic :: transport :: Endpoint > , D :: Error : Into < StdError > , { let conn = tonic :: transport :: Endpoint :: new (dst) ? . connect () . await ? ; Ok (Self :: new (conn)) } } impl < T > QueryClient < T > where T : tonic :: client :: GrpcService < tonic :: body :: BoxBody > , T :: ResponseBody : Body + Send + 'static , T :: Error : Into < StdError > , < T :: ResponseBody as Body > :: Error : Into < StdError > + Send , { pub fn new (inner : T) -> Self { let inner = tonic :: client :: Grpc :: new (inner) ; Self { inner } } pub fn with_interceptor < F > (inner : T , interceptor : F) -> QueryClient < InterceptedService < T , F >> where F : tonic :: service :: Interceptor , T : tonic :: codegen :: Service < http :: Request < tonic :: body :: BoxBody > , Response = http :: Response << T as tonic :: client :: GrpcService < tonic :: body :: BoxBody >> :: ResponseBody > > , < T as tonic :: codegen :: Service < http :: Request < tonic :: body :: BoxBody >> > :: Error : Into < StdError > + Send + Sync , { QueryClient :: new (InterceptedService :: new (inner , interceptor)) } # [doc = r" Compress requests with `gzip`."] # [doc = r""] # [doc = r" This requires the server to support it otherwise it might respond with an"] # [doc = r" error."] pub fn send_gzip (mut self) -> Self { self . inner = self . inner . send_gzip () ; self } # [doc = r" Enable decompressing responses with `gzip`."] pub fn accept_gzip (mut self) -> Self { self . inner = self . inner . accept_gzip () ; self } # [doc = " Deployments queries deployments"] pub async fn params (& mut self , request : impl tonic :: IntoRequest < super :: QueryParamsRequest > ,) -> Result < tonic :: Response < super :: QueryParamsResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/Params") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn current_valset (& mut self , request : impl tonic :: IntoRequest < super :: QueryCurrentValsetRequest > ,) -> Result < tonic :: Response < super :: QueryCurrentValsetResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/CurrentValset") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn valset_request (& mut self , request : impl tonic :: IntoRequest < super :: QueryValsetRequestRequest > ,) -> Result < tonic :: Response < super :: QueryValsetRequestResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/ValsetRequest") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn valset_confirm (& mut self , request : impl tonic :: IntoRequest < super :: QueryValsetConfirmRequest > ,) -> Result < tonic :: Response < super :: QueryValsetConfirmResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/ValsetConfirm") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn valset_confirms_by_nonce (& mut self , request : impl tonic :: IntoRequest < super :: QueryValsetConfirmsByNonceRequest > ,) -> Result < tonic :: Response < super :: QueryValsetConfirmsByNonceResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/ValsetConfirmsByNonce") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn last_valset_requests (& mut self , request : impl tonic :: IntoRequest < super :: QueryLastValsetRequestsRequest > ,) -> Result < tonic :: Response < super :: QueryLastValsetRequestsResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/LastValsetRequests") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn last_pending_valset_request_by_addr (& mut self , request : impl tonic :: IntoRequest < super :: QueryLastPendingValsetRequestByAddrRequest > ,) -> Result < tonic :: Response < super :: QueryLastPendingValsetRequestByAddrResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/LastPendingValsetRequestByAddr") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn last_pending_batch_request_by_addr (& mut self , request : impl tonic :: IntoRequest < super :: QueryLastPendingBatchRequestByAddrRequest > ,) -> Result < tonic :: Response < super :: QueryLastPendingBatchRequestByAddrResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/LastPendingBatchRequestByAddr") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn last_pending_logic_call_by_addr (& mut self , request : impl tonic :: IntoRequest < super :: QueryLastPendingLogicCallByAddrRequest > ,) -> Result < tonic :: Response < super :: QueryLastPendingLogicCallByAddrResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/LastPendingLogicCallByAddr") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn last_event_nonce_by_addr (& mut self , request : impl tonic :: IntoRequest < super :: QueryLastEventNonceByAddrRequest > ,) -> Result < tonic :: Response < super :: QueryLastEventNonceByAddrResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/LastEventNonceByAddr") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn batch_fees (& mut self , request : impl tonic :: IntoRequest < super :: QueryBatchFeeRequest > ,) -> Result < tonic :: Response < super :: QueryBatchFeeResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/BatchFees") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn outgoing_tx_batches (& mut self , request : impl tonic :: IntoRequest < super :: QueryOutgoingTxBatchesRequest > ,) -> Result < tonic :: Response < super :: QueryOutgoingTxBatchesResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/OutgoingTxBatches") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn outgoing_logic_calls (& mut self , request : impl tonic :: IntoRequest < super :: QueryOutgoingLogicCallsRequest > ,) -> Result < tonic :: Response < super :: QueryOutgoingLogicCallsResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/OutgoingLogicCalls") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn batch_request_by_nonce (& mut self , request : impl tonic :: IntoRequest < super :: QueryBatchRequestByNonceRequest > ,) -> Result < tonic :: Response < super :: QueryBatchRequestByNonceResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/BatchRequestByNonce") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn batch_confirms (& mut self , request : impl tonic :: IntoRequest < super :: QueryBatchConfirmsRequest > ,) -> Result < tonic :: Response < super :: QueryBatchConfirmsResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/BatchConfirms") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn logic_confirms (& mut self , request : impl tonic :: IntoRequest < super :: QueryLogicConfirmsRequest > ,) -> Result < tonic :: Response < super :: QueryLogicConfirmsResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/LogicConfirms") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn erc20_to_denom (& mut self , request : impl tonic :: IntoRequest < super :: QueryErc20ToDenomRequest > ,) -> Result < tonic :: Response < super :: QueryErc20ToDenomResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/ERC20ToDenom") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn denom_to_erc20 (& mut self , request : impl tonic :: IntoRequest < super :: QueryDenomToErc20Request > ,) -> Result < tonic :: Response < super :: QueryDenomToErc20Response > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/DenomToERC20") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn get_attestations (& mut self , request : impl tonic :: IntoRequest < super :: QueryAttestationsRequest > ,) -> Result < tonic :: Response < super :: QueryAttestationsResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/GetAttestations") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn get_delegate_key_by_validator (& mut self , request : impl tonic :: IntoRequest < super :: QueryDelegateKeysByValidatorAddress > ,) -> Result < tonic :: Response < super :: QueryDelegateKeysByValidatorAddressResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/GetDelegateKeyByValidator") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn get_delegate_key_by_eth (& mut self , request : impl tonic :: IntoRequest < super :: QueryDelegateKeysByEthAddress > ,) -> Result < tonic :: Response < super :: QueryDelegateKeysByEthAddressResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/GetDelegateKeyByEth") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn get_delegate_key_by_orchestrator (& mut self , request : impl tonic :: IntoRequest < super :: QueryDelegateKeysByOrchestratorAddress > ,) -> Result < tonic :: Response < super :: QueryDelegateKeysByOrchestratorAddressResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/GetDelegateKeyByOrchestrator") ; self . inner . unary (request . into_request () , path , codec) . await } pub async fn get_pending_send_to_eth (& mut self , request : impl tonic :: IntoRequest < super :: QueryPendingSendToEth > ,) -> Result < tonic :: Response < super :: QueryPendingSendToEthResponse > , tonic :: Status > { self . inner . ready () . await . map_err (| e | { tonic :: Status :: new (tonic :: Code :: Unknown , format ! ("Service was not ready: {}" , e . into ())) }) ? ; let codec = tonic :: codec :: ProstCodec :: default () ; let path = http :: uri :: PathAndQuery :: from_static ("/gravity.v1.Query/GetPendingSendToEth") ; self . inner . unary (request . into_request () , path , codec) . await } } }