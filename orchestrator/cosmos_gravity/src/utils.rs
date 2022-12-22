use crate::query::get_last_event_nonce_for_validator;
use deep_space::utils::encode_any;
use deep_space::Address as CosmosAddress;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::OutgoingLogicCall as ProtoLogicCall;
use gravity_proto::gravity::OutgoingTxBatch as ProtoBatch;
use gravity_proto::gravity::Valset as ProtoValset;
use gravity_utils::get_with_retry::RETRY_TIME;
use gravity_utils::types::LogicCall;
use gravity_utils::types::TransactionBatch;
use gravity_utils::types::Valset;
use prost_types::Any;
use tokio::time::sleep;
use tonic::transport::Channel;

/// gets the Cosmos last event nonce, no matter how long it takes.
pub async fn get_last_event_nonce_with_retry(
    client: &mut GravityQueryClient<Channel>,
    our_cosmos_address: CosmosAddress,
    prefix: String,
    evm_chain_prefix: String,
) -> u64 {
    let mut res = get_last_event_nonce_for_validator(
        client,
        our_cosmos_address,
        prefix.clone(),
        evm_chain_prefix.clone(),
    )
    .await;
    while res.is_err() {
        error!(
            "Failed to get last event nonce, is the Cosmos GRPC working? {:?}",
            res
        );
        sleep(RETRY_TIME).await;
        res = get_last_event_nonce_for_validator(
            client,
            our_cosmos_address,
            prefix.clone(),
            evm_chain_prefix.clone(),
        )
        .await;
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
