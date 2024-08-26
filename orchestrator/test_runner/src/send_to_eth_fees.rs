use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::happy_path::test_erc20_deposit_panic;
use crate::happy_path_v2::deploy_cosmos_representing_erc20_and_check_adoption;
use crate::utils::{
    footoken_metadata, get_user_key, vote_yes_on_proposals, BridgeUserKey, ValidatorKeys,
};
use crate::{
    get_deposit, get_fee, one_eth, ADDRESS_PREFIX, OPERATION_TIMEOUT, STAKING_TOKEN, TOTAL_TIMEOUT,
};
use actix::clock::sleep;
use clarity::Address as EthAddress;
use cosmos_gravity::proposals::{submit_send_to_eth_fees_proposal, SendToEthFeesProposalJson};
use cosmos_gravity::query::get_min_chain_fee_basis_points;
use cosmos_gravity::send::MSG_SEND_TO_ETH_TYPE_URL;
use cosmos_gravity::utils::get_min_send_to_eth_fee;
use deep_space::{Coin, Contact, CosmosPrivateKey, Fee, MessageArgs, Msg, PrivateKey};
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::Metadata;
use gravity_proto::gravity::{query_client::QueryClient as GravityQueryClient, MsgSendToEth};
use gravity_utils::num_conversion::one_atom;
use num::ToPrimitive;
use num256::Uint256;
use std::ops::Mul;
use std::time::{Duration, Instant};
use tonic::transport::Channel;
use web30::client::Web3;

// The voting period, in seconds. This is set in tests/container-scripts/setup-validators.sh
const GOVERNANCE_VOTING_PERIOD: u64 = 120;

pub async fn send_to_eth_fees_test(
    web30: &Web3,
    contact: &Contact,
    gravity_client: GravityQueryClient<Channel>,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_addresses: Vec<EthAddress>,
) {
    info!("Enter send to eth fees test");
    let (ibc_metadata, staker_key) = setup(
        web30,
        contact,
        &gravity_client,
        keys.clone(),
        gravity_address,
        *erc20_addresses.first().unwrap(),
    )
    .await;
    let cosmos_denom = ibc_metadata.base;
    let erc20_denom: String = format!("gravity{}", erc20_addresses.first().unwrap().clone());
    let (_staker_key, _staker_addr) = (staker_key.cosmos_key, staker_key.cosmos_address);

    let val0_cosmos_key = keys[0].validator_key;
    let val0_cosmos_addr = val0_cosmos_key.to_address(&ADDRESS_PREFIX).unwrap();
    let val0_eth_key = keys[0].eth_key;
    let val0_eth_addr = val0_eth_key.to_address();

    // Test the default fee config
    info!("Begin send to eth tests before the fee parameter has been set");
    let start_sender_bal = contact.get_balances(val0_cosmos_addr).await.unwrap();
    let fee_exp = send_to_eth_one_msg_txs(
        contact,
        val0_cosmos_key,
        val0_eth_addr,
        erc20_denom.clone(),
        cosmos_denom.clone(),
    )
    .await;
    let end_sender_bal = contact.get_balances(val0_cosmos_addr).await.unwrap();
    info!("Assert fees have been taken from the sender");
    assert_fees_collected(
        start_sender_bal,
        end_sender_bal,
        fee_exp,
        erc20_denom.clone(),
        cosmos_denom.clone(),
    );

    let start_sender_bal = contact.get_balances(val0_cosmos_addr).await.unwrap();
    let fee_exp = send_to_eth_multi_msg_txs(
        contact,
        val0_cosmos_key,
        val0_eth_addr,
        erc20_denom.clone(),
        cosmos_denom.clone(),
    )
    .await;
    let end_sender_bal = contact.get_balances(val0_cosmos_addr).await.unwrap();
    info!("Assert fees have been taken from the sender");
    assert_fees_collected(
        start_sender_bal,
        end_sender_bal,
        fee_exp,
        erc20_denom.clone(),
        cosmos_denom.clone(),
    );

    // Change the fee config to our initial production value
    submit_and_pass_send_to_eth_fees_proposal(2, contact, &keys).await;

    let start_sender_bal = contact.get_balances(val0_cosmos_addr).await.unwrap();
    let fee_exp = send_to_eth_one_msg_txs(
        contact,
        val0_cosmos_key,
        val0_eth_addr,
        erc20_denom.clone(),
        cosmos_denom.clone(),
    )
    .await;
    let end_sender_bal = contact.get_balances(val0_cosmos_addr).await.unwrap();
    info!("Assert fees have been taken from the sender");
    assert_fees_collected(
        start_sender_bal,
        end_sender_bal,
        fee_exp,
        erc20_denom.clone(),
        cosmos_denom.clone(),
    );

    let start_sender_bal = contact.get_balances(val0_cosmos_addr).await.unwrap();
    let fee_exp = send_to_eth_multi_msg_txs(
        contact,
        val0_cosmos_key,
        val0_eth_addr,
        erc20_denom.clone(),
        cosmos_denom.clone(),
    )
    .await;
    let end_sender_bal = contact.get_balances(val0_cosmos_addr).await.unwrap();
    info!("Assert fees have been taken from the sender");
    assert_fees_collected(
        start_sender_bal,
        end_sender_bal,
        fee_exp,
        erc20_denom.clone(),
        cosmos_denom.clone(),
    );

    // Change the fee config while submitting sends to eth
    // TODO: Assert these weird fees
    let _fee_exp = send_to_eth_while_changing_params(
        contact,
        &keys,
        val0_cosmos_key,
        val0_eth_addr,
        erc20_denom.clone(),
        cosmos_denom.clone(),
    )
    .await;

    // Change the fee config back to 0 and verify that the bridge still works
    submit_and_pass_send_to_eth_fees_proposal(0, contact, &keys).await;
    let start_sender_bal = contact.get_balances(val0_cosmos_addr).await.unwrap();
    let fee_exp = send_to_eth_one_msg_txs(
        contact,
        val0_cosmos_key,
        val0_eth_addr,
        erc20_denom.clone(),
        cosmos_denom.clone(),
    )
    .await;
    let end_sender_bal = contact.get_balances(val0_cosmos_addr).await.unwrap();
    info!("Assert fees have been taken from the sender");
    assert_fees_collected(
        start_sender_bal,
        end_sender_bal,
        fee_exp,
        erc20_denom.clone(),
        cosmos_denom.clone(),
    );

    let start_sender_bal = contact.get_balances(val0_cosmos_addr).await.unwrap();
    let fee_exp = send_to_eth_multi_msg_txs(
        contact,
        val0_cosmos_key,
        val0_eth_addr,
        erc20_denom.clone(),
        cosmos_denom.clone(),
    )
    .await;
    let end_sender_bal = contact.get_balances(val0_cosmos_addr).await.unwrap();
    info!("Assert fees have been taken from the sender");
    assert_fees_collected(
        start_sender_bal,
        end_sender_bal,
        fee_exp,
        erc20_denom,
        cosmos_denom,
    );
}

/// Deploys the footoken erc20 representation, sends the Eth-native ERC20s to the cosmos validators,
/// and creates a staker who has delegations to every validator
pub async fn setup(
    web30: &Web3,
    contact: &Contact,
    grpc_client: &GravityQueryClient<Channel>,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_address: EthAddress,
) -> (Metadata, BridgeUserKey) {
    info!("Begin setup, create footoken erc20");
    let ibc_metadata = footoken_metadata(contact).await;

    let _ = deploy_cosmos_representing_erc20_and_check_adoption(
        gravity_address,
        web30,
        Some(keys.clone()),
        &mut (grpc_client.clone()),
        false,
        ibc_metadata.clone(),
    )
    .await;

    info!("Send validators 100 x 10^18 of the native erc20");
    // Send the validators generated address 100 units of each erc20 from ethereum to cosmos
    for v in &keys {
        let receiver = v.validator_key.to_address(ADDRESS_PREFIX.as_str()).unwrap();
        test_erc20_deposit_panic(
            web30,
            contact,
            &mut (grpc_client.clone()),
            receiver,
            gravity_address,
            erc20_address,
            one_eth() * 1000u64.into(),
            None,
            None,
        )
        .await;
    }

    // Create a staker who should receive fee rewards too
    let staker_key = get_user_key(None);
    let staker_amt = Coin {
        denom: STAKING_TOKEN.clone(),
        amount: one_atom().mul(100u8.into()),
    };
    let zero_fee = Coin {
        denom: STAKING_TOKEN.clone(),
        amount: 0u8.into(),
    };
    info!("Send stake to staker");
    for v in &keys {
        let res = contact
            .send_coins(
                staker_amt.clone(),
                None,
                staker_key.cosmos_address,
                Some(OPERATION_TIMEOUT),
                v.validator_key,
            )
            .await;
        info!("Sent coins to staker with response {:?}", res)
    }
    info!("Staker delegating stake to each validator");
    for v in &keys {
        let validator_addr = v
            .validator_key
            .to_address((ADDRESS_PREFIX.clone() + "valoper").as_str())
            .unwrap();
        let res = contact
            .delegate_to_validator(
                validator_addr,
                staker_amt.clone(),
                zero_fee.clone(),
                staker_key.cosmos_key,
                Some(OPERATION_TIMEOUT),
            )
            .await;
        info!("Delegated to validator with response {:?}", res)
    }

    (ibc_metadata, staker_key)
}

/// Helpful data for verifying that fee tests execute as expected
#[derive(Debug)]
pub struct SendToEthFeeExpectations {
    pub expected_erc20_fee: Uint256,  // Fees to collect on erc20 bridge
    pub expected_cosmos_fee: Uint256, // Fees to collect on footoken bridge
    pub successful_erc20_bridged: Uint256, // Successful erc20 bridge amounts sent
    pub successful_cosmos_bridged: Uint256, // Successful footoken bridge amounts sent
}

/// Creates a number of txs with each containing a single MsgSendToEth, with fee values informed by
/// the current gravity params
pub async fn send_to_eth_one_msg_txs(
    contact: &Contact,
    sender: CosmosPrivateKey,
    receiver: EthAddress,
    erc20_denom: String,
    cosmos_denom: String,
) -> SendToEthFeeExpectations {
    info!("send_to_eth_one_msg_txs: Getting chain fee param");
    let current_fee_basis_points = get_min_chain_fee_basis_points(contact)
        .await
        .expect("Unable to get MinChainFeeBasisPoints");
    info!("send_to_eth_one_msg_txs: setting up transactions");
    // Create the test transactions
    let queued_transactions = setup_transactions(current_fee_basis_points, 1);
    info!("queued transactions {:?}", queued_transactions);

    // Send the transactions
    let sender_addr = sender
        .to_address(ADDRESS_PREFIX.as_str())
        .unwrap()
        .to_string();
    let tx_fee = Coin {
        denom: (*STAKING_TOKEN).clone(),
        amount: 0u8.into(),
    };
    info!("send_to_eth_one_msg_txs: sending transactions");
    send_single_msg_txs(
        contact,
        sender,
        sender_addr.clone(),
        receiver,
        cosmos_denom.clone(),
        queued_transactions.cosmos_queue_amounts.clone(),
        queued_transactions.cosmos_queue_fees.clone(),
        tx_fee.clone(),
        true,
    )
    .await;
    send_single_msg_txs(
        contact,
        sender,
        sender_addr.clone(),
        receiver,
        cosmos_denom.clone(),
        queued_transactions.cosmos_failure_amounts.clone(),
        queued_transactions.cosmos_failure_fees.clone(),
        tx_fee.clone(),
        false,
    )
    .await;
    send_single_msg_txs(
        contact,
        sender,
        sender_addr.clone(),
        receiver,
        erc20_denom.clone(),
        queued_transactions.erc20_queue_amounts.clone(),
        queued_transactions.erc20_queue_fees.clone(),
        tx_fee.clone(),
        true,
    )
    .await;
    send_single_msg_txs(
        contact,
        sender,
        sender_addr.clone(),
        receiver,
        erc20_denom.clone(),
        queued_transactions.erc20_failure_amounts.clone(),
        queued_transactions.erc20_failure_fees.clone(),
        tx_fee.clone(),
        false,
    )
    .await;

    let results = sum_expected_results(queued_transactions);
    info!(
        "send_to_eth_one_msg_txs: transactions sent, expecting fees: {}{}, {}{}",
        results.expected_erc20_fee, erc20_denom, results.expected_cosmos_fee, cosmos_denom,
    );
    // Return the expected results for verification
    results
}

#[allow(clippy::too_many_arguments)]
async fn send_single_msg_txs(
    contact: &Contact,
    sender: CosmosPrivateKey,
    sender_addr: String,
    receiver: EthAddress,
    denom: String,
    bridge_amounts: Vec<Uint256>,
    fee_amounts: Vec<Uint256>,
    tx_fee: Coin,
    expect_success: bool,
) {
    let fee_for_relayer = Coin {
        denom: denom.clone(),
        amount: 1u8.into(),
    };
    for (bridge, fee) in bridge_amounts.into_iter().zip(fee_amounts.into_iter()) {
        let bridge_coin = Coin {
            denom: denom.clone(),
            amount: bridge,
        };
        let fee_coin = Coin {
            denom: denom.clone(),
            amount: fee,
        };
        let msg_send_to_eth = MsgSendToEth {
            sender: sender_addr.clone(),
            eth_dest: receiver.to_string(),
            amount: Some(bridge_coin.into()),
            bridge_fee: Some(fee_for_relayer.clone().into()),
            chain_fee: Some(fee_coin.into()),
        };

        let msg = Msg::new(MSG_SEND_TO_ETH_TYPE_URL, msg_send_to_eth);
        let res = contact
            .send_message(
                &[msg],
                None,
                &[tx_fee.clone()],
                Some(OPERATION_TIMEOUT),
                None,
                sender,
            )
            .await;
        if expect_success {
            info!("sent msg with res {:?}", res);
        } else if res.is_ok() {
            panic!("Expected a failure due to fees, but got success? {:?}", res);
        }
    }
}

/// Creates a number of txs with each containing multiple MsgSendToEth's, with fee values informed by
/// the current gravity params
pub async fn send_to_eth_multi_msg_txs(
    contact: &Contact,
    sender: CosmosPrivateKey,
    receiver: EthAddress,
    erc20_denom: String,
    cosmos_denom: String,
) -> SendToEthFeeExpectations {
    info!("send_to_eth_multi_msg_txs: Getting chain param");
    let current_fee_basis_points = get_min_chain_fee_basis_points(contact)
        .await
        .expect("Unable to get MinChainFeeBasisPoints");
    info!("send_to_eth_multi_msg_txs: setting up transactions");
    // Create the test transactions
    let queued_transactions = setup_transactions(
        current_fee_basis_points,
        3, // Create lots of sends for us to pack into Txs
    );

    // Create and send Txs with multiple msgs in each tx
    let sender_addr = sender
        .to_address(ADDRESS_PREFIX.as_str())
        .unwrap()
        .to_string();
    let tx_fee = Coin {
        denom: (*STAKING_TOKEN).clone(),
        amount: 0u8.into(),
    };
    // A buffer of Msgs built iteratively before use with contact.send_message()
    let mut msgs = vec![];
    // The number of Msgs in our 1st, 2nd, 3rd, ... Txs
    let tx_sizes: Vec<usize> = vec![3, 6, 4, 2, 5, 7, 10, 8, 1, 2, 2, 2, 2, 2, 1];
    let mut tx_idx = 0; // Our pointer into tx_sizes
    info!(
        "send_to_eth_multi_msg_txs: sending transactions \n\n\n[{:?}]\n\n\n",
        queued_transactions
    );
    // Craft MsgSendToEth's, add them to the buffer, occasionally submit the Msgs in one Tx

    send_multi_msg_txs(
        contact,
        &mut msgs,
        &tx_sizes,
        &mut tx_idx,
        sender,
        sender_addr.clone(),
        receiver,
        cosmos_denom.clone(),
        queued_transactions.cosmos_queue_amounts.clone(),
        queued_transactions.cosmos_queue_fees.clone(),
        tx_fee.clone(),
    )
    .await;

    send_multi_msg_txs(
        contact,
        &mut msgs,
        &tx_sizes,
        &mut tx_idx,
        sender,
        sender_addr.clone(),
        receiver,
        cosmos_denom.clone(),
        queued_transactions.cosmos_failure_amounts.clone(),
        queued_transactions.cosmos_failure_fees.clone(),
        tx_fee.clone(),
    )
    .await;

    send_multi_msg_txs(
        contact,
        &mut msgs,
        &tx_sizes,
        &mut tx_idx,
        sender,
        sender_addr.clone(),
        receiver,
        cosmos_denom.clone(),
        queued_transactions.erc20_queue_amounts.clone(),
        queued_transactions.erc20_queue_fees.clone(),
        tx_fee.clone(),
    )
    .await;

    send_multi_msg_txs(
        contact,
        &mut msgs,
        &tx_sizes,
        &mut tx_idx,
        sender,
        sender_addr.clone(),
        receiver,
        cosmos_denom.clone(),
        queued_transactions.erc20_failure_amounts.clone(),
        queued_transactions.erc20_failure_fees.clone(),
        tx_fee.clone(),
    )
    .await;

    let results = sum_expected_results(queued_transactions);
    info!(
        "send_to_eth_multi_msg_txs: transactions sent, expecting fees: {:?}{}, {:?}{}",
        results.expected_erc20_fee, erc20_denom, results.expected_cosmos_fee, cosmos_denom,
    );
    results
}

#[allow(clippy::too_many_arguments)]
async fn send_multi_msg_txs(
    contact: &Contact,
    msgs: &mut Vec<Msg>,
    tx_sizes: &[usize],
    tx_idx: &mut usize,
    sender: CosmosPrivateKey,
    sender_addr: String,
    receiver: EthAddress,
    denom: String,
    bridge_amounts: Vec<Uint256>,
    fee_amounts: Vec<Uint256>,
    tx_fee: Coin,
) {
    let fee_for_relayer = Coin {
        denom: denom.clone(),
        amount: 1u8.into(),
    };
    for (bridge, fee) in bridge_amounts.into_iter().zip(fee_amounts.into_iter()) {
        let bridge_coin = Coin {
            denom: denom.clone(),
            amount: bridge,
        };
        let fee_coin = Coin {
            denom: denom.clone(),
            amount: fee,
        };
        let msg_send_to_eth = MsgSendToEth {
            sender: sender_addr.clone(),
            eth_dest: receiver.to_string(),
            amount: Some(bridge_coin.into()),
            bridge_fee: Some(fee_for_relayer.clone().into()),
            chain_fee: Some(fee_coin.into()),
        };

        let msg = Msg::new(MSG_SEND_TO_ETH_TYPE_URL, msg_send_to_eth);
        msgs.push(msg);

        if msgs.len() >= *tx_sizes.get(*tx_idx).unwrap() {
            let to_send = msgs.clone();
            msgs.clear();
            if tx_idx < &mut (tx_sizes.len() - 1) {
                *tx_idx += 1;
            }
            info!("Sending multiple MsgSendToEth: \n\n\n{:?}\n\n\n", msgs);
            let res = contact
                .send_message(
                    &to_send,
                    None,
                    &[tx_fee.clone()],
                    Some(OPERATION_TIMEOUT),
                    None,
                    sender,
                )
                .await;
            info!("Sent MsgSendToEth with res {:?}", res);
        }
    }
    if !msgs.is_empty() {
        let res = contact
            .send_message(
                msgs,
                None,
                &[tx_fee.clone()],
                Some(OPERATION_TIMEOUT),
                None,
                sender,
            )
            .await;
        info!("Sent FINAL MsgSendToEth with res {:?}", res);
    }
}
/// Creates a number of sends to Ethereum while changing the minimum fees up at the same time
pub async fn send_to_eth_while_changing_params(
    contact: &Contact,
    keys: &[ValidatorKeys],
    sender: CosmosPrivateKey,
    receiver: EthAddress,
    erc20_denom: String,
    cosmos_denom: String,
) -> SendToEthFeeExpectations {
    info!("send_to_eth_while_changing_params: Getting chain param");
    let current_fee_basis_points = get_min_chain_fee_basis_points(contact)
        .await
        .expect("Unable to get MinChainFeeBasisPoints");

    assert!(
        current_fee_basis_points != 0,
        "send_to_eth_while_changing_params requires that the current fee be configured!"
    );
    info!("send_to_eth_while_changing_params: setting up transactions");
    // Create the test transactions
    let mut queued_transactions = setup_transactions(
        current_fee_basis_points,
        3, // Create lots of sends for us to pack into Txs
    );
    let queued_transactions2 = setup_transactions(
        current_fee_basis_points * 10, // 10x the required fee amounts
        3,                             // Create lots of sends for us to pack into Txs
    );
    queued_transactions
        .cosmos_queue_amounts
        .append(&mut queued_transactions2.cosmos_queue_amounts.clone());
    queued_transactions
        .erc20_queue_amounts
        .append(&mut queued_transactions2.erc20_queue_amounts.clone());
    queued_transactions
        .cosmos_queue_fees
        .append(&mut queued_transactions2.cosmos_queue_fees.clone());
    queued_transactions
        .erc20_queue_fees
        .append(&mut queued_transactions2.erc20_queue_fees.clone());
    // Create and send Txs with multiple msgs in each tx
    let sender_address = sender.to_address(ADDRESS_PREFIX.as_str()).unwrap();
    let sender_addr = sender_address.to_string();

    // otherwise our Txs could fail due to a Tx with sequence number 3 executing before the Tx with sequence 2
    let some_stake = Coin {
        denom: (*STAKING_TOKEN).clone(),
        amount: 100u16.into(),
    };
    let args_fee = Fee {
        amount: vec![some_stake],
        gas_limit: 400_000,
        granter: None,
        payer: None,
    };

    // A template for our MessageArgs, we need to change the sequence each time we call contact.send_message()
    let args_tmpl8 = contact
        .get_message_args(sender_address, args_fee.clone(), None)
        .await
        .unwrap();

    // An effective queue of (MessageArgs, Vec<Msg>) for contact.send_message()
    let mut msg_args_queue = vec![];
    let mut msg_buf = vec![]; // A Msg buffer built iteratively and added to msg_args_queue

    // The number of txs to have in the 1st, 2nd, 3rd, ... Txs we submit
    let tx_sizes: Vec<usize> = vec![2, 4, 6, 5, 8, 3, 1, 3, 5, 4, 9, 12, 9, 1];
    let mut tx_idx = 0; // Our pointer into tx_sizes
    info!("send_to_eth_while_changing_params: creating Msgs");
    // Create a queue of multi-Msg Tx's to send later on
    for (bridge, fee) in queued_transactions
        .cosmos_queue_amounts
        .clone()
        .into_iter()
        .zip(queued_transactions.cosmos_queue_amounts.clone())
    {
        build_param_change_msgs(
            &mut msg_buf,
            &mut msg_args_queue,
            &args_tmpl8,
            &tx_sizes,
            &mut tx_idx,
            sender_addr.clone(),
            receiver,
            cosmos_denom.clone(),
            bridge,
            fee,
        );
    }
    // clear any remaining
    let to_send = msg_buf.clone();
    let tx_num: u64 = msg_args_queue.len() as u64; // which tx this is
    let mut args = args_tmpl8.clone();
    args.sequence += tx_num; // Fix sequence to avoid tx rejection
    args.fee.gas_limit = 5_000_000u64;
    msg_args_queue.push((args, to_send));

    for (bridge, fee) in queued_transactions
        .erc20_queue_amounts
        .clone()
        .into_iter()
        .zip(queued_transactions.erc20_queue_amounts.clone())
    {
        build_param_change_msgs(
            &mut msg_buf,
            &mut msg_args_queue,
            &args_tmpl8,
            &tx_sizes,
            &mut tx_idx,
            sender_addr.clone(),
            receiver,
            erc20_denom.clone(),
            bridge,
            fee,
        );
    }
    // clear any remaining
    let to_send = msg_buf.clone();
    let tx_num: u64 = msg_args_queue.len() as u64; // which tx this is
    let mut args = args_tmpl8.clone();
    args.sequence += tx_num; // Fix sequence to avoid tx rejection
    args.fee.gas_limit = 5_000_000u64;
    msg_args_queue.push((args, to_send));

    info!("send_to_eth_while_changing_params: changing the fees");
    let _ = submit_send_to_eth_fees_proposal(
        SendToEthFeesProposalJson {
            title: "Send to eth fees".to_string(),
            description: "send to eth fees".to_string(),
            min_chain_fee_basis_points: current_fee_basis_points * 10,
        },
        Coin {
            denom: (*STAKING_TOKEN).clone(),
            amount: 1000u32.into(),
        },
        Coin {
            denom: (*STAKING_TOKEN).clone(),
            amount: 0u32.into(),
        },
        contact,
        sender,
        Some(OPERATION_TIMEOUT),
    )
    .await;
    vote_yes_on_proposals(contact, keys, Some(OPERATION_TIMEOUT)).await;
    let delay = Duration::from_secs(GOVERNANCE_VOTING_PERIOD - 10);
    info!("send_to_eth_while_changing_params: waiting for proposal to start");
    sleep(delay).await;
    info!("send_to_eth_while_changing_params: proposal execution imminent, sending transactions");
    execute_queued_msgs(contact, msg_args_queue, sender).await;

    let min_expected_results = sum_expected_results(queued_transactions2);

    info!(
        "send_to_eth_while_changing_params: transactions sent, expecting fees: {:?}{}, {:?}{}",
        min_expected_results.expected_erc20_fee,
        erc20_denom,
        min_expected_results.expected_cosmos_fee,
        cosmos_denom,
    );
    min_expected_results
}

#[allow(clippy::too_many_arguments)]
fn build_param_change_msgs(
    msg_buf: &mut Vec<Msg>,
    msg_args_queue: &mut Vec<(MessageArgs, Vec<Msg>)>,
    args_tmpl8: &MessageArgs,
    tx_sizes: &[usize],
    tx_idx: &mut usize,
    sender_addr: String,
    receiver: EthAddress,
    denom: String,
    bridge: Uint256,
    fee: Uint256,
) {
    let fee_for_relayer = Coin {
        denom: denom.clone(),
        amount: 1u8.into(),
    };
    let bridge_coin = Coin {
        denom: denom.clone(),
        amount: bridge,
    };
    let fee_coin = Coin { denom, amount: fee };
    let msg_send_to_eth = MsgSendToEth {
        sender: sender_addr,
        eth_dest: receiver.to_string(),
        amount: Some(bridge_coin.into()),
        bridge_fee: Some(fee_for_relayer.into()),
        chain_fee: Some(fee_coin.into()),
    };

    let msg = Msg::new(MSG_SEND_TO_ETH_TYPE_URL, msg_send_to_eth);
    msg_buf.push(msg);

    let msgs_for_this_tx = *tx_sizes.get(*tx_idx).unwrap();
    if msg_buf.len() >= msgs_for_this_tx {
        let to_send = msg_buf.clone();
        let tx_num: u64 = msg_args_queue.len() as u64; // which tx this is

        // Create a MessageArgs which will help prioritize our Tx in the mempool
        let mut args = args_tmpl8.clone();
        args.sequence += tx_num; // Fix sequence to avoid tx rejection
        args.fee.gas_limit = 5_000_000u64;

        msg_args_queue.push((args, to_send));

        if tx_idx < &mut (tx_sizes.len() - 1) {
            *tx_idx += 1;
        }
    }
}

pub async fn execute_queued_msgs(
    contact: &Contact,
    msg_args_queue: Vec<(MessageArgs, Vec<Msg>)>,
    sender: impl PrivateKey,
) {
    for (args, msgs) in msg_args_queue {
        let res = contact
            .send_message_with_args(&msgs, None, args, Some(OPERATION_TIMEOUT), sender.clone())
            .await;
        info!("Sent MsgSendToEth with res {:?}", res);
    }
}

#[derive(Debug)]
pub struct QueuedTransactions {
    pub erc20_queue_amounts: Vec<Uint256>,
    pub erc20_queue_fees: Vec<Uint256>,
    pub cosmos_queue_amounts: Vec<Uint256>,
    pub cosmos_queue_fees: Vec<Uint256>,
    pub erc20_failure_amounts: Vec<Uint256>,
    pub erc20_failure_fees: Vec<Uint256>,
    pub cosmos_failure_amounts: Vec<Uint256>,
    pub cosmos_failure_fees: Vec<Uint256>,
}

fn setup_transactions(
    min_fee_basis_points: u64,
    tests_per_fee_amount: usize,
) -> QueuedTransactions {
    // Convert the minimum fee param to a useable fee for a send of one eth (1 * 10^18)
    let curr_fee_basis_points = Uint256::from(min_fee_basis_points);
    let erc20_bridge_amount: Uint256 = one_eth();
    let erc20_min_fee = get_min_send_to_eth_fee(erc20_bridge_amount, curr_fee_basis_points);
    let erc20_success_fees: Vec<Uint256> = get_success_test_fees(erc20_min_fee);
    let erc20_fail_fees: Vec<Uint256> = get_fail_test_fees(erc20_min_fee);
    info!(
        "setup_transactions: Created erc20 fees: \nSuccess [{:?}]\nFailure[{:?}]",
        erc20_success_fees, erc20_fail_fees
    );

    // ... and for a send of one atom (1 * 10^6)
    let cosmos_bridge_amount = one_atom();
    let cosmos_min_fee = get_min_send_to_eth_fee(cosmos_bridge_amount, curr_fee_basis_points);
    let cosmos_success_fees: Vec<Uint256> = get_success_test_fees(cosmos_min_fee);
    let cosmos_fail_fees: Vec<Uint256> = get_fail_test_fees(cosmos_min_fee);
    info!(
        "setup_transactions: Created footoken fees: \nSuccess [{:?}]\nFailure[{:?}]",
        cosmos_success_fees, cosmos_fail_fees
    );

    let mut erc20_good_amounts = vec![];
    let mut erc20_good_fees = vec![];
    let mut cosmos_good_amounts = vec![];
    let mut cosmos_good_fees = vec![];

    let mut erc20_bad_amounts = vec![];
    let mut erc20_bad_fees = vec![];
    let mut cosmos_bad_amounts = vec![];
    let mut cosmos_bad_fees = vec![];

    // Create some footoken success test cases from the above generated values
    queue_sends_to_eth(
        tests_per_fee_amount,
        cosmos_bridge_amount,
        cosmos_success_fees,
        &mut cosmos_good_amounts,
        &mut cosmos_good_fees,
    );
    // Create some footoken failure test cases from the above generated values
    queue_sends_to_eth(
        tests_per_fee_amount,
        cosmos_bridge_amount,
        cosmos_fail_fees,
        &mut cosmos_bad_amounts,
        &mut cosmos_bad_fees,
    );
    // Create some erc20 success test cases from the above generated values
    queue_sends_to_eth(
        tests_per_fee_amount,
        erc20_bridge_amount,
        erc20_good_fees.clone(),
        &mut erc20_good_amounts,
        &mut erc20_good_fees,
    );

    // Create some erc20 failure test cases from the above generated values
    queue_sends_to_eth(
        tests_per_fee_amount,
        erc20_bridge_amount,
        erc20_fail_fees,
        &mut erc20_bad_amounts,
        &mut erc20_bad_fees,
    );

    QueuedTransactions {
        erc20_queue_amounts: erc20_good_amounts,
        erc20_queue_fees: erc20_good_fees,
        cosmos_queue_amounts: cosmos_good_amounts,
        cosmos_queue_fees: cosmos_good_fees,
        erc20_failure_amounts: erc20_bad_amounts,
        erc20_failure_fees: erc20_bad_fees,
        cosmos_failure_amounts: cosmos_bad_amounts,
        cosmos_failure_fees: cosmos_bad_fees,
    }
}

fn sum_expected_results(qt: QueuedTransactions) -> SendToEthFeeExpectations {
    let mut expected_cosmos_fee = 0u8.into();
    let mut successful_cosmos_bridged = 0u8.into();
    let mut expected_erc20_fee = 0u8.into();
    let mut successful_erc20_bridged = 0u8.into();
    for csa in qt.cosmos_queue_amounts {
        successful_cosmos_bridged += csa;
    }
    for csf in qt.cosmos_queue_fees {
        expected_cosmos_fee += csf;
    }
    for esa in qt.erc20_queue_amounts {
        successful_erc20_bridged += esa;
    }
    for esf in qt.erc20_queue_fees {
        expected_erc20_fee += esf;
    }

    SendToEthFeeExpectations {
        expected_erc20_fee,
        expected_cosmos_fee,
        successful_erc20_bridged,
        successful_cosmos_bridged,
    }
}

/// Adds the desired test cases to their respective Vec's and the fee tally hashmaps
/// More sends to eth will be queued if tests_per_fee_amount is > 1
#[allow(clippy::too_many_arguments)]
fn queue_sends_to_eth(
    tests_per_fee_amount: usize,
    bridge_amount: Uint256,
    fee_amounts: Vec<Uint256>,
    bridge_collection: &mut Vec<Uint256>,
    fee_collection: &mut Vec<Uint256>,
) {
    for fee in fee_amounts {
        for _ in 0..tests_per_fee_amount {
            bridge_collection.push(bridge_amount);
            fee_collection.push(fee);
        }
    }
}
pub fn get_success_test_fees(min_fee: Uint256) -> Vec<Uint256> {
    if min_fee == 0u8.into() {
        vec![0u8.into(), 1u8.into()]
    } else {
        vec![min_fee, min_fee + 10u8.into()]
    }
}

pub fn get_fail_test_fees(min_fee: Uint256) -> Vec<Uint256> {
    if min_fee == 0u8.into() {
        vec![]
    } else {
        vec![min_fee - 1u8.into()]
    }
}

pub async fn submit_and_pass_send_to_eth_fees_proposal(
    min_chain_fee_basis_points: u64,
    contact: &Contact,
    keys: &[ValidatorKeys],
) {
    let proposal_content = SendToEthFeesProposalJson {
        title: format!(
            "Set MinChainFeeBasisPoints to {}",
            min_chain_fee_basis_points
        ),
        description: "MinChainFeeBasisPoints!".to_string(),
        min_chain_fee_basis_points,
    };
    let res = submit_send_to_eth_fees_proposal(
        proposal_content,
        get_deposit(None),
        get_fee(None),
        contact,
        keys[0].validator_key,
        Some(TOTAL_TIMEOUT),
    )
    .await;
    vote_yes_on_proposals(contact, keys, None).await;
    wait_for_proposals_to_execute(contact).await;
    info!("Gov proposal executed with {:?}", res);

    let start = Instant::now();
    while Instant::now() - start < OPERATION_TIMEOUT {
        let set_value = get_min_chain_fee_basis_points(contact)
            .await
            .expect("Unable to get MinChainFeeBasisPoints");
        if min_chain_fee_basis_points != set_value {
            continue;
        }
        info!(
            "Successfully updated MinChainFeeBasisPoints parameter to {}!",
            set_value
        );
        return;
    }
    panic!("Unable to set MinChainFeeBasisPoints");
}

pub fn assert_fees_collected(
    start_sender_bal: Vec<Coin>,
    end_sender_bal: Vec<Coin>,
    fee_expectations: SendToEthFeeExpectations,
    erc20_denom: String,
    cosmos_denom: String,
) {
    // Say we have 3 MsgSendToEths execute for (1, .0001), (2, .0002), and (3, .0003)
    // Then we expect our total balance difference to be -6.0006, if we start with 10 then we would expect to see
    // 3.9994 (current balance = start balance - amounts bridged - fees paid)
    // So we want to find fees paid = current balance - start balance + amounts bridged

    info!(
        "start_sender_bal: {:?}\nend_sender_bal: {:?}\nfee_expectations: {:?}",
        start_sender_bal, end_sender_bal, fee_expectations,
    );

    let mut cosmos_diff: i128 = 0u8.into();
    let mut erc20_diff: i128 = 0u8.into();
    // Load the map with our current balances = starting balance - bridge amount - fees
    for token in &end_sender_bal {
        if token.denom == cosmos_denom {
            cosmos_diff = token.amount.to_i128().unwrap_or(0i128);
        } else if token.denom == erc20_denom {
            erc20_diff = token.amount.to_i128().unwrap_or(0i128);
        }
        continue;
    }
    info!(
        "set ending bals: cosmos_diff {} erc20_diff {}",
        cosmos_diff, erc20_diff
    );

    // Add to the current balance the amounts bridged, to get starting balance - fees
    cosmos_diff += fee_expectations
        .successful_cosmos_bridged
        .to_i128()
        .unwrap_or(0i128);
    erc20_diff += fee_expectations
        .successful_erc20_bridged
        .to_i128()
        .unwrap_or(0i128);

    info!(
        "add bridge amounts: cosmos_diff {} erc20_diff {}",
        cosmos_diff, erc20_diff
    );
    // Find fees = starting balance - (starting balance - fees)
    for token in &start_sender_bal {
        let pre = token.amount.to_i128().unwrap_or(0i128);
        if token.denom == cosmos_denom {
            cosmos_diff = pre - cosmos_diff;
        } else if token.denom == erc20_denom {
            erc20_diff = pre - erc20_diff;
        }
        continue;
    }
    info!(
        "add starting bals, should be just fees left: cosmos_diff {} erc20_diff {}",
        cosmos_diff, erc20_diff
    );

    let expected_cosmos_fee = fee_expectations
        .expected_cosmos_fee
        .to_i128()
        .unwrap_or(0i128);

    if cosmos_diff < expected_cosmos_fee {
        panic!(
            "Unexpected amount of {} removed due to fees ({}), expected at least {}",
            cosmos_denom, cosmos_diff, expected_cosmos_fee,
        )
    }

    let expected_erc20_fee = fee_expectations
        .expected_erc20_fee
        .to_i128()
        .unwrap_or(0i128);

    if erc20_diff < expected_erc20_fee {
        panic!(
            "Unexpected amount of {} removed due to fees ({}), expected at least {}",
            erc20_denom, erc20_diff, expected_erc20_fee,
        )
    }
}
