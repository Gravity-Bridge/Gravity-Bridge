use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::get_deposit;
use crate::get_fee;
use crate::ADDRESS_PREFIX;
use crate::COSMOS_NODE_GRPC;
use crate::ETH_NODE;
use crate::STAKING_TOKEN;
use crate::TOTAL_TIMEOUT;
use crate::{one_eth, MINER_PRIVATE_KEY};
use crate::{MINER_ADDRESS, OPERATION_TIMEOUT};
use actix::System;
use clarity::{Address as EthAddress, Uint256};
use clarity::{PrivateKey as EthPrivateKey, Transaction};
use cosmos_gravity::proposals::{submit_parameter_change_proposal, submit_upgrade_proposal};
use cosmos_gravity::query::get_gravity_params;
use deep_space::address::Address as CosmosAddress;
use deep_space::client::ChainStatus;
use deep_space::coin::Coin;
use deep_space::error::CosmosGrpcError;
use deep_space::private_key::PrivateKey as CosmosPrivateKey;
use deep_space::{Contact, Fee, Msg};
use ethereum_gravity::utils::get_event_nonce;
use futures::future::join_all;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::Metadata;
use gravity_proto::cosmos_sdk_proto::cosmos::gov::v1beta1::VoteOption;
use gravity_proto::cosmos_sdk_proto::cosmos::params::v1beta1::{
    ParamChange, ParameterChangeProposal,
};
use gravity_proto::cosmos_sdk_proto::cosmos::staking::v1beta1::QueryValidatorsRequest;
use gravity_proto::cosmos_sdk_proto::cosmos::upgrade::v1beta1::{Plan, SoftwareUpgradeProposal};
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::MsgSendToCosmosClaim;
use gravity_utils::types::BatchRelayingMode;
use gravity_utils::types::BatchRequestMode;
use gravity_utils::types::GravityBridgeToolsConfig;
use gravity_utils::types::ValsetRelayingMode;
use orchestrator::main_loop::orchestrator_main_loop;
use rand::Rng;
use std::thread;
use std::time::{Duration, Instant};
use tokio::time::sleep;
use web30::jsonrpc::error::Web3Error;
use web30::{client::Web3, types::SendTxOption};

/// returns the required denom metadata for deployed the Footoken
/// token defined in our test environment
pub async fn footoken_metadata(contact: &Contact) -> Metadata {
    let metadata = contact.get_all_denoms_metadata().await.unwrap();
    for m in metadata {
        if m.base == "footoken" {
            return m;
        }
    }
    panic!("Footoken metadata not set?");
}

pub fn get_decimals(meta: &Metadata) -> u32 {
    for m in meta.denom_units.iter() {
        if m.denom == meta.display {
            return m.exponent;
        }
    }
    panic!("Invalid metadata!")
}

pub fn create_default_test_config() -> GravityBridgeToolsConfig {
    let mut no_relay_market_config = GravityBridgeToolsConfig::default();
    // enable integrated relayer by default for tests
    no_relay_market_config.orchestrator.relayer_enabled = true;
    no_relay_market_config.relayer.batch_relaying_mode = BatchRelayingMode::EveryBatch;
    no_relay_market_config.relayer.logic_call_market_enabled = false;
    no_relay_market_config.relayer.valset_relaying_mode = ValsetRelayingMode::EveryValset;
    no_relay_market_config.relayer.batch_request_mode = BatchRequestMode::EveryBatch;
    no_relay_market_config.relayer.relayer_loop_speed = 10;
    no_relay_market_config
}

pub fn create_no_batch_requests_config() -> GravityBridgeToolsConfig {
    let mut no_relay_market_config = create_default_test_config();
    no_relay_market_config.relayer.batch_request_mode = BatchRequestMode::None;
    no_relay_market_config
}

pub async fn send_eth_to_orchestrators(keys: &[ValidatorKeys], web30: &Web3) {
    let balance = web30.eth_get_balance(*MINER_ADDRESS).await.unwrap();
    info!(
        "Sending orchestrators 100 eth to pay for fees miner has {} ETH",
        balance / one_eth()
    );
    let mut eth_keys = Vec::new();
    for key in keys {
        eth_keys.push(key.eth_key.to_address());
    }
    send_eth_bulk(one_eth() * 100u16.into(), &eth_keys, web30).await;
}

pub async fn send_one_eth(dest: EthAddress, web30: &Web3) {
    send_eth_bulk(one_eth(), &[dest], web30).await;
}

pub fn get_coins(denom: &str, balances: &[Coin]) -> Option<Coin> {
    for coin in balances {
        if coin.denom.starts_with(denom) {
            return Some(coin.clone());
        }
    }
    None
}

/// This is a hardcoded very high gas value used in transaction stress test to counteract rollercoaster
/// gas prices due to the way that test fills blocks
pub const HIGH_GAS_PRICE: u64 = 1_000_000_000u64;

/// This function efficiently distributes ERC20 tokens to a large number of provided Ethereum addresses
/// the real problem here is that you can't do more than one send operation at a time from a
/// single address without your sequence getting out of whack. By manually setting the nonce
/// here we can send thousands of transactions in only a few blocks
pub async fn send_erc20_bulk(
    amount: Uint256,
    erc20: EthAddress,
    destinations: &[EthAddress],
    web3: &Web3,
) {
    check_erc20_balance(erc20, amount.clone(), *MINER_ADDRESS, web3).await;
    let mut nonce = web3
        .eth_get_transaction_count(*MINER_ADDRESS)
        .await
        .unwrap();
    let mut transactions = Vec::new();
    for address in destinations {
        let send = web3.erc20_send(
            amount.clone(),
            *address,
            erc20,
            *MINER_PRIVATE_KEY,
            Some(OPERATION_TIMEOUT),
            vec![
                SendTxOption::Nonce(nonce.clone()),
                SendTxOption::GasLimit(100_000u32.into()),
                SendTxOption::GasPriceMultiplier(5.0),
            ],
        );
        transactions.push(send);
        nonce += 1u64.into();
    }
    let txids = join_all(transactions).await;
    wait_for_txids(txids, web3).await;
    let mut balance_checks = Vec::new();
    for address in destinations {
        let check = check_erc20_balance(erc20, amount.clone(), *address, web3);
        balance_checks.push(check);
    }
    join_all(balance_checks).await;
}

/// This function efficiently distributes ETH to a large number of provided Ethereum addresses
/// the real problem here is that you can't do more than one send operation at a time from a
/// single address without your sequence getting out of whack. By manually setting the nonce
/// here we can quickly send thousands of transactions in only a few blocks
pub async fn send_eth_bulk(amount: Uint256, destinations: &[EthAddress], web3: &Web3) {
    let net_version = web3.net_version().await.unwrap();
    let mut nonce = web3
        .eth_get_transaction_count(*MINER_ADDRESS)
        .await
        .unwrap();
    let mut transactions = Vec::new();
    for address in destinations {
        let t = Transaction {
            to: *address,
            nonce: nonce.clone(),
            gas_price: HIGH_GAS_PRICE.into(),
            gas_limit: 24000u64.into(),
            value: amount.clone(),
            data: Vec::new(),
            signature: None,
        };
        let t = t.sign(&*MINER_PRIVATE_KEY, Some(net_version));
        transactions.push(t);
        nonce += 1u64.into();
    }
    let mut sends = Vec::new();
    for tx in transactions {
        sends.push(web3.eth_send_raw_transaction(tx.to_bytes().unwrap()));
    }
    let txids = join_all(sends).await;
    wait_for_txids(txids, web3).await;
}

/// utility function that waits for a large number of txids to enter a block
async fn wait_for_txids(txids: Vec<Result<Uint256, Web3Error>>, web3: &Web3) {
    let mut wait_for_txid = Vec::new();
    for txid in txids {
        let wait = web3.wait_for_transaction(txid.unwrap(), TOTAL_TIMEOUT, None);
        wait_for_txid.push(wait);
    }
    join_all(wait_for_txid).await;
}

/// utility function for bulk checking erc20 balances, used to provide
/// a single future that contains the assert as well s the request
pub async fn check_erc20_balance(
    erc20: EthAddress,
    amount: Uint256,
    address: EthAddress,
    web3: &Web3,
) {
    let new_balance = get_erc20_balance_safe(erc20, web3, address).await;
    let new_balance = new_balance.unwrap();
    assert!(new_balance >= amount.clone());
}

/// utility function for bulk checking erc20 balances, used to provide
/// a single future that contains the assert as well s the request
pub async fn get_erc20_balance_safe(
    erc20: EthAddress,
    web3: &Web3,
    address: EthAddress,
) -> Result<Uint256, Web3Error> {
    let start = Instant::now();
    // overly complicated retry logic allows us to handle the possibility that gas prices change between blocks
    // and cause any individual request to fail.
    let mut new_balance = Err(Web3Error::BadInput("Intentional Error".to_string()));
    while new_balance.is_err() && Instant::now() - start < TOTAL_TIMEOUT {
        new_balance = web3.get_erc20_balance(erc20, address).await;
        // only keep trying if our error is gas related
        if let Err(ref e) = new_balance {
            if !e.to_string().contains("maxFeePerGas") {
                break;
            }
        }
    }
    Ok(new_balance.unwrap())
}

pub fn get_user_key() -> BridgeUserKey {
    let mut rng = rand::thread_rng();
    let secret: [u8; 32] = rng.gen();
    // the starting location of the funds
    let eth_key = EthPrivateKey::from_slice(&secret).unwrap();
    let eth_address = eth_key.to_address();
    // the destination on cosmos that sends along to the final ethereum destination
    let cosmos_key = CosmosPrivateKey::from_secret(&secret);
    let cosmos_address = cosmos_key.to_address(ADDRESS_PREFIX.as_str()).unwrap();
    let mut rng = rand::thread_rng();
    let secret: [u8; 32] = rng.gen();
    // the final destination of the tokens back on Ethereum
    let eth_dest_key = EthPrivateKey::from_slice(&secret).unwrap();
    let eth_dest_address = eth_key.to_address();
    BridgeUserKey {
        eth_address,
        eth_key,
        cosmos_address,
        cosmos_key,
        eth_dest_address,
        eth_dest_key,
    }
}
#[derive(Debug, Eq, PartialEq, Clone, Copy)]
pub struct BridgeUserKey {
    // the starting addresses that get Eth balances to send across the bridge
    pub eth_address: EthAddress,
    pub eth_key: EthPrivateKey,
    // the cosmos addresses that get the funds and send them on to the dest eth addresses
    pub cosmos_address: CosmosAddress,
    pub cosmos_key: CosmosPrivateKey,
    // the location tokens are sent back to on Ethereum
    pub eth_dest_address: EthAddress,
    pub eth_dest_key: EthPrivateKey,
}

#[derive(Debug, Clone)]
pub struct ValidatorKeys {
    /// The Ethereum key used by this validator to sign Gravity bridge messages
    pub eth_key: EthPrivateKey,
    /// The Orchestrator key used by this validator to submit oracle messages and signatures
    /// to the cosmos chain
    pub orch_key: CosmosPrivateKey,
    /// The validator key used by this validator to actually sign and produce blocks
    pub validator_key: CosmosPrivateKey,
}

/// This function pays the piper for the strange concurrency model that we use for the tests
/// we launch a thread, create an actix executor and then start the orchestrator within that scope
/// previously we could just throw around futures to spawn despite not having 'send' newer versions
/// of Actix rightly forbid this and we have to take the time to handle it here.
pub async fn start_orchestrators(
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    validator_out: bool,
    orchestrator_config: GravityBridgeToolsConfig,
) {
    // used to break out of the loop early to simulate one validator
    // not running an Orchestrator
    let num_validators = keys.len();
    let mut count = 0;

    #[allow(clippy::explicit_counter_loop)]
    for k in keys {
        let config = orchestrator_config.clone();
        info!(
            "Spawning Orchestrator with delegate keys {} {} and validator key {}",
            k.eth_key.to_address(),
            k.orch_key.to_address(ADDRESS_PREFIX.as_str()).unwrap(),
            get_operator_address(k.validator_key),
        );
        let mut grpc_client = GravityQueryClient::connect(COSMOS_NODE_GRPC.as_str())
            .await
            .unwrap();
        let params = get_gravity_params(&mut grpc_client)
            .await
            .expect("Failed to get Gravity Bridge module parameters!");

        // we have only one actual futures executor thread (see the actix runtime tag on our main function)
        // but that will execute all the orchestrators in our test in parallel
        thread::spawn(move || {
            let web30 = web30::client::Web3::new(ETH_NODE.as_str(), OPERATION_TIMEOUT);
            let contact = Contact::new(
                COSMOS_NODE_GRPC.as_str(),
                OPERATION_TIMEOUT,
                ADDRESS_PREFIX.as_str(),
            )
            .unwrap();
            let fut = orchestrator_main_loop(
                k.orch_key,
                k.eth_key,
                web30,
                contact,
                grpc_client,
                gravity_address,
                params.gravity_id,
                get_fee(None),
                config,
            );
            let system = System::new();
            system.block_on(fut);
        });
        // used to break out of the loop early to simulate one validator
        // not running an orchestrator
        count += 1;
        if validator_out && count == num_validators - 1 {
            break;
        }
    }
}

// Submits a false send to cosmos for every orchestrator key in keys, sending amount of erc20_address
// tokens to cosmos_receiver, claiming to come from ethereum_sender for the given fee.
// If a timeout is supplied, contact.send_message() will block waiting for the tx to appear
// Note: These sends to cosmos are false, meaning the ethereum side will have a lower nonce than the
// cosmos side and the bridge will effectively break.
#[allow(clippy::too_many_arguments)]
pub async fn submit_false_claims(
    keys: &[CosmosPrivateKey],
    nonce: u64,
    height: u64,
    amount: Uint256,
    cosmos_receiver: CosmosAddress,
    ethereum_sender: EthAddress,
    erc20_address: EthAddress,
    contact: &Contact,
    fee: &Fee,
    timeout: Option<Duration>,
) {
    for (i, k) in keys.iter().enumerate() {
        let orch_addr = k.to_address(&contact.get_prefix()).unwrap();
        let claim = MsgSendToCosmosClaim {
            event_nonce: nonce,
            block_height: height,
            token_contract: erc20_address.to_string(),
            amount: amount.to_string(),
            cosmos_receiver: cosmos_receiver.to_string(),
            ethereum_sender: ethereum_sender.to_string(),
            orchestrator: orch_addr.to_string(),
        };
        info!("Oracle number {} submitting false deposit {:?}", i, claim);
        let msg_url = "/gravity.v1.MsgSendToCosmosClaim";
        let msg = Msg::new(msg_url, claim.clone());
        let res = contact
            .send_message(
                &[msg],
                Some("All your bridge are belong to us".to_string()),
                fee.amount.as_slice(),
                timeout,
                *k,
            )
            .await
            .expect("Failed to submit false claim");
        info!("Oracle {} false claim response {:?}", i, res);
    }
}

/// Creates a proposal to change the params of our test chain
pub async fn create_parameter_change_proposal(
    contact: &Contact,
    key: CosmosPrivateKey,
    params_to_change: Vec<ParamChange>,
) {
    let proposal = ParameterChangeProposal {
        title: "Set gravity settings!".to_string(),
        description: "test proposal".to_string(),
        changes: params_to_change,
    };
    let res = submit_parameter_change_proposal(
        proposal,
        get_deposit(),
        get_fee(None),
        contact,
        key,
        Some(TOTAL_TIMEOUT),
    )
    .await
    .unwrap();
    trace!("Gov proposal executed with {:?}", res);
}

/// Gets the operator address for a given validator private key
pub fn get_operator_address(key: CosmosPrivateKey) -> CosmosAddress {
    // this is not guaranteed to be correct, the chain may set the valoper prefix in a
    // different way, but I haven't yet seen one that does not match this pattern
    key.to_address(&format!("{}valoper", *ADDRESS_PREFIX))
        .unwrap()
}

// Prints out current stake to the console
pub async fn print_validator_stake(contact: &Contact) {
    let validators = contact
        .get_validators_list(QueryValidatorsRequest::default())
        .await
        .unwrap();
    for validator in validators {
        info!(
            "Validator {} has {} tokens",
            validator.operator_address, validator.tokens
        );
    }
}

// Simple arguments to create a proposal with
pub struct UpgradeProposalParams {
    pub upgrade_height: i64,
    pub plan_name: String,
    pub plan_info: String,
    pub proposal_title: String,
    pub proposal_desc: String,
}

// Creates and submits a SoftwareUpgradeProposal to the chain, then votes yes with all validators
pub async fn execute_upgrade_proposal(
    contact: &Contact,
    keys: &[ValidatorKeys],
    timeout: Option<Duration>,
    upgrade_params: UpgradeProposalParams,
) {
    let duration = match timeout {
        Some(dur) => dur,
        None => OPERATION_TIMEOUT,
    };

    let plan = Plan {
        name: upgrade_params.plan_name,
        time: None,
        height: upgrade_params.upgrade_height,
        info: upgrade_params.plan_info,
    };
    let proposal = SoftwareUpgradeProposal {
        title: upgrade_params.proposal_title,
        description: upgrade_params.proposal_desc,
        plan: Some(plan),
    };
    let res = submit_upgrade_proposal(
        proposal,
        get_deposit(),
        get_fee(None),
        contact,
        keys[0].validator_key,
        Some(duration),
    )
    .await
    .unwrap();
    info!("Gov proposal executed with {:?}", res);

    vote_yes_on_proposals(contact, keys, None).await;
    wait_for_proposals_to_execute(contact).await;
}

// votes yes on every proposal available
pub async fn vote_yes_on_proposals(
    contact: &Contact,
    keys: &[ValidatorKeys],
    timeout: Option<Duration>,
) {
    let duration = match timeout {
        Some(dur) => dur,
        None => OPERATION_TIMEOUT,
    };
    // Vote yes on all proposals with all validators
    let proposals = contact
        .get_governance_proposals_in_voting_period()
        .await
        .unwrap();
    trace!("Found proposals: {:?}", proposals.proposals);
    let mut futs = Vec::new();
    for proposal in proposals.proposals {
        for key in keys.iter() {
            let res =
                vote_yes_with_retry(contact, proposal.proposal_id, key.validator_key, duration);
            futs.push(res);
        }
    }
    // vote on the proposal in parallel, reducing the number of blocks we wait for all
    // the tx's to get in.
    join_all(futs).await;
}

/// this utility function repeatedly attempts to vote yes on a governance
/// proposal up to MAX_VOTES times before failing
pub async fn vote_yes_with_retry(
    contact: &Contact,
    proposal_id: u64,
    key: CosmosPrivateKey,
    timeout: Duration,
) {
    const MAX_VOTES: u64 = 5;
    let mut counter = 0;
    let mut res = contact
        .vote_on_gov_proposal(
            proposal_id,
            VoteOption::Yes,
            get_fee(None),
            key,
            Some(timeout),
        )
        .await;
    while let Err(e) = res {
        contact.wait_for_next_block(TOTAL_TIMEOUT).await.unwrap();
        res = contact
            .vote_on_gov_proposal(
                proposal_id,
                VoteOption::Yes,
                get_fee(None),
                key,
                Some(timeout),
            )
            .await;
        counter += 1;
        if counter > MAX_VOTES {
            error!(
                "Vote for proposal has failed more than {} times, error {:?}",
                MAX_VOTES, e
            );
            panic!("failed to vote{}", e);
        }
    }
    let res = res.unwrap();
    info!(
        "Voting yes on governance proposal costing {} gas",
        res.gas_used
    );
}

// Checks that cosmos_account has each balance specified in expected_cosmos_coins.
// Note: ignores balances not in expected_cosmos_coins
pub async fn check_cosmos_balances(
    contact: &Contact,
    cosmos_account: CosmosAddress,
    expected_cosmos_coins: &[Coin],
) {
    let mut num_found = 0;

    let start = Instant::now();

    while Instant::now() - start < TOTAL_TIMEOUT {
        let mut good = true;
        let curr_balances = contact.get_balances(cosmos_account).await.unwrap();
        // These loops use loop labels, see the documentation on loop labels here for more information
        // https://doc.rust-lang.org/reference/expressions/loop-expr.html#loop-labels
        'outer: for bal in curr_balances.iter() {
            if num_found == expected_cosmos_coins.len() {
                break 'outer; // done searching entirely
            }
            'inner: for j in 0..expected_cosmos_coins.len() {
                if num_found == expected_cosmos_coins.len() {
                    break 'outer; // done searching entirely
                }
                if expected_cosmos_coins[j].denom != bal.denom {
                    continue;
                }
                let check = expected_cosmos_coins[j].amount == bal.amount;
                good = check;
                if !check {
                    warn!(
                        "found balance {}! expected {} trying again",
                        bal, expected_cosmos_coins[j].amount
                    );
                }
                num_found += 1;
                break 'inner; // done searching for this particular balance
            }
        }

        let check = num_found == curr_balances.len();
        // if it's already false don't set to true
        good = check || good;
        if !check {
            warn!(
                "did not find the correct balance for each expected coin! found {} of {}, trying again",
                num_found,
                curr_balances.len()
            );
        }
        if good {
            return;
        } else {
            sleep(Duration::from_secs(1)).await;
        }
    }
    panic!("Failed to find correct balances in check_cosmos_balances")
}

/// utility function for bulk checking erc20 balances, used to provide
/// a single future that contains the assert as well s the request
pub async fn get_event_nonce_safe(
    gravity_contract_address: EthAddress,
    web3: &Web3,
    caller_address: EthAddress,
) -> Result<u64, Web3Error> {
    let start = Instant::now();
    // overly complicated retry logic allows us to handle the possibility that gas prices change between blocks
    // and cause any individual request to fail.
    let mut new_balance = Err(Web3Error::BadInput("Intentional Error".to_string()));
    while new_balance.is_err() && Instant::now() - start < TOTAL_TIMEOUT {
        new_balance = get_event_nonce(gravity_contract_address, caller_address, web3).await;
        // only keep trying if our error is gas related
        if let Err(ref e) = new_balance {
            if !e.to_string().contains("maxFeePerGas") {
                break;
            }
        }
    }
    Ok(new_balance.unwrap())
}

/// waits for the cosmos chain to start producing blocks, used to prevent race conditions
/// where our tests try to start running before the Cosmos chain is ready
pub async fn wait_for_cosmos_online(contact: &Contact, timeout: Duration) {
    let start = Instant::now();
    while let Err(CosmosGrpcError::NodeNotSynced) | Err(CosmosGrpcError::ChainNotRunning) =
        contact.wait_for_next_block(timeout).await
    {
        sleep(Duration::from_secs(1)).await;
        if Instant::now() - start > timeout {
            panic!("Cosmos node has not come online during timeout!")
        }
    }
    contact.wait_for_next_block(timeout).await.unwrap();
}

/// This function returns the valoper address of a validator
/// to whom delegating the returned amount of staking token will
/// create a 5% or greater change in voting power, triggering the
/// creation of a validator set update.
pub async fn get_validator_to_delegate_to(contact: &Contact) -> (CosmosAddress, Coin) {
    let validators = contact.get_active_validators().await.unwrap();
    let mut total_bonded_stake: Uint256 = 0u8.into();
    let mut has_the_least = None;
    let mut lowest = 0u8.into();
    for v in validators {
        let amount: Uint256 = v.tokens.parse().unwrap();
        total_bonded_stake += amount.clone();

        if lowest == 0u8.into() || amount < lowest {
            lowest = amount;
            has_the_least = Some(v.operator_address.parse().unwrap());
        }
    }

    // since this is five percent of the total bonded stake
    // delegating this to the validator who has the least should
    // do the trick
    let five_percent = total_bonded_stake / 20u8.into();
    let five_percent = Coin {
        denom: STAKING_TOKEN.clone(),
        amount: five_percent,
    };

    (has_the_least.unwrap(), five_percent)
}

/// Waits for a particular block to be created
/// Returns an error if the chain fails to progress in a timely manner or the chain is not running
/// Panics if the block has already been surpassed
pub async fn wait_for_block(contact: &Contact, height: u64) -> Result<(), CosmosGrpcError> {
    let status = contact.get_chain_status().await?;
    let mut curr_height = match status {
        // Check the current height
        ChainStatus::Syncing => return Err(CosmosGrpcError::NodeNotSynced),
        ChainStatus::WaitingToStart => return Err(CosmosGrpcError::ChainNotRunning),
        ChainStatus::Moving { block_height } => {
            if block_height > height {
                panic!(
                    "Block height {} surpassed, current height is {}",
                    height, block_height
                );
            }
            block_height
        }
    };
    while curr_height < height {
        // Wait for the desired height
        contact.wait_for_next_block(OPERATION_TIMEOUT).await?; // Err if any block takes 30s+
        let new_status = contact.get_chain_status().await?;
        if let ChainStatus::Moving { block_height } = new_status {
            curr_height = block_height
        } else {
            // wait_for_next_block checks every second, so it's not likely the chain could halt for
            // an upgrade before we find the desired height
            return Err(CosmosGrpcError::BadResponse(
                "Wait for block: Chain was running and now it's not?".to_string(),
            ));
        }
    }
    Ok(())
}
