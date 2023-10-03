use std::collections::HashMap;
use std::str::FromStr;
use std::time::{Duration, Instant};

use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::{get_deposit, get_fee, STAKING_TOKEN, TOTAL_TIMEOUT};
use crate::{
    happy_path::test_erc20_deposit_panic,
    happy_path_v2::deploy_cosmos_representing_erc20_and_check_adoption, one_eth, utils::*,
    ADDRESS_PREFIX, OPERATION_TIMEOUT,
};
use clarity::Address as EthAddress;
use cosmos_gravity::proposals::{submit_auction_params_proposal, AuctionParamsProposalJson};
use cosmos_gravity::send::MSG_BID_TYPE_URL;
use deep_space::client::type_urls::MSG_FUND_COMMUNITY_POOL_TYPE_URL;
use deep_space::error::CosmosGrpcError;
use deep_space::{Address as CosmosAddress, Coin, Contact, Msg, PrivateKey};
use gravity_proto::auction::query_client::QueryClient as AuctionQueryClient;
use gravity_proto::auction::{
    Auction, MsgBid, QueryAuctionByIdRequest, QueryAuctionPeriodRequest, QueryAuctionsRequest,
    QueryParamsRequest,
};
use gravity_proto::cosmos_sdk_proto::cosmos::distribution::v1beta1::query_client::QueryClient as DistributionQueryClient;
use gravity_proto::cosmos_sdk_proto::cosmos::distribution::v1beta1::{
    MsgFundCommunityPool, QueryCommunityPoolRequest,
};
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::QueryDenomToErc20Request;
use gravity_utils::num_conversion::one_atom;
use lazy_static::lazy_static;
use num256::Uint256;
use rand::Rng;
use tokio::time::sleep;
use tonic::transport::Channel;
use web30::client::Web3;

lazy_static! {
    pub static ref AUCTION_ADDRESS: CosmosAddress =
        deep_space::address::get_module_account_address("auction", Some(&*ADDRESS_PREFIX))
            .expect("Failed to get auction module address");
}

// Ensure that the auction module params cannot be updated to auction off the ugraviton supply
#[allow(clippy::too_many_arguments)]
pub async fn auction_invalid_params_test(contact: &Contact, keys: Vec<ValidatorKeys>) {
    set_non_auctionable_tokens(contact, &keys, vec![]).await;
}

// Populate the community pool with tokens before bidding on auctions
#[allow(clippy::too_many_arguments)]
pub async fn auction_test_static(
    web30: &Web3,
    contact: &Contact,
    grpc_client: GravityQueryClient<Channel>,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_addresses: Vec<EthAddress>,
) {
    let no_relay_market_config = create_no_batch_requests_config();
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;

    let auction_users = setup(
        web30,
        contact,
        &grpc_client,
        keys,
        gravity_address,
        erc20_addresses[0],
    )
    .await;

    let mut auction_qc = AuctionQueryClient::connect(contact.get_url())
        .await
        .expect("Unable to connect to auction query client");
    let period = auction_qc
        .auction_period(QueryAuctionPeriodRequest {})
        .await
        .unwrap()
        .into_inner()
        .auction_period
        .unwrap();
    info!("Current period: {period:?}");
    let auctions = auction_qc
        .auctions(QueryAuctionsRequest {})
        .await
        .unwrap()
        .into_inner()
        .auctions;

    if auctions.is_empty() {
        panic!("Expecting at least some auctions to be open, found none");
    }
    info!("Found auctions {:?}", auctions);

    let min_bid_fee = auction_qc
        .params(QueryParamsRequest {})
        .await
        .unwrap()
        .into_inner()
        .params
        .unwrap()
        .min_bid_fee;
    let mut bids = Vec::new();
    bids.push((
        true,
        auction_users.get(0).unwrap(),
        MsgBid {
            auction_id: auctions.get(0).unwrap().id,
            bidder: auction_users.get(0).unwrap().cosmos_address.to_string(),
            amount: 100_000,
            bid_fee: min_bid_fee,
        },
    )); // Successful bid
    bids.push((
        true,
        auction_users.get(1).unwrap(),
        MsgBid {
            auction_id: auctions.get(0).unwrap().id,
            bidder: auction_users.get(1).unwrap().cosmos_address.to_string(),
            amount: 150_000,
            bid_fee: min_bid_fee + 1,
        },
    )); // Successful bid
    bids.push((
        true,
        auction_users.get(1).unwrap(),
        MsgBid {
            auction_id: auctions.get(1).unwrap().id,
            bidder: auction_users.get(1).unwrap().cosmos_address.to_string(),
            amount: 50_000,
            bid_fee: min_bid_fee,
        },
    )); // Successful bid
    bids.push((
        false,
        auction_users.get(1).unwrap(),
        MsgBid {
            auction_id: auctions.get(0).unwrap().id,
            bidder: auction_users.get(1).unwrap().cosmos_address.to_string(),
            amount: 170_000,
            bid_fee: min_bid_fee,
        },
    )); // Rebid not allowed
    bids.push((
        false,
        auction_users.get(1).unwrap(),
        MsgBid {
            auction_id: auctions.get(1).unwrap().id,
            bidder: auction_users.get(1).unwrap().cosmos_address.to_string(),
            amount: 75_000,
            bid_fee: min_bid_fee,
        },
    )); // Rebid not allowed
    bids.push((
        true,
        auction_users.get(0).unwrap(),
        MsgBid {
            auction_id: auctions.get(1).unwrap().id,
            bidder: auction_users.get(0).unwrap().cosmos_address.to_string(),
            amount: 75_000,
            bid_fee: min_bid_fee,
        },
    )); // Successful bid
    bids.push((
        false,
        auction_users.get(0).unwrap(),
        MsgBid {
            auction_id: auctions.get(0).unwrap().id,
            bidder: auction_users.get(0).unwrap().cosmos_address.to_string(),
            amount: 170_000,
            bid_fee: 5,
        },
    )); // Fee too low
    bids.push((
        true,
        auction_users.get(0).unwrap(),
        MsgBid {
            auction_id: auctions.get(0).unwrap().id,
            bidder: auction_users.get(0).unwrap().cosmos_address.to_string(),
            amount: 170_000,
            bid_fee: min_bid_fee,
        },
    )); // Successful bid

    for bid_params in bids {
        execute_and_validate_bid(contact, bid_params).await;
    }
}

// Similar to auction_test_static but randomly generates bids based on criteria, and executes over multiple auction periods
#[allow(clippy::too_many_arguments)]
pub async fn auction_test_random(
    web30: &Web3,
    contact: &Contact,
    grpc_client: GravityQueryClient<Channel>,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_addresses: Vec<EthAddress>,
) {
    let no_relay_market_config = create_no_batch_requests_config();
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;
    let mut auction_qc = AuctionQueryClient::connect(contact.get_url())
        .await
        .expect("Unable to connect to auction query client");

    // Set up the first round of auctions
    let auction_users = setup(
        web30,
        contact,
        &grpc_client,
        keys.clone(),
        gravity_address,
        erc20_addresses[0],
    )
    .await;
    wait_for_next_period(auction_qc.clone()).await;

    let period = auction_qc
        .auction_period(QueryAuctionPeriodRequest {})
        .await
        .unwrap()
        .into_inner()
        .auction_period
        .unwrap();
    info!("Current period: {period:?}");
    let auctions: Vec<Auction> = auction_qc
        .auctions(QueryAuctionsRequest {})
        .await
        .unwrap()
        .into_inner()
        .auctions;
    if auctions.is_empty() {
        panic!("Expecting at least some auctions to be open, found none");
    }
    info!("Found auctions {:?}", auctions);
    let min_bid_fee = auction_qc
        .params(QueryParamsRequest {})
        .await
        .unwrap()
        .into_inner()
        .params
        .unwrap()
        .min_bid_fee;

    // Create a randomly generated set of bids
    let mut bids = Vec::new();
    // Auctions
    let id0 = auctions.get(0).unwrap().id;
    let id1 = auctions.get(1).unwrap().id;
    let id2 = auctions.get(2).unwrap().id;
    // Highest bids
    let mut h0: u64 = 0;
    let mut h1: u64 = 0;
    let mut h2: u64 = 0;
    // Bid successfully 7 times on this period:
    for i in 0..7 {
        let user = auction_users
            .get(i % auction_users.len())
            .expect("invalid user index");
        let s0 = create_successful_bid(*user, h0, min_bid_fee, id0);
        h0 = s0.amount;
        bids.push((true, user, s0));
        let s1 = create_successful_bid(*user, h1, min_bid_fee, id1);
        h1 = s1.amount;
        bids.push((false, user, create_low_fee_bid(*user, h0, min_bid_fee, id0)));
        bids.push((true, user, s1));
        let s2 = create_successful_bid(*user, h2, min_bid_fee, id2);
        h2 = s2.amount;
        bids.push((true, user, s2));
        bids.push((false, user, create_low_amt_bid(*user, h1, min_bid_fee, id1)));
    }

    for bid_params in bids.clone() {
        execute_and_validate_bid(contact, bid_params).await;
    }

    // Re-seed the pool to set up the next round of auctions
    seed_pool_multi(contact, &keys, erc20_addresses[0]).await;

    wait_for_next_period(auction_qc.clone()).await;
    let period = auction_qc
        .auction_period(QueryAuctionPeriodRequest {})
        .await
        .unwrap()
        .into_inner()
        .auction_period
        .unwrap();
    info!("Current period: {period:?}");
    let auctions: Vec<Auction> = auction_qc
        .auctions(QueryAuctionsRequest {})
        .await
        .unwrap()
        .into_inner()
        .auctions;
    info!("Found auctions {:?}", auctions);

    // Check that the last successful bidder received the auction tokens
    assert_user_won_auctions(contact, *(bids.clone().last().unwrap().1), &auctions[0..3]).await;

    // Create the next round of bids
    let mut bids = Vec::new();
    // Auctions
    let id0 = auctions.get(0).unwrap().id;
    let id1 = auctions.get(1).unwrap().id;
    // Highest bids
    let mut h0: u64 = 0;
    let mut h1: u64 = 0;
    for i in 0..5 {
        let user = auction_users
            .get(i % auction_users.len())
            .expect("invalid user index");
        let s0 = create_successful_bid(*user, h0, min_bid_fee, id0);
        h0 = s0.amount;
        bids.push((true, user, s0));
        let s1 = create_successful_bid(*user, h1, min_bid_fee, id1);
        h1 = s1.amount;
        bids.push((true, user, s1));
        bids.push((false, user, create_low_fee_bid(*user, h0, min_bid_fee, id0)));
        bids.push((false, user, create_low_amt_bid(*user, h1, min_bid_fee, id1)));
    }

    for bid_params in bids.clone() {
        execute_and_validate_bid(contact, bid_params).await;
    }

    // Check that the successful bidder received the auction tokens
    wait_for_next_period(auction_qc.clone()).await;
    assert_user_won_auctions(contact, *(bids.clone().last().unwrap().1), &auctions[0..2]).await;

    info!("Successful auciton random test!");
}

// Creates a footoken representation, sends 100 of the given erc20 to each of the validators, and sets the send to eth fee at a 100% rate
pub async fn setup(
    web30: &Web3,
    contact: &Contact,
    grpc_client: &GravityQueryClient<Channel>,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    erc20_address: EthAddress,
) -> Vec<BridgeUserKey> {
    let mut grpc_client = grpc_client.clone();

    let footoken = footoken_metadata(contact).await;
    if grpc_client
        .denom_to_erc20(QueryDenomToErc20Request {
            denom: footoken.base.clone(),
        })
        .await
        .is_err()
    {
        info!("Begin setup, create footoken erc20");
        let _ = deploy_cosmos_representing_erc20_and_check_adoption(
            gravity_address,
            web30,
            Some(keys.clone()),
            &mut (grpc_client.clone()),
            false,
            footoken.clone(),
        )
        .await;
    }

    info!("Send validators 1000 x 10^18 of an eth native erc20");
    // Send the validators generated address 100 units of each erc20 from ethereum to cosmos
    for v in &keys {
        let receiver = v.validator_key.to_address(&ADDRESS_PREFIX).unwrap();
        test_erc20_deposit_panic(
            web30,
            contact,
            &mut (grpc_client.clone()),
            receiver,
            gravity_address,
            erc20_address,
            one_eth() * 1_000_000u64.into(),
            None,
            None,
        )
        .await;
    }
    let denom = format!("gravity{}", erc20_address);

    set_auction_length(contact, &keys, 45).await;

    let mut dist_qc = DistributionQueryClient::connect(contact.get_url())
        .await
        .expect("Unable to connect to distribution query client");
    let pool = dist_qc
        .community_pool(QueryCommunityPoolRequest {})
        .await
        .expect("Unable to get community pool")
        .into_inner()
        .pool;
    if pool.iter().any(|v| v.denom == denom) {
        info!("The community pool has already been seeded with the bridged ERC20, skipping pool seeding");
    } else {
        seed_pool_multi(contact, &keys, erc20_address).await;
    }

    let mut users = Vec::new();
    for _ in 0..keys.len() {
        users.push(get_user_key(None));
    }

    for (i, v) in keys.iter().enumerate() {
        let dest = users.get(i).unwrap().cosmos_address;
        let amount = one_atom() * 10_000_000u64.into();
        info!("Sending {amount:?} to {dest:?}");
        let res = contact
            .send_coins(
                Coin {
                    amount,
                    denom: STAKING_TOKEN.to_string(),
                },
                Some(get_fee(None)),
                dest,
                Some(OPERATION_TIMEOUT),
                v.validator_key,
            )
            .await;
        res.expect("Failed to send staking coins to auction user");
        let amount = one_eth() * 1000u64.into();
        let res = contact
            .send_coins(
                Coin {
                    amount,
                    denom: denom.clone(),
                },
                Some(get_fee(None)),
                dest,
                Some(OPERATION_TIMEOUT),
                v.validator_key,
            )
            .await;
        res.expect("Failed to send ERC20 coins to auction user");
    }
    users
}

// Seeds the community pool with the bridged `erc20_address`, footoken, and footoken2
async fn seed_pool_multi(contact: &Contact, keys: &[ValidatorKeys], erc20_address: EthAddress) {
    let denom = format!("gravity{}", erc20_address);
    seed_pool(contact, keys, denom).await;
    let footoken = footoken_metadata(contact).await;
    seed_pool(contact, keys, footoken.base).await;
    let footoken2 = get_metadata(contact, "footoken2").await;
    seed_pool(contact, keys, footoken2.base).await;
}

// Populates the community pool by submitting MsgFundCommunityPool
async fn seed_pool(contact: &Contact, keys: &[ValidatorKeys], denom: String) {
    let bridge: Uint256 = 10_000000u64.into();
    for _ in 0..3 {
        for v in keys {
            let seed_coin = Coin {
                denom: denom.clone(),
                amount: bridge,
            };
            let seed_msg = MsgFundCommunityPool {
                amount: vec![seed_coin.into()],
                depositor: v
                    .validator_key
                    .to_address(&contact.get_prefix())
                    .unwrap()
                    .to_string(),
            };
            let msg = Msg::new(MSG_FUND_COMMUNITY_POOL_TYPE_URL, seed_msg);
            contact
                .send_message(
                    &[msg],
                    None,
                    &[get_fee(Some(STAKING_TOKEN.to_string()))],
                    Some(Duration::from_secs(30)),
                    v.validator_key,
                )
                .await
                .expect("Failed to fund community pool");
        }
    }
}

/// Sets the auction length parameter to the given value
async fn set_auction_length(contact: &Contact, keys: &[ValidatorKeys], length: u64) {
    let params = AuctionParamsProposalJson {
        title: "Set auction params".to_string(),
        description: "Auction Params!".to_string(),
        auction_length: Some(length),
        ..Default::default()
    };

    submit_and_pass_auction_params_proposal(contact, keys, params).await;
}

async fn set_non_auctionable_tokens(
    contact: &Contact,
    keys: &[ValidatorKeys],
    non_auctionable_tokens: Vec<String>,
) {
    let params = AuctionParamsProposalJson {
        title: "Set non_auctionable_tokens".to_string(),
        description: "Set non_auctionable_tokens".to_string(),
        non_auctionable_tokens: Some(non_auctionable_tokens),
        ..Default::default()
    };

    let res = submit_auction_params_proposal(
        params,
        get_deposit(None),
        get_fee(None),
        contact,
        keys[0].validator_key,
        Some(TOTAL_TIMEOUT),
    )
    .await;
    assert!(res.is_err());
    info!("Successfully tested auction params validation");
}

// Submits, votes yes, and waits for a proposal to update the Auction params
pub async fn submit_and_pass_auction_params_proposal(
    contact: &Contact,
    keys: &[ValidatorKeys],
    params: AuctionParamsProposalJson,
) {
    let mut auction_qc = AuctionQueryClient::connect(contact.get_url())
        .await
        .expect("Unable to connect to auction query client");
    let res = submit_auction_params_proposal(
        params.clone(),
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

    let post_params = auction_qc
        .params(QueryParamsRequest {})
        .await
        .expect("Unable to get params")
        .into_inner()
        .params
        .expect("auction module returned no params");
    assert_changed_auction_params(params, post_params);
}

pub fn assert_changed_auction_params(proposal: AuctionParamsProposalJson, post_params: Params) {
    if let Some(auction_length) = proposal.auction_length {
        assert_eq!(auction_length, post_params.auction_length);
    }
    if let Some(min_bid_fee) = proposal.min_bid_fee {
        assert_eq!(min_bid_fee, post_params.min_bid_fee);
    }
    if let Some(non_auctionable_tokens) = proposal.non_auctionable_tokens {
        assert_eq!(non_auctionable_tokens, post_params.non_auctionable_tokens);
    }

    if let Some(burn_winning_bids) = proposal.burn_winning_bids {
        assert_eq!(burn_winning_bids, post_params.burn_winning_bids);
    }

    if let Some(enabled) = proposal.enabled {
        assert_eq!(enabled, post_params.enabled);
    }
}
// Generates a random bid which should be successful
pub fn create_successful_bid(
    user: BridgeUserKey,
    highest_bid: u64,
    min_bid_fee: u64,
    auction_id: u64,
) -> MsgBid {
    let amount = rand::thread_rng().gen_range(highest_bid..highest_bid + 9999_000000);
    let bid_fee = rand::thread_rng().gen_range(min_bid_fee..min_bid_fee + 9999_000000);
    MsgBid {
        auction_id,
        bidder: user.cosmos_address.to_string(),
        amount,
        bid_fee,
    }
}

// Generates a random bid which should fail because of a low fee amount
pub fn create_low_fee_bid(
    user: BridgeUserKey,
    highest_bid: u64,
    min_bid_fee: u64,
    auction_id: u64,
) -> MsgBid {
    let amount = rand::thread_rng().gen_range(highest_bid..highest_bid + 9999_000000);
    let bid_fee = rand::thread_rng().gen_range(0..min_bid_fee);
    MsgBid {
        auction_id,
        bidder: user.cosmos_address.to_string(),
        amount,
        bid_fee,
    }
}

// Generates a random bid which should fail because of a low bid amount
pub fn create_low_amt_bid(
    user: BridgeUserKey,
    highest_bid: u64,
    min_bid_fee: u64,
    auction_id: u64,
) -> MsgBid {
    let amount = rand::thread_rng().gen_range(0..highest_bid);
    let bid_fee = rand::thread_rng().gen_range(min_bid_fee..min_bid_fee + 9999_000000);
    MsgBid {
        auction_id,
        bidder: user.cosmos_address.to_string(),
        amount,
        bid_fee,
    }
}

// Executes the given bid, validating balance changes as a result of the transaction
pub async fn execute_and_validate_bid(
    contact: &Contact,
    bid_params: (bool, &BridgeUserKey, MsgBid),
) {
    let (exp_success, user, bid) = bid_params;
    let pre_user_balances = contact
        .get_balances(user.cosmos_address)
        .await
        .expect("unable to get user balances before bid");
    let pre_module_balances = contact
        .get_balances(*AUCTION_ADDRESS)
        .await
        .expect("unable to get module balances before bid");
    info!("Executing msg {bid:?}");

    let bid_amount = bid.amount;
    let bid_fee = bid.bid_fee;
    let bid_id = bid.auction_id;
    // previous_bid is 0 or whatever the previous bid amount for this auction was
    let previous_bid: Uint256 =
        get_auction(contact.get_url(), bid_id)
            .await
            .map_or(0u8.into(), |auction| {
                auction
                    .highest_bid
                    .map_or(0u8.into(), |prev_bid| prev_bid.bid_amount.into())
            });
    let tx_fee = get_fee(Some(STAKING_TOKEN.to_string()));
    let msg = Msg::new(MSG_BID_TYPE_URL, bid);
    let res = contact
        .send_message(
            &[msg],
            None,
            &[tx_fee.clone()],
            Some(OPERATION_TIMEOUT),
            user.cosmos_key,
        )
        .await;
    let post_user_balances = contact
        .get_balances(user.cosmos_address)
        .await
        .expect("unable to get user balances after bid");
    let post_module_balances = contact
        .get_balances(*AUCTION_ADDRESS)
        .await
        .expect("unable to get module balances after bid");

    if exp_success {
        let res = res.expect("expected success");
        let expected_user_change: Uint256 = tx_fee.amount + bid_amount.into() + bid_fee.into();
        let expected_module_change: Uint256 = Uint256::from(bid_amount) - previous_bid;

        info!(
            "Expecting user balance change: {} ({:?} -> {:?})",
            expected_user_change, pre_user_balances, post_user_balances
        );
        info!("Log: {}", res.raw_log);
        validate_balance_changes(
            pre_user_balances,
            post_user_balances,
            Coin {
                amount: expected_user_change,
                denom: STAKING_TOKEN.to_string(),
            },
            true,
        );
        info!(
            "Expecting module balance change: {} ({:?} -> {:?})",
            expected_module_change, pre_module_balances, post_module_balances
        );
        validate_balance_changes(
            pre_module_balances,
            post_module_balances,
            Coin {
                amount: expected_module_change,
                denom: STAKING_TOKEN.to_string(),
            },
            false,
        );
    } else {
        res.expect_err("expected failure");
        assert_eq!(pre_user_balances, post_user_balances);
        assert_eq!(pre_module_balances, post_module_balances);
    }
}

fn validate_balance_changes(
    pre_balances: Vec<Coin>,
    post_balances: Vec<Coin>,
    expected_change: Coin,
    expecting_decrease: bool,
) {
    let zero = Coin {
        denom: expected_change.denom.clone(),
        amount: 0u8.into(),
    };
    let pre_map = coins_to_map(pre_balances);
    let post_map = coins_to_map(post_balances);

    let pre_change = pre_map.get(&expected_change.denom).unwrap_or(&zero);
    let post_change = post_map.get(&expected_change.denom).unwrap_or(&zero);

    let actual: Uint256 = if expecting_decrease {
        pre_change.amount - post_change.amount
    } else {
        post_change.amount - pre_change.amount
    };
    let actual_coin = Coin {
        denom: expected_change.denom.clone(),
        amount: actual,
    };
    assert_eq!(expected_change, actual_coin);

    for (denom, pre_coin) in pre_map {
        if denom == expected_change.denom {
            // already validated
            continue;
        }

        let post_coin = post_map.get(&denom).unwrap_or_else(|| {
            panic!(
                "No balance of {} after the transaction",
                expected_change.denom
            )
        });
        assert_eq!(pre_coin, *post_coin);
    }
}

fn coins_to_map(coins: Vec<Coin>) -> HashMap<String, Coin> {
    let mut map: HashMap<String, Coin> = HashMap::new();
    for coin in coins {
        map.insert(coin.denom.clone(), coin);
    }
    map
}

// Waits until a new AuctionPeriod begins
// Panics if over `TOTAL_TIMEOUT` seconds elapse
pub async fn wait_for_next_period(auction_qc: AuctionQueryClient<Channel>) {
    let mut auction_qc = auction_qc;
    let period = auction_qc
        .auction_period(QueryAuctionPeriodRequest {})
        .await
        .expect("Unable to get auction period")
        .into_inner()
        .auction_period
        .expect("no period in response");
    let loop_pause = Duration::from_secs(10);
    let start = Instant::now();
    loop {
        if Instant::now() - start > TOTAL_TIMEOUT {
            panic!("Timed out waiting for next auction period");
        }
        let new_period = auction_qc
            .auction_period(QueryAuctionPeriodRequest {})
            .await
            .expect("Unable to get auction period")
            .into_inner()
            .auction_period
            .expect("no period in response");
        if new_period != period {
            return;
        }
        sleep(loop_pause).await;
    }
}

// Checks that the given `user` currently holds all the awards given by `auctions`
async fn assert_user_won_auctions(contact: &Contact, user: BridgeUserKey, auctions: &[Auction]) {
    for auction in auctions {
        let amount = auction.amount.clone().unwrap();
        let bal = contact
            .get_balance(user.cosmos_address, amount.denom)
            .await
            .unwrap()
            .unwrap();
        if bal.amount.lt(&Uint256::from_str(&amount.amount).unwrap()) {
            panic!(
                "User {} did not win auction {:?}",
                user.cosmos_address, auction
            );
        }
    }
}

async fn get_auction(grpc_url: String, auction_id: u64) -> Result<Auction, CosmosGrpcError> {
    let mut auction_qc = AuctionQueryClient::connect(grpc_url)
        .await
        .expect("Unable to connect to auction query client");
    let auction = auction_qc
        .auction_by_id(QueryAuctionByIdRequest { auction_id })
        .await?
        .into_inner()
        .auction;

    auction.map_or(
        Err(CosmosGrpcError::BadResponse(
            "No such auction returned".to_string(),
        )),
        |a| Ok(a),
    )
}
