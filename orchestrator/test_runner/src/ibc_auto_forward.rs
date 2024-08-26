use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::happy_path::send_erc20_deposit;
use crate::utils::*;
use crate::OPERATION_TIMEOUT;
use crate::{
    get_ibc_chain_id, one_eth, ADDRESS_PREFIX, COSMOS_NODE_GRPC, IBC_ADDRESS_PREFIX, IBC_NODE_GRPC,
    STAKING_TOKEN,
};
use clarity::Address as EthAddress;
use cosmos_gravity::proposals::UPDATE_HRP_IBC_CHANNEL_PROPOSAL;
use cosmos_gravity::send::MSG_EXECUTE_IBC_AUTO_FORWARDS_TYPE_URL;
use deep_space::address::Address as CosmosAddress;
use deep_space::client::type_urls::MSG_TRANSFER_TYPE_URL;
use deep_space::error::CosmosGrpcError;
use deep_space::private_key::{CosmosPrivateKey, PrivateKey};
use deep_space::utils::decode_any;
use deep_space::utils::encode_any;
use deep_space::{Coin as DSCoin, Contact, Msg};
use gravity_proto::cosmos_sdk_proto::bech32ibc::bech32ibc::v1::UpdateHrpIbcChannelProposal;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::{
    v1beta1 as Bank, v1beta1::query_client::QueryClient as BankQueryClient,
};
use gravity_proto::cosmos_sdk_proto::cosmos::base::v1beta1::Coin;
use gravity_proto::cosmos_sdk_proto::ibc::applications::transfer::v1::MsgTransfer;
use gravity_proto::cosmos_sdk_proto::ibc::applications::transfer::{
    v1 as IbcTransferV1, v1::query_client::QueryClient as IbcTransferQueryClient,
};
use gravity_proto::cosmos_sdk_proto::ibc::core::channel::v1::query_client::QueryClient as IbcChannelQueryClient;
use gravity_proto::cosmos_sdk_proto::ibc::core::channel::v1::IdentifiedChannel;
use gravity_proto::cosmos_sdk_proto::ibc::core::channel::v1::{
    QueryChannelClientStateRequest, QueryChannelsRequest,
};
use gravity_proto::cosmos_sdk_proto::ibc::lightclients::tendermint::v1::ClientState;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::{
    MsgExecuteIbcAutoForwards, PendingIbcAutoForward, QueryPendingIbcAutoForwards,
};
use gravity_utils::error::GravityError;
use gravity_utils::num_conversion::one_atom;
use num256::Uint256;
use std::cmp::Ordering;
use std::ops::{Add, Mul};
use std::str::FromStr;
use std::time::Instant;
use std::time::{Duration, SystemTime};
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use web30::client::Web3;

// Tests IBC transfers and IBC Auto-Forwarding from gravity to another chain (gravity-test-1 -> ibc-test-1)
pub async fn ibc_auto_forward_test(
    web30: &Web3,
    gravity_client: GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    ibc_keys: Vec<CosmosPrivateKey>,
    gravity_address: EthAddress,
    erc20_address: EthAddress,
) {
    let no_relay_market_config = create_default_test_config();
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;

    let ibc_user_keys = get_user_key(Some("cosmos"));

    let gravity_channel_qc = IbcChannelQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect channel query client");
    let ibc_bank_qc = BankQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect bank query client");
    let ibc_transfer_qc = IbcTransferQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect ibc-transfer query client");

    // Wait for the ibc channel to be created and find the channel ids
    let channel_id_timeout = Duration::from_secs(60 * 5);
    let gravity_channel_id = get_channel_id(
        gravity_channel_qc,
        get_ibc_chain_id(),
        Some(channel_id_timeout),
    )
    .await
    .expect("Could not find gravity-test-1 channel");

    // Test an IBC transfer of 1 stake from gravity-test-1 to ibc-test-1
    let sender = keys[0].validator_key;
    let receiver = ibc_keys[0].to_address(&IBC_ADDRESS_PREFIX).unwrap();
    test_ibc_transfer(
        contact,
        ibc_bank_qc.clone(),
        ibc_transfer_qc.clone(),
        sender,
        receiver,
        None,
        None,
        gravity_channel_id.clone(),
        Duration::from_secs(60 * 5),
    )
    .await;

    info!("\n\n!!!!!!!!!! Start IBC Auto-Forward Happy Path Test !!!!!!!!!!\n\n");
    setup_gravity_auto_forwards(
        contact,
        (*IBC_ADDRESS_PREFIX).clone(),
        gravity_channel_id.clone(),
        sender,
        &keys,
    )
    .await;
    test_ibc_auto_forward_happy_path(
        web30,
        contact,
        gravity_client.clone(),
        ibc_bank_qc.clone(),
        ibc_transfer_qc.clone(),
        sender,
        ibc_user_keys.cosmos_address,
        gravity_address,
        erc20_address,
        one_eth(),
    )
    .await
    .expect("Failed IBC Auto-Forward Happy Path Test");
    info!("Successful IBC Auto-Forward Happy Path Test");

    // Make sure users can't "hijack" the chain by forwarding all "gravity1" balances to another chain
    // This attack would also require the foreign chain to obstruct IBC transfers back to gravity,
    // but still worst case scenario is a chain restart from last valid state with genesis scrubbing
    info!("\n\n!!!!!!!!!! Start IBC Auto-Forward Native Hijack Test !!!!!!!!!!\n\n");
    setup_native_hijack(contact, gravity_channel_id.clone(), sender, &keys).await;

    test_ibc_auto_forward_native_hijack(
        web30,
        contact,
        gravity_client.clone(),
        ibc_bank_qc.clone(),
        ibc_transfer_qc.clone(),
        ibc_user_keys,
        gravity_address,
        erc20_address,
        one_eth(),
    )
    .await
    .expect("Failed IBC Auto-Forward Native Hijack Test");
    info!("Successful IBC Auto-Forward Native Hijack Prevention");

    // Make sure tokens sent to an unregistered chain wind up on gravity without producing an
    // ibc auto forward
    info!("\n\n!!!!!!!!!! Start IBC Auto-Forward Unregistered Chain Test !!!!!!!!!!\n\n");

    let cavity_user_keys = get_user_key(Some("cavity")); // Worst Gravity clone name possible
    test_ibc_auto_forward_unregistered_chain(
        web30,
        contact,
        gravity_client.clone(),
        ibc_bank_qc.clone(),
        ibc_transfer_qc.clone(),
        cavity_user_keys,
        gravity_address,
        erc20_address,
        one_eth(),
    )
    .await
    .expect("Failed IBC Auto-Forward Unregistered Chain Test");
    info!("Successful IBC Auto-Forward Unregistered Chain Handling");
}

// Sends 1 gravity-test-1 stake from `sender` to `receiver` on ibc-test-1 and asserts receipt of funds
#[allow(clippy::too_many_arguments)]
pub async fn test_ibc_transfer(
    contact: &Contact,                     // Src chain's deep_space client
    dst_bank_qc: BankQueryClient<Channel>, // Dst chain's GRPC x/bank query client
    dst_ibc_transfer_qc: IbcTransferQueryClient<Channel>, // Dst chain's GRPC ibc-transfer query client
    sender: impl PrivateKey,                              // The Src chain's funds sender
    receiver: CosmosAddress,                              // The Dst chain's funds receiver
    coin: Option<Coin>,                                   // The coin to send to receiver
    fee_coin: Option<DSCoin>, // The fee to pay for submitting the transfer msg
    channel_id: String,       // The Src chain's ibc channel connecting to Dst
    packet_timeout: Duration, // Used to create ibc-transfer timeout-timestamp
) -> bool {
    let sender_address = sender.to_address(&ADDRESS_PREFIX).unwrap().to_string();
    let pre_bal = get_ibc_balance(
        receiver,
        (*STAKING_TOKEN).to_string(),
        None,
        dst_bank_qc.clone(),
        dst_ibc_transfer_qc.clone(),
        None,
    )
    .await;

    let timeout_timestamp = SystemTime::now()
        .add(packet_timeout)
        .duration_since(SystemTime::UNIX_EPOCH)
        .unwrap()
        .as_nanos() as u64;
    info!("Calculated 150 minutes from now: {:?}", timeout_timestamp);
    let coin = coin.unwrap_or(Coin {
        denom: STAKING_TOKEN.to_string(),
        amount: one_atom().to_string(),
    });
    let msg_transfer = MsgTransfer {
        source_port: "transfer".to_string(),
        source_channel: channel_id,
        token: Some(coin.clone()),
        sender: sender_address,
        receiver: receiver.to_string(),
        timeout_height: None,
        timeout_timestamp, // 150 minutes from now
        ..Default::default()
    };
    info!("Submitting MsgTransfer {:?}", msg_transfer);
    let msg_transfer = Msg::new(MSG_TRANSFER_TYPE_URL, msg_transfer);
    let fee_coin = fee_coin.unwrap_or(DSCoin {
        amount: 100u16.into(),
        denom: (*STAKING_TOKEN).to_string(),
    });
    let send_res = contact
        .send_message(
            &[msg_transfer],
            Some("Test Relaying".to_string()),
            &[fee_coin],
            Some(OPERATION_TIMEOUT),
            None,
            sender,
        )
        .await;
    info!("Sent MsgTransfer with response {:?}", send_res);

    // Give the ibc-relayer a bit of time to work in the event of multiple runs
    delay_for(Duration::from_secs(10)).await;

    let start_bal = Some(match pre_bal.clone() {
        Some(coin) => Uint256::from_str(&coin.amount).unwrap(),
        None => 0u8.into(),
    });

    let post_bal = get_ibc_balance(
        receiver,
        (*STAKING_TOKEN).to_string(),
        start_bal,
        dst_bank_qc,
        dst_ibc_transfer_qc,
        None,
    )
    .await;
    match (pre_bal, post_bal) {
        (None, None) => {
            error!("Failed to transfer stake to ibc-test-1 user {}!", receiver);
            return false;
        }
        (None, Some(post)) => {
            if post.amount != coin.amount {
                error!(
                    "Incorrect ibc stake balance for user {}: actual {} != expected {}",
                    receiver, post.amount, coin.amount,
                );
                return false;
            }
            info!(
                "Successfully transfered {} stake (aka {}) to ibc-test-1!",
                coin.amount, post.denom
            );
        }
        (Some(pre), Some(post)) => {
            let amount_uint = Uint256::from_str(&coin.amount).unwrap();
            let pre_amt = Uint256::from_str(&pre.amount).unwrap();
            let post_amt = Uint256::from_str(&post.amount).unwrap();
            if post_amt < pre_amt || post_amt - pre_amt != amount_uint {
                error!(
                    "Incorrect ibc stake balance for user {}: actual {} != expected {}",
                    receiver,
                    post.amount,
                    (pre_amt + amount_uint),
                );
                return false;
            }
            info!(
                "Successfully transfered {} stake (aka {}) to ibc-test-1!",
                coin.amount, post.denom
            );
        }
        (Some(_), None) => {
            error!(
                "User wound up with no balance after ibc transfer? {}",
                receiver,
            );
            return false;
        }
    }
    true
}

// Retrieves the channel connecting the chain behind `ibc_channel_qc` and the chain with id `foreign_chain_id`
// Retries up to `timeout` (or OPERATION_TIMEOUT if None), checking each channel's client state to find the foreign chain's id
pub async fn get_channel(
    ibc_channel_qc: IbcChannelQueryClient<Channel>, // The Src chain's IbcChannelQueryClient
    foreign_chain_id: String,                       // The chain-id of the Dst chain
    timeout: Option<Duration>,
) -> Result<IdentifiedChannel, CosmosGrpcError> {
    let mut ibc_channel_qc = ibc_channel_qc;
    let timeout = match timeout {
        Some(t) => t,
        None => OPERATION_TIMEOUT,
    };

    let start = Instant::now();
    while Instant::now() - start < timeout {
        let channels = ibc_channel_qc
            .channels(QueryChannelsRequest { pagination: None })
            .await;
        if channels.is_err() {
            delay_for(Duration::from_secs(5)).await;
            continue;
        }
        let channels = channels.unwrap().into_inner().channels;
        for channel in channels {
            // Make an IBC Channel ClientState request with port=transfer channel=channel.channel_id
            let client_state_res = ibc_channel_qc
                .channel_client_state(QueryChannelClientStateRequest {
                    port_id: "transfer".to_string(),
                    channel_id: channel.channel_id.clone(),
                })
                .await;
            if client_state_res.is_err() {
                continue;
            }

            let client_state = client_state_res
                .unwrap()
                .into_inner()
                .identified_client_state;
            if client_state.is_none() || client_state.clone().unwrap().client_state.is_none() {
                error!(
                    "Got None response for {}/transfer client state!",
                    channel.channel_id.clone()
                );
                continue;
            }
            let client_state_any = client_state.unwrap().client_state.unwrap();

            // Check to see if this client state contains foreign_chain_id (e.g. "cavity-1")
            let client_state = decode_any::<ClientState>(client_state_any).unwrap();
            if client_state.chain_id == foreign_chain_id {
                return Ok(channel);
            }
        }
    }
    Err(CosmosGrpcError::BadResponse("No such channel".to_string()))
}

// Retrieves just the ID of the channel connecting the chain behind `ibc_channel_qc` and the chain with id `foreign_chain_id`
// Retries up to `timeout` (or OPERATION_TIMEOUT if None), checking each channel's client state to find the foreign chain's id
pub async fn get_channel_id(
    ibc_channel_qc: IbcChannelQueryClient<Channel>, // The Src chain's IbcChannelQueryClient
    foreign_chain_id: String,                       // The chain-id of the Dst chain
    timeout: Option<Duration>,
) -> Result<String, CosmosGrpcError> {
    Ok(get_channel(ibc_channel_qc, foreign_chain_id, timeout)
        .await?
        .channel_id)
}

// Retrieves the balance `account` holds of `src_denom`'s IBC representation
// Note: The Coin returned has the ibc/<HASH> denom, not the `src_chain` denom
// Retries up to `timeout` or OPERATION_TIMEOUT if not provided
pub async fn get_ibc_balance(
    account: CosmosAddress,                // The account's balance to check on dst
    src_denom: String,                     // The name of the asset on Src chain
    starting_balance: Option<Uint256>,     // Expected ibc asset starting balance to ignore
    dst_bank_qc: BankQueryClient<Channel>, // Dst chain's Bank GRPC client
    dst_ibc_transfer_qc: IbcTransferQueryClient<Channel>, // Dst chain's ibc-transfer GRPC client
    timeout: Option<Duration>,             // How long to loop for checking account's balance
) -> Option<Coin> {
    let mut dst_bank_qc = dst_bank_qc;
    let mut dst_ibc_transfer_qc = dst_ibc_transfer_qc;
    let timeout = match timeout {
        Some(t) => t,
        None => OPERATION_TIMEOUT,
    };
    let start = Instant::now();
    let mut discovered_balance = None;
    let expected_balance = match starting_balance {
        Some(expected) => expected,
        None => 0u8.into(),
    };
    while Instant::now() - start < timeout {
        let res = dst_bank_qc
            .all_balances(Bank::QueryAllBalancesRequest {
                address: account.to_string(),
                pagination: None,
            })
            .await;
        let res = res.expect("No response from bank balance query?");

        // Check each ibc/ balance the account holds
        for bal in res.into_inner().balances {
            if bal.denom.clone()[..4] == *"ibc/".to_string() {
                // only consider ibc denoms
                let hash = bal.denom.clone();
                let hash = &hash[4..]; // Strip leading 'ibc/'
                let denom_res = dst_ibc_transfer_qc
                    .denom_trace(IbcTransferV1::QueryDenomTraceRequest {
                        hash: hash.to_string(),
                    })
                    .await
                    .unwrap();
                let denom_trace = denom_res.into_inner().denom_trace;
                if denom_trace.is_none() {
                    // weird, but not critical for the test
                    error!(
                        "Found an 'ibc/'-prefixed denom which is not registered with ibc-transfer?"
                    );
                    continue;
                }

                if denom_trace.unwrap().base_denom == src_denom {
                    discovered_balance = Some(bal);
                    break;
                }
            }
        }

        if let Some(discovered) = discovered_balance.clone() {
            let amt = Uint256::from_str(&discovered.amount).unwrap();
            if amt == expected_balance {
                break;
            }
        }
        delay_for(Duration::from_secs(1)).await;
    }
    discovered_balance
}

// Creates and ratifies a bech32ibc prefix -> IBC channel mapping
pub async fn setup_gravity_auto_forwards(
    contact: &Contact, // Src chain's deep_space client
    prefix: String,
    source_channel: String,
    sender: CosmosPrivateKey, // The Src chain's funds sender
    keys: &[ValidatorKeys],
) {
    let proposal_any = encode_any(
        UpdateHrpIbcChannelProposal {
            title: "title here".to_string(),
            description: "description here".to_string(),
            hrp: prefix,
            source_channel,
            ics_to_height_offset: u64::MAX,
            ics_to_time_offset: None,
        },
        UPDATE_HRP_IBC_CHANNEL_PROPOSAL,
    );

    let _proposal_res = contact
        .create_gov_proposal(
            proposal_any,
            DSCoin {
                denom: (*STAKING_TOKEN).clone(),
                amount: one_atom().mul(10u8.into()),
            },
            DSCoin {
                denom: (*STAKING_TOKEN).clone(),
                amount: 0u8.into(),
            },
            sender,
            Some(OPERATION_TIMEOUT),
        )
        .await
        .expect("Unable to submit UpdateHrpIbcChannelProposal");

    vote_yes_on_proposals(contact, keys, None).await;
    wait_for_proposals_to_execute(contact).await;
}

// Initiates a SendToCosmos with a CosmosReceiver prefixed by "cosmos1", potentially clears a pending
// IBC Auto-Forward and asserts that the bridged ERC20 is received on ibc-test-1
#[allow(clippy::too_many_arguments)]
pub async fn test_ibc_auto_forward_happy_path(
    web30: &Web3,
    contact: &Contact,
    gravity_client: GravityQueryClient<Channel>, // Src chain's Gravity GRPC client
    dst_bank_qc: BankQueryClient<Channel>,       // Dst chain's Bank GRPC client
    dst_ibc_transfer_qc: IbcTransferQueryClient<Channel>, // Dst chain's ibc-transfer GRPC client
    forwarder: CosmosPrivateKey, // user who submits MsgExecutePendingIbcAutoForwards
    dest: CosmosAddress,         // The bridged + auto-forwarded ERC20 receiver
    gravity_address: EthAddress, // Address of the gravity contract
    erc20_address: EthAddress,   // Address of the ERC20 to send to dest on ibc-test-1
    amount: Uint256,             // The amount of erc20_address token to send to dest on ibc-test-1
) -> Result<(), GravityError> {
    // Make the test idempotent by getting the user's balance now
    let bridged_erc20 = "gravity".to_string() + &erc20_address.clone().to_string();
    let pre_forward_balance = get_ibc_balance(
        dest,
        bridged_erc20.clone(),
        None,
        dst_bank_qc.clone(),
        dst_ibc_transfer_qc.clone(),
        Some(Duration::from_secs(5)),
    )
    .await;
    info!("Found pre-forward-balance of {:?}", pre_forward_balance);
    // First Send to Cosmos
    send_erc20_deposit(
        web30,
        &mut gravity_client.clone(),
        dest,
        gravity_address,
        erc20_address,
        amount,
    )
    .await?;

    // Check for a Pending IBC Auto-Forward (which may have already been cleared by the running relayer)
    let pending =
        wait_for_pending_ibc_auto_forwards(gravity_client.clone(), None, Some(OPERATION_TIMEOUT))
            .await;

    // Attempt to clear the Pending forward
    if !(pending.is_err() || pending.unwrap().is_empty()) {
        info!("Discovered pending IBC Auto Forward(s) that the relayer hasn't picked up, clearing it!");
        let msg_execute_forwards = Msg::new(
            MSG_EXECUTE_IBC_AUTO_FORWARDS_TYPE_URL,
            MsgExecuteIbcAutoForwards {
                forwards_to_clear: 1,
                executor: forwarder.to_address(&ADDRESS_PREFIX).unwrap().to_string(),
            },
        );
        let _res = contact
            .send_message(
                &[msg_execute_forwards],
                None,
                &[DSCoin {
                    denom: (*STAKING_TOKEN).clone(),
                    amount: 0u8.into(),
                }],
                Some(OPERATION_TIMEOUT),
                None,
                forwarder,
            )
            .await?;
        info!("Sleeping to give the ibc-relayer time to work");
        delay_for(OPERATION_TIMEOUT).await;
    }

    let start_bal = match pre_forward_balance.clone() {
        Some(coin) => Some(Uint256::from_str(&coin.amount).unwrap()),
        None => None,
    };
    // Check the Foreign Receiver's balance has increased by the appropriate amount
    let post_forward_balance = get_ibc_balance(
        dest,
        bridged_erc20.clone(),
        start_bal,
        dst_bank_qc.clone(),
        dst_ibc_transfer_qc.clone(),
        Some(Duration::from_secs(60)),
    )
    .await;
    // Potential race condition: Slow gravity relayers and/or ibc relayer
    info!("Found a post-forward-balance of {:?}", post_forward_balance);
    match (pre_forward_balance, post_forward_balance) {
        (None, None) => {
            panic!("Never found an ibc auto-forward balance for user {}", dest);
        }
        (None, Some(post)) => {
            if Uint256::from_str(&post.amount).unwrap() != amount {
                panic!(
                    "Incorrect ibc auto-forward balance for user {}: actual {} != expected {}",
                    dest, post.amount, amount,
                );
            }
            info!(
                "Successful IBC auto-forward of amount {} to {}",
                amount, dest,
            );
            Ok(())
        }
        (Some(pre), Some(post)) => {
            let pre_amt = Uint256::from_str(&pre.amount).unwrap();
            let post_amt = Uint256::from_str(&post.amount).unwrap();
            if post_amt < pre_amt || pre_amt - post_amt != amount {
                panic!(
                    "Incorrect ibc auto-forward balance for user {}: actual {} != expected {}",
                    dest,
                    post.amount,
                    (pre_amt + amount)
                );
            }
            Ok(())
        }
        (Some(_), None) => {
            panic!(
                "User wound up with no balance after ibc auto-forward? {}",
                dest,
            );
        }
    }
}

// Waits for Pending IBC Auto Forwards to enter the queue by repeatedly querying via GRPC
// Returns a TimeoutError error if the GRPC endpoint never contains at least `expected`
// Optionally takes an `expected` number of pending forwards to wait for
// Optionally takes a timeout, otherwise waits for OPERATION_TIMEOUT seconds
pub async fn wait_for_pending_ibc_auto_forwards(
    gravity_client: GravityQueryClient<Channel>,
    expected: Option<u32>,
    timeout: Option<Duration>,
) -> Result<Vec<PendingIbcAutoForward>, GravityError> {
    let mut gravity_client = gravity_client;
    let timeout = match timeout {
        Some(t) => t,
        None => OPERATION_TIMEOUT,
    };
    let expected = expected.unwrap_or(1);
    let start = Instant::now();
    while Instant::now() - start < timeout {
        let res = gravity_client
            .get_pending_ibc_auto_forwards(QueryPendingIbcAutoForwards { limit: 0 })
            .await?
            .into_inner();
        if res.pending_ibc_auto_forwards.is_empty()
            || res.pending_ibc_auto_forwards.len() < expected as usize
        {
            delay_for(Duration::from_secs(5)).await;
            continue;
        }
        return Ok(res.pending_ibc_auto_forwards);
    }
    Err(GravityError::TimeoutError)
}

// Provides arguments for test_ibc_auto_forward_failure style tests
pub struct IbcAutoForwardFailureTest {
    pub src_gravity_client: GravityQueryClient<Channel>, // Src chain's Gravity GRPC client
    pub dst_bank_qc: BankQueryClient<Channel>,           // Dst chain's Bank GRPC client
    pub dst_ibc_transfer_qc: IbcTransferQueryClient<Channel>, // Dst chain's ibc-transfer GRPC client
    pub cosmos_receiver: CosmosAddress,                       // The SendToCosmos receiver
    pub dst_address: CosmosAddress, // The balance holding address to check on Dst chain
    pub src_address: CosmosAddress, // The balance holding address to check on Src chain
    pub gravity_address: EthAddress, // Address of the gravity contract
    pub erc20_address: EthAddress,  // Address of the ERC20 to send to dest user
    pub amount: Uint256, // The amount of erc20_address token to send to dest on ibc-test-1

    pub forward_pending: bool, // True -> Attempt to execute pending ibc auto forwards
}

// Provides a format for a single invalid IBC Auto-Forward test case
// A SendToCosmos is first initiated with receiver as input.dest_keys and token input.erc20_address
// Any pending IBC Auto-Forwards are optionally executed, and pre + post balances are gathered from
// both involved chains, finally both *_analyzer functions are called to determine success or failure (panic!())
pub async fn test_ibc_auto_forward_failure<
    F: Fn(Option<Coin>, Option<Coin>),
    G: Fn(Option<DSCoin>, Option<DSCoin>),
>(
    web30: &Web3,
    contact: &Contact,
    ibc_result_analyzer: Box<F>, // Checks to perform on foreign chain balance changes
    gravity_result_analyzer: Box<G>, // Checks to perform on gravity chain balance changes
    input: IbcAutoForwardFailureTest,
) -> Result<(), GravityError> {
    // Make the test idempotent by getting the user's balance now
    let bridged_erc20 = "gravity".to_string() + &input.erc20_address.clone().to_string();
    let ibc_pre_forward_balance = get_ibc_balance(
        input.dst_address,
        bridged_erc20.clone(),
        None,
        input.dst_bank_qc.clone(),
        input.dst_ibc_transfer_qc.clone(),
        Some(Duration::from_secs(5)),
    )
    .await;
    let gravity_pre_forward_balance = contact
        .get_balance(input.src_address, bridged_erc20.clone())
        .await?;
    info!(
        "Found pre-forward-balance: ibc {:?}, gravity {:?}",
        ibc_pre_forward_balance, gravity_pre_forward_balance
    );
    // First Send to Cosmos
    send_erc20_deposit(
        web30,
        &mut input.src_gravity_client.clone(),
        input.cosmos_receiver,
        input.gravity_address,
        input.erc20_address,
        input.amount,
    )
    .await?;

    if input.forward_pending {
        // Check for a Pending IBC Auto-Forward (which may have already been cleared by the running relayer)
        let pending = wait_for_pending_ibc_auto_forwards(
            input.src_gravity_client.clone(),
            None,
            Some(OPERATION_TIMEOUT),
        )
        .await;

        // Attempt to clear the Pending forward
        if let Ok(pending) = pending {
            if !pending.is_empty() {
                info!("Discovered pending IBC Auto Forward(s) that the relayer hasn't picked up, checking to see if it is invalid!");
                for pend in pending {
                    if pend.foreign_receiver.starts_with("gravity1") {
                        panic!(
                            "Found a gravity-prefixed pending IBC Auto-Forward! {:?}",
                            pend
                        );
                    }
                }
            }
        }
    }

    // Assert the balance was not received on the foreign chain
    let start_bal = match ibc_pre_forward_balance.clone() {
        Some(coin) => Some(Uint256::from_str(&coin.amount).unwrap()),
        None => None,
    };
    // Check the Foreign Receiver's balance has increased by the appropriate amount
    let ibc_post_forward_balance = get_ibc_balance(
        input.dst_address,
        bridged_erc20.clone(),
        start_bal,
        input.dst_bank_qc.clone(),
        input.dst_ibc_transfer_qc.clone(),
        Some(Duration::from_secs(60)),
    )
    .await;

    let gravity_post_forward_balance = contact
        .get_balance(input.src_address, bridged_erc20)
        .await?;

    info!(
        "Found post-forward-balance: ibc {:?}, gravity {:?}",
        ibc_post_forward_balance, gravity_post_forward_balance
    );

    // Call the match functions to determine if the test passed or failed based on balance changes
    // Potential race condition: Slow gravity relayers/block time
    ibc_result_analyzer(ibc_pre_forward_balance, ibc_post_forward_balance);
    // Potential race condition: Slow gravity relayers/block time
    gravity_result_analyzer(gravity_pre_forward_balance, gravity_post_forward_balance);

    Ok(())
}

// Creates and ratifies a "native hijack" bech32ibc proposal, which registers "gravity" as a prefix
// for a foreign chain
pub async fn setup_native_hijack(
    contact: &Contact, // Src chain's deep_space client
    source_channel: String,
    sender: CosmosPrivateKey, // The Src chain's funds sender
    keys: &[ValidatorKeys],
) {
    setup_gravity_auto_forwards(
        contact,
        (*ADDRESS_PREFIX).clone(),
        source_channel,
        sender,
        keys,
    )
    .await;
}

// Initiates a "SendToCosmos Native Hijack": Runs a SendToCosmos with CosmosReceiver prefixed by
// "gravity1", asserts no pending IBC Auto-Forward created, and asserts that the bridged ERC20 is
// NOT received on ibc-test-1, but rather on gravity-test-1 to a gravity re-prefixed account
#[allow(clippy::too_many_arguments)]
pub async fn test_ibc_auto_forward_native_hijack(
    web30: &Web3,
    contact: &Contact,
    src_gravity_client: GravityQueryClient<Channel>, // Src chain's Gravity GRPC client
    dst_bank_qc: BankQueryClient<Channel>,           // Dst chain's Bank GRPC client
    dst_ibc_transfer_qc: IbcTransferQueryClient<Channel>, // Dst chain's ibc-transfer GRPC client
    dest_keys: BridgeUserKey,                        // The bridged + auto-forwarded ERC20 receiver
    gravity_address: EthAddress,                     // Address of the gravity contract
    erc20_address: EthAddress, // Address of the ERC20 to send to dest on ibc-test-1
    amount: Uint256,           // The amount of erc20_address token to send to dest on ibc-test-1
) -> Result<(), GravityError> {
    let ibc_dest = dest_keys.cosmos_address;
    let gravity_prefixed_dest = dest_keys
        .cosmos_key
        .to_address(&ADDRESS_PREFIX.clone())
        .unwrap();
    let amt = amount;
    let ibc_match = move |pre_balance: Option<Coin>, post_balance: Option<Coin>| {
        match (pre_balance, post_balance) {
            (None, None) => {
                info!(
                    "Never found a native hijack ibc balance for user {}, still need to check local balance",
                    ibc_dest.to_string(),
                );
            }
            (None, Some(post)) => {
                if Uint256::from_str(&post.amount).unwrap() == amt {
                    panic!(
                        "Failed SendToCosmos native hijack: balance transfer to ibc user {}: actual {} != expected {}",
                        ibc_dest,
                        post.amount,
                        "0",
                    );
                } else {
                    warn!(
                        "Discovered potentially unrelated native hijack ibc balance: ibc transferred amount {} to {}",
                        amt.to_string(),
                        ibc_dest
                    );
                }
            }
            (Some(pre), Some(post)) => {
                let pre_amt = Uint256::from_str(&pre.amount).unwrap();
                let post_amt = Uint256::from_str(&post.amount).unwrap();
                if pre_amt == post_amt {
                    // Balance unchaged is good
                    info!(
                        "User {}'s IBC balance unchanged after native hijack attemt, still need to check local balance",
                        ibc_dest,
                    )
                } else if post_amt > pre_amt && post_amt - pre_amt == amt {
                    panic!(
                        "Discovered native hijack with ibc user {}: actual {} != expected {}",
                        ibc_dest, post.amount, pre_amt
                    );
                } else {
                    // Some other sort of balance change, could be an interfering test running?
                    warn!(
                        "Discovered potentially unrelated native hijack ibc balance: ibc balance is {} was {} with user {}",
                        post.amount,
                        pre.amount,
                        ibc_dest
                    );
                }
            }
            (Some(_), None) => {
                info!(
                    "User wound up with no ibc balance after native hijack attempt {}, still need to check local balance",
                    ibc_dest
                );
            }
        };
    };

    let amt = amount;
    let gravity_match = move |pre_balance: Option<DSCoin>, post_balance: Option<DSCoin>| {
        match (pre_balance, post_balance) {
            (None, None) => {
                panic!(
                    "Failed SendToCosmos native hijack: Never found a SendToCosmos local transfer balance for user {}",
                    gravity_prefixed_dest
                );
            }
            (None, Some(post)) => {
                if post.amount != amt {
                    panic!( // At this point there's no explanation for the lack of funds
                            "Discovered potentially unrelated native hijack local balance of amount {} with user {}",
                            amt,
                            gravity_prefixed_dest
                    );
                } else {
                    info!(
                        "Successful SendToCosmos native hijack prevention: Discovered local user {}: balance {} ",
                        gravity_prefixed_dest,
                        post.amount,
                    );
                }
            }
            (Some(pre), Some(post)) => {
                match post.amount.cmp(&(pre.amount + amt)) {
                    Ordering::Less => {
                        panic!( // At this point there's no explanation for the lack of funds
                           "Failed SendToCosmos native hijack: Discovered unexpected local balance with user {}: actual {} != expected {}",
                           gravity_prefixed_dest,
                           post.amount,
                           (pre.amount + amt)
                        );
                    }
                    Ordering::Equal => {
                        info!(
                            "Successful SendToCosmos native hijack prevention: Discovered local user {}: balance {} ",
                            ibc_dest.to_string(),
                            post.amount,
                        );
                    }
                    Ordering::Greater => {
                        warn!( // Somehow the balance is less than we would expect
                            "Discovered unexpected native hijack local balance of amount {} != expected {} with user {}",
                            post.amount,
                            (pre.amount + amt).to_string(),
                            gravity_prefixed_dest
                        );
                    }
                }
            }
            (Some(_), None) => {
                panic!(
                    "Failed SendToCosmos native hijack: User wound up with no local balance after hijack attempt? {}",
                    gravity_prefixed_dest
                );
            }
        };
    };

    test_ibc_auto_forward_failure(
        web30,
        contact,
        Box::new(ibc_match),
        Box::new(gravity_match),
        IbcAutoForwardFailureTest {
            src_gravity_client,
            dst_bank_qc,
            dst_ibc_transfer_qc,
            src_address: gravity_prefixed_dest,
            dst_address: ibc_dest,
            cosmos_receiver: gravity_prefixed_dest,
            gravity_address,
            erc20_address,
            amount,
            forward_pending: true,
        },
    )
    .await
}

// Initiates a SendToCosmos to an unregistered ibc chain, asserts no pending IBC Auto-Forward
// created, and asserts that the bridged ERC20 is NOT received on ibc-test-1, but rather on
// gravity-test-1 to a gravity re-prefixed account
#[allow(clippy::too_many_arguments)]
pub async fn test_ibc_auto_forward_unregistered_chain(
    web30: &Web3,
    contact: &Contact,
    src_gravity_client: GravityQueryClient<Channel>, // Src chain's Gravity GRPC client
    dst_bank_qc: BankQueryClient<Channel>,           // Dst chain's Bank GRPC client
    dst_ibc_transfer_qc: IbcTransferQueryClient<Channel>, // Dst chain's ibc-transfer GRPC client
    dest_keys: BridgeUserKey,                        // The bridged + auto-forwarded ERC20 receiver
    gravity_address: EthAddress,                     // Address of the gravity contract
    erc20_address: EthAddress, // Address of the ERC20 to send to dest on ibc-test-1
    amount: Uint256,           // The amount of erc20_address token to send to dest on ibc-test-1
) -> Result<(), GravityError> {
    // The account on ibc-test-one that should NOT receive funds
    let ibc_address = dest_keys
        .cosmos_key
        .to_address(&IBC_ADDRESS_PREFIX.clone())
        .unwrap();
    // The address put in the SendToCosmos proposal, should end up going to gravity_prefixed_dest
    let cosmos_receiver = dest_keys.cosmos_address;
    // The local gravity account which should receive the funds
    let gravity_prefixed_dest = dest_keys
        .cosmos_key
        .to_address(&ADDRESS_PREFIX.clone())
        .unwrap();
    let amt = amount;
    // Ideally we do not see a balance change here
    let ibc_match = move |pre_balance: Option<Coin>, post_balance: Option<Coin>| {
        match (pre_balance, post_balance) {
            (None, None) => {
                info!(
                    "Never found an unregistered ibc balance for user {}, still need to check local balance",
                    ibc_address,
                );
            }
            (None, Some(post)) => {
                if Uint256::from_str(&post.amount).unwrap() == amt {
                    panic!(
                        "Failed SendToCosmos unregistered chain: balance transfer to ibc user {}: actual {} != expected {}",
                        ibc_address,
                        post.amount,
                        amt,
                    );
                } else {
                    warn!(
                        "Discovered potentially unrelated native hijack ibc balance: ibc amount was {} is {} for user {}",
                        "0",
                        post.amount,
                        ibc_address
                    );
                }
            }
            (Some(pre), Some(post)) => {
                let pre_amt = Uint256::from_str(&pre.amount).unwrap();
                let post_amt = Uint256::from_str(&post.amount).unwrap();

                if post_amt >= pre_amt && post_amt - pre_amt == amt {
                    panic!(
                        "Failed SendToCosmos unregistered chain: user {} actual balance {} != expected {}",
                        ibc_address,
                        post.amount,
                        pre_amt
                    );
                } else {
                    warn!(
                        "Discovered potentially unrelated native hijack ibc balance: ibc balance is {} was {} with user {}",
                        post.amount,
                        pre.amount,
                        ibc_address
                    );
                }
            }
            (Some(_), None) => {
                info!(
                    "User wound up with no balance ibc native hijack attempt {}, still need to check local balance",
                    ibc_address
                );
            }
        };
    };

    let amt = amount;
    // Ideally we see the balance increase by amount tokens
    let gravity_match = move |pre_balance: Option<DSCoin>, post_balance: Option<DSCoin>| {
        match (pre_balance, post_balance) {
            (None, None) => {
                panic!(
                        "Failed SendToCosmos unregistered chain: Never found a SendToCosmos local transfer balance for user {}",
                        gravity_prefixed_dest
                    );
            }
            (None, Some(post)) => {
                if post.amount != amt {
                    panic!( // At this point there's no explanation for the lack of funds
                                "Discovered potentially unrelated unregistered chain local balance of amount {} with user {}",
                                amt,
                                gravity_prefixed_dest
                        );
                } else {
                    info!(
                            "Successful SendToCosmos unregistered chain handling: Discovered local user {}: balance {} ",
                            gravity_prefixed_dest,
                            post.amount,
                        );
                }
            }
            (Some(pre), Some(post)) => {
                match post.amount.cmp(&(pre.amount + amt)) {
                    Ordering::Less => {
                        panic!( // At this point there's no explanation for the lack of funds
                            "Failed SendToCosmos unregistered chain: Discovered unexpected local balance with user {}: actual {} != expected {}",
                            gravity_prefixed_dest,
                            post.amount,
                            (pre.amount + amt)
                        );
                    }
                    Ordering::Equal => {
                        info!(
                            "Successful SendToCosmos unregistered chain handling: Discovered local user {}: balance {} ",
                            gravity_prefixed_dest,
                            post.amount,
                        );
                    }
                    Ordering::Greater => {
                        panic!( // At this point there's no explanation for the lack of funds
                            "Failed SendToCosmos unregistered chain: Discovered unexpected local balance with user {}: actual {} != expected {}",
                            gravity_prefixed_dest,
                            post.amount,
                            (pre.amount + amt)
                        );
                    }
                }
            }
            (Some(_), None) => {
                panic!(
                    "Failed SendToCosmos unregistered chain: User wound up with no local balance after hijack attempt? {}",
                    gravity_prefixed_dest,
                );
            }
        };
    };

    test_ibc_auto_forward_failure(
        web30,
        contact,
        Box::new(ibc_match),
        Box::new(gravity_match),
        IbcAutoForwardFailureTest {
            src_gravity_client,
            dst_bank_qc,
            dst_ibc_transfer_qc,
            src_address: gravity_prefixed_dest, // Should receive funds
            dst_address: ibc_address,           // should NOT receive funds
            cosmos_receiver, // invalid cavity address which has no bech32ibc channel mapping
            gravity_address,
            erc20_address,
            amount,
            forward_pending: true,
        },
    )
    .await
}
