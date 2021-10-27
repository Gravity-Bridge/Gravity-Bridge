use crate::get_fee;
use crate::ADDRESS_PREFIX;
use crate::COSMOS_NODE_GRPC;
use crate::ETH_NODE;
use crate::TOTAL_TIMEOUT;
use crate::{one_eth, MINER_PRIVATE_KEY};
use crate::{MINER_ADDRESS, OPERATION_TIMEOUT};
use actix::System;
use clarity::{Address as EthAddress, Uint256};
use clarity::{PrivateKey as EthPrivateKey, Transaction};
use deep_space::address::Address as CosmosAddress;
use deep_space::coin::Coin;
use deep_space::private_key::PrivateKey as CosmosPrivateKey;
use deep_space::utils::encode_any;
use deep_space::{Contact, Fee, Msg};
use ethereum_gravity::utils::get_event_nonce;
use futures::future::join_all;
use gravity_proto::cosmos_sdk_proto::cosmos::gov::v1beta1::VoteOption;
use gravity_proto::cosmos_sdk_proto::cosmos::params::v1beta1::{
    ParamChange, ParameterChangeProposal,
};
use gravity_proto::cosmos_sdk_proto::cosmos::staking::v1beta1::QueryValidatorsRequest;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::MsgSendToCosmosClaim;
use gravity_utils::types::GravityBridgeToolsConfig;
use orchestrator::main_loop::orchestrator_main_loop;
use rand::Rng;
use std::thread;
use std::time::{Duration, Instant};
use tokio::time::sleep;
use web30::jsonrpc::error::Web3Error;
use web30::{client::Web3, types::SendTxOption};

pub fn create_default_test_config() -> GravityBridgeToolsConfig {
    let mut no_relay_market_config = GravityBridgeToolsConfig::default();
    no_relay_market_config.relayer.batch_market_enabled = false;
    no_relay_market_config.relayer.valset_market_enabled = false;
    no_relay_market_config.relayer.logic_call_market_enabled = false;
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
        eth_keys.push(key.eth_key.to_public_key().unwrap());
    }
    send_eth_bulk(one_eth() * 100u16.into(), &eth_keys, web30).await;
}

pub async fn send_one_eth(dest: EthAddress, web30: &Web3) {
    send_eth_bulk(one_eth(), &[dest], web30).await;
}

pub async fn check_cosmos_balance(
    denom: &str,
    address: CosmosAddress,
    contact: &Contact,
) -> Option<Coin> {
    let account_info = contact.get_balances(address).await.unwrap();
    trace!("Cosmos balance {:?}", account_info);
    for coin in account_info {
        // make sure the name and amount is correct
        if coin.denom.starts_with(denom) {
            return Some(coin);
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
    let eth_address = eth_key.to_public_key().unwrap();
    // the destination on cosmos that sends along to the final ethereum destination
    let cosmos_key = CosmosPrivateKey::from_secret(&secret);
    let cosmos_address = cosmos_key.to_address(ADDRESS_PREFIX.as_str()).unwrap();
    let mut rng = rand::thread_rng();
    let secret: [u8; 32] = rng.gen();
    // the final destination of the tokens back on Ethereum
    let eth_dest_key = EthPrivateKey::from_slice(&secret).unwrap();
    let eth_dest_address = eth_key.to_public_key().unwrap();
    BridgeUserKey {
        eth_address,
        eth_key,
        cosmos_address,
        cosmos_key,
        eth_dest_address,
        eth_dest_key,
    }
}
#[derive(Debug, Eq, PartialEq)]
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
            k.eth_key.to_public_key().unwrap(),
            k.orch_key.to_address(ADDRESS_PREFIX.as_str()).unwrap(),
            k.validator_key
                .to_address(&format!("{}valoper", ADDRESS_PREFIX.as_str()))
                .unwrap()
        );
        let grpc_client = GravityQueryClient::connect(COSMOS_NODE_GRPC.as_str())
            .await
            .unwrap();
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
                get_fee(),
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
            .await;
        info!("Oracle {} false claim response {:?}", i, res);
    }
}

/// Creates a proposal to change the params of our test chain
pub async fn create_parameter_change_proposal(
    contact: &Contact,
    key: CosmosPrivateKey,
    deposit: Coin,
    params_to_change: Vec<ParamChange>,
) {
    let proposal = ParameterChangeProposal {
        title: "Set gravity settings!".to_string(),
        description: "test proposal".to_string(),
        changes: params_to_change,
    };
    let any = encode_any(
        proposal,
        "/cosmos.params.v1beta1.ParameterChangeProposal".to_string(),
    );

    let res = contact
        .create_gov_proposal(any, deposit, get_fee(), key, Some(TOTAL_TIMEOUT))
        .await
        .unwrap();
    trace!("Gov proposal submitted with {:?}", res);
    let res = contact.wait_for_tx(res, TOTAL_TIMEOUT).await.unwrap();
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
    for validator in validators.validators {
        info!(
            "Validator {} has {} tokens",
            validator.operator_address, validator.tokens
        );
    }
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
    info!("Found proposals: {:?}", proposals.proposals);
    for proposal in proposals.proposals {
        for key in keys.iter() {
            info!("Voting yes on governance proposal");
            let res = contact
                .vote_on_gov_proposal(
                    proposal.proposal_id,
                    VoteOption::Yes,
                    get_fee(),
                    key.validator_key,
                    Some(duration),
                )
                .await
                .unwrap();
            contact.wait_for_tx(res, TOTAL_TIMEOUT).await.unwrap();
        }
    }
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
