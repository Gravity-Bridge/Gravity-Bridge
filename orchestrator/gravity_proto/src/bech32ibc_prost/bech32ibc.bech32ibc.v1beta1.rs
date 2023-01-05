/// An HrpIbcRecord maps a bech32 human-readable prefix to an IBC source
/// channel
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct HrpIbcRecord {
    /// The bech32 human readable prefix that serves as the key
    #[prost(string, tag = "1")]
    pub hrp: ::prost::alloc::string::String,
    /// the channel by which the packet will be sent
    #[prost(string, tag = "2")]
    pub source_channel: ::prost::alloc::string::String,
    #[prost(uint64, tag = "3")]
    pub ics_to_height_offset: u64,
    #[prost(message, optional, tag = "4")]
    pub ics_to_time_offset: ::core::option::Option<::prost_types::Duration>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryHrpIbcRecordsRequest {}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryHrpIbcRecordsResponse {
    #[prost(message, repeated, tag = "1")]
    pub hrp_ibc_records: ::prost::alloc::vec::Vec<HrpIbcRecord>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryHrpIbcRecordRequest {
    #[prost(string, tag = "1")]
    pub hrp: ::prost::alloc::string::String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryHrpIbcRecordResponse {
    #[prost(message, optional, tag = "1")]
    pub hrp_ibc_record: ::core::option::Option<HrpIbcRecord>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryNativeHrpRequest {}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct QueryNativeHrpResponse {
    #[prost(string, tag = "1")]
    pub native_hrp: ::prost::alloc::string::String,
}
/// Generated client implementations.
pub mod query_client {
    #![allow(unused_variables, dead_code, missing_docs, clippy::let_unit_value)]
    use tonic::codegen::*;
    #[derive(Debug, Clone)]
    pub struct QueryClient<T> {
        inner: tonic::client::Grpc<T>,
    }
    impl QueryClient<tonic::transport::Channel> {
        /// Attempt to create a new client by connecting to a given endpoint.
        pub async fn connect<D>(dst: D) -> Result<Self, tonic::transport::Error>
        where
            D: std::convert::TryInto<tonic::transport::Endpoint>,
            D::Error: Into<StdError>,
        {
            let conn = tonic::transport::Endpoint::new(dst)?.connect().await?;
            Ok(Self::new(conn))
        }
    }
    impl<T> QueryClient<T>
    where
        T: tonic::client::GrpcService<tonic::body::BoxBody>,
        T::Error: Into<StdError>,
        T::ResponseBody: Body<Data = Bytes> + Send + 'static,
        <T::ResponseBody as Body>::Error: Into<StdError> + Send,
    {
        pub fn new(inner: T) -> Self {
            let inner = tonic::client::Grpc::new(inner);
            Self { inner }
        }
        pub fn with_interceptor<F>(
            inner: T,
            interceptor: F,
        ) -> QueryClient<InterceptedService<T, F>>
        where
            F: tonic::service::Interceptor,
            T::ResponseBody: Default,
            T: tonic::codegen::Service<
                http::Request<tonic::body::BoxBody>,
                Response = http::Response<
                    <T as tonic::client::GrpcService<tonic::body::BoxBody>>::ResponseBody,
                >,
            >,
            <T as tonic::codegen::Service<http::Request<tonic::body::BoxBody>>>::Error:
                Into<StdError> + Send + Sync,
        {
            QueryClient::new(InterceptedService::new(inner, interceptor))
        }

        /// HrpIbcRecords returns to full list of records
        pub async fn hrp_ibc_records(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryHrpIbcRecordsRequest>,
        ) -> Result<tonic::Response<super::QueryHrpIbcRecordsResponse>, tonic::Status> {
            self.inner.ready().await.map_err(|e| {
                tonic::Status::new(
                    tonic::Code::Unknown,
                    format!("Service was not ready: {}", e.into()),
                )
            })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/bech32ibc.bech32ibc.v1beta1.Query/HrpIbcRecords",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        /// HrpIbcRecord returns the record for a requested HRP
        pub async fn hrp_ibc_record(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryHrpIbcRecordRequest>,
        ) -> Result<tonic::Response<super::QueryHrpIbcRecordResponse>, tonic::Status> {
            self.inner.ready().await.map_err(|e| {
                tonic::Status::new(
                    tonic::Code::Unknown,
                    format!("Service was not ready: {}", e.into()),
                )
            })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/bech32ibc.bech32ibc.v1beta1.Query/HrpIbcRecord",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
        /// NativeHrp returns the chain's native HRP
        pub async fn native_hrp(
            &mut self,
            request: impl tonic::IntoRequest<super::QueryNativeHrpRequest>,
        ) -> Result<tonic::Response<super::QueryNativeHrpResponse>, tonic::Status> {
            self.inner.ready().await.map_err(|e| {
                tonic::Status::new(
                    tonic::Code::Unknown,
                    format!("Service was not ready: {}", e.into()),
                )
            })?;
            let codec = tonic::codec::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/bech32ibc.bech32ibc.v1beta1.Query/NativeHrp",
            );
            self.inner.unary(request.into_request(), path, codec).await
        }
    }
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct GenesisState {
    #[prost(string, tag = "1")]
    pub native_hrp: ::prost::alloc::string::String,
    #[prost(message, repeated, tag = "2")]
    pub hrp_ibc_records: ::prost::alloc::vec::Vec<HrpIbcRecord>,
}
/// UpdateHrpIBCRecordProposal is a gov Content type for adding a new record
/// between a bech32 prefix and an IBC (port, channel).
/// It can be used to add a new record to the set. It can also be
/// used to update the IBC channel to associate with a specific denom. If channel
/// is set to "", it will remove the record from the set.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct UpdateHrpIbcChannelProposal {
    #[prost(string, tag = "1")]
    pub title: ::prost::alloc::string::String,
    #[prost(string, tag = "2")]
    pub description: ::prost::alloc::string::String,
    #[prost(string, tag = "3")]
    pub hrp: ::prost::alloc::string::String,
    #[prost(string, tag = "4")]
    pub source_channel: ::prost::alloc::string::String,
    #[prost(uint64, tag = "5")]
    pub ics_to_height_offset: u64,
    #[prost(message, optional, tag = "6")]
    pub ics_to_time_offset: ::core::option::Option<::prost_types::Duration>,
}
