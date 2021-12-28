//! Command line argument definitions for Gravity bridge tools
//! See the clap documentation for how exactly this works, note that doc comments are displayed to the user

use clap::Parser;
use clarity::Address as EthAddress;
use clarity::PrivateKey as EthPrivateKey;
use deep_space::PrivateKey as CosmosPrivateKey;
use deep_space::{address::Address as CosmosAddress, Coin};
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
    Client(ClientOpts),
    Gov(GovOpts),
    Keys(KeyOpts),
    Init(InitOpts),
}

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
}

/// Send Cosmos tokens to Ethereum
#[derive(Parser)]
pub struct CosmosToEthOpts {
    /// Cosmos mnemonic phrase containing the tokens you would like to send
    #[clap(short, long, parse(try_from_str))]
    pub cosmos_phrase: CosmosPrivateKey,
    /// (Optional) The Cosmos gRPC server that will be used to submit the transaction
    #[clap(long, default_value = "http://localhost:9090")]
    pub cosmos_grpc: String,
    /// The Denom and amount you wish to send eg: 100ugraviton
    #[clap(short, long, parse(try_from_str))]
    pub amount: Coin,
    /// The Cosmos Denom and amount to pay Cosmos chain fees eg: 1ugraviton
    #[clap(short, long, parse(try_from_str))]
    pub fee: Coin,
    /// The amount you want to pay in bridge fees, these are used to pay relayers
    /// on Ethereum and must be of the same denomination as `amount`
    #[clap(short, long, parse(try_from_str))]
    pub bridge_fee: Coin,
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
    #[clap(long, default_value = "http://localhost:8545")]
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
    #[clap(long, default_value = "http://localhost:9090")]
    pub cosmos_grpc: String,
    /// (Optional) The Ethereum RPC server that will be used to submit the transaction
    #[clap(long, default_value = "http://localhost:8545")]
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

/// Manage keys
#[derive(Parser)]
pub struct KeyOpts {
    #[clap(subcommand)]
    pub subcmd: KeysSubcommand,
}

#[derive(Parser)]
pub enum KeysSubcommand {
    RegisterOrchestratorAddress(RegisterOrchestratorAddressOpts),
    SetEthereumKey(SetEthereumKeyOpts),
    SetOrchestratorKey(SetOrchestratorKeyOpts),
    Show,
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
    ///
    #[clap(short, long, parse(try_from_str))]
    pub key: EthPrivateKey,
}

/// Add a Cosmos private key to use as the Orchestrator address
#[derive(Parser)]
pub struct SetOrchestratorKeyOpts {
    #[clap(short, long)]
    pub phrase: String,
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
    /// (Optional) The Cosmos gRPC server that will be used to perform the query
    #[clap(short, long, default_value = "http://localhost:9090")]
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
    #[clap(long, default_value = "http://localhost:9090")]
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
    #[clap(long, default_value = "http://localhost:9090")]
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
    #[clap(long, default_value = "http://localhost:9090")]
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
/// cause an irresolveable deadlock in the oracle. If this where to occur passing a OracleUnhaltProposal
/// will reset the oracle and allow the bridge to progress normally
#[derive(Parser)]
pub struct OracleUnhaltProposalOpts {
    /// (Optional) The Cosmos gRPC server that will be used to submit the transaction
    #[clap(long, default_value = "http://localhost:9090")]
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
