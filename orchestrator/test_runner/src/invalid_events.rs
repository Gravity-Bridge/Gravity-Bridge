//! This is a test for invalid string based deposits, the goal is to torture test the implementation
//! with every possible variant of invalid data and ensure that in all cases the community pool deposit
//! works correctly.

use crate::happy_path::test_erc20_deposit_panic;
use crate::happy_path_v2::deploy_cosmos_representing_erc20_and_check_adoption;
use crate::one_eth;
use crate::utils::create_default_test_config;
use crate::utils::get_user_key;
use crate::utils::start_orchestrators;
use crate::utils::ValidatorKeys;
use crate::MINER_ADDRESS;
use crate::MINER_PRIVATE_KEY;
use crate::TOTAL_TIMEOUT;
use clarity::abi::encode_call;
use clarity::abi::Token;
use clarity::Address as EthAddress;
use clarity::Address;
use deep_space::Contact;
use ethereum_gravity::send_to_cosmos::SEND_TO_COSMOS_GAS_LIMIT;
use ethereum_gravity::utils::get_event_nonce;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
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
    let erc20_denom = format!("gravity{}", erc20_address);

    // figure out how many of a given erc20 we already have on startup so that we can
    // keep track of incrementation. This makes it possible to run this test again without
    // having to restart your test chain
    let community_pool_contents = contact.get_community_pool_coins().await.unwrap();
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
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;

    for test_value in get_deposit_test_strings() {
        // next we send an invalid string deposit, we use byte encoding here so that we can attempt a totally invalid send
        send_to_cosmos_invalid(erc20_address, gravity_address, test_value, web30).await;

        // send some coins across the correct way, make sure they arrive
        // note send_to_cosmos_invalid does not wait for the actual oracle
        // to complete like this function does, since this deposit will have
        // a latter event nonce it will effectively wait for the invalid deposit
        // to complete as well
        let user_keys = get_user_key();
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
        let community_pool_contents = contact.get_community_pool_coins().await.unwrap();
        for coin in community_pool_contents {
            if coin.denom == erc20_denom {
                let expected = starting_pool_amount + one_eth();
                if coin.amount != expected {
                    error!(
                        "Expected {} in the community pool found {}.",
                        expected, coin.amount
                    );
                    error!("This means an invalid deposit has been 'lost' in the bridge, instead of allowing it's funds to be used by the Community pool");
                    panic!("Lost an invalid deposit!");
                } else {
                    starting_pool_amount = expected;
                }
            }
        }
    }
    for test_value in get_erc20_test_values() {
        deploy_invalid_erc20(gravity_address, web30, keys.clone(), test_value).await;
        web30.wait_for_next_block(TOTAL_TIMEOUT).await.unwrap();
    }

    web30.wait_for_next_block(TOTAL_TIMEOUT).await.unwrap();

    let token_to_send_to_eth = "footoken".to_string();
    let token_to_send_to_eth_display_name = "mfootoken".to_string();

    // make sure this actual deployment works after all the bad ones
    let _ = deploy_cosmos_representing_erc20_and_check_adoption(
        gravity_address,
        web30,
        None,
        &mut grpc_client,
        false,
        token_to_send_to_eth.clone(),
        token_to_send_to_eth_display_name.clone(),
    )
    .await;

    info!("Successfully completed the invalid events test")
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

    //test_strings
    Vec::new()
}

fn get_erc20_test_values() -> Vec<Erc20Params> {
    // A series of test strings designed to torture our implementation.
    let mut test_strings = Vec::new();

    // the maximum size I could get OpenEthereum ERC20 to accept
    // maybe higher in the future
    const MAX_SIZE: usize = 5_000;

    // start with normal utf-8 and odd decimals values
    let bad = "bad".to_string().as_bytes().to_vec();
    test_strings.push(Erc20Params {
        erc20_symbol: bad.clone(),
        erc20_name: bad.clone(),
        cosmos_denom: bad.clone(),
        decimals: 0,
    });

    test_strings.push(Erc20Params {
        erc20_symbol: bad.clone(),
        erc20_name: bad.clone(),
        cosmos_denom: bad,
        decimals: 255,
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
        .check_erc20_approved(erc20, *MINER_ADDRESS, gravity_contract)
        .await;
    let start = Instant::now();
    // keep trying while there's still time
    while approved.is_err() && Instant::now() - start < TOTAL_TIMEOUT {
        approved = web3
            .check_erc20_approved(erc20, *MINER_ADDRESS, gravity_contract)
            .await;
    }

    let approved = approved.unwrap();
    if !approved {
        let nonce = web3
            .eth_get_transaction_count(*MINER_ADDRESS)
            .await
            .unwrap();
        let options = vec![SendTxOption::Nonce(nonce.clone())];
        approve_nonce = Some(nonce);
        let txid = web3
            .approve_erc20_transfers(erc20, *MINER_PRIVATE_KEY, gravity_contract, None, options)
            .await
            .unwrap();
        trace!(
            "We are not approved for ERC20 transfers, approving txid: {:#066x}",
            txid
        );
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
        .send_transaction(
            gravity_contract,
            encode_call(
                "sendToCosmos(address,string,uint256)",
                &[
                    erc20.into(),
                    encoded_destination_address,
                    one_eth().clone().into(),
                ],
            )
            .unwrap(),
            0u32.into(),
            *MINER_ADDRESS,
            *MINER_PRIVATE_KEY,
            vec![SendTxOption::GasLimitMultiplier(3.0)],
        )
        .await
        .unwrap();

    web3.wait_for_transaction(tx_hash.clone(), TOTAL_TIMEOUT, None)
        .await
        .unwrap();
}

struct Erc20Params {
    cosmos_denom: Vec<u8>,
    erc20_name: Vec<u8>,
    erc20_symbol: Vec<u8>,
    decimals: u8,
}

async fn deploy_invalid_erc20(
    gravity_address: EthAddress,
    web30: &Web3,
    keys: Vec<ValidatorKeys>,
    erc20_params: Erc20Params,
) {
    let starting_event_nonce = get_event_nonce(
        gravity_address,
        keys[0].eth_key.to_public_key().unwrap(),
        web30,
    )
    .await
    .unwrap();

    let tx_hash = web30
        .send_transaction(
            gravity_address,
            encode_call(
                "deployERC20(string,string,string,uint8)",
                &[
                    Token::UnboundedBytes(erc20_params.cosmos_denom),
                    Token::UnboundedBytes(erc20_params.erc20_name),
                    Token::UnboundedBytes(erc20_params.erc20_symbol),
                    erc20_params.decimals.into(),
                ],
            )
            .unwrap(),
            0u32.into(),
            *MINER_ADDRESS,
            *MINER_PRIVATE_KEY,
            vec![SendTxOption::GasPriceMultiplier(2.0)],
        )
        .await
        .unwrap();

    web30
        .wait_for_transaction(tx_hash.clone(), TOTAL_TIMEOUT, None)
        .await
        .unwrap();

    let ending_event_nonce = get_event_nonce(
        gravity_address,
        keys[0].eth_key.to_public_key().unwrap(),
        web30,
    )
    .await
    .unwrap();

    assert!(starting_event_nonce != ending_event_nonce);
    info!(
        "Successfully deployed an invalid ERC20 on Cosmos with event nonce {}",
        ending_event_nonce
    );
}
