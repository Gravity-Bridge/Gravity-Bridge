const DEFAULT_DOMAIN: &str = "localhost";
const DEFAULT_PORT: u16 = 8545;
const EVM_CHAIN_ID: u64 = 999999;

use actix_cors::Cors;
use actix_web::{post, web, App, HttpResponse, HttpServer};
use log::{debug, info};
use rustls::pki_types::pem::PemObject;
use rustls::pki_types::{CertificateDer, PrivateKeyDer};
use rustls::ServerConfig;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::sync::Mutex;
use std::time::{Duration, Instant};

use lazy_static::lazy_static;

const INCR_BLOCK_SECONDS: u64 = 10;

lazy_static!(
    // Used to spoof block progression, incremented on a request every INCR_BLOCK_SECONDS
    static ref BLOCK_NUMBER: Mutex<u64> = Mutex::new(64u64);
    // Used to track when the BLOCK_NUMBER last changed
    static ref BLOCK_TIME: Mutex<Instant> = Mutex::new(Instant::now());
);

#[derive(Deserialize, Debug)]
#[allow(dead_code)]
pub struct RequestBody {
    method: String,
    id: Value,
    params: Option<Value>,
    jsonrpc: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct Response<R = Value> {
    pub id: Value,
    pub jsonrpc: String,
    pub result: R,
}

pub fn int_to_hex(i: u64) -> String {
    format!("0x{i:02x}")
}

pub fn jsonrpc_response<R>(r: R, id: Value) -> Response<R> {
    Response::<R> {
        id,
        jsonrpc: "2.0".into(),
        result: r,
    }
}

// Fetches the number in the BLOCK_NUMBER mutex
pub fn get_block_number() -> u64 {
    *(*BLOCK_NUMBER).lock().unwrap()
}

// Increments the number in the BLOCK_NUMBER mutex, returning the value it held before this call
// additionally updating the BLOCK_TIME mutex with the current instant
pub fn get_and_incr_block_number() -> u64 {
    let mut bt = (*BLOCK_TIME).lock().unwrap();
    let mut bn = (*BLOCK_NUMBER).lock().unwrap();
    let n = *bn;
    *bn += 1;
    *bt = Instant::now();

    n
}

/// This is a helper api endpoint to serve Gravity's EVM Chain ID to wallets like MetaMask which require a valid response
/// for network configuration. All requests should be made to the root endpoint but have a "method" in the response body.
/// See https://ethereum.org/en/developers/docs/apis/json-rpc/#json-rpc-methods for more Ethereum JSONRPC methods that
/// may need to be implemented for MetaMask to function.
#[post("/")]
async fn request_dispatcher(req_body: web::Json<RequestBody>) -> HttpResponse {
    debug!("Got request: {req_body:?}");
    match req_body.method.as_str() {
        "net_version" => net_version(req_body.into_inner()).await,
        "eth_chainId" => eth_chainId(req_body.into_inner()).await,
        "eth_blockNumber" => eth_blockNumber(req_body.into_inner()).await,
        "eth_getBlockByNumber" => eth_getBlockByNumber(req_body.into_inner()).await,
        "eth_getBalance" => eth_getBalance(req_body.into_inner()).await,
        "eth_feeHistory" => eth_feeHistory(req_body.into_inner()).await,
        "eth_gasPrice" => eth_gasPrice(req_body.into_inner()).await,
        _ => default_response(),
    }
}

/// Returns a hardcoded response for the "net_version" Ethereum JSONRPC method
async fn net_version(req_body: RequestBody) -> HttpResponse {
    let res = jsonrpc_response(EVM_CHAIN_ID, req_body.id);
    HttpResponse::Ok().json(res)
}

/// Returns a hardcoded response for the "eth_chainId" Ethereum JSONRPC method, which is a standard JSONRPC response
/// including a "result" value in hexadecimal format. e.g. Ethereum mainnet responds with:
///     {"jsonrpc":"2.0", "id":1, "result":"0x1"}
#[allow(non_snake_case)]
async fn eth_chainId(req_body: RequestBody) -> HttpResponse {
    let res = jsonrpc_response(int_to_hex(EVM_CHAIN_ID), req_body.id);
    HttpResponse::Ok().json(res)
}

/// Returns a semi-hardcoded response for the "eth_blockNumber" Ethereum JSONRPC method
/// The block number will be incremented if over INCR_BLOCK_SECONDS have elapsed
#[allow(non_snake_case)]
async fn eth_blockNumber(req_body: RequestBody) -> HttpResponse {
    // Potentially increment the block number if a new minute has elapsed
    let time_since_block = { Instant::now() - *BLOCK_TIME.lock().unwrap() };
    let bn = if time_since_block > Duration::from_secs(INCR_BLOCK_SECONDS) {
        get_and_incr_block_number()
    } else {
        get_block_number()
    };

    let res = jsonrpc_response(int_to_hex(bn), req_body.id);
    HttpResponse::Ok().json(res)
}

/// Returns a spoofed block response for the "eth_blockNumber" Ethereum JSONRPC method
#[allow(non_snake_case)]
async fn eth_getBlockByNumber(req_body: RequestBody) -> HttpResponse {
    let params = req_body.params.unwrap_or_default();
    let block_number = params.as_array().unwrap().first().unwrap();
    let body = format!(
        r#"{{"baseFeePerGas":"0x60667e448","difficulty":"0x0","extraData":"0x6265617665726275696c642e6f7267","gasLimit":"0x1c9c380","gasUsed":"0x1419a2b","hash":"0x7ef1be67e10135f0ef82d9ba0172eceaccfa7e646e7[0/1129]9fbd1f8e46281","logsBloom":"0xd1b3199265a9028c81885c50b298192180d048e1000770103f83e9a6f6bb3113d05736ecd00c6760095183204614416db6a52038ff27b34b96aa06b8923bbdf88bd4de0f05e48928f800f9ef42ca10b801b3004b3dec5c0945907a70883d005099246d44120eac988765a26022c16d50c0c1e630090ebee8d655ee90016f0144a508b5ed0ac2877a15fe702cd7b2ce47450688e9ed2b42bcdcf52bf0d1b106a93bf139e077626e410b8840971c665c310456c26a5025ab2d41cfaf1753595f7ef4d0109611c95ceb6b40188945abd49c2369e0fb5f6ed4346a5d09e7875962543dd1a91880854670299d0e27fb64bb3c2520d1041bd82d4e83c9ad12681d2c1f","miner":"0x95222290dd7278aa3ddd389cc1e1d165cc4bafe5","mixHash":"0x123c22aa38b9258149878e7e01eb589221d16916fe54ab338136378adea40069","nonce":"0x0000000000000000","number":"{block_number}","parentHash":"0x17e54d73dd8a4acd401298644d19cad29b2eb16a1eb29b285bbf607d19daa260","receiptsRoot":"0xc5f93242aae63dacfc9185693268d13dd3932cc1101f9c3295d596a705ab05fa","sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347","size":"0x57df0","stateRoot":"0xd656b01c88b05b6ee5312ecac8b7b09bbe9e9df18f98b27e479cd77300f009d7","timestamp":"0x64d65a7b","totalDifficulty":"0xc70d815d562d3cfa955","transactions":["0x87e44c8bb3c3baecf245c01d8eba493e26238c357702e6ea9b084edfd3cd32cf"],"transactionsRoot":"0x01b20a1f122b64e63a77e9bede89596ba929209dcaa719064ccdfd708973bb60","uncles":[],"withdrawals":[{{"index":"0xd168a3","validatorIndex":"0x5480c","address":"0xb9d7934878b5fb9610b3fe8a5e441e8fad7e293f","amount":"0xe7a6d9"}}],"withdrawalsRoot":"0x56c41a678621930d4bae47118f22dcd8bb465a3f14e29c0de28481d4c6b451da"}}"#
    );
    // let body = format!(r#"{{"baseFeePerGas":"0x60667e448","number":"{block_number}"}}"#);
    let res = jsonrpc_response(body, req_body.id);
    HttpResponse::Ok().json(res)
}

/// Returns a spoofed block fee history response for the "eth_feeHistory" Ethereum JSONRPC method
/// In manual testing it appears MetaMask will query for the 5 most recent blocks. The request is in `req_body` so it is possible to generalize,
/// however the response format is flexible so hardcoding 5 value responses is likely to work for a while.
/// We spoof the response by returning 5 values (rewards [0, 0, 0]; baseFeePerGas 0x7; gasUsedRatio 0) and
/// use current block number - 5 as the oldestBlock
#[allow(non_snake_case)]
async fn eth_feeHistory(req_body: RequestBody) -> HttpResponse {
    let block_number = get_block_number();
    let five_blocks_back = block_number - 5;
    let body = format!(
        r#"{{"oldestBlock":"{}","reward":[["0x0","0x0","0x0"],["0x0","0x0","0x0"],["0x0","0x0","0x0"],["0x0","0x0","0x0"],["0x0","0x0","0x0"]],"baseFeePerGas":["0x7","0x7","0x7","0x7","0x7"],"gasUsedRatio":[0,0,0,0,0]}}"#,
        int_to_hex(five_blocks_back)
    );
    let res = jsonrpc_response(body, req_body.id);
    HttpResponse::Ok().json(res)
}

#[allow(non_snake_case)]
async fn eth_gasPrice(req_body: RequestBody) -> HttpResponse {
    let body = "0x4ffce8fc3".to_string();
    let res = jsonrpc_response(body, req_body.id);
    HttpResponse::Ok().json(res)
}

/// Returns a hardcoded response for the "eth_getBalance" Ethereum JSONRPC method
#[allow(non_snake_case)]
async fn eth_getBalance(req_body: RequestBody) -> HttpResponse {
    let res = jsonrpc_response(int_to_hex(1000000000000000000), req_body.id);
    HttpResponse::Ok().json(res)
}

// Returns a failure response to indicate that the request failed
fn default_response() -> HttpResponse {
    HttpResponse::NotAcceptable().finish()
}

pub async fn run(
    domain: Option<String>,
    port: Option<String>,
    use_ssl: Option<bool>,
    cert_chain_path: Option<String>,
    cert_key_path: Option<String>,
) -> std::io::Result<()> {
    openssl_probe::init_ssl_cert_env_vars();

    let domain = domain.unwrap_or(DEFAULT_DOMAIN.to_string());
    let port = port.unwrap_or(DEFAULT_PORT.to_string());
    let use_ssl = use_ssl.unwrap_or(false);
    let cert_chain_path =
        cert_chain_path.unwrap_or(format!("/etc/letsencrypt/live/{}/fullchain.pem", domain));
    let cert_key_path =
        cert_key_path.unwrap_or(format!("/etc/letsencrypt/live/{}/privkey.pem", domain));

    let server = HttpServer::new(move || {
        App::new()
            .wrap(
                Cors::default()
                    .allow_any_origin()
                    .allow_any_header()
                    .allow_any_method(),
            )
            .service(request_dispatcher)
    });

    let server = if use_ssl {
        let cert_chain = CertificateDer::pem_file_iter(cert_chain_path)
            .unwrap()
            .map(|cert| cert.unwrap())
            .collect();
        let keys = PrivateKeyDer::from_pem_file(cert_key_path).unwrap();
        let config = ServerConfig::builder()
            .with_no_client_auth()
            .with_single_cert(cert_chain, keys)
            .unwrap();
        info!("Binding to SSL");
        server.bind_rustls_0_23(format!("{}:{}", domain, port), config)?
    } else {
        server.bind(format!("{}:{}", domain, port))?
    };

    server.run().await?;

    Ok(())
}
