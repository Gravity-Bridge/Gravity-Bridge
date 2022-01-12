use gravity_utils::types::MetricsConfig;
use prometheus_exporter::prometheus::register_int_counter;
use prometheus_exporter::prometheus::{IntCounter};
use std::net::SocketAddr;
use lazy_static::lazy_static;



lazy_static! {

    pub static ref MAJOR_ERROR: IntCounter =
        register_int_counter!("orchestator_major_errors_count", "Both ETH & Cosmos rpc errors").unwrap();
    pub static ref MAJOR_ETH_ERROR: IntCounter =
        register_int_counter!("orchestator_major_eth_errors_count", "ETH rpc errors").unwrap();

    pub static ref MAJOR_COSMOS_ERROR: IntCounter =
        register_int_counter!("orchestator_major_cosmos_errors_count", "Cosmos rpc errors").unwrap();
}


pub fn metrics_server(config: &MetricsConfig) {
    // Parse address used to bind exporter to.
    let addr_raw = &config.metrics_bind;
    let addr: SocketAddr = addr_raw.parse().expect("can not parse listen addr");
    // Start exporter
    prometheus_exporter::start(addr).expect("can not start exporter");
}
