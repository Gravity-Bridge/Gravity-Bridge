use crate::args::IbcAutoForwardOpts;
use crate::utils::TIMEOUT;
use cosmos_gravity::query::get_all_pending_ibc_auto_forwards;
use cosmos_gravity::send::execute_pending_ibc_auto_forwards;
use deep_space::Coin;
use gravity_utils::connection_prep::create_rpc_connections;
use std::process::exit;

pub async fn ibc_auto_forward(args: IbcAutoForwardOpts, address_prefix: String) {
    let grpc_url = args.cosmos_grpc;

    let connections = create_rpc_connections(address_prefix, Some(grpc_url), None, TIMEOUT).await;
    let contact = connections.contact.unwrap();
    let mut grpc = connections.grpc.unwrap();

    let cosmos_key = match args.cosmos_phrase {
        Some(phrase) => phrase,
        None => {
            error!("Please provide a Cosmos private key to execute pending IBC auto forwards");
            exit(1);
        }
    };

    let fee = match args.fees {
        None => Coin {
            amount: 0u8.into(),
            denom: "ugraviton".to_string(),
        },
        Some(f) => f,
    };

    let forwards_to_execute = args.forwards_to_execute;

    let pending_forwards = get_all_pending_ibc_auto_forwards(&mut grpc).await;

    if pending_forwards.is_empty() {
        info!("No pending IBC auto forwards found");
        return;
    }

    info!(
        "Found {} pending IBC auto forwards, executing up to {}",
        pending_forwards.len(),
        forwards_to_execute,
    );

    let res =
        execute_pending_ibc_auto_forwards(&contact, cosmos_key, fee, forwards_to_execute).await;

    match res {
        Ok(_) => info!("Successfully executed pending IBC auto forwards"),
        Err(e) => {
            error!("Error submitting MsgExecuteIbcAutoForwards: {}", e);
            exit(1);
        }
    }
}
