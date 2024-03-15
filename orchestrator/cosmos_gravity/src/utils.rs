use crate::query::{get_last_event_nonce_for_validator, get_min_chain_fee_basis_points};
use deep_space::client::ChainStatus;
use deep_space::error::CosmosGrpcError;
use deep_space::utils::encode_any;
use deep_space::{Address as CosmosAddress, Contact};
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::OutgoingLogicCall as ProtoLogicCall;
use gravity_proto::gravity::OutgoingTxBatch as ProtoBatch;
use gravity_proto::gravity::Valset as ProtoValset;
use gravity_utils::get_with_retry::RETRY_TIME;
use gravity_utils::types::LogicCall;
use gravity_utils::types::TransactionBatch;
use gravity_utils::types::Valset;
use num256::Uint256;
use prost_types::Any;
use std::ops::Mul;
use tokio::time::sleep;
use tonic::metadata::AsciiMetadataValue;
use tonic::transport::Channel;
use tonic::{IntoRequest, Request};

const BASIS_POINT_DIVISOR: u64 = 10000;

/// gets the Cosmos last event nonce, no matter how long it takes.
pub async fn get_last_event_nonce_with_retry(
    client: &mut GravityQueryClient<Channel>,
    our_cosmos_address: CosmosAddress,
    prefix: String,
) -> u64 {
    let mut res =
        get_last_event_nonce_for_validator(client, our_cosmos_address, prefix.clone()).await;
    while res.is_err() {
        error!(
            "Failed to get last event nonce, is the Cosmos GRPC working? {:?}",
            res
        );
        sleep(RETRY_TIME).await;
        res = get_last_event_nonce_for_validator(client, our_cosmos_address, prefix.clone()).await;
    }
    res.unwrap()
}

pub enum BadSignatureEvidence {
    Valset(Valset),
    Batch(TransactionBatch),
    LogicCall(LogicCall),
}

impl BadSignatureEvidence {
    pub fn to_any(&self) -> Any {
        match self {
            BadSignatureEvidence::Valset(v) => {
                let v: ProtoValset = v.into();
                encode_any(v, "/gravity.v1.Valset".to_string())
            }
            BadSignatureEvidence::Batch(b) => {
                let b: ProtoBatch = b.into();
                encode_any(b, "/gravity.v1.OutgoingTxBatch".to_string())
            }
            BadSignatureEvidence::LogicCall(l) => {
                let l: ProtoLogicCall = l.into();
                encode_any(l, "/gravity.v1.OutgoingLogicCall".to_string())
            }
        }
    }
}

/// Converts a standard GRPC query Request into a historical one at the given `past_height` by adding
/// the "x-cosmos-block-height" gRPC metadata to the request
/// `req` should be a standard GRPC request like cosmos_sdk_proto_althea::cosmos::bank::v1beta1::QueryBalancesRequest
///
/// Returns a Request with the set gRPC metadata
pub fn historical_grpc_query<T>(req: impl IntoRequest<T>, past_height: u64) -> Request<T> {
    let mut request = req.into_request();
    request.metadata_mut().insert(
        "x-cosmos-block-height",
        AsciiMetadataValue::from(past_height),
    );
    request
}

/// Fetches the chain status and returns only the current height, or an error
pub async fn get_current_cosmos_height(contact: &Contact) -> Result<u64, CosmosGrpcError> {
    let curr_height = contact.get_chain_status().await.unwrap();
    if let ChainStatus::Moving { block_height } = curr_height {
        Ok(block_height)
    } else {
        Err(CosmosGrpcError::BadResponse(
            "Chain is not moving".to_string(),
        ))
    }
}

/// Fetches the current Gravity Bridge MinChainFeeBasisPoints and calculates the minimum ChainFee for a MsgSendToEth
pub async fn get_reasonable_send_to_eth_fee(
    contact: &Contact,
    bridge_amount: Uint256,
) -> Result<Uint256, CosmosGrpcError> {
    let min_fee_basis_points = get_min_chain_fee_basis_points(contact).await?;
    Ok(get_min_send_to_eth_fee(
        bridge_amount,
        min_fee_basis_points.into(),
    ))
}

/// Calculates the minimum `ChainFee` for a MsgSendToEth given the amount and the current MinChainFeeBasisPoints param
pub fn get_min_send_to_eth_fee(bridge_amount: Uint256, min_fee_basis_points: Uint256) -> Uint256 {
    Uint256(
        bridge_amount
            .div_ceil(Uint256::from(BASIS_POINT_DIVISOR).0)
            .mul(min_fee_basis_points.0),
    )
}
