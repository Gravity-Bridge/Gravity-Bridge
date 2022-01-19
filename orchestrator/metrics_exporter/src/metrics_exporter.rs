use gravity_utils::types::MetricsConfig;
use lazy_static::lazy_static;
use prometheus_exporter::prometheus::{
    register_int_counter, register_int_counter_vec, register_int_gauge_vec,
};
use prometheus_exporter::prometheus::{IntCounter, IntCounterVec, IntGaugeVec};
use std::net::SocketAddr;

lazy_static! {

    //  Errors
    pub static ref ERRORS_TOTAL: IntCounter =
        register_int_counter!("orchestrator_errors_count_total", "Total errors since startup").unwrap();
    pub static ref ERROR: IntCounterVec =
        register_int_counter_vec!("orchestrator_errors_count_cosmos_eth", "Both ETH & Cosmos related errors", &["error_message"]).unwrap();
    pub static ref ERROR_ETH: IntCounterVec =
        register_int_counter_vec!("orchestrator_errors_count_eth", "ETH related errors", &["error_message"]).unwrap();
    pub static ref ERROR_COSMOS: IntCounterVec =
        register_int_counter_vec!("orchestrator_errors_count_cosmos", "Cosmos related errors", &["error_message"]).unwrap();
    pub static ref ERROR_UNCLASSIFIED: IntCounterVec =
        register_int_counter_vec!("orchestrator_errors_count_unclassified", "Chech orchestrator logs for more details", &["error_message"]).unwrap();

    // Warnings
    pub static ref WARNINGS_TOTAL: IntCounter =
        register_int_counter!("orchestrator_warnings_count_total", "Total warnings since startup").unwrap();
    pub static ref WARNING: IntCounterVec =
        register_int_counter_vec!("orchestrator_warnings_count_cosmos_eth", "Both ETH & Cosmos related warnings", &["warn_message"]).unwrap();
    pub static ref WARNING_ETH: IntCounterVec =
        register_int_counter_vec!("orchestrator_warnings_count_eth", "ETH related warnings", &["warn_message"]).unwrap();
    pub static ref WARNING_COSMOS: IntCounterVec =
        register_int_counter_vec!("orchestrator_warnings_count_cosmos", "Cosmos related warnings", &["warn_message"]).unwrap();
    pub static ref WARNING_UNCLASSIFIED: IntCounterVec =
        register_int_counter_vec!("orchestrator_warnings_count_unclassified", "Chech orchestrator logs for more details", &["warn_message"]).unwrap();

    // Information gauges
    pub static ref LATEST_INFO: IntGaugeVec =
        register_int_gauge_vec!("orchestrator_information", "Latest orchestrator information", &["gauge"]).unwrap();
}

pub fn metrics_errors_counter(s: i32, e: &str) {
    match s {
        0 => ERROR.with_label_values(&[e]).inc(),
        1 => ERROR_ETH.with_label_values(&[e]).inc(),
        2 => ERROR_COSMOS.with_label_values(&[e]).inc(),
        _ => ERROR_UNCLASSIFIED.with_label_values(&[e]).inc(),
    }
    ERRORS_TOTAL.inc()
}

pub fn metrics_warnings_counter(s: i32, e: &str) {
    match s {
        0 => WARNING.with_label_values(&[e]).inc(),
        1 => WARNING_ETH.with_label_values(&[e]).inc(),
        2 => WARNING_COSMOS.with_label_values(&[e]).inc(),
        _ => WARNING_UNCLASSIFIED.with_label_values(&[e]).inc(),
    }
    WARNINGS_TOTAL.inc()
}

pub fn metrics_latest(u: u64, e: &str) {
    match i64::try_from(u).is_ok() {
        true => {
            LATEST_INFO.with_label_values(&[e]).set(u as i64);
        }
        false => {}
    }
}

pub fn metrics_server(config: &MetricsConfig) {
    // Parse address used to bind exporter to.
    let addr_raw = &config.metrics_bind;
    let addr: SocketAddr = addr_raw.parse().expect("can not parse listen addr");
    // Start exporter
    prometheus_exporter::start(addr).expect("can not start exporter");
}

/// Test overflowing bigint
#[test]
fn test_overflow_big_integer() {
    let res = i64::try_from(18446744073709551615u64).is_err();
    assert!(res);
}
