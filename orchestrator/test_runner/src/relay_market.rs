//! This is the testing module for relay market functionality, testing that
//! relayers utilize web30 to interact with a testnet to obtain coin swap values
//! and determine whether relays should happen or not
use crate::happy_path::test_erc20_deposit_panic;
use crate::utils::{get_erc20_balance_safe, send_one_eth, start_orchestrators, ValidatorKeys};
use crate::ADDRESS_PREFIX;
use crate::MINER_PRIVATE_KEY;
use crate::TOTAL_TIMEOUT;
use crate::{one_eth, MINER_ADDRESS};
use clarity::PrivateKey as EthPrivateKey;
use clarity::{Address as EthAddress, Uint256};
use cosmos_gravity::query::get_oldest_unsigned_transaction_batches;
use cosmos_gravity::send::send_to_eth;
use deep_space::coin::Coin;
use deep_space::private_key::{CosmosPrivateKey, PrivateKey};
use deep_space::{Address, Contact};
use ethereum_gravity::utils::get_tx_batch_nonce;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_utils::types::GravityBridgeToolsConfig;
use rand::Rng;
use std::time::{Duration, Instant};
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use web30::amm::{DAI_CONTRACT_ADDRESS, WETH_CONTRACT_ADDRESS};
use web30::client::Web3;
use web30::jsonrpc::error::Web3Error;

pub async fn relay_market_test(
    web30: &Web3,
    grpc_client: GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
) {
    let grpc_client = &mut grpc_client.clone();
    test_batches(web30, grpc_client, contact, keys, gravity_address).await
}

async fn test_batches(
    web30: &Web3,
    grpc_client: &mut GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
) {
    // Start Orchestrators with the default config, but modified to enable the integrated
    // relayer by default
    let mut default_config = GravityBridgeToolsConfig::default();
    default_config.orchestrator.relayer_enabled = true;
    default_config.relayer.relayer_loop_speed = 10;
    start_orchestrators(keys.clone(), gravity_address, false, default_config).await;

    test_good_batch(
        web30,
        grpc_client,
        contact,
        keys.clone(),
        gravity_address,
        *DAI_CONTRACT_ADDRESS,
    )
    .await;
    test_bad_batch(
        web30,
        grpc_client,
        contact,
        keys.clone(),
        gravity_address,
        *DAI_CONTRACT_ADDRESS,
    )
    .await;
}

async fn setup_batch_test(
    web30: &Web3,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_contract: EthAddress,
    bridge_fee_amount: Uint256,
    grpc_client: &mut GravityQueryClient<Channel>,
) -> (Coin, Uint256, CosmosPrivateKey, Address, EthAddress) {
    let mut grpc_client = grpc_client.clone();
    info!("Starting batch test!");

    // Acquire 10,000 WETH
    let weth_acquired = web30
        .wrap_eth(
            one_eth() * 10000u16.into(),
            *MINER_PRIVATE_KEY,
            None,
            Some(TOTAL_TIMEOUT),
        )
        .await;
    assert!(
        weth_acquired.is_ok(),
        "Unable to wrap eth via web30.wrap_eth() {:?}",
        weth_acquired
    );
    // Acquire 1,000 WETH worth of DAI (probably ~23,000 DAI)
    info!("Starting swap!");
    let start = Instant::now();
    let mut token_acquired = Err(Web3Error::BadInput("Dummy Error".to_string()));
    while Instant::now() - start < TOTAL_TIMEOUT {
        token_acquired = web30
            .swap_uniswap_v3(
                *MINER_PRIVATE_KEY,
                *WETH_CONTRACT_ADDRESS,
                erc20_contract,
                None,
                one_eth() * 1000u16.into(),
                None,
                None,
                None,
                None,
                None,
                Some(TOTAL_TIMEOUT),
            )
            .await;
        if token_acquired.is_ok() {
            break;
        }
    }
    info!("Swap result is {:?}", token_acquired);
    assert!(
        token_acquired.is_ok(),
        "Unable to give the miner 1000 WETH worth of {}",
        erc20_contract
    );

    // Generate an address to send funds
    let mut rng = rand::thread_rng();
    let secret: [u8; 32] = rng.gen();
    let dest_cosmos_private_key = CosmosPrivateKey::from_secret(&secret);
    let dest_cosmos_address = dest_cosmos_private_key
        .to_address(ADDRESS_PREFIX.as_str())
        .unwrap();
    let dest_eth_private_key = EthPrivateKey::from_bytes(secret).unwrap();
    let dest_eth_address = dest_eth_private_key.to_address();

    // Send the generated address 300 dai from ethereum to cosmos
    for _ in 0u32..3 {
        test_erc20_deposit_panic(
            web30,
            contact,
            &mut grpc_client,
            dest_cosmos_address,
            gravity_address,
            erc20_contract,
            one_eth() * 100u64.into(),
            None,
            None,
        )
        .await;
    }

    // Send the validator 100 dai for later
    let requester_cosmos_private_key = keys[0].validator_key;
    let requester_address = requester_cosmos_private_key
        .to_address(&contact.get_prefix())
        .unwrap();
    test_erc20_deposit_panic(
        web30,
        contact,
        &mut grpc_client,
        requester_address,
        gravity_address,
        erc20_contract,
        one_eth() * 100u64.into(),
        None,
        None,
    )
    .await;
    let cdai_held = contact
        .get_balance(dest_cosmos_address, format!("gravity{}", erc20_contract))
        .await
        .unwrap()
        .unwrap();
    let cdai_name = cdai_held.denom.clone();
    let cdai_amount = cdai_held.amount;
    info!(
        "generated address' cosmos balance of {} is {}",
        cdai_name, cdai_amount
    );

    let bridge_denom_fee = Coin {
        amount: bridge_fee_amount,
        denom: cdai_name.clone(),
    };
    info!("bridge_denom_fee {:?}", bridge_denom_fee);
    let send_amount = one_eth() * 200u8.into();
    info!(
        "Sending {} {} from {} on Cosmos back to Ethereum",
        send_amount, cdai_name, dest_cosmos_address
    );
    let res = send_to_eth(
        dest_cosmos_private_key,
        dest_eth_address,
        Coin {
            denom: cdai_name.clone(),
            amount: send_amount,
        },
        bridge_denom_fee.clone(),
        None,
        bridge_denom_fee.clone(),
        contact,
    )
    .await
    .unwrap();
    info!("Sent tokens to Ethereum with {:?}", res);
    (
        cdai_held.clone(),
        send_amount,
        requester_cosmos_private_key,
        requester_address,
        dest_eth_address,
    )
}

async fn wait_for_batch(
    expect_batch: bool,
    web30: &Web3,
    contact: &Contact,
    grpc_client: &mut GravityQueryClient<Channel>,
    requester_address: Address,
    erc20_contract: EthAddress,
    gravity_address: EthAddress,
) -> u64 {
    contact.wait_for_next_block(TOTAL_TIMEOUT).await.unwrap();

    get_oldest_unsigned_transaction_batches(grpc_client, requester_address, contact.get_prefix())
        .await
        .expect("Failed to get batch to sign");

    let mut current_eth_batch_nonce =
        get_tx_batch_nonce(gravity_address, erc20_contract, *MINER_ADDRESS, web30)
            .await
            .expect("Failed to get current eth valset");
    let starting_batch_nonce = current_eth_batch_nonce;

    let start = Instant::now();
    while starting_batch_nonce == current_eth_batch_nonce {
        info!(
            "Batch is not yet submitted {}>, waiting",
            starting_batch_nonce
        );
        current_eth_batch_nonce =
            get_tx_batch_nonce(gravity_address, erc20_contract, *MINER_ADDRESS, web30)
                .await
                .expect("Failed to get current eth tx batch nonce");
        delay_for(Duration::from_secs(4)).await;
        if Instant::now() - start > TOTAL_TIMEOUT {
            if expect_batch {
                panic!("Failed to submit transaction batch set");
            } else {
                break;
            }
        }
    }
    if !expect_batch && starting_batch_nonce != current_eth_batch_nonce {
        panic!(
            "Expected to not have a batch update, but observed nonce {} change to {}",
            starting_batch_nonce, current_eth_batch_nonce
        );
    }

    current_eth_batch_nonce
}

async fn test_good_batch(
    web30: &Web3,
    grpc_client: &mut GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_contract: EthAddress,
) {
    let bridge_fee_amount = one_eth() * 10u8.into();
    let (
        cdai_held,
        send_amount,
        _requester_cosmos_private_key,
        requester_address,
        dest_eth_address,
    ) = setup_batch_test(
        web30,
        contact,
        keys,
        gravity_address,
        erc20_contract,
        bridge_fee_amount,
        grpc_client,
    )
    .await;

    info!("Requesting transaction batch for 20 CosmosDai");

    let current_eth_batch_nonce = wait_for_batch(
        true,
        web30,
        contact,
        grpc_client,
        requester_address,
        erc20_contract,
        gravity_address,
    )
    .await;

    let txid = web30
        .send_transaction(
            dest_eth_address,
            Vec::new(),
            1_000_000_000_000_000_000u128.into(),
            *MINER_PRIVATE_KEY,
            vec![],
        )
        .await
        .expect("Failed to send Eth to validator {}");
    web30
        .wait_for_transaction(txid, TOTAL_TIMEOUT, None)
        .await
        .unwrap();

    // we have to send this address one eth so that it can perform contract calls
    send_one_eth(dest_eth_address, web30).await;
    let dest_eth_bal = get_erc20_balance_safe(erc20_contract, web30, dest_eth_address)
        .await
        .unwrap();

    assert_eq!(
        dest_eth_bal, send_amount,
        "destination eth balance {} != {}",
        dest_eth_bal, send_amount,
    );

    info!(
        "Successfully updated txbatch nonce to {} and sent {}{} tokens to Ethereum!",
        current_eth_batch_nonce, cdai_held.amount, cdai_held.denom
    );
}

async fn test_bad_batch(
    web30: &Web3,
    grpc_client: &mut GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_contract: EthAddress,
) {
    let bridge_fee_amount: Uint256 = 2500u32.into();
    let (
        cdai_held,
        send_amount,
        _requester_cosmos_private_key,
        requester_address,
        dest_eth_address,
    ) = setup_batch_test(
        web30,
        contact,
        keys,
        gravity_address,
        erc20_contract,
        bridge_fee_amount,
        grpc_client,
    )
    .await;

    info!("Requesting transaction batch for very little CosmosDAI");

    let current_eth_batch_nonce = wait_for_batch(
        false,
        web30,
        contact,
        grpc_client,
        requester_address,
        erc20_contract,
        gravity_address,
    )
    .await;

    // we have to send this address one eth so that it can perform contract calls
    send_one_eth(dest_eth_address, web30).await;
    let dest_eth_bal = get_erc20_balance_safe(erc20_contract, web30, dest_eth_address)
        .await
        .unwrap();

    assert_ne!(
        dest_eth_bal, send_amount,
        "destination eth balance {} == {}",
        dest_eth_bal, send_amount,
    );

    info!(
        "Successfully updated txbatch nonce to {} and sent {}{} tokens to Ethereum!",
        current_eth_batch_nonce, cdai_held.amount, cdai_held.denom
    );
}
