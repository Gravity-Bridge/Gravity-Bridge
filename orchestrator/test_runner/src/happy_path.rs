use crate::get_fee;
use crate::utils::*;
use crate::MINER_ADDRESS;
use crate::MINER_PRIVATE_KEY;
use crate::OPERATION_TIMEOUT;
use crate::STAKING_TOKEN;
use crate::STARTING_STAKE_PER_VALIDATOR;
use crate::TOTAL_TIMEOUT;
use bytes::BytesMut;
use clarity::{Address as EthAddress, Uint256};
use cosmos_gravity::query::get_attestations;
use cosmos_gravity::send::{send_request_batch, send_to_eth};
use cosmos_gravity::{query::get_oldest_unsigned_transaction_batch, send::send_ethereum_claims};
use deep_space::address::Address as CosmosAddress;
use deep_space::coin::Coin;
use deep_space::private_key::PrivateKey as CosmosPrivateKey;
use deep_space::Contact;
use ethereum_gravity::utils::get_event_nonce;
use ethereum_gravity::utils::get_valset_nonce;
use ethereum_gravity::{send_to_cosmos::send_to_cosmos, utils::get_tx_batch_nonce};
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::MsgSendToCosmosClaim;
use gravity_proto::gravity::MsgValsetUpdatedClaim;
use gravity_utils::types::SendToCosmosEvent;
use prost::Message;
use rand::Rng;
use std::any::type_name;
use std::thread::sleep;
use std::time::Duration;
use std::time::Instant;
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use web30::client::Web3;

pub async fn happy_path_test(
    web30: &Web3,
    grpc_client: GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_address: EthAddress,
    validator_out: bool,
) {
    let mut grpc_client = grpc_client;

    let no_relay_market_config = create_default_test_config();
    start_orchestrators(
        keys.clone(),
        gravity_address,
        validator_out,
        no_relay_market_config,
    )
    .await;

    // bootstrapping tests finish here and we move into operational tests

    // send 3 valset updates to make sure the process works back to back
    // don't do this in the validator out test because it changes powers
    // randomly and may actually make it impossible for that test to pass
    // by random re-allocation of powers. If we had 5 or 10 validators
    // instead of 3 this wouldn't be a problem. But with 3 not even 1% of
    // power can be reallocated to the down validator before things stop
    // working. We'll settle for testing that the initial valset (generated
    // with the first block) is successfully updated
    if !validator_out {
        for _ in 0u32..2 {
            test_valset_update(web30, contact, &mut grpc_client, &keys, gravity_address).await;
        }
    } else {
        wait_for_nonzero_valset(web30, gravity_address).await;
    }

    // generate an address for coin sending tests, this ensures test imdepotency
    let user_keys = get_user_key();

    // the denom and amount of the token bridged from Ethereum -> Cosmos
    // so the denom is the gravity<hash> token name
    // Send a token 3 times
    for _ in 0u32..3 {
        test_erc20_deposit(
            web30,
            contact,
            &mut grpc_client,
            user_keys.cosmos_address,
            gravity_address,
            erc20_address,
            100u64.into(),
        )
        .await;
    }

    let event_nonce = get_event_nonce(gravity_address, *MINER_ADDRESS, web30)
        .await
        .unwrap();

    // We are going to submit a duplicate tx with nonce 1
    // This had better not increase the balance again
    // this test may have false positives if the timeout is not
    // long enough. TODO check for an error on the cosmos send response
    submit_duplicate_erc20_send(
        event_nonce, // Duplicate the current nonce
        contact,
        erc20_address,
        1u64.into(),
        user_keys.cosmos_address,
        &keys,
    )
    .await;

    // we test a batch by sending a transaction
    test_batch(
        contact,
        &mut grpc_client,
        web30,
        user_keys.eth_address,
        gravity_address,
        keys[0].validator_key,
        user_keys.cosmos_key,
        erc20_address,
    )
    .await;
}

// Iterates each attestation known by the grpc endpoint by calling `get_attestations()`
// Executes the input closure `f` against each attestation's decoded claim
// This is useful for testing that certain attestations exist in the oracle,
// see the consumers of this function (check_valset_update_attestation(),
// check_send_to_cosmos_attestation(), etc.) for examples of usage
//
// `F` is the type of the closure, a state mutating function which may be called multiple times
// `F` functions take a single parameter of type `T`, which is some sort of `Message`
// The type `T` is very important as it dictates how we decode the message
pub async fn iterate_attestations<F: FnMut(T), T: Message + Default>(
    grpc_client: &mut GravityQueryClient<Channel>,
    f: &mut F,
) {
    let attestations = get_attestations(grpc_client, None)
        .await
        .expect("Something happened while getting attestations after delegating to validator");
    for (i, att) in attestations.into_iter().enumerate() {
        let claim = att.clone().claim;
        trace!("Processing attestation {}", i);
        if claim.is_none() {
            trace!("Attestation returned with no claim: {:?}", att);
            continue;
        }
        let claim = claim.unwrap();
        let mut buf = BytesMut::with_capacity(claim.value.len());
        buf.extend_from_slice(&claim.value);

        // Here we use the `T` type to decode whatever type of message this attestation holds
        // for use in the `f` function
        let decoded = T::decode(buf);

        // Decoding errors indicate there is some other attestation we don't care about
        if decoded.is_err() {
            debug!(
                "Found an attestation which is not a {}: {:?}",
                type_name::<T>(),
                att,
            );
            continue;
        }
        let decoded = decoded.unwrap();
        f(decoded);
    }
}

pub async fn wait_for_nonzero_valset(web30: &Web3, gravity_address: EthAddress) {
    let start = Instant::now();
    let mut current_eth_valset_nonce = get_valset_nonce(gravity_address, *MINER_ADDRESS, web30)
        .await
        .expect("Failed to get current eth valset");

    while 0 == current_eth_valset_nonce {
        info!("Validator set is not yet updated to 0>, waiting",);
        current_eth_valset_nonce = get_valset_nonce(gravity_address, *MINER_ADDRESS, web30)
            .await
            .expect("Failed to get current eth valset");
        delay_for(Duration::from_secs(4)).await;
        if Instant::now() - start > TOTAL_TIMEOUT {
            panic!("Failed to update validator set");
        }
    }
}

pub async fn test_valset_update(
    web30: &Web3,
    contact: &Contact,
    grpc_client: &mut GravityQueryClient<Channel>,
    keys: &[ValidatorKeys],
    gravity_address: EthAddress,
) {
    get_valset_nonce(
        gravity_address,
        keys[0].eth_key.to_public_key().unwrap(),
        web30,
    )
    .await
    .expect("Incorrect Gravity Address or otherwise unable to contact Gravity");

    let mut grpc_client = grpc_client.clone();
    // if we don't do this the orchestrators may run ahead of us and we'll be stuck here after
    // getting credit for two loops when we did one
    let starting_eth_valset_nonce = get_valset_nonce(gravity_address, *MINER_ADDRESS, web30)
        .await
        .expect("Failed to get starting eth valset");
    let start = Instant::now();

    // now we send a valset request that the orchestrators will pick up on
    // in this case we send it as the first validator because they can pay the fee
    info!("Sending in valset request");

    // this is hacky and not really a good way to test validator set updates in a highly
    // repeatable fashion. What we really need to do is be aware of the total staking state
    // and manipulate the validator set very intentionally rather than kinda blindly like
    // we are here. For example the more your run this function the less this fixed amount
    // makes any difference, eventually it will fail because the change to the total staked
    // percentage is too small.
    let mut rng = rand::thread_rng();
    let validator_to_change = rng.gen_range(0..keys.len());
    let delegate_address = get_operator_address(keys[validator_to_change].validator_key);
    let amount = Coin {
        denom: STAKING_TOKEN.to_string(),
        amount: (STARTING_STAKE_PER_VALIDATOR / 4).into(),
    };
    info!(
        "Delegating {} to {} in order to generate a validator set update",
        amount, delegate_address
    );
    contact
        .delegate_to_validator(
            delegate_address,
            amount,
            get_fee(),
            keys[1].validator_key,
            Some(TOTAL_TIMEOUT),
        )
        .await
        .unwrap();

    check_valset_update_attestation(&mut grpc_client, keys).await;

    let mut current_eth_valset_nonce = get_valset_nonce(gravity_address, *MINER_ADDRESS, web30)
        .await
        .expect("Failed to get current eth valset");

    while starting_eth_valset_nonce == current_eth_valset_nonce {
        info!(
            "Validator set is not yet updated to {}>, waiting",
            starting_eth_valset_nonce
        );
        current_eth_valset_nonce = get_valset_nonce(gravity_address, *MINER_ADDRESS, web30)
            .await
            .expect("Failed to get current eth valset");
        delay_for(Duration::from_secs(4)).await;
        if Instant::now() - start > TOTAL_TIMEOUT {
            panic!("Failed to update validator set");
        }
    }
    assert!(starting_eth_valset_nonce != current_eth_valset_nonce);
    info!("Validator set successfully updated!");
}

// Checks for a MsgValsetUpdatedClaim attestation where every validator is represented
async fn check_valset_update_attestation(
    grpc_client: &mut GravityQueryClient<Channel>,
    keys: &[ValidatorKeys],
) {
    let mut found = true;
    iterate_attestations(grpc_client, &mut |decoded: MsgValsetUpdatedClaim| {
        // Check that each bridge validator is one of the addresses in our keys
        for bridge_val in decoded.members {
            let found_val = keys.iter().any(|key: &ValidatorKeys| {
                let eth_pub_key = key.eth_key.to_public_key().unwrap().to_string();
                bridge_val.ethereum_address == eth_pub_key
            });
            if !found_val {
                warn!(
                    "Could not find BridgeValidator eth pub key {} in keys",
                    bridge_val.ethereum_address
                );
            }
            found &= found_val;
        }
    })
    .await;
    assert!(
        found,
        "Could not find the valset updated attestation we were looking for!"
    );
    info!("Found the expected MsgValsetUpdatedClaim attestation");
}

/// this function tests Ethereum -> Cosmos
pub async fn test_erc20_deposit(
    web30: &Web3,
    contact: &Contact,
    grpc_client: &mut GravityQueryClient<Channel>,
    dest: CosmosAddress,
    gravity_address: EthAddress,
    erc20_address: EthAddress,
    amount: Uint256,
) {
    get_valset_nonce(gravity_address, *MINER_ADDRESS, web30)
        .await
        .expect("Incorrect Gravity Address or otherwise unable to contact Gravity");

    let mut grpc_client = grpc_client.clone();
    let start_coin = check_cosmos_balance("gravity", dest, contact).await;
    info!(
        "Sending to Cosmos from {} to {} with amount {}",
        *MINER_ADDRESS, dest, amount
    );
    // we send some erc20 tokens to the gravity contract to register a deposit
    let tx_id = send_to_cosmos(
        erc20_address,
        gravity_address,
        amount.clone(),
        dest,
        *MINER_PRIVATE_KEY,
        None,
        web30,
        vec![],
    )
    .await
    .expect("Failed to send tokens to Cosmos");
    info!("Send to Cosmos txid: {:#066x}", tx_id);

    check_send_to_cosmos_attestation(&mut grpc_client, erc20_address, dest, *MINER_ADDRESS).await;

    let _tx_res = web30
        .wait_for_transaction(tx_id, OPERATION_TIMEOUT, None)
        .await
        .expect("Send to cosmos transaction failed to be included into ethereum side");

    let start = Instant::now();
    while Instant::now() - start < TOTAL_TIMEOUT {
        match (
            start_coin.clone(),
            check_cosmos_balance("gravity", dest, contact).await,
        ) {
            (Some(start_coin), Some(end_coin)) => {
                if start_coin.amount + amount.clone() == end_coin.amount
                    && start_coin.denom == end_coin.denom
                {
                    info!(
                        "Successfully bridged ERC20 {}{} to Cosmos! Balance is now {}{}",
                        amount, start_coin.denom, end_coin.amount, end_coin.denom
                    );
                    return;
                }
            }
            (None, Some(end_coin)) => {
                if amount == end_coin.amount {
                    info!(
                        "Successfully bridged ERC20 {}{} to Cosmos! Balance is now {}{}",
                        amount, end_coin.denom, end_coin.amount, end_coin.denom
                    );
                    return;
                } else {
                    panic!("Failed to bridge ERC20!")
                }
            }
            _ => {}
        }
        info!("Waiting for ERC20 deposit");
        contact.wait_for_next_block(TOTAL_TIMEOUT).await.unwrap();
    }
    panic!("Failed to bridge ERC20!")
}

// Tries up to TOTAL_TIMEOUT time to find a MsgSendToCosmosClaim attestation created in the
// test_erc20_deposit test
async fn check_send_to_cosmos_attestation(
    grpc_client: &mut GravityQueryClient<Channel>,
    erc20_address: EthAddress,
    receiver: CosmosAddress,
    sender: EthAddress,
) {
    let start = Instant::now();
    let mut found = false;
    loop {
        iterate_attestations(grpc_client, &mut |decoded: MsgSendToCosmosClaim| {
            let right_contract = decoded.token_contract == erc20_address.to_string();
            let right_destination = decoded.cosmos_receiver == receiver.to_string();
            let right_sender = decoded.ethereum_sender == sender.to_string();
            found = right_contract && right_destination && right_sender;
        })
        .await;
        if found {
            break;
        } else if Instant::now() - start > TOTAL_TIMEOUT {
            panic!("Could not find the send_to_cosmos attestation we were looking for!");
        }
        sleep(Duration::from_secs(5))
    }
    info!("Found the expected MsgSendToCosmosClaim attestation");
}

#[allow(clippy::too_many_arguments)]
async fn test_batch(
    contact: &Contact,
    grpc_client: &mut GravityQueryClient<Channel>,
    web30: &Web3,
    dest_eth_address: EthAddress,
    gravity_address: EthAddress,
    requester_cosmos_private_key: CosmosPrivateKey,
    dest_cosmos_private_key: CosmosPrivateKey,
    erc20_contract: EthAddress,
) {
    get_valset_nonce(gravity_address, *MINER_ADDRESS, web30)
        .await
        .expect("Incorrect Gravity Address or otherwise unable to contact Gravity");

    let mut grpc_client = grpc_client.clone();
    let dest_cosmos_address = dest_cosmos_private_key
        .to_address(&contact.get_prefix())
        .unwrap();
    let coin = check_cosmos_balance("gravity", dest_cosmos_address, contact)
        .await
        .unwrap();
    let token_name = coin.denom;
    let amount = coin.amount;

    let bridge_denom_fee = Coin {
        denom: token_name.clone(),
        amount: 1u64.into(),
    };
    let amount = amount - 5u64.into();
    info!(
        "Sending {}{} from {} on Cosmos back to Ethereum",
        amount, token_name, dest_cosmos_address
    );

    let res = send_to_eth(
        dest_cosmos_private_key,
        dest_eth_address,
        Coin {
            denom: token_name.clone(),
            amount: amount.clone(),
        },
        bridge_denom_fee.clone(),
        bridge_denom_fee.clone(),
        contact,
    )
    .await
    .unwrap();
    info!("Sent tokens to Ethereum with {:?}", res);

    info!("Requesting transaction batch");
    send_request_batch(
        requester_cosmos_private_key,
        token_name.clone(),
        get_fee(),
        contact,
        None,
    )
    .await
    .unwrap();

    contact.wait_for_next_block(TOTAL_TIMEOUT).await.unwrap();
    let requester_address = requester_cosmos_private_key
        .to_address(&contact.get_prefix())
        .unwrap();
    get_oldest_unsigned_transaction_batch(
        &mut grpc_client,
        requester_address,
        contact.get_prefix(),
    )
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
            panic!("Failed to submit transaction batch set");
        }
    }

    let txid = web30
        .send_transaction(
            dest_eth_address,
            Vec::new(),
            1_000_000_000_000_000_000u128.into(),
            *MINER_ADDRESS,
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
    assert_eq!(
        web30
            .get_erc20_balance(erc20_contract, dest_eth_address)
            .await
            .unwrap(),
        amount
    );
    info!(
        "Successfully updated txbatch nonce to {} and sent {}{} tokens to Ethereum!",
        current_eth_batch_nonce, amount, token_name
    );
}

// this function submits a EthereumBridgeDepositClaim to the module with a given nonce. This can be set to be a nonce that has
// already been submitted to test the nonce functionality.
#[allow(clippy::too_many_arguments)]
async fn submit_duplicate_erc20_send(
    nonce: u64,
    contact: &Contact,
    erc20_address: EthAddress,
    amount: Uint256,
    receiver: CosmosAddress,
    keys: &[ValidatorKeys],
) {
    let start_coin = check_cosmos_balance("gravity", receiver, contact)
        .await
        .expect("Did not find coins!");

    let ethereum_sender = "0x912fd21d7a69678227fe6d08c64222db41477ba0"
        .parse()
        .unwrap();

    let event = SendToCosmosEvent {
        event_nonce: nonce,
        block_height: 500u16.into(),
        erc20: erc20_address,
        sender: ethereum_sender,
        destination: receiver,
        amount,
    };

    // iterate through all validators and try to send an event with duplicate nonce
    for k in keys.iter() {
        let c_key = k.orch_key;
        let res = send_ethereum_claims(
            contact,
            c_key,
            vec![event.clone()],
            vec![],
            vec![],
            vec![],
            vec![],
            get_fee(),
        )
        .await
        .unwrap();
        info!("Submitted duplicate sendToCosmos event: {:?}", res);
    }

    contact.wait_for_next_block(TOTAL_TIMEOUT).await.unwrap();

    if let Some(end_coin) = check_cosmos_balance("gravity", receiver, contact).await {
        if start_coin.amount == end_coin.amount && start_coin.denom == end_coin.denom {
            info!("Successfully failed to duplicate ERC20!");
        } else {
            panic!("Duplicated ERC20!")
        }
    } else {
        panic!("Duplicate test failed for unknown reasons!");
    }
}
