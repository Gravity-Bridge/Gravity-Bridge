//! This crate provides Gravity proto definitions in Rust and also re-exports cosmos_sdk_proto for use by downstream
//! crates. By default around a dozen proto files are generated and places into the prost folder. We could then proceed
//! to fix up all these files and use them as the required dependencies to the Gravity file, but we chose instead to replace
//! those paths with references ot upstream cosmos-sdk-proto and delete the other files. This reduces cruft in this repo even
//! if it does make for a somewhat more confusing proto generation process.

pub use cosmos_sdk_proto;
pub mod gravity {
    include!("prost/gravity.v1.rs");
    include!("ethereum_claim.rs");
}

/// Bech32ibc protobuf definitions
#[cfg(feature = "bech32ibc")]
#[cfg_attr(docsrs, doc(cfg(feature = "bech32ibc")))]
pub mod bech32ibc {
    /// Bech32 prefix -> IBC Channel mapping
    pub mod bech32ibc {
        pub mod v1 {
            include!("bech32ibc_prost/bech32ibc.bech32ibc.v1beta1.rs");
        }
    }
}

/// Ethermint protobuf definitions
#[cfg(feature = "ethermint")]
#[cfg_attr(docsrs, doc(cfg(feature = "ethermint")))]
pub mod ethermint {
    /// Ethermint EthSecp256k1 support
    pub mod crypto {
        pub mod v1 {
            pub mod ethsecp256k1 {
                include!("ethermint_prost/ethermint.crypto.v1.ethsecp256k1.rs");
            }
        }
    }
    /// EVM support
    pub mod evm {
        pub mod v1 {
            include!("ethermint_prost/ethermint.evm.v1.rs");
        }
    }
    /// Feemarket support
    pub mod feemarket {
        pub mod v1 {
            include!("ethermint_prost/ethermint.feemarket.v1.rs");
        }
    }
    /// Ethermint types
    pub mod types {
        pub mod v1 {
            include!("ethermint_prost/ethermint.types.v1.rs");
        }
    }
}
