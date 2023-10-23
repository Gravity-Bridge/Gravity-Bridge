use jsonrpc_server::server::run as run_server;

use crate::args::JsonrpcServerOpts;

pub async fn jsonrpc_server(args: JsonrpcServerOpts) {
    let res = run_server(
        args.domain,
        args.port,
        args.use_ssl,
        args.cert_chain_path,
        args.cert_key_path,
    )
    .await;

    match res {
        Err(e) => error!("JSONRPC server halted with {}", e.to_string()),
        Ok(_) => info!("JSONRPC server terminated without error"),
    }
}
