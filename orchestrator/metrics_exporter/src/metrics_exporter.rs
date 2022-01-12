use gravity_utils::types::MetricsConfig;
use lazy_static::lazy_static;
use prometheus_exporter::prometheus::{IntCounter, IntCounterVec};
use prometheus_exporter::prometheus::{
    register_int_counter, register_int_counter_vec,
};
use std::net::SocketAddr;


pub fn metrics_error_counter(s: i32, e: &str) {
        match s  {
            0 => MAJOR_ERROR.with_label_values(&[e]).inc(),
            1 => MAJOR_ERROR_ETH.with_label_values(&[e]).inc(),
            2 => MAJOR_ERROR_COSMOS.with_label_values(&[e]).inc(),
            _ => MAJOR_ERROR_UNCLASSIFIED.with_label_values(&[e]).inc(),
        }
        TOTAL_ERRORS.inc()

}
lazy_static! {
    pub static ref TOTAL_ERRORS: IntCounter =
        register_int_counter!("orchestator_total_major_errors_count", "Total error counter since stratup").unwrap();
    pub static ref MAJOR_ERROR: IntCounterVec =
        register_int_counter_vec!("orchestator_major_errors_count", "Both ETH & Cosmos related errors", &["error_message"]).unwrap();
    pub static ref MAJOR_ERROR_ETH: IntCounterVec =
        register_int_counter_vec!("orchestator_major_eth_errors_count", "ETH related errors", &["error_message"]).unwrap();

    pub static ref MAJOR_ERROR_COSMOS: IntCounterVec =
        register_int_counter_vec!("orchestator_major_cosmos_errors_count", "Cosmos related errors", &["error_message"]).unwrap();

    pub static ref MAJOR_ERROR_UNCLASSIFIED: IntCounterVec =
        register_int_counter_vec!("orchestator_major_unclassified_errors_count", "Chech orchestator logs for more details", &["error_message"]).unwrap();
}


pub fn metrics_server(config: &MetricsConfig) {
    // Parse address used to bind exporter to.
    let addr_raw = &config.metrics_bind;
    let addr: SocketAddr = addr_raw.parse().expect("can not parse listen addr");
    // Start exporter
    prometheus_exporter::start(addr).expect("can not start exporter");
}
