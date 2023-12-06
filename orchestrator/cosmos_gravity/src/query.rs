use std::convert::TryFrom;

use clarity::Address as EthAddress;
use deep_space::address::Address;
use deep_space::error::CosmosGrpcError;
use deep_space::Contact;
use gravity_proto::auction::query_client::QueryClient as AuctionQueryClient;
use gravity_proto::auction::Params as AuctionParams;
use gravity_proto::auction::QueryParamsRequest as QueryAuctionParamsRequest;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::Params;
use gravity_proto::gravity::QueryAttestationsRequest;
use gravity_proto::gravity::QueryBatchConfirmsRequest;
use gravity_proto::gravity::QueryBatchFeeRequest;
use gravity_proto::gravity::QueryBatchFeeResponse;
use gravity_proto::gravity::QueryCurrentValsetRequest;
use gravity_proto::gravity::QueryDenomToErc20Request;
use gravity_proto::gravity::QueryDenomToErc20Response;
use gravity_proto::gravity::QueryErc20ToDenomRequest;
use gravity_proto::gravity::QueryErc20ToDenomResponse;
use gravity_proto::gravity::QueryLastEventNonceByAddrRequest;
use gravity_proto::gravity::QueryLastPendingBatchRequestByAddrRequest;
use gravity_proto::gravity::QueryLastPendingLogicCallByAddrRequest;
use gravity_proto::gravity::QueryLastPendingValsetRequestByAddrRequest;
use gravity_proto::gravity::QueryLastValsetRequestsRequest;
use gravity_proto::gravity::QueryLogicConfirmsRequest;
use gravity_proto::gravity::QueryOutgoingLogicCallsRequest;
use gravity_proto::gravity::QueryOutgoingTxBatchesRequest;
use gravity_proto::gravity::QueryParamsRequest;
use gravity_proto::gravity::QueryPendingSendToEth;
use gravity_proto::gravity::QueryPendingSendToEthResponse;
use gravity_proto::gravity::QueryValsetConfirmsByNonceRequest;
use gravity_proto::gravity::QueryValsetRequestRequest;
use gravity_proto::gravity::{Attestation, PendingIbcAutoForward, QueryPendingIbcAutoForwards};
use gravity_utils::error::GravityError;
use gravity_utils::types::*;
use tonic::transport::Channel;

/// Gets the Gravity module parameters from the Gravity module
pub async fn get_gravity_params(
    client: &mut GravityQueryClient<Channel>,
) -> Result<Params, GravityError> {
    let request = client.params(QueryParamsRequest {}).await?.into_inner();
    Ok(request.params.unwrap())
}

/// get the valset for a given nonce (block) height
pub async fn get_valset(
    client: &mut GravityQueryClient<Channel>,
    nonce: u64,
) -> Result<Option<Valset>, GravityError> {
    let request = client
        .valset_request(QueryValsetRequestRequest { nonce })
        .await?;
    let valset = request.into_inner().valset;
    let valset = valset.map(|v| v.into());
    Ok(valset)
}

/// get the current valset. You should never sign this valset
/// valset requests create a consensus point around the block height
/// that transaction got in. Without that consensus point everyone trying
/// to sign the 'current' valset would run into slight differences and fail
/// to produce a viable update.
pub async fn get_current_valset(
    client: &mut GravityQueryClient<Channel>,
) -> Result<Valset, GravityError> {
    let request = client.current_valset(QueryCurrentValsetRequest {}).await?;
    let valset = request.into_inner().valset;
    if let Some(valset) = valset {
        Ok(valset.into())
    } else {
        error!("Current valset returned None? This should be impossible");
        Err(GravityError::InvalidBridgeStateError(
            "Must have a current valset!".to_string(),
        ))
    }
}

/// This hits the /pending_valset_requests endpoint and will provide
/// an array of validator sets we have not already signed
pub async fn get_oldest_unsigned_valsets(
    client: &mut GravityQueryClient<Channel>,
    address: Address,
    prefix: String,
) -> Result<Vec<Valset>, GravityError> {
    let request = client
        .last_pending_valset_request_by_addr(QueryLastPendingValsetRequestByAddrRequest {
            address: address.to_bech32(prefix).unwrap(),
        })
        .await?;
    let valsets = request.into_inner().valsets;
    // convert from proto valset type to rust valset type
    let valsets = valsets.iter().map(|v| v.into()).collect();
    Ok(valsets)
}

/// this input views the last five valset requests that have been made, useful if you're
/// a relayer looking to ferry confirmations
pub async fn get_latest_valsets(
    client: &mut GravityQueryClient<Channel>,
) -> Result<Vec<Valset>, GravityError> {
    let request = client
        .last_valset_requests(QueryLastValsetRequestsRequest {})
        .await?;
    let valsets = request.into_inner().valsets;
    Ok(valsets.iter().map(|v| v.into()).collect())
}

/// get all valset confirmations for a given nonce
pub async fn get_all_valset_confirms(
    client: &mut GravityQueryClient<Channel>,
    nonce: u64,
) -> Result<Vec<ValsetConfirmResponse>, GravityError> {
    let request = client
        .valset_confirms_by_nonce(QueryValsetConfirmsByNonceRequest { nonce })
        .await?;
    let confirms = request.into_inner().confirms;
    let mut parsed_confirms = Vec::new();
    for item in confirms {
        let v: ValsetConfirmResponse = ValsetConfirmResponse::try_from(&item)?;
        parsed_confirms.push(v)
    }
    Ok(parsed_confirms)
}

pub async fn get_oldest_unsigned_transaction_batches(
    client: &mut GravityQueryClient<Channel>,
    address: Address,
    prefix: String,
) -> Result<Vec<TransactionBatch>, GravityError> {
    let request = client
        .last_pending_batch_request_by_addr(QueryLastPendingBatchRequestByAddrRequest {
            address: address.to_bech32(prefix).unwrap(),
        })
        .await?;
    let batches = request.into_inner().batch;

    let mut ret_batches = Vec::new();

    for batch in batches {
        ret_batches.push(TransactionBatch::try_from(batch)?);
    }
    Ok(ret_batches)
}

/// gets the latest 100 transaction batches, regardless of token type
/// for relayers to consider relaying
pub async fn get_latest_transaction_batches(
    client: &mut GravityQueryClient<Channel>,
) -> Result<Vec<TransactionBatch>, GravityError> {
    let request = client
        .outgoing_tx_batches(QueryOutgoingTxBatchesRequest {})
        .await?;
    let batches = request.into_inner().batches;
    let mut out = Vec::new();
    for batch in batches {
        out.push(TransactionBatch::try_from(batch)?)
    }
    Ok(out)
}

/// get all batch confirmations for a given nonce and denom
pub async fn get_transaction_batch_signatures(
    client: &mut GravityQueryClient<Channel>,
    nonce: u64,
    contract_address: EthAddress,
) -> Result<Vec<BatchConfirmResponse>, GravityError> {
    let request = client
        .batch_confirms(QueryBatchConfirmsRequest {
            nonce,
            contract_address: contract_address.to_string(),
        })
        .await?;
    let batch_confirms = request.into_inner().confirms;
    let mut out = Vec::new();
    for confirm in batch_confirms {
        out.push(BatchConfirmResponse::try_from(confirm)?)
    }
    Ok(out)
}

/// Gets the last event nonce that a given validator has attested to, this lets us
/// catch up with what the current event nonce should be if a oracle is restarted
pub async fn get_last_event_nonce_for_validator(
    client: &mut GravityQueryClient<Channel>,
    address: Address,
    prefix: String,
) -> Result<u64, GravityError> {
    let request = client
        .last_event_nonce_by_addr(QueryLastEventNonceByAddrRequest {
            address: address.to_bech32(prefix).unwrap(),
        })
        .await?;
    Ok(request.into_inner().event_nonce)
}

/// Gets the 100 latest logic calls for a relayer to consider relaying
pub async fn get_latest_logic_calls(
    client: &mut GravityQueryClient<Channel>,
) -> Result<Vec<LogicCall>, GravityError> {
    let request = client
        .outgoing_logic_calls(QueryOutgoingLogicCallsRequest {})
        .await?;
    let calls = request.into_inner().calls;
    let mut out = Vec::new();
    for call in calls {
        out.push(LogicCall::try_from(call)?);
    }
    Ok(out)
}

pub async fn get_logic_call_signatures(
    client: &mut GravityQueryClient<Channel>,
    invalidation_id: Vec<u8>,
    invalidation_nonce: u64,
) -> Result<Vec<LogicCallConfirmResponse>, GravityError> {
    let request = client
        .logic_confirms(QueryLogicConfirmsRequest {
            invalidation_id,
            invalidation_nonce,
        })
        .await?;
    let call_confirms = request.into_inner().confirms;
    let mut out = Vec::new();
    for confirm in call_confirms {
        out.push(LogicCallConfirmResponse::try_from(confirm)?)
    }
    Ok(out)
}

pub async fn get_oldest_unsigned_logic_calls(
    client: &mut GravityQueryClient<Channel>,
    address: Address,
    prefix: String,
) -> Result<Vec<LogicCall>, GravityError> {
    let request = client
        .last_pending_logic_call_by_addr(QueryLastPendingLogicCallByAddrRequest {
            address: address.to_bech32(prefix).unwrap(),
        })
        .await?;
    let calls = request.into_inner().call;

    let mut ret_calls = Vec::new();

    for call in calls {
        ret_calls.push(LogicCall::try_from(call)?);
    }
    Ok(ret_calls)
}

pub async fn get_attestations(
    client: &mut GravityQueryClient<Channel>,
    limit: Option<u64>,
) -> Result<Vec<Attestation>, GravityError> {
    let request = client
        .get_attestations(QueryAttestationsRequest {
            limit: limit.unwrap_or(50u64),
            order_by: String::new(),
            claim_type: String::new(),
            nonce: 0,
            height: 0,
            use_v1_key: false,
        })
        .await?;
    let attestations = request.into_inner().attestations;
    Ok(attestations)
}

/// Get a list of transactions going to the EVM blockchain that are pending for a given user.
pub async fn get_pending_send_to_eth(
    client: &mut GravityQueryClient<Channel>,
    sender_address: Address,
) -> Result<QueryPendingSendToEthResponse, GravityError> {
    let request = client
        .get_pending_send_to_eth(QueryPendingSendToEth {
            sender_address: sender_address.to_string(),
        })
        .await?;
    Ok(request.into_inner())
}

/// Gets erc20 for a given denom, this can take two forms a gravity0x... address where it's really
/// just stripping the gravity prefix, or it can take a native asset like 'gravity' and return a erc20
/// contract that represents it. This later case is also true for IBC coins
pub async fn get_denom_to_erc20(
    client: &mut GravityQueryClient<Channel>,
    denom: String,
) -> Result<QueryDenomToErc20Response, GravityError> {
    let request = client
        .denom_to_erc20(QueryDenomToErc20Request { denom })
        .await?;
    Ok(request.into_inner())
}

/// looks up the correct erc20 to represent this denom, for Ethereum originated assets this is just
/// prefixing with 'gravity' but for native or IBC assets a lookup will be performed for the correct
/// adopted address
pub async fn get_erc20_to_denom(
    client: &mut GravityQueryClient<Channel>,
    erc20: EthAddress,
) -> Result<QueryErc20ToDenomResponse, GravityError> {
    let request = client
        .erc20_to_denom(QueryErc20ToDenomRequest {
            erc20: erc20.to_string(),
        })
        .await?;
    Ok(request.into_inner())
}

/// Get a list of fees for all pending batches
pub async fn get_pending_batch_fees(
    client: &mut GravityQueryClient<Channel>,
) -> Result<QueryBatchFeeResponse, GravityError> {
    let request = client.batch_fees(QueryBatchFeeRequest {}).await?;
    Ok(request.into_inner())
}

/// Queries the Gravity chain for Pending Ibc Auto Forwards, returning an empty vec if there is an error
pub async fn get_all_pending_ibc_auto_forwards(
    grpc_client: &mut GravityQueryClient<Channel>,
) -> Vec<PendingIbcAutoForward> {
    let pending_forwards = grpc_client
        .get_pending_ibc_auto_forwards(QueryPendingIbcAutoForwards { limit: 0 })
        .await;
    if let Err(status) = pending_forwards {
        // don't print errors during the upgrade test, which involves running
        // a newer orchestrator against an older chain due to current design limitations.
        if !status.message().contains("unknown method") {
            warn!(
                "Received an error when querying for pending ibc auto forwards: {}",
                status.message()
            );
        }
        return vec![];
    }

    let pending_forwards = pending_forwards.unwrap();
    pending_forwards.into_inner().pending_ibc_auto_forwards
}

// Fetches the MinChainFeeBasisPoints param from the Gravity module, parsing into a u64.
// If no value is set, returns 0. Panics if an invalid value is set.
pub async fn get_min_chain_fee_basis_points(contact: &Contact) -> Result<u64, CosmosGrpcError> {
    // Get the current minimum fee parameter
    let fee_param = contact
        .get_param("gravity", "MinChainFeeBasisPoints")
        .await?
        .param;

    // Wrap whatever result we produce in an Ok()
    Ok(match fee_param {
        Some(param) => {
            let v = param.value.trim_matches('"');
            if v.is_empty() {
                0u64
            } else {
                serde_json::from_str(v).unwrap()
            }
        }
        None => 0u64,
    })
}

// Gets the auction module params
pub async fn get_auction_module_params(
    contact: &Contact,
) -> Result<AuctionParams, CosmosGrpcError> {
    let mut auction_qc = AuctionQueryClient::connect(contact.get_url()).await?;

    let params = auction_qc
        .params(QueryAuctionParamsRequest {})
        .await?
        .into_inner()
        .params
        .ok_or(CosmosGrpcError::BadResponse(
            "no params returned".to_string(),
        ))?;

    Ok(params)
}
