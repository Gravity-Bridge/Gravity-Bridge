/// EthAccount implements the authtypes.AccountI interface and embeds an
/// authtypes.BaseAccount type. It is compatible with the auth AccountKeeper.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct EthAccount {
    #[prost(message, optional, tag = "1")]
    pub base_account:
        ::core::option::Option<super::super::super::cosmos::auth::v1beta1::BaseAccount>,
    #[prost(string, tag = "2")]
    pub code_hash: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ExtensionOptionsWeb3Tx {
    /// typed data chain id used only in EIP712 Domain and should match
    /// Ethereum network ID in a Web3 provider (e.g. Metamask).
    #[prost(uint64, tag = "1")]
    pub typed_data_chain_id: u64,
    /// fee payer is an account address for the fee payer. It will be validated
    /// during EIP712 signature checking.
    #[prost(string, tag = "2")]
    pub fee_payer: ::prost::alloc::string::String,
    /// fee payer sig is a signature data from the fee paying account,
    /// allows to perform fee delegation when using EIP712 Domain.
    #[prost(bytes = "vec", tag = "3")]
    pub fee_payer_sig: ::prost::alloc::vec::Vec<u8>,
}
