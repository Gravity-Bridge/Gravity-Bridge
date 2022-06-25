use cosmos_gravity::{
    query::get_all_pending_ibc_auto_forwards, send::execute_pending_ibc_auto_forwards,
};
use deep_space::{Coin, Contact, CosmosPrivateKey};
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_utils::types::RelayerConfig;
use std::time::{Duration, Instant};
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;

/// This function contains the ibc auto forward executor primary loop, which periodically queries for
/// pending IBC Auto Forwards and submits a MsgExecuteIbcAutoForwards to the gravity chain to handle
/// these pending messages.
/// Note that this is necessary due to a Tendermint bug preventing Gravity from initiating IBC transfers
/// in EndBlocker. Moving the IBC transfers to a queue which can be cleared in a Tx solves the issue.
#[allow(clippy::too_many_arguments)]
pub async fn ibc_auto_forward_loop(
    cosmos_key: Option<CosmosPrivateKey>,
    contact: &Contact,
    grpc_client: GravityQueryClient<Channel>,
    fee: Option<Coin>,
    relayer_config: RelayerConfig,
) {
    let mut grpc_client = grpc_client;

    if cosmos_key.is_none() {
        error!("Unable to execute pending ibc auto forwards with no configured private key!");
        return;
    }
    let cosmos_key = cosmos_key.unwrap();

    let fee = match fee {
        None => Coin {
            amount: 0u8.into(),
            denom: "ugraviton".to_string(),
        },
        Some(f) => f,
    };

    loop {
        let loop_start = Instant::now();
        let pending_forwards = get_all_pending_ibc_auto_forwards(&mut grpc_client).await;
        let should_execute_pending_ibc_auto_forwards = !pending_forwards.is_empty();

        if should_execute_pending_ibc_auto_forwards {
            info!(
                "Executing {}/{} pending ibc auto forwards",
                relayer_config.ibc_auto_forwards_to_execute,
                pending_forwards.len()
            );

            let res = execute_pending_ibc_auto_forwards(
                contact,
                cosmos_key,
                fee.clone(),
                relayer_config.ibc_auto_forwards_to_execute,
            )
            .await;
            if res.is_err() {
                warn!(
                    "Error submitting MsgExecuteIbcAutoForwards! {}",
                    res.err().unwrap()
                )
            }
        }

        // a bit of logic that tries to keep things running every ibc_auto_forward_loop_speed seconds
        // exactly, this is not required for any specific reason.
        let elapsed = Instant::now() - loop_start;
        let loop_speed = Duration::from_secs(relayer_config.ibc_auto_forward_loop_speed);
        if elapsed < loop_speed {
            delay_for(loop_speed - elapsed).await;
        }
    }
}
