//! Command line argument definitions for Gravity bridge tools
//! See the clap documentation for how exactly this works, note that doc comments are displayed to the user

use clap::Parser;
use clarity::Address as EthAddress;
use clarity::PrivateKey as EthPrivateKey;
use deep_space::{address::Address as CosmosAddress, Coin};
use deep_space::{CosmosPrivateKey, EthermintPrivateKey};
use std::path::PathBuf;

/// Gravity Bridge tools (gbt) provides tools for interacting with the Althea Gravity bridge for Cosmos based blockchains.
#[derive(Parser)]
#[clap(version = env!("CARGO_PKG_VERSION"), author = "Justin Kilpatrick <justin@althea.net>")]
pub struct Opts {
    /// Increase the logging verbosity
    #[clap(short, long)]
    pub verbose: bool,
    /// Decrease the logging verbosity
    #[clap(short, long)]
    pub quiet: bool,
    /// The home directory for Gravity Bridge Tools, by default
    /// $HOME/.althea_gbt/
    #[clap(short, long, parse(from_str))]
    pub home: Option<PathBuf>,
    /// Set the address prefix for the Cosmos chain
    #[clap(short, long, default_value = "gravity")]
    pub address_prefix: String,
    #[clap(subcommand)]
    pub subcmd: SubCommand,
}

#[derive(Parser)]
pub enum SubCommand {
    Orchestrator(OrchestratorOpts),
    Relayer(RelayerOpts),
    JsonrpcServer(JsonrpcServerOpts),
    Client(ClientOpts),
    Gov(GovOpts),
    Keys(KeyOpts),
    Init(InitOpts),
}

const DEFAULT_GRPC_ADDRESS: &str = "https://gravitychain.io:9090";
const DEFAULT_ETH_RPC_ADDRESS: &str = "https://eth.althea.net";

/// The Gravity Bridge orchestrator is required for all validators of the Cosmos chain running
/// the Gravity Bridge module. It contains an Ethereum Signer, Oracle, and optional relayer
#[derive(Parser)]
pub struct OrchestratorOpts {
    /// Cosmos mnemonic phrase containing the tokens you would like to send
    #[clap(short, long, parse(try_from_str))]
    pub cosmos_phrase: Option<CosmosPrivateKey>,
    /// An Ethereum private key containing ETH to pay for fees, this will also hold the relayers earnings
    /// in the near future it will be possible to disable the Orchestrators integrated relayer
    #[clap(short, long, parse(try_from_str))]
    pub ethereum_key: Option<EthPrivateKey>,
    /// (Optional) The Cosmos gRPC server that will be used
    #[clap(long, default_value = "http://localhost:9090")]
    pub cosmos_grpc: String,
    /// (Optional) The Ethereum RPC server that will be used
    #[clap(long, default_value = "http://localhost:8545")]
    pub ethereum_rpc: String,
    /// The Cosmos Denom and amount to pay Cosmos chain fees
    #[clap(short, long, parse(try_from_str))]
    pub fees: Coin,
    /// The address fo the Gravity contract on Ethereum
    #[clap(short, long, parse(try_from_str))]
    pub gravity_contract_address: Option<EthAddress>,
}

/// The Gravity Bridge Relayer is an unpermissioned role that takes data from the Cosmos blockchain
/// packages it into Ethereum transactions and is paid to submit these transactions to the Ethereum blockchain
/// The relayer will attempt to only relay profitable transactions, but there is no guarantee that it will succeed
#[derive(Parser)]
pub struct RelayerOpts {
    /// An Ethereum private key containing ETH to pay for fees, this will also hold the relayers earnings
    /// This overrides the key set in the config, which will be used if no key is provided here
    #[clap(short, long, parse(try_from_str))]
    pub ethereum_key: Option<EthPrivateKey>,
    /// Cosmos mnemonic phrase containing tokens used to pay fees on Cosmos for requesting batches
    /// This overrides the key set in the config, which will be used if no key is provided here.
    /// If no key is provided and no key is set in the config, this relayer will not request batches
    #[clap(long, parse(try_from_str))]
    pub cosmos_phrase: Option<CosmosPrivateKey>,
    /// (Optional) The Cosmos Denom and amount to pay Cosmos chain fees. If not set this relayer will not automatically
    /// request batches
    #[clap(short, long, parse(try_from_str))]
    pub fees: Option<Coin>,
    /// The address fo the Gravity contract on Ethereum
    #[clap(short, long, parse(try_from_str))]
    pub gravity_contract_address: Option<EthAddress>,
    /// (Optional) The Ethereum RPC server that will be used
    #[clap(long, default_value = "http://localhost:8545")]
    pub ethereum_rpc: String,
    /// (Optional) The Cosmos gRPC server that will be used to
    #[clap(short, long, default_value = "http://localhost:9090")]
    pub cosmos_grpc: String,
}

/// The Gravity Bridge Jsonrpc Server is an HTTP Server that roughly mimics the results of an Ethereum-based blockchain
/// so that MetaMask will allow signatures over Gravity Bridge transactions (using EIP-712 signatures)
/// The information returned by this server may be completely inaccurate, and results in the MetaMask user interface
/// should be ignored
#[derive(Parser)]
pub struct JsonrpcServerOpts {
    /// The domain name under which the server will operate; default "localhost"
    #[clap(long, parse(try_from_str))]
    pub domain: Option<String>,

    /// The port number from which the server will serve; default "8545"
    #[clap(long, parse(try_from_str))]
    pub port: Option<String>,

    /// Whether or not to enable HTTPS via TLS, uses cert_chain_path and cert_key_path or their default values; default false
    #[clap(long, parse(try_from_str))]
    pub use_ssl: Option<bool>,

    /// A path to the SSL certificate, only used if use_ssl is true; default /etc/letsencrypt/live/<domain name>/fullchain.pem
    #[clap(long, parse(try_from_str))]
    pub cert_chain_path: Option<String>,

    /// A path to the SSL key file, only used if use_ssl is true; default /etc/letsencrypt/live/<domain name>/privkey.pem
    #[clap(long, parse(try_from_str))]
    pub cert_key_path: Option<String>,
}

/// The Gravity Bridge client contains helpful command line tools for interacting with the Gravity bridge
#[derive(Parser)]
pub struct ClientOpts {
    #[clap(subcommand)]
    pub subcmd: ClientSubcommand,
}

#[derive(Parser)]
pub enum ClientSubcommand {
    CosmosToEth(CosmosToEthOpts),
    EthToCosmos(EthToCosmosOpts),
    DeployErc20Representation(DeployErc20RepresentationOpts),
    SpotRelay(SpotRelayOpts),
    RequestAllBatches(RequestAllBatchesOpts),
}

/// Send Cosmos tokens to Ethereum
#[derive(Parser)]
pub struct CosmosToEthOpts {
    /// Cosmos mnemonic phrase containing the tokens you would like to send
    #[clap(short, long, parse(try_from_str))]
    pub cosmos_phrase: CosmosPrivateKey,
    /// (Optional) The Cosmos gRPC server that will be used to submit the transaction
    #[clap(long, default_value = DEFAULT_GRPC_ADDRESS)]
    pub cosmos_grpc: String,
    /// The Denom and amount you wish to send eg: 100ugraviton
    #[clap(short, long, parse(try_from_str))]
    pub amount: Coin,
    /// The Cosmos Denom and amount to pay Cosmos chain fees eg: 1ugraviton
    #[clap(short, long, parse(try_from_str))]
    pub fee: Coin,
    /// The amount you want to pay in bridge fees, this is used to pay relayers
    /// on Ethereum and must be of the same denomination as `amount` dependent on governance.
    /// Please ask in the official discord for more information.
    #[clap(short, long, parse(try_from_str))]
    pub bridge_fee: Coin,
    /// The amount you want to pay as a chain fee, this is used to pay Gravity Bridge
    /// stakers and must be at least a certain percentage of `amount`
    #[clap(short, long, parse(try_from_str))]
    pub chain_fee: Coin,
    /// The destination address on the Ethereum chain
    #[clap(short, long, parse(try_from_str))]
    pub eth_destination: EthAddress,
}

/// Send an Ethereum ERC20 token to Cosmos
#[derive(Parser)]
pub struct EthToCosmosOpts {
    /// The Ethereum private key to use for sending tokens
    #[clap(long, parse(try_from_str))]
    pub ethereum_key: EthPrivateKey,
    /// (Optional) The Ethereum RPC server that will be used to submit the transaction
    #[clap(long, default_value = DEFAULT_ETH_RPC_ADDRESS)]
    pub ethereum_rpc: String,
    /// The address fo the Gravity contract on Ethereum
    #[clap(short, long, parse(try_from_str))]
    pub gravity_contract_address: EthAddress,
    /// The ERC20 contract address of the ERC20 you are sending
    #[clap(short, long, parse(try_from_str))]
    pub token_contract_address: EthAddress,
    /// The amount of tokens you are sending eg. 1.2
    #[clap(short, long, parse(try_from_str))]
    pub amount: f64,
    /// The destination address on the Cosmos blockchain
    #[clap(short, long, parse(try_from_str))]
    pub destination: CosmosAddress,
}

/// Deploy an ERC20 representation of a Cosmos asset on the Ethereum chain
/// this can only be run once for each time of Cosmos asset
#[derive(Parser)]
pub struct DeployErc20RepresentationOpts {
    /// (Optional) The Cosmos gRPC server that will be used to submit the transaction
    #[clap(long, default_value = DEFAULT_GRPC_ADDRESS)]
    pub cosmos_grpc: String,
    /// (Optional) The Ethereum RPC server that will be used to submit the transaction
    #[clap(long, default_value = DEFAULT_ETH_RPC_ADDRESS)]
    pub ethereum_rpc: String,
    /// The Cosmos Denom you wish to create an ERC20 representation for
    #[clap(short, long)]
    pub cosmos_denom: String,
    /// An Ethereum private key, containing enough ETH to pay for the transaction
    #[clap(short, long, parse(try_from_str))]
    pub ethereum_key: EthPrivateKey,
    /// The address fo the Gravity contract on Ethereum
    #[clap(short, long, parse(try_from_str))]
    pub gravity_contract_address: Option<EthAddress>,
}

/// Requests and relays a batch of a specific token type.
/// This can be used to easily relay a batch without any special configuration as a one off operation.
/// WARNING: This command will relay a batch, you will recieve the fees attached to the batch but the
/// command itself does not check that the batch is profitable. Check on https://info.gravitychain.io
/// to view pending batchs and figure out what their fees are worth.
#[derive(Parser)]
pub struct SpotRelayOpts {
    /// (Optional) The Cosmos gRPC server that will be used to submit the transaction
    #[clap(long, default_value = DEFAULT_GRPC_ADDRESS)]
    pub cosmos_grpc: String,
    /// (Optional) The Ethereum RPC server that will be used to submit the transaction
    #[clap(long, default_value = DEFAULT_ETH_RPC_ADDRESS)]
    pub ethereum_rpc: String,
    /// The token or denom you wish to relay, can be a ERC20 address or ibc token address if Cosmos originated
    /// Not all tokens are built into the human readable lookup list
    /// Examples: Nym, DAI, 0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48, ibc/E341F178AB30AC89CF18B9559D90EF830419B5A4B50945EF800FD68DE840A91E
    #[clap(short, long)]
    pub token: String,
    /// An Ethereum private key, containing enough ETH to pay for the transaction
    #[clap(short, long, parse(try_from_str))]
    pub ethereum_key: EthPrivateKey,
    /// (Optional) The address fo the Gravity contract on Ethereum, this should be auto filled
    /// from chain parameters
    #[clap(short, long, parse(try_from_str))]
    pub gravity_contract_address: Option<EthAddress>,
    /// (Optional) Cosmos mnemonic phrase used for requesting batches if they are not already pending
    #[clap(short, long, parse(try_from_str))]
    pub cosmos_phrase: Option<CosmosPrivateKey>,
    /// The Cosmos Denom and amount to pay Cosmos chain fees, if blank no fee will be paid
    #[clap(short, long, parse(try_from_str))]
    pub fees: Option<Coin>,
}

/// Requests all possible batches for all token types. Useful to deal with relayers that will
/// only relay a batch if it is profitable and already requested.
#[derive(Parser)]
pub struct RequestAllBatchesOpts {
    /// (Optional) The Cosmos gRPC server that will be used to submit the transaction
    #[clap(long, default_value = DEFAULT_GRPC_ADDRESS)]
    pub cosmos_grpc: String,
    /// (Optional) The address fo the Gravity contract on Ethereum, this should be auto filled
    /// from chain parameters
    #[clap(short, long, parse(try_from_str))]
    pub gravity_contract_address: Option<EthAddress>,
    /// (Optional) Cosmos mnemonic phrase used for requesting batches if they are not already pending
    #[clap(short, long, parse(try_from_str))]
    pub cosmos_phrase: Option<CosmosPrivateKey>,
    /// The Cosmos Denom and amount to pay Cosmos chain fees, if blank no fee will be paid
    #[clap(short, long, parse(try_from_str))]
    pub fees: Option<Coin>,
}

/// Manage keys
#[derive(Parser)]
pub struct KeyOpts {
    #[clap(subcommand)]
    pub subcmd: KeysSubcommand,
}

#[allow(clippy::large_enum_variant)]
#[derive(Parser)]
pub enum KeysSubcommand {
    RegisterOrchestratorAddress(RegisterOrchestratorAddressOpts),
    SetEthereumKey(SetEthereumKeyOpts),
    SetOrchestratorKey(SetOrchestratorKeyOpts),
    Show,
    RecoverFunds(RecoverFundsOpts),
}

/// Register delegate keys for the Gravity Orchestrator.
/// this is a mandatory part of setting up a Gravity Orchestrator
/// If you would like sign using a ledger see `cosmos tx gravity set-orchestrator-address` instead
#[derive(Parser)]
pub struct RegisterOrchestratorAddressOpts {
    /// The Cosmos private key of the validator
    #[clap(short, long, parse(try_from_str))]
    pub validator_phrase: CosmosPrivateKey,
    /// (Optional) The Ethereum private key to register, will be generated if not provided
    #[clap(short, long, parse(try_from_str))]
    pub ethereum_key: Option<EthPrivateKey>,
    /// (Optional) The phrase for the Cosmos key to register, will be generated if not provided.
    #[clap(short, long, parse(try_from_str))]
    pub cosmos_phrase: Option<String>,
    /// (Optional) The Cosmos gRPC server that will be used to submit the transaction
    #[clap(long, default_value = "http://localhost:9090")]
    pub cosmos_grpc: String,
    /// The Cosmos Denom and amount to pay Cosmos chain fees
    #[clap(short, long, parse(try_from_str))]
    pub fees: Coin,
    /// Do not save keys to disk for later use with `orchestrator start`
    #[clap(long)]
    pub no_save: bool,
}

/// Add an Ethereum private key for use with either the Relayer or the Orchestrator
#[derive(Parser)]
pub struct SetEthereumKeyOpts {
    #[clap(short, long, parse(try_from_str))]
    pub key: EthPrivateKey,
}

/// Add a Cosmos private key to use as the Orchestrator address
#[derive(Parser)]
pub struct SetOrchestratorKeyOpts {
    #[clap(short, long)]
    pub phrase: String,
}

/// Recover inaccessible funds from a failed IBC Auto Forward to an Ethermint chain (e.g. Evmos, Canto)
/// by either sending them back to Ethereum, or to another Gravity address you control.
/// To send to Ethereum, provide the --send-to-eth flag.
/// To send to another Gravity address, provide the --send-on-cosmos flag.
#[derive(Parser)]
pub struct RecoverFundsOpts {
    /// Use this flag if you want to recover your funds by sending them back to Ethereum
    #[clap(long)]
    pub send_to_eth: bool,
    /// Use this flag if you want to recover your funds by sending them to another Gravity address
    #[clap(long)]
    pub send_on_cosmos: bool,
    /// Ethereum mnemonic phrase or 0x... PrivateKey containing the tokens you would like to send
    #[clap(short, long, parse(try_from_str))]
    pub ethereum_key: EthermintPrivateKey,
    /// (Optional) The Cosmos gRPC server that will be used to submit the transaction
    #[clap(long, default_value = DEFAULT_GRPC_ADDRESS)]
    pub cosmos_grpc: String,
    /// The Denom and amount you wish to send eg: 100ugraviton
    #[clap(short, long, parse(try_from_str))]
    pub amount: Coin,
    /// The Cosmos Denom and amount to pay for submitting a Cosmos Tx eg: 1ugraviton
    #[clap(long, parse(try_from_str))]
    pub cosmos_fee: Option<Coin>,
    /// The Send to Eth ChainFee, which is paid to stakers on Gravity Bridge Chain.
    /// If you leave this blank then a sensible amount will automatically be computed for you
    /// and deducted from your `amount` value. If provided this must be the same denom as
    /// the `amount` token.
    #[clap(long, parse(try_from_str))]
    pub chain_fee: Option<Coin>,
    /// The amount you want to pay in bridge fees, these are used to pay relayers
    /// on Ethereum and must be of the same denomination as `amount`
    /// **Only use this with the send-to-eth flag**
    #[clap(long, parse(try_from_str))]
    pub eth_bridge_fee: Option<Coin>,
    /// The destination address on the Ethereum chain
    /// **Only use this with the send-to-eth flag**
    #[clap(long, parse(try_from_str))]
    pub eth_destination: Option<EthAddress>,
    /// The Gravity address which should receive the funds
    /// **Only use this with the send-on-cosmos flag**
    #[clap(long, parse(try_from_str))]
    pub cosmos_destination: Option<CosmosAddress>,
}

/// Initialize configuration
#[derive(Parser)]
pub struct InitOpts {}

/// The Gravity Bridge Governance subcommand contains tools for interacting with governance and submitting
/// proposal types custom to Gravity Bridge
#[derive(Parser)]
pub struct GovOpts {
    #[clap(subcommand)]
    pub subcmd: GovSubcommand,
}

#[derive(Parser)]
pub enum GovSubcommand {
    #[clap(subcommand)]
    /// Submit custom governance proposal types
    Submit(GovSubmitSubcommand),
    #[clap(subcommand)]
    /// Query info about custom governance proposal types
    Query(GovQuerySubcommand),
}

#[derive(Parser)]
pub enum GovSubmitSubcommand {
    IbcMetadata(IbcMetadataProposalOpts),
    Airdrop(AirdropProposalOpts),
    EmergencyBridgeHalt(EmergencyBridgeHaltProposalOpts),
    OracleUnhalt(OracleUnhaltProposalOpts),
}

#[derive(Parser)]
pub enum GovQuerySubcommand {
    Airdrop(AirdropQueryOpts),
}

#[derive(Parser)]
/// Queries active airdrop proposals and pretty-prints the interpreted data
pub struct AirdropQueryOpts {
    /// (Optional) The Cosmos gRPC server that will be used to submit the proposal
    #[clap(long, default_value = DEFAULT_GRPC_ADDRESS)]
    pub cosmos_grpc: String,
    /// (Optional) query airdrops not actively being voted on
    #[clap(short, long)]
    pub query_history: bool,
    /// (Optional) display full recipients list for airdrops over 100 members
    #[clap(short, long)]
    pub full_list: bool,
}

/// An IBC metadata proposal is a Governance proposal which allows setting denom metadata
/// for an IBC token. This is an essential first setup in taking IBC tokens to Ethereum.
/// The provided denom metadata will be used to set the name, symbol, description, and decimals
/// for the resulting ERC20
#[derive(Parser)]
pub struct IbcMetadataProposalOpts {
    /// (Optional) The Cosmos gRPC server that will be used to submit the transaction
    #[clap(long, default_value = DEFAULT_GRPC_ADDRESS)]
    pub cosmos_grpc: String,
    /// The phrase for an address containing enough funds to submit the proposal.
    #[clap(short, long, parse(try_from_str))]
    pub cosmos_phrase: CosmosPrivateKey,
    /// Path to the proposal.json
    #[clap(short, long, parse(try_from_str))]
    pub json: PathBuf,
    /// The Cosmos Denom and amount to pay the governance proposal deposit
    #[clap(short, long, parse(try_from_str))]
    pub deposit: Coin,
    /// The Cosmos Denom and amount to pay Cosmos chain fees
    #[clap(short, long, parse(try_from_str))]
    pub fees: Coin,
}

/// An Airdrop Proposal allows the community to create, vote on, and execute
/// an airdrop. A list of addresses are all send an equal amount of tokens from the community pool.
#[derive(Parser)]
pub struct AirdropProposalOpts {
    /// (Optional) The Cosmos gRPC server that will be used to submit the transaction
    #[clap(long, default_value = DEFAULT_GRPC_ADDRESS)]
    pub cosmos_grpc: String,
    /// The phrase for an address containing enough funds to submit the proposal.
    #[clap(short, long, parse(try_from_str))]
    pub cosmos_phrase: CosmosPrivateKey,
    /// Path to the proposal.json
    #[clap(short, long, parse(try_from_str))]
    pub json: PathBuf,
    /// The Cosmos Denom and amount to pay the governance proposal deposit
    #[clap(short, long, parse(try_from_str))]
    pub deposit: Coin,
    /// The Cosmos Denom and amount to pay Cosmos chain fees
    #[clap(short, long, parse(try_from_str))]
    pub fees: Coin,
}

/// In case of a critical bug or other event involving the bridge the Gravity Bridge community may
/// choose to halt the operation of the bridge. What this will do is pause the processing of SendToEth
/// MsgCreateBatch and all Oracle claims messages. Effectively stopping the bridge from functioning until
/// governance takes action to turn it on again
#[derive(Parser)]
pub struct EmergencyBridgeHaltProposalOpts {
    /// (Optional) The Cosmos gRPC server that will be used to submit the transaction
    #[clap(long, default_value = DEFAULT_GRPC_ADDRESS)]
    pub cosmos_grpc: String,
    /// The phrase for an address containing enough funds to submit the proposal.
    #[clap(short, long, parse(try_from_str))]
    pub cosmos_phrase: CosmosPrivateKey,
    /// Path to the proposal.json
    #[clap(short, long, parse(try_from_str))]
    pub json: PathBuf,
    /// The Cosmos Denom and amount to pay the governance proposal deposit
    #[clap(short, long, parse(try_from_str))]
    pub deposit: Coin,
    /// The Cosmos Denom and amount to pay Cosmos chain fees
    #[clap(short, long, parse(try_from_str))]
    pub fees: Coin,
}

/// If there is a fork on the Ethereum mainnet it may cause disagreement in the bridge Oracle
/// since there is no way for validators to retract a claim once made normally this fork would
/// cause an irresolveable deadlock in the oracle. If this were to occur passing a OracleUnhaltProposal
/// will reset the oracle and allow the bridge to progress normally
#[derive(Parser)]
pub struct OracleUnhaltProposalOpts {
    /// (Optional) The Cosmos gRPC server that will be used to submit the transaction
    #[clap(long, default_value = DEFAULT_GRPC_ADDRESS)]
    pub cosmos_grpc: String,
    /// The phrase for an address containing enough funds to submit the proposal.
    #[clap(short, long, parse(try_from_str))]
    pub cosmos_phrase: CosmosPrivateKey,
    /// Path to the proposal.json
    #[clap(short, long, parse(try_from_str))]
    pub json: PathBuf,
    /// The Cosmos Denom and amount to pay the governance proposal deposit
    #[clap(short, long, parse(try_from_str))]
    pub deposit: Coin,
    /// The Cosmos Denom and amount to pay Cosmos chain fees
    #[clap(short, long, parse(try_from_str))]
    pub fees: Coin,
}
