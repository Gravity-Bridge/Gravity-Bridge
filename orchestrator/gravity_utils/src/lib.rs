//! This crate is for common functions and types for the Gravity rust code

#![allow(clippy::result_large_err)]

#[macro_use]
extern crate serde_derive;
#[macro_use]
extern crate log;

pub mod connection_prep;
pub mod error;
pub mod get_with_retry;
pub mod num_conversion;
pub mod prices;
pub mod types;
