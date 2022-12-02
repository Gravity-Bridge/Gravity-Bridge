use crate::query::{get_last_event_nonce_for_validator, get_min_chain_fee_basis_points};
use clarity::Address as EthAddress;
use clarity::Uint256;
use deep_space::client::ChainStatus;
use deep_space::error::CosmosGrpcError;
use deep_space::utils::encode_any;
use deep_space::{Address as CosmosAddress, Contact};
use futures::future::join_all;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::Erc20Token as ProtoErc20Token;
use gravity_proto::gravity::OutgoingLogicCall as ProtoLogicCall;
use gravity_proto::gravity::OutgoingTxBatch as ProtoBatch;
use gravity_proto::gravity::Valset as ProtoValset;
use gravity_utils::error::GravityError;
use gravity_utils::get_with_retry::RETRY_TIME;
use gravity_utils::types::TransactionBatch;
use gravity_utils::types::Valset;
use gravity_utils::types::{
    Erc20DeployedEvent, LogicCall, LogicCallExecutedEvent, SendToCosmosEvent,
    TransactionBatchExecutedEvent, ValsetUpdatedEvent,
};
use prost_types::Any;
use std::collections::{HashMap, HashSet};
use std::convert::TryFrom;
use std::ops::Mul;
use tokio::time::sleep;
use tonic::metadata::AsciiMetadataValue;
use tonic::transport::Channel;
use tonic::{IntoRequest, Request};
use web30::client::Web3;

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
        AsciiMetadataValue::try_from(past_height).expect("Invalid height provided"),
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

#[allow(clippy::too_many_arguments)]
/// Collects the needed Gravity.sol balances for EthereumClaim submissions by first learning which
/// contracts to monitor, then collecting all the ethereum heights needed and then finally
/// querying the ERC20 balances required, packing it all up into a Map
pub async fn collect_eth_balances_for_claims(
    web3: &Web3,
    gravity_contract: EthAddress,
    monitored_erc20s: Vec<EthAddress>,
    deposits: &[SendToCosmosEvent],
    withdraws: &[TransactionBatchExecutedEvent],
    erc20_deploys: &[Erc20DeployedEvent],
    logic_calls: &[LogicCallExecutedEvent],
    valsets: &[ValsetUpdatedEvent],
) -> Result<HashMap<Uint256, Vec<ProtoErc20Token>>, GravityError> {
    let heights =
        get_heights_from_eth_claims(deposits, withdraws, erc20_deploys, logic_calls, valsets).await;
    if heights.is_empty() {
        return Ok(HashMap::new()); // return early
    }

    collect_eth_balances_at_heights(web3, gravity_contract, &monitored_erc20s, &heights).await
}

/// Fetches and parses the gravity MonitoredTokenAddresses governance param as a Vec
pub async fn get_gravity_monitored_erc20s(
    contact: &Contact,
) -> Result<Vec<EthAddress>, GravityError> {
    let erc20s = contact
        .get_param("gravity", "MonitoredTokenAddresses")
        .await?;
    let mut erc20s = erc20s
        .param
        .expect("No response for the gravity MonitoredTokenAddresses param!")
        .value;

    // Remove [ and ] from the ends of the string, if present
    if let Some(rest) = erc20s.strip_prefix('[') {
        if let Some(middle) = rest.strip_suffix(']') {
            erc20s = middle.to_string();
        } else {
            return Err(CosmosGrpcError::BadResponse(format!(
                "MonitoredTokenAddresses begins with [ but does not end with ] :{}",
                erc20s
            ))
            .into());
        }
    }

    // Parse all members of the comma separated string, returning an error if any are invalid
    let erc20s = erc20s.split(',');
    let mut results: Vec<EthAddress> = vec![];
    for e in erc20s {
        if e.is_empty() {
            continue;
        }
        let addr = EthAddress::parse_and_validate(&e);
        match addr {
            Ok(address) => results.push(address),
            Err(err) => {
                error!(
                    "Invalid erc20 {} found in gravity monitored erc20s: {}",
                    e, err
                );
                return Err(err.into());
            }
        }
    }
    Ok(results)
}

/// Collects the block_height value from each of the input *Event collections
pub async fn get_heights_from_eth_claims(
    deposits: &[SendToCosmosEvent],
    withdraws: &[TransactionBatchExecutedEvent],
    erc20_deploys: &[Erc20DeployedEvent],
    logic_calls: &[LogicCallExecutedEvent],
    valsets: &[ValsetUpdatedEvent],
) -> Vec<Uint256> {
    let mut heights = HashSet::new();
    for d in deposits {
        heights.insert(d.block_height.clone());
    }
    for w in withdraws {
        heights.insert(w.block_height.clone());
    }
    for e in erc20_deploys {
        heights.insert(e.block_height.clone());
    }
    for l in logic_calls {
        heights.insert(l.block_height.clone());
    }
    for v in valsets {
        heights.insert(v.block_height.clone());
    }

    heights.into_iter().collect::<Vec<Uint256>>()
}

/// Fetches the balances of the Gravity.sol contract balance of each erc20 at each ethereum height provided
/// Does not populate the result for a height if any eth balance could not be obtained
pub async fn collect_eth_balances_at_heights(
    web3: &Web3,
    gravity_contract: EthAddress,
    erc20s: &[EthAddress],
    heights: &[Uint256],
) -> Result<HashMap<Uint256, Vec<ProtoErc20Token>>, GravityError> {
    let mut balances_by_height = HashMap::new();
    for h in heights {
        let bals = collect_eth_balances_at_height(web3, gravity_contract, erc20s, h.clone()).await;
        if bals.is_err() {
            info!(
                "Could not query gravity eth balances at height {}",
                h.to_string()
            );
            continue;
        }
        balances_by_height.insert(h.clone(), bals.unwrap());
    }

    Ok(balances_by_height)
}

/// Fetches the balances of the Gravity.sol contract at the provided ethereum block height
/// Returns an error if any of the underlying queries return an error
pub async fn collect_eth_balances_at_height(
    web3: &Web3,
    gravity_contract: EthAddress,
    erc20s: &[EthAddress],
    height: Uint256,
) -> Result<Vec<ProtoErc20Token>, GravityError> {
    let mut futs = vec![];
    for e in erc20s {
        futs.push(web3.get_erc20_balance_at_height(*e, gravity_contract, Some(height.clone())));
    }
    let res = join_all(futs).await;
    let mut results = vec![];
    // Order of res is preserved, so we can assign the erc20 by index
    for (i, r) in res.into_iter().enumerate() {
        let erc20 = erc20s[i];
        results.push(ProtoErc20Token {
            contract: erc20.to_string(),
            amount: r?.to_string(),
        });
    }

    Ok(results)
}
