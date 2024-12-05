use crate::args::RequestAllBatchesOpts;
use crate::utils::TIMEOUT;
use cosmos_gravity::query::get_pending_batch_fees;
use cosmos_gravity::send::send_request_batch;
use gravity_proto::gravity::QueryErc20ToDenomRequest;
use gravity_utils::connection_prep::create_rpc_connections;
use std::process::exit;

pub async fn request_all_batches(args: RequestAllBatchesOpts, address_prefix: String) {
    let grpc_url = args.cosmos_grpc;

    let connections = create_rpc_connections(address_prefix, Some(grpc_url), None, TIMEOUT).await;
    let contact = connections.contact.unwrap();
    let mut grpc = connections.grpc.unwrap();
    let cosmos_key = match args.cosmos_phrase {
        Some(phrase) => phrase,
        None => {
            error!("Please enter a key phrase to request batches on Gravity");
            exit(1);
        }
    };

    let batch_fees = get_pending_batch_fees(&mut grpc).await.unwrap();
    for potential_batch in batch_fees.batch_fees {
        info!(
            "{} transactions for token type {} where found in the queue requesting a batch",
            potential_batch.token, potential_batch.tx_count
        );
        let denom = grpc
            .erc20_to_denom(QueryErc20ToDenomRequest {
                erc20: potential_batch.token.clone(),
            })
            .await
            .unwrap();
        match send_request_batch(
            cosmos_key,
            denom.into_inner().denom,
            args.fees.clone(),
            &contact,
        )
        .await
        {
            Ok(_) => {
                info!(
                    "Successfully requested a batch for token type {}",
                    potential_batch.token
                );
            }
            Err(e) => {
                error!(
                    "Failed to request a batch for token type {} with error {}",
                    potential_batch.token, e
                );
            }
        }
    }
}
