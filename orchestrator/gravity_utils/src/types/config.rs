//! contains configuration structs that need to be accessed across crates.

use clarity::{Address as EthAddress, Uint256};

/// Global configuration struct for Gravity bridge tools
#[derive(Serialize, Deserialize, Debug, PartialEq, Default, Clone)]
pub struct GravityBridgeToolsConfig {
    pub relayer: RelayerConfig,
    pub orchestrator: OrchestratorConfig,
    pub metrics: MetricsConfig,
}

/// Toml serializable configuration struct for Gravity bridge tools
#[derive(Serialize, Deserialize, Debug, PartialEq, Default, Clone)]
pub struct TomlGravityBridgeToolsConfig {
    #[serde(default = "TomlRelayerConfig::default")]
    pub relayer: TomlRelayerConfig,
    #[serde(default = "OrchestratorConfig::default")]
    pub orchestrator: OrchestratorConfig,
    #[serde(default = "MetricsConfig::default")]
    pub metrics: MetricsConfig,
}

impl From<TomlGravityBridgeToolsConfig> for GravityBridgeToolsConfig {
    fn from(input: TomlGravityBridgeToolsConfig) -> Self {
        GravityBridgeToolsConfig {
            relayer: input.relayer.into(),
            orchestrator: input.orchestrator,
            metrics: input.metrics,
        }
    }
}

/// Relayer configuration options
#[derive(Serialize, Deserialize, Debug, PartialEq, Clone)]
pub struct RelayerConfig {
    pub valset_relaying_mode: ValsetRelayingMode,
    pub batch_request_mode: BatchRequestMode,
    pub batch_relaying_mode: BatchRelayingMode,
    pub logic_call_market_enabled: bool,
    /// the speed at which the relayer attempts to relay batches logic calls and valsets, in seconds
    /// higher values reduce the chances of money lost to a collision
    /// also controls the batch request loop speed, however that is offset by batch_request_relay_offset seconds
    pub relayer_loop_speed: u64,
    /// the speed at which the relayer fetches ethereum gas prices for altruistic relaying, in seconds
    /// the gas tracker will store ALTRUISTIC_SAMPLES samples of gas prices to determine when it is currently a
    /// "low-fee" period for altruistic batch requests/batch relaying
    pub gas_tracker_loop_speed: u64,
    /// the delay between the batch request and relaying loops, in seconds
    /// requesting a batch typically takes 20-25 seconds to process before it can be relayed, this
    /// offset enables requested batches to be relayed immediately
    pub batch_request_relay_offset: u64,
    /// the number of gas price samples an altruistic relayer will wait for before attempting to relay any batches
    /// to avoid submitting batches with too little information about gas price changes
    pub altruistic_batch_relaying_samples_delay: u64,
    /// the number of samples the gas tracker will store before overwriting the oldest samples
    /// ensure that altruistic_gas_price_samples * gas_tracker_loop_speed covers the period in seconds the relayer
    /// should be aware of
    pub altruistic_gas_price_samples: u64,
    /// specifies the lowest x% of observed gas prices an altruistic relayer is willing to pay for relaying to ethereum
    /// also controls when batches are requested
    /// acceptable gas prices are determined by the samples in the gas tracker, so both
    /// gas_tracker_loop_speed and altruistic_gas_price_samples will play a role in this decision
    pub altruistic_acceptable_gas_price_percentage: f32,
    /// the speed at which the relayer checks for pending ibc auto forwards, in seconds
    pub ibc_auto_forward_loop_speed: u64,
    /// the number of pending ibc auto forwards to attempt to execute per loop
    pub ibc_auto_forwards_to_execute: u64,
}

/// Relayer configuration that's is more easily parsable with toml
#[derive(Serialize, Deserialize, Debug, PartialEq, Clone)]
pub struct TomlRelayerConfig {
    #[serde(default = "default_valset_relaying_mode")]
    pub valset_relaying_mode: TomlValsetRelayingMode,
    #[serde(default = "default_batch_request_mode")]
    pub batch_request_mode: BatchRequestMode,
    #[serde(default = "default_batch_relaying_mode")]
    pub batch_relaying_mode: TomlBatchRelayingMode,
    #[serde(default = "default_logic_call_market_enabled")]
    pub logic_call_market_enabled: bool,
    #[serde(default = "default_relayer_loop_speed")]
    pub relayer_loop_speed: u64,
    #[serde(default = "default_gas_tracker_loop_speed")]
    pub gas_tracker_loop_speed: u64,
    #[serde(default = "default_batch_request_relay_offset")]
    pub batch_request_relay_offset: u64,
    #[serde(default = "default_altruistic_batch_relaying_samples_delay")]
    pub altruistic_batch_relaying_samples_delay: u64,
    #[serde(default = "default_altruistic_gas_price_samples")]
    pub altruistic_gas_price_samples: u64,
    #[serde(default = "default_altruistic_acceptable_gas_price_percentage")]
    pub altruistic_acceptable_gas_price_percentage: f32,
    #[serde(default = "default_ibc_auto_forward_loop_speed")]
    pub ibc_auto_forward_loop_speed: u64,
    #[serde(default = "default_ibc_auto_forwards_to_execute")]
    pub ibc_auto_forwards_to_execute: u64,
}

impl From<TomlRelayerConfig> for RelayerConfig {
    fn from(input: TomlRelayerConfig) -> Self {
        RelayerConfig {
            valset_relaying_mode: input.valset_relaying_mode.into(),
            batch_relaying_mode: input.batch_relaying_mode.into(),
            batch_request_mode: input.batch_request_mode,
            logic_call_market_enabled: input.logic_call_market_enabled,
            relayer_loop_speed: input.relayer_loop_speed,
            gas_tracker_loop_speed: input.gas_tracker_loop_speed,
            batch_request_relay_offset: input.batch_request_relay_offset,
            altruistic_batch_relaying_samples_delay: input.altruistic_batch_relaying_samples_delay,
            altruistic_gas_price_samples: input.altruistic_gas_price_samples,
            altruistic_acceptable_gas_price_percentage: input
                .altruistic_acceptable_gas_price_percentage,
            ibc_auto_forward_loop_speed: input.ibc_auto_forward_loop_speed,
            ibc_auto_forwards_to_execute: input.ibc_auto_forwards_to_execute,
        }
    }
}

/// The various possible modes for relaying validator set updates
#[derive(Serialize, Deserialize, Debug, PartialEq, Clone, Copy)]
pub enum ValsetRelayingMode {
    /// Only ever relay profitable valsets, regardless of all other
    /// considerations. Profitable being defined as the value of
    /// the reward token in uniswap being greater than WETH cost of
    /// relaying * margin
    ProfitableOnly { margin: f32 },
    /// Relay validator sets when continued operation of the chain
    /// requires it, this will cost some ETH
    Altruistic,
    /// Relay every validator set update, mostly for developer use
    EveryValset,
}

/// A version of valset relaying mode that's easy to serialize as toml
#[derive(Serialize, Deserialize, Debug, PartialEq, Clone)]
pub struct TomlValsetRelayingMode {
    mode: String,
    margin: Option<f32>,
}

impl From<TomlValsetRelayingMode> for ValsetRelayingMode {
    fn from(input: TomlValsetRelayingMode) -> Self {
        match input.mode.as_str() {
            "ProfitableOnly" | "profitableonly" | "PROFITABLEONLY" => {
                ValsetRelayingMode::ProfitableOnly {
                    margin: input.margin.unwrap(),
                }
            }
            "Altruistic" | "altruistic" | "ALTRUISTIC" => ValsetRelayingMode::Altruistic,
            "EveryValset" | "everyvalset" | "EVERYVALSET" => ValsetRelayingMode::EveryValset,
            _ => panic!("Invalid TomlValsetRelayingMode"),
        }
    }
}

/// The various possible modes for automatic requests of batches
#[derive(Serialize, Deserialize, Debug, PartialEq, Eq, Clone, Copy)]
pub enum BatchRequestMode {
    /// Only request batches at times of minimum gas fees to provide maximum utility
    /// with donated funds
    Altruistic,
    /// Only ever request profitable batches, regardless of all other
    /// considerations, this is fuzzier than the other modes
    ProfitableOnly,
    /// Every possible valid batch should be requested
    EveryBatch,
    /// Does not automatically request batches
    None,
}

/// A whitelisted token that will be relayed given the batch
/// provides at least amount of this specific token
#[derive(Serialize, Deserialize, Debug, PartialEq, Eq, Clone)]
pub struct WhitelistToken {
    /// the price of this token, denominated in weth per coin
    pub price: Uint256,
    /// the number of decimals this token has between it's base unit and a single coin
    pub decimals: u8,
    /// the token which the batch must have the specified amount
    /// of to be relayed
    pub token: EthAddress,
}

/// The various possible modes for batch relaying
#[derive(Serialize, Deserialize, Debug, PartialEq, Clone)]
pub enum BatchRelayingMode {
    /// Submits batches at times of minimum gas fees to provide maximum
    /// utility with donated funds
    Altruistic,
    /// Every possible batch is relayed, mostly for developers
    EveryBatch,
    /// Only consider batches that are profitable as defined by
    /// the given token being listed in Uniswap for a WETH value
    /// higher than cost of relaying * margin
    ProfitableOnly { margin: f32 },
    /// Consider and relay batches that are profitable as previously
    /// defined, but also consider specific tokens with the given value
    /// as an acceptable reward. This is an advanced mode and may lose money
    /// if not carefully configured
    ProfitableWithWhitelist {
        /// The margin for all token types not in the whitelist
        margin: f32,
        whitelist: Vec<WhitelistToken>,
    },
}

/// A version of BatchRelaying mode that is easy to serialize as toml
#[derive(Serialize, Deserialize, Debug, PartialEq, Clone)]
pub struct TomlBatchRelayingMode {
    mode: String,
    margin: Option<f32>,
    whitelist: Option<Vec<WhitelistToken>>,
}

impl From<TomlBatchRelayingMode> for BatchRelayingMode {
    fn from(input: TomlBatchRelayingMode) -> Self {
        match input.mode.as_str() {
            "Altruistic" | "altruistic" | "ALTRUISTIC" => BatchRelayingMode::Altruistic,
            "EveryBatch" | "everybatch" | "EVERYBATCH" => BatchRelayingMode::EveryBatch,
            "ProfitableOnly" | "profitableonly" | "PROFITABLEONLY" => {
                BatchRelayingMode::ProfitableOnly {
                    margin: input.margin.unwrap(),
                }
            }
            "ProfitableWithWhitelist" | "profitablewithwhitelist" | "PROFITABLEWITHWHITELIST" => {
                BatchRelayingMode::ProfitableWithWhitelist {
                    margin: input.margin.unwrap(),
                    whitelist: input.whitelist.unwrap(),
                }
            }
            _ => panic!("Bad TomlBatchRelayingMode"),
        }
    }
}

fn default_batch_relaying_mode() -> TomlBatchRelayingMode {
    TomlBatchRelayingMode {
        mode: "ProfitableOnly".to_string(),
        margin: Some(1.1),
        whitelist: None,
    }
}

fn default_logic_call_market_enabled() -> bool {
    true
}

fn default_valset_relaying_mode() -> TomlValsetRelayingMode {
    TomlValsetRelayingMode {
        mode: "Altruistic".to_string(),
        margin: None,
    }
}

fn default_batch_request_mode() -> BatchRequestMode {
    BatchRequestMode::ProfitableOnly
}

fn default_relayer_loop_speed() -> u64 {
    600
}

fn default_gas_tracker_loop_speed() -> u64 {
    60
}

fn default_batch_request_relay_offset() -> u64 {
    45
}

fn default_altruistic_batch_relaying_samples_delay() -> u64 {
    5
}

pub fn default_altruistic_gas_price_samples() -> u64 {
    2000
}

fn default_altruistic_acceptable_gas_price_percentage() -> f32 {
    0.05
}

fn default_ibc_auto_forward_loop_speed() -> u64 {
    60
}

fn default_ibc_auto_forwards_to_execute() -> u64 {
    50
}

impl Default for RelayerConfig {
    fn default() -> Self {
        RelayerConfig {
            valset_relaying_mode: default_valset_relaying_mode().into(),
            batch_request_mode: default_batch_request_mode(),
            batch_relaying_mode: default_batch_relaying_mode().into(),
            logic_call_market_enabled: default_logic_call_market_enabled(),
            relayer_loop_speed: default_relayer_loop_speed(),
            gas_tracker_loop_speed: default_gas_tracker_loop_speed(),
            batch_request_relay_offset: default_batch_request_relay_offset(),
            altruistic_batch_relaying_samples_delay:
                default_altruistic_batch_relaying_samples_delay(),
            altruistic_gas_price_samples: default_altruistic_gas_price_samples(),
            altruistic_acceptable_gas_price_percentage:
                default_altruistic_acceptable_gas_price_percentage(),
            ibc_auto_forward_loop_speed: default_ibc_auto_forward_loop_speed(),
            ibc_auto_forwards_to_execute: default_ibc_auto_forwards_to_execute(),
        }
    }
}

impl Default for TomlRelayerConfig {
    fn default() -> Self {
        TomlRelayerConfig {
            valset_relaying_mode: default_valset_relaying_mode(),
            batch_request_mode: default_batch_request_mode(),
            batch_relaying_mode: default_batch_relaying_mode(),
            logic_call_market_enabled: default_logic_call_market_enabled(),
            relayer_loop_speed: default_relayer_loop_speed(),
            gas_tracker_loop_speed: default_gas_tracker_loop_speed(),
            batch_request_relay_offset: default_batch_request_relay_offset(),
            altruistic_batch_relaying_samples_delay:
                default_altruistic_batch_relaying_samples_delay(),
            altruistic_gas_price_samples: default_altruistic_gas_price_samples(),
            altruistic_acceptable_gas_price_percentage:
                default_altruistic_acceptable_gas_price_percentage(),
            ibc_auto_forward_loop_speed: default_ibc_auto_forward_loop_speed(),
            ibc_auto_forwards_to_execute: default_ibc_auto_forwards_to_execute(),
        }
    }
}

/// Orchestrator configuration options
#[derive(Serialize, Deserialize, Debug, PartialEq, Eq, Clone, Copy)]
pub struct OrchestratorConfig {
    /// If this Orchestrator should run an integrated relayer or not
    #[serde(default = "default_relayer_enabled")]
    pub relayer_enabled: bool,
    /// Whether to check that the ethereum node supports "finalized" blocks
    #[serde(default = "default_check_eth_rpc")]
    pub check_eth_rpc: bool,
}

fn default_relayer_enabled() -> bool {
    false
}

fn default_check_eth_rpc() -> bool {
    true
}

impl Default for OrchestratorConfig {
    fn default() -> Self {
        OrchestratorConfig {
            relayer_enabled: default_relayer_enabled(),
            check_eth_rpc: default_check_eth_rpc(),
        }
    }
}

/// Metrics server configuration options
#[derive(Serialize, Deserialize, Debug, PartialEq, Eq, Clone)]
pub struct MetricsConfig {
    /// If this Orchestrator should run an integrated metrics server or not
    #[serde(default = "default_metrics_enabled")]
    pub metrics_enabled: bool,
    /// Bind to specified ip:port
    #[serde(default = "default_metrics_bind")]
    pub metrics_bind: String,
}

fn default_metrics_enabled() -> bool {
    false
}

fn default_metrics_bind() -> String {
    "127.0.0.1:6631".to_string()
}

impl Default for MetricsConfig {
    fn default() -> Self {
        MetricsConfig {
            metrics_enabled: default_metrics_enabled(),
            metrics_bind: default_metrics_bind(),
        }
    }
}
