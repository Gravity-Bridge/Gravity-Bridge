//! contains configuration structs that need to be accessed across crates.

/// Global configuration struct for Gravity bridge tools
#[derive(Serialize, Deserialize, Debug, PartialEq, Eq, Default, Clone)]
pub struct GravityBridgeToolsConfig {
    #[serde(default = "RelayerConfig::default")]
    pub relayer: RelayerConfig,
    #[serde(default = "OrchestratorConfig::default")]
    pub orchestrator: OrchestratorConfig,
    #[serde(default = "MetricsConfig::default")]
    pub metrics: MetricsConfig,
}

/// Relayer configuration options
#[derive(Serialize, Deserialize, Debug, PartialEq, Eq, Clone)]
pub struct RelayerConfig {
    #[serde(default = "default_valset_market_enabled")]
    pub valset_market_enabled: bool,
    #[serde(default = "default_batch_market_enabled")]
    pub batch_market_enabled: bool,
    #[serde(default = "default_logic_call_market_enabled")]
    pub logic_call_market_enabled: bool,
}

// Disabled for bridge launch as some valsets need to be relayed before the
// ethereum-representation of cosmos tokens appear on uniswap. Enabling this would
// halt the bridge launch.
fn default_valset_market_enabled() -> bool {
    false
}

fn default_batch_market_enabled() -> bool {
    true
}

fn default_logic_call_market_enabled() -> bool {
    true
}

impl Default for RelayerConfig {
    fn default() -> Self {
        RelayerConfig {
            valset_market_enabled: default_valset_market_enabled(),
            batch_market_enabled: default_batch_market_enabled(),
            logic_call_market_enabled: default_logic_call_market_enabled(),
        }
    }
}

/// Orchestrator configuration options
#[derive(Serialize, Deserialize, Debug, PartialEq, Eq, Clone)]
pub struct OrchestratorConfig {
    /// If this Orchestrator should run an integrated relayer or not
    #[serde(default = "default_relayer_enabled")]
    pub relayer_enabled: bool,
}

fn default_relayer_enabled() -> bool {
    true
}

impl Default for OrchestratorConfig {
    fn default() -> Self {
        OrchestratorConfig {
            relayer_enabled: default_relayer_enabled(),
        }
    }
}

/// Metrics configuration options
#[derive(Serialize, Deserialize, Debug, PartialEq, Eq, Clone)]
pub struct MetricsConfig {
    /// If this Orchestrator should run an integrated metrics or not
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
