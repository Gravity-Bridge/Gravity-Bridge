use crate::{get_fee, one_eth, one_eth_128, one_hundred_eth, utils::*, TOTAL_TIMEOUT};
use clarity::{Address as EthAddress, Uint256};
use cosmos_gravity::{
    query::get_pending_send_to_eth,
    send::{cancel_send_to_eth, send_request_batch, send_to_eth},
    utils::get_reasonable_send_to_eth_fee,
};
use deep_space::coin::Coin;
use deep_space::Contact;
use ethereum_gravity::{send_to_cosmos::send_to_cosmos, utils::get_tx_batch_nonce};
use futures::future::join_all;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use rand::{rngs::ThreadRng, seq::SliceRandom, Rng};
use std::{
    collections::{HashMap, HashSet},
    time::{Duration, Instant},
};
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use web30::{client::Web3, types::SendTxOption};

const TIMEOUT: Duration = Duration::from_secs(120);

/// The number of users we will be simulating for this test, each user
/// will get one token from each token type in erc20_addresses and send it
/// across the bridge to Cosmos as a deposit and then send it back to a different
/// Ethereum address in a transaction batch
/// So the total number of
/// Ethereum sends = (2 * NUM_USERS)
/// ERC20 sends = (erc20_addresses.len() * NUM_USERS)
/// Gravity Deposits = (erc20_addresses.len() * NUM_USERS)
/// Batches executed = erc20_addresses.len() * (NUM_USERS / 100)
const NUM_USERS: usize = 100;
pub const STARTING_ETH: u64 = 200; // The starting ETH amount, in whole units of ETH

/// Perform a stress test by sending thousands of
/// transactions and producing large batches
#[allow(clippy::too_many_arguments)]
pub async fn transaction_stress_test(
    web30: &Web3,
    contact: &Contact,
    grpc_client: GravityQueryClient<Channel>,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_addresses: Vec<EthAddress>,
) {
    let mut grpc_client = grpc_client;

    let no_relay_market_config = create_no_batch_requests_config();
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;

    // Generate NUM_USERS user keys to send ETH and multiple types of tokens
    let mut user_keys = Vec::new();
    for _ in 0..NUM_USERS {
        user_keys.push(get_user_key(None));
    }

    prep_users_for_deposit(&user_keys, &erc20_addresses, web30).await;

    let sent_amounts = test_bulk_send_to_cosmos(
        &user_keys,
        gravity_address,
        &erc20_addresses,
        web30,
        contact,
    )
    .await;

    // now that the users have sent to cosmos we check that they have the right amounts
    // and test execution of large (full batches)
    let denoms = lock_funds_in_pool(
        &sent_amounts,
        &keys,
        &user_keys,
        &erc20_addresses,
        contact,
        false,
    )
    .await;

    // randomly select a user to cancel their transaction, as part of this test
    // we make sure that this user withdraws absolutely zero tokens
    let mut rng = rand::thread_rng();
    let user_who_cancels = user_keys.choose(&mut rng).unwrap();
    let pending = get_pending_send_to_eth(&mut grpc_client, user_who_cancels.cosmos_address)
        .await
        .unwrap();
    // if batch creation is made automatic this becomes a race condition we'll have to consider
    assert!(pending.transfers_in_batches.is_empty());
    assert!(!pending.unbatched_transfers.is_empty());

    let denom = denoms.iter().next().unwrap().clone();
    let bridge_fee = Coin {
        denom,
        amount: 1u8.into(),
    };
    // cancel all outgoing transactions for this user
    for tx in pending.unbatched_transfers {
        let res = cancel_send_to_eth(
            user_who_cancels.cosmos_key,
            bridge_fee.clone(),
            contact,
            tx.id,
        )
        .await
        .unwrap();
        info!("{:?}", res);
    }

    contact.wait_for_next_block(TIMEOUT).await.unwrap();

    // check that the cancelation worked
    let pending = get_pending_send_to_eth(&mut grpc_client, user_who_cancels.cosmos_address)
        .await
        .unwrap();
    info!("{:?}", pending);
    assert!(pending.transfers_in_batches.is_empty());
    assert!(pending.unbatched_transfers.is_empty());

    // this user will have someone else attempt to cancel their transaction
    let mut victim = None;
    for key in user_keys.iter() {
        if key != user_who_cancels {
            victim = Some(key);
            break;
        }
    }
    let pending = get_pending_send_to_eth(&mut grpc_client, victim.unwrap().cosmos_address)
        .await
        .unwrap();
    // try to cancel the victims transactions and ensure failure
    for tx in pending.unbatched_transfers {
        let res = cancel_send_to_eth(
            user_who_cancels.cosmos_key,
            bridge_fee.clone(),
            contact,
            tx.id,
        )
        .await;
        info!("{:?}", res);
    }

    for denom in denoms {
        info!("Requesting batch for {}", denom);
        let res = send_request_batch(keys[0].validator_key, denom, Some(get_fee(None)), contact)
            .await
            .unwrap();
        info!("batch request response is {:?}", res);
    }

    // we sent a random amount below 100 tokens to each user, now we're sending
    // it all back, so we should only be down 500 + the ChainFee's paid
    let starting_eth = one_eth() * STARTING_ETH.into();
    let max_expected_sent = one_hundred_eth();
    // Users are not refunded the ChainFee value they pay for the send
    let max_nonrefundable_amount = get_reasonable_send_to_eth_fee(contact, max_expected_sent)
        .await
        .expect("Unable to get reasonable fee!");

    let min_expected_balance = starting_eth - 500u16.into() - max_nonrefundable_amount;

    let start = Instant::now();
    let mut good = true;
    let mut found_canceled = false;
    while Instant::now() - start < TOTAL_TIMEOUT {
        good = true;
        found_canceled = false;
        for keys in user_keys.iter() {
            let e_dest_addr = keys.eth_dest_address;
            for token in erc20_addresses.iter() {
                let bal = get_erc20_balance_safe(*token, web30, e_dest_addr)
                    .await
                    .unwrap();

                let min_expected_canceled_balance =
                    starting_eth - sent_amounts[keys][token] - max_nonrefundable_amount;

                if e_dest_addr == user_who_cancels.eth_address {
                    if bal >= min_expected_canceled_balance {
                        info!("We successfully found the user who canceled their sends!");
                        found_canceled = true;
                    }
                } else if bal < min_expected_balance {
                    good = false;
                }
            }
        }
        if good && found_canceled {
            info!(
                "All {} withdraws to Ethereum bridged successfully!",
                NUM_USERS * erc20_addresses.len()
            );
            break;
        }
        delay_for(Duration::from_secs(5)).await;
    }
    if !(good && found_canceled) {
        panic!(
            "Failed to perform all {} withdraws to Ethereum!",
            NUM_USERS * erc20_addresses.len()
        );
    }

    // we should find a batch nonce greater than zero since all the batches
    // executed
    for token in erc20_addresses {
        assert!(
            get_tx_batch_nonce(gravity_address, token, keys[0].eth_key.to_address(), web30)
                .await
                .unwrap()
                > 0
        )
    }
}

/// Preps a provided list of keys for depsoiting to Gravity bridge by sending them eth and ERC20 test
/// tokens in the test environment
pub async fn prep_users_for_deposit(
    user_keys: &[BridgeUserKey],
    erc20_addresses: &[EthAddress],
    web30: &Web3,
) {
    // the sending eth addresses need Ethereum to send ERC20 tokens to the bridge
    let sending_eth_addresses: Vec<EthAddress> = user_keys.iter().map(|i| i.eth_address).collect();
    // the destination eth addresses need Ethereum to perform a contract call and get their erc20 balances
    let dest_eth_addresses: Vec<EthAddress> =
        user_keys.iter().map(|i| i.eth_dest_address).collect();
    let mut eth_destinations = Vec::new();
    eth_destinations.extend(sending_eth_addresses.clone());
    eth_destinations.extend(dest_eth_addresses);
    let start_amt: Uint256 = one_eth() * 2u8.into();
    send_eth_bulk(start_amt, &eth_destinations, web30).await;
    info!("Sent {} addresses {} ETH", NUM_USERS, start_amt);

    // now we need to send all the sending eth addresses erc20's to send
    let starting_eth: Uint256 = one_eth() * STARTING_ETH.into();
    for token in erc20_addresses.iter() {
        send_erc20_bulk(starting_eth, *token, &sending_eth_addresses, web30).await;
        info!("Sent {} addresses {} {}", NUM_USERS, STARTING_ETH, token);
    }
    // wait one block to make sure all sends are processed
    web30.wait_for_next_block(TOTAL_TIMEOUT).await.unwrap();
}

/// Takes a list of users prepared by prep_users_for_deposit and sends deposits from all of them
/// across the bridge, checking that the resulting funds arrived and are correct, deposit amounts
/// are random with a returned mapping of keys to deposit amounts
pub async fn test_bulk_send_to_cosmos(
    user_keys: &[BridgeUserKey],
    gravity_address: EthAddress,
    erc20_addresses: &[EthAddress],
    web30: &Web3,
    contact: &Contact,
) -> HashMap<BridgeUserKey, HashMap<EthAddress, Uint256>> {
    let mut amount_sent_per_token_type = HashMap::new();
    for user in user_keys {
        amount_sent_per_token_type.insert(*user, HashMap::new());
    }
    let mut rng = ThreadRng::default();

    for token in erc20_addresses.iter() {
        let mut sends = Vec::new();
        for keys in user_keys.iter() {
            let amount = generate_test_send_amount(&mut rng);
            amount_sent_per_token_type
                .get_mut(keys)
                .unwrap()
                .insert(*token, amount);

            let fut = send_to_cosmos(
                *token,
                gravity_address,
                amount,
                keys.cosmos_address,
                keys.eth_key,
                Some(TIMEOUT),
                web30,
                vec![SendTxOption::GasPriceMultiplier(5.0)],
            );
            sends.push(fut);
        }
        let txids = join_all(sends).await;
        let mut wait_for_txid = Vec::new();
        for txid in txids {
            let wait = web30.wait_for_transaction(txid.unwrap(), TIMEOUT, None);
            wait_for_txid.push(wait);
        }
        let results = join_all(wait_for_txid).await;
        for result in results {
            let result = result.unwrap();
            result.get_block_number().unwrap();
        }
        info!(
            "Locked 100 {} from {} into the Gravity Ethereum Contract",
            token, NUM_USERS
        );
        web30.wait_for_next_block(TOTAL_TIMEOUT).await.unwrap();
    }

    let start = Instant::now();
    let mut good = true;
    while Instant::now() - start < TOTAL_TIMEOUT {
        good = true;
        for keys in user_keys.iter() {
            let c_addr = keys.cosmos_address;
            let balances = contact.get_balances(c_addr).await.unwrap();
            for token in erc20_addresses.iter() {
                let mut found = false;
                for balance in balances.iter() {
                    if balance.denom.contains(&token.to_string())
                        && balance.amount == amount_sent_per_token_type[keys][token]
                    {
                        found = true;
                    }
                }
                if !found {
                    good = false;
                }
            }
        }
        if good {
            info!(
                "All {} deposits bridged to Cosmos successfully!",
                user_keys.len() * erc20_addresses.len()
            );
            break;
        }
        delay_for(Duration::from_secs(5)).await;
    }
    // check that balances on Ethereum have been decremented correctly
    let starting_eth: Uint256 = one_eth() * STARTING_ETH.into();
    for keys in user_keys.iter() {
        let e_dest_addr = keys.eth_dest_address;
        for token in erc20_addresses.iter() {
            let bal = get_erc20_balance_safe(*token, web30, e_dest_addr)
                .await
                .unwrap();
            let expected_balance = starting_eth - amount_sent_per_token_type[keys][token];
            if bal != expected_balance {
                panic!("Failed to decrement all balances on Ethereum!");
            }
        }
    }

    if !good {
        panic!(
            "Failed to perform all {} deposits to Cosmos!",
            user_keys.len() * erc20_addresses.len()
        );
    }
    amount_sent_per_token_type
}

/// Generates a test send amount that is sufficient to pay the bridge fee back
/// and less than the deposit size
pub fn generate_test_send_amount(rng: &mut ThreadRng) -> Uint256 {
    // we must generate an amount less than this
    let one_hundred_eth = one_eth_128() * 100;
    let res: u128 = rng.gen_range(501..one_hundred_eth);
    res.into()
}

/// Generates a sendToEth transaction for each user sending their whole
/// balance back to Ethereum, if request batches is true this function
/// will generate the maximum number of batches possible for the created tx's
pub async fn lock_funds_in_pool(
    sent_amounts: &HashMap<BridgeUserKey, HashMap<EthAddress, Uint256>>,
    validator_keys: &[ValidatorKeys],
    user_keys: &[BridgeUserKey],
    erc20_addresses: &[EthAddress],
    contact: &Contact,
    request_batches: bool,
) -> HashSet<String> {
    // a counter to ensure that each batch we request is larger than the last by one tx
    let mut last_batch_request = 1;

    let mut denoms = HashSet::new();
    for token in erc20_addresses.iter() {
        let mut futs = Vec::new();
        for keys in user_keys.iter() {
            let c_addr = keys.cosmos_address;
            let c_key = keys.cosmos_key;
            let e_dest_addr = keys.eth_dest_address;
            let balances = contact.get_balances(c_addr).await.unwrap();
            // this way I don't have to hardcode a denom and we can change the way denoms are formed
            // without changing this test.
            let mut send_coin = None;
            for balance in balances {
                if balance.denom.contains(&token.to_string()) {
                    send_coin = Some(balance.clone());
                    denoms.insert(balance.denom);
                }
            }
            let mut send_coin = send_coin.unwrap();
            let denom = send_coin.denom.clone();

            // Get a sufficient fee for this Tx, and remove that from the amount to send so the address doesn't run out
            // of funds
            let chain_fee_amount =
                get_reasonable_send_to_eth_fee(contact, sent_amounts[keys][token])
                    .await
                    .expect("Unable to get reasonable fee!");
            let send_amount = sent_amounts[keys][token] - 500u16.into() - chain_fee_amount;

            send_coin.amount = send_amount;

            let send_fee = Coin {
                denom: send_coin.denom.clone(),
                amount: 1u8.into(),
            };
            let chain_fee = Coin {
                denom: send_coin.denom.clone(),
                amount: chain_fee_amount,
            };
            let res = send_to_eth(
                c_key,
                e_dest_addr,
                send_coin,
                send_fee.clone(),
                Some(chain_fee),
                send_fee,
                contact,
            );
            futs.push(res);

            // request progressively bigger batches
            if request_batches && futs.len() > last_batch_request {
                last_batch_request = futs.len();

                let results = join_all(futs).await;
                for result in results {
                    let result = result.unwrap();
                    trace!("SendToEth result {:?}", result);
                }
                futs = Vec::new();

                info!("Requesting batch for {}", denom);
                let res = send_request_batch(
                    validator_keys[0].validator_key,
                    denom,
                    Some(get_fee(None)),
                    contact,
                )
                .await
                .unwrap();
                info!("batch request response is {:?}", res);
            }
        }
        let results = join_all(futs).await;
        for result in results {
            let result = result.unwrap();
            trace!("SendToEth result {:?}", result);
        }
        info!(
            "Successfully placed {} {} into the tx pool",
            NUM_USERS, token
        );
    }
    denoms
}
