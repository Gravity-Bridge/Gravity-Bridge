//! This is a test for invalid string based deposits, the goal is to torture test the implementation
//! with every possible variant of invalid data and ensure that in all cases the community pool deposit
//! works correctly. Sanitized deployments are no-ops; unsanitized invalid deployments stop the oracle.

use crate::get_fee;
use crate::happy_path::test_erc20_deposit_panic;
use crate::happy_path_v2::deploy_cosmos_representing_erc20_and_check_adoption;
use crate::one_eth;
use crate::unhalt_bridge::get_nonces;
use crate::utils::create_default_test_config;
use crate::utils::footoken_metadata;
use crate::utils::get_event_nonce_safe;
use crate::utils::get_user_key;
use crate::utils::set_cosmos_bridgeable_tokens;
use crate::utils::start_orchestrators;
use crate::utils::ValidatorKeys;
use crate::MINER_ADDRESS;
use crate::MINER_PRIVATE_KEY;
use crate::TOTAL_TIMEOUT;
use clarity::abi::encode_call;
use clarity::abi::AbiToken as Token;
use clarity::Address as EthAddress;
use clarity::Address;
use cosmos_gravity::query::{get_attestations, get_erc20_to_denom};
use cosmos_gravity::send::send_ethereum_claims;
use deep_space::Contact;
use ethereum_gravity::send_to_cosmos::SEND_TO_COSMOS_GAS_LIMIT;
use gravity_proto::gravity::v1::claim_hash_components::Components;
use gravity_proto::gravity::v1::query_client::QueryClient as GravityQueryClient;
use gravity_utils::types::event_signatures::ERC20_DEPLOYED_EVENT_SIG;
use gravity_utils::types::{Erc20DeployedEvent, EthereumEvent};
use rand::distributions::Alphanumeric;
use rand::thread_rng;
use rand::Rng;
use std::time::Instant;
use tonic::transport::Channel;
use web30::client::Web3;
use web30::types::SendTxOption;

pub async fn invalid_events(
    web30: &Web3,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_address: EthAddress,
    grpc_client: GravityQueryClient<Channel>,
) {
    let mut grpc_client = grpc_client;
    let erc20_denom = format!("gravity{erc20_address}");

    // figure out how many of a given erc20 we already have on startup so that we can
    // keep track of incrementation. This makes it possible to run this test again without
    // having to restart your test chain
    let community_pool_contents = contact.query_community_pool().await.unwrap();
    let mut starting_pool_amount = None;
    for coin in community_pool_contents {
        if coin.denom == erc20_denom {
            starting_pool_amount = Some(coin.amount);
            break;
        }
    }
    if starting_pool_amount.is_none() {
        starting_pool_amount = Some(0u8.into())
    }
    let mut starting_pool_amount = starting_pool_amount.unwrap();

    let no_relay_market_config = create_default_test_config();
    start_orchestrators(
        keys[1..].to_vec(),
        gravity_address,
        false,
        no_relay_market_config,
    )
    .await;

    for test_value in get_deposit_test_strings() {
        // next we send an invalid string deposit, we use byte encoding here so that we can attempt a totally invalid send
        send_to_cosmos_invalid(erc20_address, gravity_address, test_value, web30).await;

        // send some coins across the correct way, make sure they arrive
        // note send_to_cosmos_invalid does not wait for the actual oracle
        // to complete like this function does, since this deposit will have
        // a latter event nonce it will effectively wait for the invalid deposit
        // to complete as well
        let user_keys = get_user_key(None);
        test_erc20_deposit_panic(
            web30,
            contact,
            &mut grpc_client,
            user_keys.cosmos_address,
            gravity_address,
            erc20_address,
            one_eth(),
            None,
            None,
        )
        .await;

        // finally we check that the deposit has been added to the community pool
        let community_pool_contents = contact.query_community_pool().await.unwrap();
        let actual = community_pool_contents
            .iter()
            .find(|coin| coin.denom == erc20_denom)
            .map(|coin| coin.amount)
            .unwrap_or_else(|| 0u8.into());
        starting_pool_amount += one_eth();
        assert_eq!(
            actual, starting_pool_amount,
            "Invalid deposit did not reach the community pool"
        );
    }

    web30.wait_for_next_block(TOTAL_TIMEOUT).await.unwrap();

    // footoken must be on the CosmosBridgeableTokens allowlist or admission
    // will reject the ERC20 deployment even though the metadata is valid
    let footoken = footoken_metadata(contact).await;
    set_cosmos_bridgeable_tokens(contact, &keys, vec![footoken.clone()]).await;

    // Register a valid deployment before exercising rejected mappings.
    let _ = deploy_cosmos_representing_erc20_and_check_adoption(
        gravity_address,
        web30,
        None,
        &mut grpc_client,
        false,
        footoken,
    )
    .await;

    test_sanitized_erc20_deployments(
        web30,
        contact,
        &keys,
        gravity_address,
        erc20_address,
        &mut grpc_client,
    )
    .await;

    let starting_nonces = get_nonces(&mut grpc_client, &keys, &contact.get_prefix()).await;
    let starting_attestations = get_attestations(&mut grpc_client, Some(1000))
        .await
        .unwrap();
    let mut rejected_contracts = Vec::new();
    for test_value in get_unsanitized_erc20_test_values() {
        let event = deploy_invalid_erc20(gravity_address, web30, keys.clone(), test_value).await;
        rejected_contracts.push(event.erc20_address);
        let result = send_ethereum_claims(
            contact,
            keys[0].orch_key,
            vec![],
            vec![],
            vec![event],
            vec![],
            vec![],
            get_fee(None),
        )
        .await;
        let rejection = match result {
            Err(error) => error.to_string(),
            Ok(transaction) => {
                assert_ne!(transaction.code(), 0, "Invalid deployment was admitted");
                transaction.raw_log()
            }
        };
        assert!(
            rejection.contains("CosmosBridgeableTokens whitelist")
                || rejection.contains("collides with an eth-originated gravity denom"),
            "Unexpected invalid deployment rejection: {}",
            rejection
        );
        assert_eq!(
            get_nonces(&mut grpc_client, &keys, &contact.get_prefix()).await,
            starting_nonces,
            "Invalid deployment advanced a validator nonce"
        );
    }
    let attestations = get_attestations(&mut grpc_client, Some(1000))
        .await
        .unwrap();
    assert_eq!(
        attestations, starting_attestations,
        "Invalid deployments must not change the attestation store or observation state"
    );
    for contract in rejected_contracts {
        let mapping = get_erc20_to_denom(&mut grpc_client, contract)
            .await
            .unwrap();
        assert!(
            !mapping.cosmos_originated,
            "Invalid deployment created a Cosmos mapping"
        );
        assert_eq!(mapping.denom, format!("gravity{contract}"));
    }

    info!("Successfully completed the invalid events test")
}

async fn test_sanitized_erc20_deployments(
    web30: &Web3,
    contact: &Contact,
    keys: &[ValidatorKeys],
    gravity_address: EthAddress,
    erc20_address: EthAddress,
    grpc_client: &mut GravityQueryClient<Channel>,
) {
    info!("INVALID_EVENTS: sanitized deployment no-ops preserve oracle progression");
    for test_value in get_erc20_test_values() {
        let event = deploy_invalid_erc20(gravity_address, web30, keys.to_vec(), test_value).await;
        let expected_denom = format!("gravity{}", event.erc20_address);
        assert_eq!(event.cosmos_denom, expected_denom);
        assert!(event.name.is_empty());
        assert!(event.symbol.is_empty());
        assert_eq!(event.decimals, 0);

        let user = get_user_key(None);
        test_erc20_deposit_panic(
            web30,
            contact,
            grpc_client,
            user.cosmos_address,
            gravity_address,
            erc20_address,
            one_eth(),
            None,
            None,
        )
        .await;

        let attestations = get_attestations(grpc_client, Some(1000)).await.unwrap();
        let matching: Vec<_> = attestations
            .iter()
            .filter_map(|attestation| {
                match attestation.claim_components.as_ref()?.components.as_ref()? {
                    Components::Erc20Deployed(claim) if claim.event_nonce == event.event_nonce => {
                        Some((attestation, claim))
                    }
                    _ => None,
                }
            })
            .collect();
        assert_eq!(
            matching.len(),
            1,
            "Expected one sanitized deployment attestation"
        );
        let (attestation, claim) = matching[0];
        assert!(
            attestation.observed,
            "Sanitized deployment was not observed"
        );
        assert_eq!(attestation.votes.len(), keys.len() - 1);
        assert_eq!(claim.cosmos_denom, expected_denom);
        assert_eq!(claim.token_contract, event.erc20_address.to_string());
        assert_eq!(claim.eth_block_height, event.get_block_height());
        assert!(claim.name.is_empty());
        assert!(claim.symbol.is_empty());
        assert_eq!(claim.decimals, 0);
        let mapping = get_erc20_to_denom(grpc_client, event.erc20_address)
            .await
            .unwrap();
        assert!(!mapping.cosmos_originated, "No-op created a Cosmos mapping");
        assert_eq!(mapping.denom, expected_denom);
    }
}

fn get_deposit_test_strings() -> Vec<Vec<u8>> {
    // A series of test strings designed to torture our implementation.
    let mut test_strings = Vec::new();

    // the maximum size of a message I could get Geth 1.10.8 to accept
    // may be larger in the future.
    const MAX_SIZE: usize = 100_000;

    // normal utf-8
    let bad = "bad destination".to_string();
    test_strings.push(bad.as_bytes().to_vec());

    // someone is trying to deposit to an eth address
    let incorrect = "0x00000000000000000000000089bde264cc4e819326482e041d4ae167981935ce";
    test_strings.push(incorrect.as_bytes().to_vec());

    // a very long, but valid utf8 string
    let rand_string: String = thread_rng()
        .sample_iter(&Alphanumeric)
        .take(MAX_SIZE)
        .map(char::from)
        .collect();
    test_strings.push(rand_string.as_bytes().to_vec());

    // generate a random but invalid utf-8 string
    let mut rand_invalid: Vec<u8> = (0..32).map(|_| rand::random::<u8>()).collect();
    while String::from_utf8(rand_invalid.clone()).is_ok() {
        rand_invalid = (0..32).map(|_| rand::random::<u8>()).collect();
    }
    test_strings.push(rand_invalid);

    // generate a random but invalid utf-8 string, but this time longer
    let mut rand_invalid_long: Vec<u8> = (0..MAX_SIZE).map(|_| rand::random::<u8>()).collect();
    while String::from_utf8(rand_invalid_long.clone()).is_ok() {
        rand_invalid_long = (0..MAX_SIZE).map(|_| rand::random::<u8>()).collect();
    }
    test_strings.push(rand_invalid_long);

    test_strings
}

fn get_unsanitized_erc20_test_values() -> Vec<Erc20Params> {
    IntoIterator::into_iter([0, 255])
        .map(|decimals| Erc20Params {
            erc20_symbol: b"bad".to_vec(),
            erc20_name: b"bad".to_vec(),
            cosmos_denom: b"bad".to_vec(),
            decimals,
        })
        .collect()
}

fn get_erc20_test_values() -> Vec<Erc20Params> {
    // A series of test strings designed to torture our implementation.
    let mut test_strings = Vec::new();

    // Upper bound on the ERC20 name/symbol/denom byte length we test with.
    // Geth dev mode has a block gas limit of ~11.5M, so we need to keep the size
    const MAX_SIZE: usize = 2_000;

    for denom in [
        "gravityjunk".to_string(),
        "gravity2".to_string(),
        "gravity0x2260fac5e5542a773aa44fbcfedf7c193bc2c599".to_string(),
        "gravity20x2260fac5e5542a773aa44fbcfedf7c193bc2c599".to_string(),
        "ibc/bad".to_string(),
        format!("ibc/{}", "a".repeat(64)),
        "foo/bar".to_string(),
        "foo\\bar".to_string(),
        "x".repeat(257),
    ] {
        test_strings.push(Erc20Params {
            cosmos_denom: denom.into_bytes(),
            erc20_name: b"invalid".to_vec(),
            erc20_symbol: b"INVALID".to_vec(),
            decimals: 0,
        });
    }
    for (name, symbol) in [
        (vec![b'x'; 257], b"INVALID".to_vec()),
        (b"invalid".to_vec(), vec![b'x'; 65]),
    ] {
        test_strings.push(Erc20Params {
            cosmos_denom: b"unapproved".to_vec(),
            erc20_name: name,
            erc20_symbol: symbol,
            decimals: 0,
        });
    }

    let blank = String::new().as_bytes().to_vec();
    test_strings.push(Erc20Params {
        erc20_symbol: blank.clone(),
        erc20_name: blank.clone(),
        cosmos_denom: blank,
        decimals: 6,
    });

    // move into testing long but valid utf8
    // a very long, but valid utf8 string
    let rand_string: String = thread_rng()
        .sample_iter(&Alphanumeric)
        .take(MAX_SIZE)
        .map(char::from)
        .collect();
    let rand_string = rand_string.as_bytes().to_vec();
    test_strings.push(Erc20Params {
        erc20_symbol: rand_string.clone(),
        erc20_name: rand_string.clone(),
        cosmos_denom: rand_string,
        decimals: 0,
    });

    // generate a random but invalid utf-8 string
    let mut rand_invalid: Vec<u8> = (0..32).map(|_| rand::random::<u8>()).collect();
    while String::from_utf8(rand_invalid.clone()).is_ok() {
        rand_invalid = (0..32).map(|_| rand::random::<u8>()).collect();
    }
    test_strings.push(Erc20Params {
        erc20_symbol: rand_invalid.clone(),
        erc20_name: rand_invalid.clone(),
        cosmos_denom: rand_invalid,
        decimals: 0,
    });

    // generate a random but invalid utf-8 string, but this time longer
    let mut rand_invalid_long: Vec<u8> = (0..MAX_SIZE).map(|_| rand::random::<u8>()).collect();
    while String::from_utf8(rand_invalid_long.clone()).is_ok() {
        rand_invalid_long = (0..MAX_SIZE).map(|_| rand::random::<u8>()).collect();
    }
    test_strings.push(Erc20Params {
        erc20_symbol: rand_invalid_long.clone(),
        erc20_name: rand_invalid_long.clone(),
        cosmos_denom: rand_invalid_long,
        decimals: 0,
    });

    test_strings
}

/// produces an invalid send to cosmos, accepts bytes so that we can test
/// all sorts of invalid utf-8
pub async fn send_to_cosmos_invalid(
    erc20: Address,
    gravity_contract: Address,
    cosmos_destination: Vec<u8>,
    web3: &Web3,
) {
    let mut approve_nonce = None;

    // rapidly changing gas prices can cause this to fail, a quick retry loop here
    // retries in a way that assists our transaction stress test
    let mut approved = web3
        .get_erc20_allowance(erc20, *MINER_ADDRESS, gravity_contract, vec![])
        .await
        .unwrap()
        >= web3
            .get_erc20_balance(erc20, *MINER_ADDRESS, vec![])
            .await
            .unwrap();
    let start = Instant::now();
    // keep trying while there's still time
    while !approved && Instant::now() - start < TOTAL_TIMEOUT {
        approved = web3
            .get_erc20_allowance(erc20, *MINER_ADDRESS, gravity_contract, vec![])
            .await
            .unwrap()
            >= web3
                .get_erc20_balance(erc20, *MINER_ADDRESS, vec![])
                .await
                .unwrap();
    }

    if !approved {
        let nonce = web3
            .eth_get_transaction_count(*MINER_ADDRESS)
            .await
            .unwrap();
        let options = vec![SendTxOption::Nonce(nonce)];
        approve_nonce = Some(nonce);
        let txid = web3
            .erc20_approve(
                erc20,
                web3.get_erc20_balance(erc20, *MINER_ADDRESS, vec![])
                    .await
                    .unwrap(),
                *MINER_PRIVATE_KEY,
                gravity_contract,
                None,
                options,
            )
            .await
            .unwrap();
        trace!("We are not approved for ERC20 transfers, approving txid: {txid:#066x}");
        web3.wait_for_transaction(txid, TOTAL_TIMEOUT, None)
            .await
            .unwrap();
    }

    let mut options = vec![SendTxOption::GasLimit(SEND_TO_COSMOS_GAS_LIMIT.into())];
    // if we have run an approval we should increment our nonce by one so that
    // we can be sure our actual tx can go in immediately behind
    if let Some(nonce) = approve_nonce {
        options.push(SendTxOption::Nonce(nonce + 1u8.into()));
    }

    // unbounded bytes shares the same actual encoding as strings
    let encoded_destination_address = Token::UnboundedBytes(cosmos_destination);

    let tx_hash = web3
        .send_prepared_transaction(
            web3.prepare_transaction(
                gravity_contract,
                encode_call(
                    "sendToCosmos(address,string,uint256)",
                    &[erc20.into(), encoded_destination_address, one_eth().into()],
                )
                .unwrap(),
                0u32.into(),
                *MINER_PRIVATE_KEY,
                vec![SendTxOption::GasLimitMultiplier(3.0)],
            )
            .await
            .unwrap(),
        )
        .await
        .unwrap();

    web3.wait_for_transaction(tx_hash, TOTAL_TIMEOUT, None)
        .await
        .unwrap();
}

struct Erc20Params {
    cosmos_denom: Vec<u8>,
    erc20_name: Vec<u8>,
    erc20_symbol: Vec<u8>,
    decimals: u8,
}

impl Erc20Params {
    fn into_tokens(self) -> Vec<Token> {
        let mut tokens: Vec<Token> =
            IntoIterator::into_iter([self.cosmos_denom, self.erc20_name, self.erc20_symbol])
                .map(|value| {
                    if value.is_empty() {
                        Token::Dynamic(vec![])
                    } else {
                        Token::UnboundedBytes(value)
                    }
                })
                .collect();
        tokens.push(self.decimals.into());
        tokens
    }

    fn encode(self) -> Vec<u8> {
        encode_call(
            "deployERC20(string,string,string,uint8)",
            &self.into_tokens(),
        )
        .unwrap()
    }
}

#[test]
fn invalid_deployment_fixtures_encode() {
    for params in get_erc20_test_values()
        .into_iter()
        .chain(get_unsanitized_erc20_test_values())
    {
        assert!(params.encode().len() >= 4 + 7 * 32);
    }
}

#[test]
fn deployment_fixtures_sanitize_only_invalid_content() {
    use web30::types::{Data, Log};

    let contract: EthAddress = "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599"
        .parse()
        .unwrap();
    let canonical = format!("gravity{contract}");
    let cases = get_erc20_test_values()
        .into_iter()
        .map(|params| (params, true))
        .chain(
            get_unsanitized_erc20_test_values()
                .into_iter()
                .map(|params| (params, false)),
        );
    for (params, sanitized) in cases {
        let mut tokens = params.into_tokens();
        tokens.push(7u8.into());
        let encoded = encode_call("event(string,string,string,uint8,uint256)", &tokens).unwrap();
        let mut topic = vec![0; 12];
        topic.extend_from_slice(contract.as_bytes());
        let log = Log {
            removed: Some(false),
            log_index: None,
            transaction_index: None,
            transaction_hash: None,
            block_hash: None,
            block_number: Some(42u8.into()),
            address: contract,
            data: Data(encoded[4..].to_vec()),
            topics: vec![Data(vec![0; 32]), Data(topic)],
            type_: None,
        };
        let event = Erc20DeployedEvent::from_log(&log).unwrap();
        assert_eq!(event.event_nonce, 7);
        assert_eq!(event.get_block_height(), 42);
        assert_eq!(event.erc20_address, contract);
        assert_eq!(
            event.cosmos_denom == canonical
                && event.name.is_empty()
                && event.symbol.is_empty()
                && event.decimals == 0,
            sanitized,
            "Unexpected deployment sanitization: {event:?}"
        );
    }
}

async fn deploy_invalid_erc20(
    gravity_address: EthAddress,
    web30: &Web3,
    keys: Vec<ValidatorKeys>,
    erc20_params: Erc20Params,
) -> Erc20DeployedEvent {
    let starting_event_nonce =
        get_event_nonce_safe(gravity_address, web30, keys[0].eth_key.to_address())
            .await
            .unwrap();

    let tx_hash = web30
        .send_prepared_transaction(
            web30
                .prepare_transaction(
                    gravity_address,
                    erc20_params.encode(),
                    0u32.into(),
                    *MINER_PRIVATE_KEY,
                    vec![SendTxOption::GasPriceMultiplier(2.0)],
                )
                .await
                .unwrap(),
        )
        .await
        .unwrap();

    let transaction = web30
        .wait_for_transaction(tx_hash, TOTAL_TIMEOUT, None)
        .await
        .unwrap();

    let ending_event_nonce =
        get_event_nonce_safe(gravity_address, web30, keys[0].eth_key.to_address())
            .await
            .unwrap();

    assert!(starting_event_nonce != ending_event_nonce);
    let block = transaction.get_block_number().unwrap();
    let logs = web30
        .check_for_events(
            block,
            Some(block),
            vec![gravity_address],
            vec![ERC20_DEPLOYED_EVENT_SIG],
        )
        .await
        .unwrap();
    let events = Erc20DeployedEvent::from_logs(&logs).unwrap();
    assert_eq!(
        events.len(),
        1,
        "Expected one deployment in the receipt block"
    );
    let event = events.into_iter().next().unwrap();
    assert!(event.event_nonce > starting_event_nonce);
    assert!(event.event_nonce <= ending_event_nonce);
    event
}
