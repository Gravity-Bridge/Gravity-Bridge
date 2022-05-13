use thiserror::Error;

#[derive(Error, Debug)]
pub enum RelayerError {
    #[error("unable to make gas tracker history smaller than it already is")]
    InvalidGasTrackerResize,
}
