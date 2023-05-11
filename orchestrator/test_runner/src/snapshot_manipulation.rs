use std::time::Duration;
use std::{collections::VecDeque};

use cosmos_gravity::query::get_gravity_params;
use cosmos_gravity::send::MSG_SEND_TO_ETH_TYPE_URL;
use deep_space::client::type_urls::MSG_DELEGATE_TYPE_URL;
use deep_space::{Contact, CosmosPrivateKey, Coin, Address as CosmosAddress, PrivateKey, Msg};
use futures::future::join;
use gravity_proto::cosmos_sdk_proto::cosmos::staking::v1beta1::MsgDelegate;
use gravity_proto::gravity::MsgSendToEth;
use gravity_proto::gravity::query_client::QueryClient;
use gravity_utils::types::{RelayerConfig};

use num::Zero;
use rand::{Rng};
use relayer::main_loop::single_relayer_iteration;
use tokio::time::sleep;
use tonic::transport::Channel;
use web30::client::Web3;
use clarity::Address as EthAddress;
use clarity::PrivateKey as EthPrivateKey;
use clarity::Uint256;

use orchestrator::main_loop::{single_eth_signer_iteration, single_oracle_iteration};
use crate::{STAKING_TOKEN, one_eth, OPERATION_TIMEOUT, ADDRESS_PREFIX};
use crate::happy_path::test_erc20_deposit_panic;
use crate::happy_path_v2::deploy_cosmos_representing_erc20_and_check_adoption;
use crate::utils::{ValidatorKeys, get_user_key, BridgeUserKey, footoken_metadata, stake_metadata, create_default_test_config};

/// Attempts to create snapshots at strange moments to rigorously test the calculation of snapshot balances
#[allow(clippy::too_many_arguments)]
pub async fn snapshot_manipulation_test(
    web30: &Web3,
    grpc: QueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_addresses: Vec<EthAddress>,
) {
    info!("Beginning SNAPSHOT_MANIPULATION test!");
    let user = get_user_key(None);
    let erc20_address = erc20_addresses[0];
    let params = get_gravity_params(&mut grpc.clone()).await.unwrap();
    let eth_key = keys[0].eth_key;
    let cosm_key = keys[0].orch_key;
    let relayer_conf = create_default_test_config().relayer;
    let zero_fee = Coin{ amount: Uint256::zero(), denom: STAKING_TOKEN.clone() };
    // After this the send_user will hold 1000 each of the ERC20 (1000 * 10^18) and footoken (1000 * 10^6)
    info!("Sending user needed tokens");
    let foo = setup(
        web30,
        grpc.clone(),
        contact,
        gravity_address,
        &params.gravity_id,
        keys.clone(),
        erc20_address,
        user,
        &relayer_conf,
        cosm_key,
        eth_key,
        zero_fee,
    ).await;
    // Create a queue of many sends to eth, but request no batches
    info!("Generating many sends to eth");
    let foo_send_amount = Coin{amount: 1_000000u128.into(), denom: foo.clone()};
    let foo_bridge_fee = Coin{amount: 1u8.into(), denom: foo.clone()};
    let foo_chain_fee = Coin{amount: 200u8.into(), denom: foo.clone()}; // 2 basis points, in case that is needed
    let foo_sends_to_eth = get_many_sends_to_eth(user, 15, foo_send_amount, foo_bridge_fee, foo_chain_fee);

    let erc20_voucher = format!("gravity{}", erc20_address.to_string());
    let erc20_send_amount = Coin{amount: 1_000000000000000000u128.into(), denom: erc20_voucher.to_string()};
    let erc20_bridge_fee = Coin{amount: 1u8.into(), denom: erc20_voucher.to_string()};
    let erc20_chain_fee = Coin{amount: 2_00000000000000u128.into(), denom: erc20_voucher.to_string()}; // 2 basis points, in case that is needed
    let erc20_sends_to_eth = get_many_sends_to_eth(user, 15, erc20_send_amount, erc20_bridge_fee, erc20_chain_fee);

    // Interleave all of the Msgs into one vec like so: vec![foo, erc20, foo, ...]
    let mut all_sends : Vec<TestMessage> = vec![];
    for (l, r) in  foo_sends_to_eth.into_iter().zip(erc20_sends_to_eth.into_iter()) {
        all_sends.push(l);
        all_sends.push(r);
    }
    let num_sends = all_sends.len();
    let mut all_sends = all_sends.into_iter();

    // Create a queue of delegations which would trigger valset updates
    info!("Generating many delegations");
    let delegations = get_many_delegations(contact, user.cosmos_key, 20).await;
    let num_delegations = delegations.len();
    let mut delegations = delegations.into_iter();

    // Create a method to delegate, sign valset updates, relay them (single relayer iteration), and then oracle them back to cosmos
    // critically the function needs to have these updates happen in the middle of a block where many transactions are going through,
    // and as a batch executed event is being oracle'd as well
    info!("Arranging Msgs for Txs");
    let send_to_eth_sandwich: Vec<TestMessage> = ste_delegate_ste(&mut all_sends, &mut delegations);
    let delegation_sandwich: Vec<TestMessage> = delegate_ste_delegate(&mut all_sends, &mut delegations);
    let multi_token_ste_sandwich: Vec<TestMessage> = delegate_ste_ste_delegate(&mut all_sends, &mut delegations);
    let random_assortment_a: Vec<TestMessage> = pick_msgs(&mut all_sends, &mut delegations, 5);
    let random_assortment_b: Vec<TestMessage> = pick_msgs(&mut all_sends, &mut delegations, 5);
    let random_assortment_c: Vec<TestMessage> = pick_msgs(&mut all_sends, &mut delegations, 5);
    let rest: Vec<TestMessage> = all_the_rest(&mut all_sends, num_sends, &mut delegations, num_delegations);

    let msg_collections = &[send_to_eth_sandwich, delegation_sandwich, multi_token_ste_sandwich, random_assortment_a, random_assortment_b, random_assortment_c, rest];

    execute_messages(
        contact,
        web30,
        &grpc,
        gravity_address,
        &params.gravity_id,
        relayer_conf,
        eth_key,
        cosm_key,
        user.cosmos_key,
        msg_collections,
    ).await;
}

pub async fn setup(
    web30: &Web3,
    grpc: QueryClient<Channel>,
    contact: &Contact,
    gravity_address: EthAddress,
    gravity_id: &str,
    keys: Vec<ValidatorKeys>,
    erc20: EthAddress,
    user: BridgeUserKey,
    relayer_config: &RelayerConfig,
    orchestrator_cosmos_key: CosmosPrivateKey,
    orchestrator_eth_key: EthPrivateKey,
    orchestrator_fee: Coin,
) -> String {
    let footoken = footoken_metadata(contact).await;
    let stake = stake_metadata(contact).await;


    // Send the delegator 10k STAKE
    let stake_send = Coin{amount: 2500_000000u64.into(), denom: stake.base.clone()};
    for (i, key) in (&keys).into_iter().enumerate() {
        info!("Validator {i} sending tokens to user");
        contact.send_coins(stake_send.clone(), None, user.cosmos_address, None, key.validator_key).await.unwrap();
    }
    sleep(Duration::from_secs(15)).await;
    // Send the user 1k (6 decimals) footoken
    let foo_send = Coin{amount: 250_000000u64.into(), denom: footoken.base.clone()};
    for (i, key) in (&keys).into_iter().enumerate() {
        info!("Validator {i} sending tokens to user");
        contact.send_coins(foo_send.clone(), None, user.cosmos_address, None, key.validator_key).await.unwrap();
    }

    info!("Deploying footoken if it does not exist on eth");
    let mut grpc_clone = grpc.clone();
    let f1 = deploy_cosmos_representing_erc20_and_check_adoption(
        gravity_address,
        web30,
        Some(keys.clone()),
        &mut grpc_clone,
        false,
        footoken.clone(),
    );
    let mut oracle_event_to_check = Uint256::zero();
    let mut oracle_block_to_check = Uint256::zero();
    let f2 = run_orchestrator_iterations(
        contact,
        web30,
        &grpc,
        gravity_address,
        gravity_id,
        relayer_config,
        true,
        orchestrator_eth_key,
        orchestrator_cosmos_key,
        orchestrator_fee.clone(),
        &mut oracle_event_to_check,
        &mut oracle_block_to_check,
        3,
    );
    join(f1, f2).await;

    // Send the user 1k (18 decimals) of the erc20
    info!("Send the user 1000 x 10^18 of the erc20");
    let receiver = user.cosmos_address;
    let mut grpc_clone = grpc.clone();
    let f1 = test_erc20_deposit_panic(
        web30,
        contact,
        &mut grpc_clone,
        receiver,
        gravity_address,
        erc20,
        one_eth() * 1000u64.into(),
        None,
        None,
    );
    let mut oracle_event_to_check = Uint256::zero();
    let mut oracle_block_to_check = Uint256::zero();
    let f2 = run_orchestrator_iterations(
        contact,
        web30,
        &grpc,
        gravity_address,
        gravity_id,
        relayer_config,
        true,
        orchestrator_eth_key,
        orchestrator_cosmos_key,
        orchestrator_fee.clone(),
        &mut oracle_event_to_check,
        &mut oracle_block_to_check,
        10,
    );

    join(f1, f2).await;

    footoken.base
}

// An enum to capture the types of Msgs sent via this test
#[derive(Debug)]
pub enum TestMessage {
    Send(MsgSendToEth),
    Delegate(MsgDelegate),
}

/// Constructs up to `limit` MsgSendToEth's and packs them into a queue, all signed by `private_key`
/// These Msgs will transfer `amount` to Ethereum, paying a relayer the `bridge_fee` and the chain the `chain_fee`
pub fn get_many_sends_to_eth(user: BridgeUserKey, limit: usize, amount: Coin, bridge_fee: Coin, chain_fee: Coin) -> VecDeque<TestMessage> {
    let mut results: VecDeque<TestMessage> = VecDeque::with_capacity(limit);
    for _ in 0..limit {
        // Create a Msg Send to Eth and append to the results
        let value = MsgSendToEth {
            sender: user.cosmos_address.to_string(),
            eth_dest: user.eth_dest_address.to_string(),
            amount: Some(amount.clone().into()),
            bridge_fee: Some(bridge_fee.clone().into()),
            chain_fee: Some(chain_fee.clone().into()),
        };
        info!("Created Msg: {value:?}");
        // let msg = Msg::new(MSG_SEND_TO_ETH_TYPE_URL, value);
        let msg = TestMessage::Send(value);
        results.push_back(msg)
    }
    results
}

/// Constructs up to `limit` MsgDelegate's and packs them into a queue, all signed by `private_key`
pub async  fn get_many_delegations(
    contact: &Contact,
    private_key: CosmosPrivateKey,
    limit: usize,
) -> VecDeque<TestMessage> {
    let mut msgs = VecDeque::new();
    let our_address = private_key.to_address(&contact.get_prefix()).unwrap();
    let mut vs = ValidatorStake::new();
    for _ in 0..limit {
        let (validator_address, amount_to_delegate) = anticipate_validator_to_delegate_to(contact, &mut vs).await;
        let deleg = MsgDelegate {
            amount: Some(amount_to_delegate.into()),
            delegator_address: our_address.to_string(),
            validator_address: validator_address.to_string(),
        };

        info!("Created Msg: {deleg:?}");
        // let msg = Msg::new(MSG_DELEGATE_TYPE_URL, deleg);
        let msg = TestMessage::Delegate(deleg);
        msgs.push_back(msg);
    }

    msgs
}

/// Used to locally estimate delegations which will trigger a valset update
struct ValidatorStake {
    // The validators and their associated power via stake delegated to them
    stake_by_validator: Vec<(CosmosAddress, Uint256)>,
    // The total power, or stake, that all the validators hold
    total_bonded_stake: Uint256,
}
impl ValidatorStake {
    fn new() -> Self {
        ValidatorStake { stake_by_validator: vec![], total_bonded_stake: 0u8.into() }
    }
}

/// Determines the validator and amount of delegation to make in order to trigger a valset update, anticipating
/// that the total amount staked and the individual validators stake amounts are successfully updated according to
/// recommendations.
/// This deviates from get_validator_to_delegate_to because it does not require messages to be submitted,
/// thus allowing us to prepare many delegations which will have the desired effect.
/// 
/// While this func is async it only makes asynchronous requests on the first call.
/// 
/// This function is best used like so
/// ```ignore
///     use deep_space::Contact;
///     let contact = Contact::new();
///     let validator_stake = ValidatorStake::new();
///     for i in 0..10 {
///         let (delegate_to, amount) = anticipate_validator_to_delegate_to(&contact, &mut validator_stake).await;
///     }
/// ```
async fn anticipate_validator_to_delegate_to(contact: &Contact, validator_stake: &mut ValidatorStake) -> (CosmosAddress, Coin) {
    let mut has_the_least = None;
    let mut total_bonded_stake: Uint256 = 0u8.into();
    let five_percent: Uint256;
    // First run: get the validator set and populate validator_stake
    if validator_stake.stake_by_validator.is_empty() {
        let validators = contact.get_active_validators().await.unwrap();
        let mut lowest = 0u8.into();
        for v in validators {
            let amount: Uint256 = v.tokens.parse().unwrap();
            total_bonded_stake += amount;
            validator_stake.stake_by_validator.push((v.operator_address.parse().unwrap(), amount));

            if lowest == 0u8.into() || amount < lowest {
                lowest = amount;
                has_the_least = Some(v.operator_address.parse().unwrap());
            }
        }
        validator_stake.total_bonded_stake = total_bonded_stake;
        five_percent = total_bonded_stake / 20u8.into();
    } else {
        // Subsequent run: calculate and anticipate successful change
        total_bonded_stake = validator_stake.total_bonded_stake;
        five_percent = total_bonded_stake / 20u8.into();
        let mut lowest = 0u8.into();
        let mut lowest_i = 0u8.into();
        for (i, (addr, stake)) in validator_stake.stake_by_validator.iter().enumerate() {
            if lowest == 0u8.into() || stake < &lowest {
                lowest = *stake;
                lowest_i = i;
                has_the_least = Some(*addr);
            }
        }
        validator_stake.stake_by_validator[lowest_i] = (validator_stake.stake_by_validator[lowest_i].0, validator_stake.stake_by_validator[lowest_i].1 + five_percent);
    }

    // since this is five percent of the total bonded stake
    // delegating this to the validator who has the least should
    // do the trick
    let five_percent = Coin {
        denom: STAKING_TOKEN.clone(),
        amount: five_percent,
    };

    (has_the_least.unwrap(), five_percent)
}

/// Creates a SendToEth, Delegate, SendToEth "sandwich" of Msgs
pub fn ste_delegate_ste(sends: &mut dyn Iterator<Item=TestMessage>, delegations: &mut dyn Iterator<Item=TestMessage>) -> Vec<TestMessage> {
    info!("Picking ste, deleg, ste");
    let result = vec![sends.next().unwrap(), delegations.next().unwrap(), sends.next().unwrap()];
    result
}

/// Creates a Delegate, SendToEth, Delegate "sandwich" of Msgs
pub fn delegate_ste_delegate(sends: &mut dyn Iterator<Item=TestMessage>, delegations: &mut dyn Iterator<Item=TestMessage>) -> Vec<TestMessage> {
    info!("Picking deleg, ste, deleg");
    let result = vec![delegations.next().unwrap(), sends.next().unwrap(), delegations.next().unwrap()];
    result
}

/// Creates a Delegate, SendToEth, SendToEth, Delegate "sandwich" of Msgs
pub fn delegate_ste_ste_delegate(sends: &mut dyn Iterator<Item=TestMessage>, delegations: &mut dyn Iterator<Item=TestMessage>) -> Vec<TestMessage> {
    info!("Picking deleg, ste, ste, deleg");
    let result = vec![delegations.next().unwrap(), sends.next().unwrap(), sends.next().unwrap(), delegations.next().unwrap()];
    result
}


/// Picks a random number of `sends` and then up to `limit` of `delegations`, returning a Vec with length equal to `limit`
pub fn pick_msgs(sends: &mut dyn Iterator<Item=TestMessage>, delegations: &mut dyn Iterator<Item=TestMessage>, limit: usize) -> Vec<TestMessage> {
    info!("Picking randomly");
    let mut rng = rand::thread_rng();
    let sends_to_take = rng.gen_range(0..(limit+1));
    let deleg_to_take = limit - sends_to_take;

    let mut msgs = vec![];
    msgs.append(&mut sends.take(sends_to_take).collect::<Vec<_>>());
    msgs.append(&mut delegations.take(deleg_to_take).collect::<Vec<_>>());
    msgs
}

/// Randomly picks the remainder of `sends` and `delegations`, placing them into the returned Vec
pub fn all_the_rest(sends: &mut dyn Iterator<Item=TestMessage>, mut num_sends: usize, delegations: &mut dyn Iterator<Item=TestMessage>, mut num_delegations: usize) -> Vec<TestMessage> {
    info!("Collecting the rest");
    let mut msgs = vec![];
    let mut rng = rand::thread_rng();

    // "While there are any msgs left in either iterator"
    while num_sends > 0 || num_delegations > 0 {
        info!("num_sends: {num_sends}, num_delegations: {num_delegations}");
        let take_send = rng.gen::<f32>() < 0.5;
        if take_send {
            let msgs_to_take = rng.gen_range(0..(num_sends+1));
            num_sends -= msgs_to_take;
            msgs.append(&mut sends.take(msgs_to_take).collect::<Vec<_>>());
        } else {
            let msgs_to_take = rng.gen_range(0..(num_delegations+1));
            num_delegations -= msgs_to_take;
            msgs.append(&mut delegations.take(msgs_to_take).collect::<Vec<_>>());
        }
    }

    msgs
}

/// Exeuctes a collection of message combos in separate Txs
pub async fn execute_messages(
    contact: &Contact,
    web30: &Web3,
    grpc_client: &QueryClient<Channel>,
    gravity_contract_address: EthAddress,
    gravity_id: &str,
    relayer_config: RelayerConfig,
    orchestrator_eth_key: EthPrivateKey,
    orchestrator_cosmos_key: CosmosPrivateKey,
    user_cosmos_key: CosmosPrivateKey,
    msg_collections: &[Vec<TestMessage>],
) {
    let mut event_to_check = 0u8.into();
    let mut block_to_check = 0u8.into();
    let zero_fee = Coin{ amount: Uint256::zero(), denom: STAKING_TOKEN.clone() };
    for (i, coll) in msg_collections.iter().enumerate() {
        info!("Executing Msg collection {i}: {coll:?}");
        let msgs: Vec<Msg> = coll.into_iter().map(
            |m| match m {
                TestMessage::Send(send) => {Msg::new(MSG_SEND_TO_ETH_TYPE_URL, send.clone())},
                TestMessage::Delegate(delegate) => {Msg::new(MSG_DELEGATE_TYPE_URL, delegate.clone())}
            }
        ).collect();
        contact.send_message(&msgs, Some(format!("Collection {i}")), &[], Some(OPERATION_TIMEOUT), user_cosmos_key).await.unwrap();

        run_orchestrator_iterations(
            contact,
            web30,
            grpc_client,
            gravity_contract_address,
            gravity_id,
            &relayer_config,
            true,
            orchestrator_eth_key,
            orchestrator_cosmos_key,
            zero_fee.clone(),
            &mut event_to_check,
            &mut block_to_check,
            1,
        ).await;

        
    }
}

pub async fn run_orchestrator_iterations(
    contact: &Contact,
    web30: &Web3,
    grpc_client: &QueryClient<Channel>,
    gravity_contract_address: EthAddress,
    gravity_id: &str,
    relayer_config: &RelayerConfig,
    relayer_should_relay_altruistic: bool,
    orchestrator_eth_key: EthPrivateKey,
    orchestrator_cosmos_key: CosmosPrivateKey,
    fee: Coin,
    oracle_event_to_check: &mut Uint256,
    oracle_block_to_check: &mut Uint256,
    iterations: usize,
) {
    let cosmos_address = orchestrator_cosmos_key.to_address(ADDRESS_PREFIX.as_str()).unwrap();
    let eth_address = orchestrator_eth_key.to_address();
    for _ in 0..iterations {
        single_relayer_iteration(
            orchestrator_eth_key,
            Some(orchestrator_cosmos_key),
            Some(fee.clone()),
            contact,
            web30,
            grpc_client,
            gravity_contract_address,
            gravity_id,
            &relayer_config,
            relayer_should_relay_altruistic,
        ).await;
        single_eth_signer_iteration(
            grpc_client.clone(),
            contact,
            web30,
            gravity_contract_address,
            orchestrator_cosmos_key,
            cosmos_address,
            orchestrator_eth_key,
            eth_address,
            fee.clone(),
        ).await;
        let res = single_oracle_iteration(
            grpc_client.clone(),
            contact,
            web30,
            gravity_contract_address,
            orchestrator_cosmos_key,
            cosmos_address,
            eth_address,
            *oracle_event_to_check,
            *oracle_block_to_check,
            fee.clone(),
        ).await;
        if let Some(nonces) = res {
            // Update the nonces on success
            (*oracle_event_to_check, *oracle_block_to_check) = (nonces.event_nonce, nonces.block_number);
        }
    }
}