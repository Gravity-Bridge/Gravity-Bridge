use crate::args::SpotRelayOpts;
use crate::utils::TIMEOUT;
use clarity::Address as EthAddress;
use cosmos_gravity::query::{
    get_gravity_params, get_latest_transaction_batches, get_pending_batch_fees,
    get_transaction_batch_signatures,
};
use cosmos_gravity::send::send_request_batch;
use ethereum_gravity::message_signatures::encode_tx_batch_confirm_hashed;
use ethereum_gravity::submit_batch::send_eth_transaction_batch;
use gravity_proto::gravity::query_client::QueryClient;
use gravity_proto::gravity::{QueryDenomToErc20Request, QueryErc20ToDenomRequest};
use gravity_utils::connection_prep::{check_for_eth, create_rpc_connections};
use gravity_utils::types::TransactionBatch;
use relayer::find_latest_valset::find_latest_valset;
use std::process::exit;
use tonic::transport::Channel;

pub async fn spot_relay(args: SpotRelayOpts, address_prefix: String) {
    let grpc_url = args.cosmos_grpc;
    let ethereum_rpc = args.ethereum_rpc;
    let ethereum_key = args.ethereum_key;

    let connections =
        create_rpc_connections(address_prefix, Some(grpc_url), Some(ethereum_rpc), TIMEOUT).await;
    let web3 = connections.web3.unwrap();
    let contact = connections.contact.unwrap();

    let mut grpc = connections.grpc.unwrap();

    let ethereum_public_key = ethereum_key.to_address();
    check_for_eth(ethereum_public_key, &web3).await;

    let params = get_gravity_params(&mut grpc).await.unwrap();
    let gravity_id = params.gravity_id;

    let gravity_contract_address = if let Some(c) = args.gravity_contract_address {
        c
    } else {
        let c = params.bridge_ethereum_address.parse();
        if c.is_err() {
            error!("The Gravity address is not yet set as a chain parameter! You must specify --gravity-contract-address");
            exit(1);
        }
        c.unwrap()
    };

    // init complete, now we have to figure out what token the user has asked us to relay
    let gravity_denom = user_token_name_to_gravity_token(args.token.clone(), &mut grpc).await;
    let gravity_denom = match gravity_denom {
        Some(gd) => gd,
        None => {
            error!("Failed to decode your intended token name {}", args.token);
            return;
        }
    };
    let ethereum_erc20: EthAddress = grpc
        .denom_to_erc20(QueryDenomToErc20Request {
            denom: gravity_denom.clone(),
        })
        .await
        .unwrap()
        .into_inner()
        .erc20
        .parse()
        .unwrap();

    // first we check if there's an active not timed out batch waiting

    let latest_eth_height = web3.eth_block_number().await.unwrap();
    let latest_batches = get_latest_transaction_batches(&mut grpc)
        .await
        .expect("Failed to get latest batches");
    // in theory there can be multiple valid batches for each token type of increasing profitability
    // the current logic here will pick the most profitable one
    let mut target_batch: Option<TransactionBatch> = None;
    for current_batch in latest_batches {
        if current_batch.token_contract == ethereum_erc20 {
            if let Some(t) = target_batch.clone() {
                if t.total_fee.amount < current_batch.total_fee.amount {
                    target_batch = Some(current_batch);
                }
            } else {
                target_batch = Some(current_batch);
            }
        }
    }
    // we have the most profitable batch in queue for this type
    if let Some(batch) = target_batch {
        if latest_eth_height > batch.batch_timeout.into() {
            warn!(
                "Found pending batch {} for {}, but it has timed out",
                batch.nonce, args.token
            );
            warn!("These tokens will return to the pool and be rebatched when the next deposit / batch to the bridge goes through");
            warn!("The best way to speed this along would be to make a small deposit via the normal frontend. Registering that deposit will free this batch");
            return;
        }
        info!("Found pending batch {} for {}", batch.nonce, args.token);
        let sigs = get_transaction_batch_signatures(&mut grpc, batch.nonce, ethereum_erc20)
            .await
            .expect("Failed to get sigs for batch!");
        if sigs.is_empty() {
            panic!("Failed to get sigs for batch");
        }

        let current_valset = find_latest_valset(&mut grpc, gravity_contract_address, &web3).await;
        if current_valset.is_err() {
            error!("Could not get current valset! {:?}", current_valset);
            return;
        }
        let current_valset = current_valset.unwrap();

        // this checks that the signatures for the batch are actually possible to submit to the chain
        let hash = encode_tx_batch_confirm_hashed(gravity_id.clone(), batch.clone());

        if let Err(e) = current_valset.order_sigs(&hash, &sigs, false) {
            error!("Current validator set is not valid to relay this batch, a validator set update must be submitted!");
            error!("{:?}", e);
            return;
        }

        info!(
            "Attempting to relay batch {} for {} with reward of {} base units",
            batch.nonce, args.token, batch.total_fee.amount
        );

        let res = send_eth_transaction_batch(
            current_valset.clone(),
            batch,
            &sigs,
            &web3,
            TIMEOUT,
            gravity_contract_address,
            gravity_id.clone(),
            ethereum_key,
        )
        .await;
        match res {
            Ok(_) => info!("Batch submission was successful! Check Etherscan and your wallet"),
            Err(e) => error!("Batch submission has failed {:?}", e),
        }
        return;
    }

    // in this case there are no pending batches of this type we need to request a batch if possible

    let batch_fees = get_pending_batch_fees(&mut grpc).await.unwrap();
    let mut batch_to_reqeust = None;
    for potential_batch in batch_fees.batch_fees {
        let token: EthAddress = potential_batch.token.parse().unwrap();
        if token == ethereum_erc20 {
            batch_to_reqeust = Some(potential_batch);
        }
    }

    match batch_to_reqeust {
        Some(btr) => {

    if let Some(cosmos_key) = args.cosmos_phrase {
        info!("{} transactions for token type {} where found in the queue requesting a batch", args.token, btr.tx_count);
        let res = send_request_batch(
            cosmos_key,
            gravity_denom,
            args.fees,
            &contact,
        )
        .await;

        match res {
            Ok(_) => info!("Batch successfully requested, please wait about 60 seconds and run this command again to relay it"),
            Err(e) => {
                if e.to_string().contains("would not be more profitable") {
                    info!("Batch would not have been more profitable, no new batch created, how did you get here?");
                } else {
                    warn!("Failed to request batch with {:?}", e);
                }

            },
        }

    } else {
        warn!("No pending batches for token type {} where found. A potential batch with {} transactions could be requested.", args.token, btr.tx_count);
        warn!("You have not provided a Gravity private key to send the batch request (this account only needs the smallest amount of dust for fees)");
        warn!("Please re-run this command with the --cosmos-private-key argument filled out");
    }
        },
        None => info!("There is no pending batch or pending transactions to create a batch for {} please check the token name you are using", args.token),
    }
}

/// This takes a huamn input of a token name and translates it to the gravity denom that we need to operate
async fn user_token_name_to_gravity_token(
    user_provided_token_name: String,
    grpc: &mut QueryClient<Channel>,
) -> Option<String> {
    // the token we are actually relaying after decoding, this is the name on the Gravity side
    let mut token_to_relay = None;

    // user provided an eth address, we need to check if it's cosmos originated
    if let Ok(eth_address) = user_provided_token_name.parse() {
        let eth_address: EthAddress = eth_address;
        let denom = grpc
            .erc20_to_denom(QueryErc20ToDenomRequest {
                erc20: eth_address.to_string(),
            })
            .await
            .unwrap();
        token_to_relay = Some(denom.into_inner().denom);
    }
    // user provided a gravity token name or ibc denom, this is good as is
    if user_provided_token_name.starts_with("gravity0x")
        | user_provided_token_name.starts_with("ibc/")
    {
        token_to_relay = Some(user_provided_token_name.clone())
    }
    if let Some(denom) = lookup_human_readable_denom(user_provided_token_name) {
        token_to_relay = Some(denom)
    }

    token_to_relay
}

/// provides a simple lookup mapping of common token names, returns none if mapping is not
/// successfull
fn lookup_human_readable_denom(user_input: String) -> Option<String> {
    const LOOKUP_TABLE: [(&str, &str); 27] = [
        ("USDC", "gravity0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"),
        ("USDT", "gravity0xdAC17F958D2ee523a2206206994597C13D831ec7"),
        ("grav", "ugraviton"),
        ("gravity", "ugraviton"),
        ("weth", "gravity0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"),
        ("eth", "gravity0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"),
        (
            "ethereum",
            "gravity0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
        ),
        ("shib", "gravity0x95aD61b0a150d79219dCF64E1E6Cc01f0B64C4cE"),
        ("pepe", "gravity0x6982508145454Ce325dDbE47a25d4ec3d2311933"),
        ("wbtc", "gravity0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599"),
        (
            "wsteth",
            "gravity0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0",
        ),
        ("steth", "gravity0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0"),
        (
            "nym",
            "ibc/0C273962C274B2C05B22D9474BFE5B84D6A6FCAD198CB9B0ACD35EA521A36606",
        ),
        (
            "atom",
            "ibc/2E5D0AC026AC1AFA65A23023BA4F24BB8DDF94F118EDC0BAD6F625BFC557CDED",
        ),
        (
            "cmdx",
            "ibc/29A7122D024B5B8FA8A2EFBB4FA47272C25C8926AA005A96807127208082DAB3",
        ),
        (
            "comdex",
            "ibc/29A7122D024B5B8FA8A2EFBB4FA47272C25C8926AA005A96807127208082DAB3",
        ),
        (
            "stars",
            "ibc/4F393C3FCA4190C0A6756CE7F6D897D5D1BE57D6CCB80D0BC87393566A7B6602",
        ),
        (
            "huahua",
            "ibc/048BE20AE2E6BFD4142C547E04F17E5F94363003A12B7B6C084E08101BFCF7D1",
        ),
        (
            "mntl",
            "ibc/00F2B62EB069321A454B708876476AFCD9C23C8C9C4A5A206DDF1CD96B645057",
        ),
        (
            "fund",
            "ibc/D157AD8A50DAB0FC4EB95BBE1D9407A590FA2CDEE04C90A76C005089BF76E519",
        ),
        (
            "lunc",
            "ibc/7F04F5D641285808E1C6A01D266C3D9BE1C23473BF3D01AC31E621CFF72DBF24",
        ),
        (
            "ustc",
            "ibc/50896BE248180B0341B4A679CF49249ECF70032AF1307BFAF0233D35F0D25665",
        ),
        (
            "kuji",
            "ibc/6BEE6DBC35E5CCB3C8ADA943CF446735E6A3D48B174FEE027FAB3410EDE6319C",
        ),
        (
            "usk",
            "ibc/AD355DD10DF3C25CD42B5812F34077A1235DF343ED49A633B4E76AE98F3B78BC",
        ),
        (
            "plq",
            "ibc/2782B87D755389B565D59F15E202E6E3B8B3E1408034D2FAA4E02A0CA10911B2",
        ),
        (
            "cmst",
            "ibc/A1481E16B62E1C2614E5FB945BABCBDD89E4F8C4E78B60016F531E57BA6B8E21",
        ),
        (
            "juno",
            "ibc/DF8D00B4B31B55AFCA9BAF192BC36C67AA06D9987DCB96490661BCAB63C27006",
        ),
    ];

    for (human_name, denom) in LOOKUP_TABLE {
        if user_input.eq_ignore_ascii_case(human_name) {
            return Some(denom.to_string());
        }
    }
    None
}
