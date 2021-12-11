//! This module provides useful tools for handling the Contact and Web30 connections for the relayer, orchestrator and various other utilities.
//! It's a common problem to have conflicts between ipv4 and ipv6 localhost and this module is first and foremost supposed to resolve that problem
//! by trying more than one thing to handle potentially misconfigured inputs.

use clarity::Address as EthAddress;
use deep_space::error::CosmosGrpcError;
use deep_space::Address as CosmosAddress;
use deep_space::Contact;
use deep_space::{client::ChainStatus, Coin};
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::QueryDelegateKeysByEthAddress;
use gravity_proto::gravity::QueryDelegateKeysByOrchestratorAddress;
use std::process::exit;
use std::time::Duration;
use tokio::time::sleep as delay_for;
use tonic::transport::Channel;
use url::Url;
use web30::client::Web3;

use crate::get_with_retry::get_balances_with_retry;
use crate::get_with_retry::get_eth_balances_with_retry;

pub struct Connections {
    pub web3: Option<Web3>,
    pub grpc: Option<GravityQueryClient<Channel>>,
    pub contact: Option<Contact>,
}

/// Returns the three major RPC connections required for Gravity
/// operation in a error resilient manner. TODO find some way to generalize
/// this so that it's less ugly
pub async fn create_rpc_connections(
    address_prefix: String,
    grpc_url: Option<String>,
    eth_rpc_url: Option<String>,
    timeout: Duration,
) -> Connections {
    let mut web3 = None;
    let mut grpc = None;
    let mut contact = None;
    if let Some(grpc_url) = grpc_url {
        let url = Url::parse(&grpc_url)
            .unwrap_or_else(|_| panic!("Invalid Cosmos gRPC url {}", grpc_url));
        check_scheme(&url, &grpc_url);
        let cosmos_grpc_url = grpc_url.trim_end_matches('/').to_string();
        // try the base url first.
        let try_base = GravityQueryClient::connect(cosmos_grpc_url.clone()).await;
        match try_base {
            // it worked, lets go!
            Ok(val) => {
                grpc = Some(val);
                contact = Some(Contact::new(&cosmos_grpc_url, timeout, &address_prefix).unwrap());
            }
            // did not work, now we check if it's localhost
            Err(e) => {
                warn!(
                    "Failed to access Cosmos gRPC with {:?} trying fallback options",
                    e
                );
                if grpc_url.to_lowercase().contains("localhost") {
                    let port = url.port().unwrap_or(80);
                    // this should be http or https
                    let prefix = url.scheme();
                    let ipv6_url = format!("{}://::1:{}", prefix, port);
                    let ipv4_url = format!("{}://127.0.0.1:{}", prefix, port);
                    let ipv6 = GravityQueryClient::connect(ipv6_url.clone()).await;
                    let ipv4 = GravityQueryClient::connect(ipv4_url.clone()).await;
                    warn!("Trying fallback urls {} {}", ipv6_url, ipv4_url);
                    match (ipv4, ipv6) {
                        (Ok(v), Err(_)) => {
                            info!("Url fallback succeeded, your cosmos gRPC url {} has been corrected to {}", grpc_url, ipv4_url);
                            contact = Some(Contact::new(&ipv4_url, timeout, &address_prefix).unwrap());
                            grpc = Some(v)
                        },
                        (Err(_), Ok(v)) => {
                            info!("Url fallback succeeded, your cosmos gRPC url {} has been corrected to {}", grpc_url, ipv6_url);
                            contact = Some(Contact::new(&ipv6_url, timeout, &address_prefix).unwrap());
                            grpc = Some(v)
                        },
                        (Ok(_), Ok(_)) => panic!("This should never happen? Why didn't things work the first time?"),
                        (Err(_), Err(_)) => panic!("Could not connect to Cosmos gRPC, are you sure it's running and on the specified port? {}", grpc_url)
                    }
                } else if url.port().is_none() || url.scheme() == "http" {
                    let body = url.host_str().unwrap_or_else(|| {
                        panic!("Cosmos gRPC url contains no host? {}", grpc_url)
                    });
                    // transparently upgrade to https if available, we can't transparently downgrade for obvious security reasons
                    let https_on_80_url = format!("https://{}:80", body);
                    let https_on_443_url = format!("https://{}:443", body);
                    let https_on_80 = GravityQueryClient::connect(https_on_80_url.clone()).await;
                    let https_on_443 = GravityQueryClient::connect(https_on_443_url.clone()).await;
                    warn!(
                        "Trying fallback urls {} {}",
                        https_on_443_url, https_on_80_url
                    );
                    match (https_on_80, https_on_443) {
                        (Ok(v), Err(_)) => {
                            info!("Https upgrade succeeded, your cosmos gRPC url {} has been corrected to {}", grpc_url, https_on_80_url);
                            contact = Some(Contact::new(&https_on_80_url, timeout, &address_prefix).unwrap());
                            grpc = Some(v)
                        },
                        (Err(_), Ok(v)) => {
                            info!("Https upgrade succeeded, your cosmos gRPC url {} has been corrected to {}", grpc_url, https_on_443_url);
                            contact = Some(Contact::new(&https_on_443_url, timeout, &address_prefix).unwrap());
                            grpc = Some(v)
                        },
                        (Ok(_), Ok(_)) => panic!("This should never happen? Why didn't things work the first time?"),
                        (Err(_), Err(_)) => panic!("Could not connect to Cosmos gRPC, are you sure it's running and on the specified port? {}", grpc_url)
                    }
                } else {
                    panic!("Could not connect to Cosmos gRPC! please check your grpc url {} for errors {:?}", grpc_url, e)
                }
            }
        }
    }
    if let Some(eth_rpc_url) = eth_rpc_url {
        let url = Url::parse(&eth_rpc_url)
            .unwrap_or_else(|_| panic!("Invalid Ethereum RPC url {}", eth_rpc_url));
        check_scheme(&url, &eth_rpc_url);
        let eth_url = eth_rpc_url.trim_end_matches('/');
        let base_web30 = Web3::new(eth_url, timeout);
        let try_base = base_web30.eth_block_number().await;
        match try_base {
            // it worked, lets go!
            Ok(_) => web3 = Some(base_web30),
            // did not work, now we check if it's localhost
            Err(e) => {
                warn!(
                    "Failed to access Ethereum RPC with {:?} trying fallback options",
                    e
                );
                if eth_url.to_lowercase().contains("localhost") {
                    let port = url.port().unwrap_or(80);
                    // this should be http or https
                    let prefix = url.scheme();
                    let ipv6_url = format!("{}://::1:{}", prefix, port);
                    let ipv4_url = format!("{}://127.0.0.1:{}", prefix, port);
                    let ipv6_web3 = Web3::new(&ipv6_url, timeout);
                    let ipv4_web3 = Web3::new(&ipv4_url, timeout);
                    let ipv6_test = ipv6_web3.eth_block_number().await;
                    let ipv4_test = ipv4_web3.eth_block_number().await;
                    warn!("Trying fallback urls {} {}", ipv6_url, ipv4_url);
                    match (ipv4_test, ipv6_test) {
                        (Ok(_), Err(_)) => {
                            info!("Url fallback succeeded, your Ethereum rpc url {} has been corrected to {}", eth_rpc_url, ipv4_url);
                            web3 = Some(ipv4_web3)
                        }
                        (Err(_), Ok(_)) => {
                            info!("Url fallback succeeded, your Ethereum  rpc url {} has been corrected to {}", eth_rpc_url, ipv6_url);
                            web3 = Some(ipv6_web3)
                        },
                        (Ok(_), Ok(_)) => panic!("This should never happen? Why didn't things work the first time?"),
                        (Err(_), Err(_)) => panic!("Could not connect to Ethereum rpc, are you sure it's running and on the specified port? {}", eth_rpc_url)
                    }
                } else if url.port().is_none() || url.scheme() == "http" {
                    let body = url.host_str().unwrap_or_else(|| {
                        panic!("Ethereum rpc url contains no host? {}", eth_rpc_url)
                    });
                    // transparently upgrade to https if available, we can't transparently downgrade for obvious security reasons
                    let https_on_80_url = format!("https://{}:80", body);
                    let https_on_443_url = format!("https://{}:443", body);
                    let https_on_80_web3 = Web3::new(&https_on_80_url, timeout);
                    let https_on_443_web3 = Web3::new(&https_on_443_url, timeout);
                    let https_on_80_test = https_on_80_web3.eth_block_number().await;
                    let https_on_443_test = https_on_443_web3.eth_block_number().await;
                    warn!(
                        "Trying fallback urls {} {}",
                        https_on_443_url, https_on_80_url
                    );
                    match (https_on_80_test, https_on_443_test) {
                        (Ok(_), Err(_)) => {
                            info!("Https upgrade succeeded, your Ethereum rpc url {} has been corrected to {}", eth_rpc_url, https_on_80_url);
                            web3 = Some(https_on_80_web3)
                        },
                        (Err(_), Ok(_)) => {
                            info!("Https upgrade succeeded, your Ethereum rpc url {} has been corrected to {}", eth_rpc_url, https_on_443_url);
                            web3 = Some(https_on_443_web3)
                        },
                        (Ok(_), Ok(_)) => panic!("This should never happen? Why didn't things work the first time?"),
                        (Err(_), Err(_)) => panic!("Could not connect to Ethereum rpc, are you sure it's running and on the specified port? {}", eth_rpc_url)
                    }
                } else {
                    panic!("Could not connect to Ethereum rpc! please check your grpc url {} for errors {:?}", eth_rpc_url, e)
                }
            }
        }
    }

    Connections {
        web3,
        grpc,
        contact,
    }
}

/// Verify that a url has an http or https prefix
fn check_scheme(input: &Url, original_string: &str) {
    if !(input.scheme() == "http" || input.scheme() == "https") {
        panic!(
            "Your url {} has an invalid scheme, please chose http or https",
            original_string
        )
    }
}

/// This function will wait until the Cosmos node is ready, this is intended
/// for situations such as when a node is syncing or when a node is waiting on
/// a halted chain.
pub async fn wait_for_cosmos_node_ready(contact: &Contact) {
    const WAIT_TIME: Duration = Duration::from_secs(10);
    loop {
        let res = contact.get_chain_status().await;
        match res {
            Ok(ChainStatus::Syncing) => {
                info!("Cosmos node is syncing Standing by")
            }
            Ok(ChainStatus::WaitingToStart) => {
                info!("Cosmos node is waiting for the chain to start, Standing by")
            }
            Ok(ChainStatus::Moving { .. }) => {
                break;
            }
            Err(e) => warn!(
                "Could not get syncing status, is your Cosmos node up? {:?}",
                e
            ),
        }
        delay_for(WAIT_TIME).await;
    }
}

/// This function checks the orchestrator delegate addresses
/// for consistency what this means is that it takes the Ethereum
/// address and Orchestrator address from the Orchestrator and checks
/// that both are registered and internally consistent.
pub async fn check_delegate_addresses(
    client: &mut GravityQueryClient<Channel>,
    delegate_eth_address: EthAddress,
    delegate_orchestrator_address: CosmosAddress,
    prefix: &str,
) {
    let eth_response = client
        .get_delegate_key_by_eth(QueryDelegateKeysByEthAddress {
            eth_address: delegate_eth_address.to_string(),
        })
        .await;
    let orchestrator_response = client
        .get_delegate_key_by_orchestrator(QueryDelegateKeysByOrchestratorAddress {
            orchestrator_address: delegate_orchestrator_address.to_bech32(prefix).unwrap(),
        })
        .await;
    trace!("{:?} {:?}", eth_response, orchestrator_response);
    match (eth_response, orchestrator_response) {
        (Ok(e), Ok(o)) => {
            let e = e.into_inner();
            let o = o.into_inner();
            let req_delegate_orchestrator_address: CosmosAddress =
                e.orchestrator_address.parse().unwrap();
            let req_delegate_eth_address: EthAddress = o.eth_address.parse().unwrap();
            if req_delegate_eth_address != delegate_eth_address
                && req_delegate_orchestrator_address != delegate_orchestrator_address
            {
                error!("Your Gravity Delegate addresses are both incorrect!");
                error!("If you are getting this error you must have made at least two validators and mixed up the keys between them");
                error!(
                    "You provided {}  Correct Value {}",
                    delegate_eth_address, req_delegate_eth_address
                );
                error!(
                    "You provided {}  Correct Value {}",
                    delegate_orchestrator_address, req_delegate_orchestrator_address
                );
                error!("In order to resolve this issue locate the key phrase and private key you registered for this validator and run the following commands");
                error!("`gbt keys set-ethereum-key --key \"eth private key\"`");
                error!("`gbt keys set-orchestrator-key --phrase \"orchestrator key phrase\"`");
                error!("If you can not find the private key and phrase for these addresses you will need to create a new validator");
                error!("If you are seeing this error please read this documentation carefully https://github.com/Gravity-Bridge/Gravity-Docs/blob/main/docs/setting-up-a-validator.md#generate-your-delegate-keys");
                exit(1);
            } else if req_delegate_eth_address != delegate_eth_address {
                error!("Your Delegate Ethereum address is incorrect!");
                error!(
                    "You provided {}  Correct Value {}",
                    delegate_eth_address, req_delegate_eth_address
                );
                error!("In order to resolve this issue locate the private key you registered for this validator and run the following command");
                error!("`gbt keys set-ethereum-key --key \"eth private key\"`");
                error!("If you are seeing this error please read this documentation carefully https://github.com/Gravity-Bridge/Gravity-Docs/blob/main/docs/setting-up-a-validator.md#generate-your-delegate-keys");
                exit(1);
            } else if req_delegate_orchestrator_address != delegate_orchestrator_address {
                error!("Your Delegate Orchestrator address is incorrect!");
                error!(
                    "You provided {}  Correct Value {}",
                    delegate_eth_address, req_delegate_eth_address
                );
                error!("In order to resolve this issue locate the key phrase you registered for this validator and run the following command");
                error!("`gbt keys set-orchestrator-key --phrase \"orchestrator key phrase\"`");
                error!("If you are seeing this error please read this documentation carefully https://github.com/Gravity-Bridge/Gravity-Docs/blob/main/docs/setting-up-a-validator.md#generate-your-delegate-keys");
                exit(1);
            }

            if e.validator_address != o.validator_address {
                error!(
                    "You are using Gravity delegate keys from two different validator addresses!"
                );
                error!("If you get this error message I would just blow everything away and start again");
                error!("If you are seeing this error please read this documentation carefully https://github.com/Gravity-Bridge/Gravity-Docs/blob/main/docs/setting-up-a-validator.md#generate-your-delegate-keys");
                exit(1);
            }
        }
        (Err(e), Ok(_)) => {
            error!("Your Gravity Orchestrator Ethereum key is incorrect, please double check you private key. If you can't locate the correct private key you will need to create a new validator {:?}", e);
            error!("If you are seeing this error please read this documentation carefully https://github.com/Gravity-Bridge/Gravity-Docs/blob/main/docs/setting-up-a-validator.md#generate-your-delegate-keys");
            exit(1);
        }
        (Ok(_), Err(e)) => {
            error!("Your Gravity Orchestrator Cosmos key is incorrect, please double check your phrase. If you can't locate the correct phrase you will need to create a new validator {:?}", e);
            error!("If you are seeing this error please read this documentation carefully https://github.com/Gravity-Bridge/Gravity-Docs/blob/main/docs/setting-up-a-validator.md#generate-your-delegate-keys");
            exit(1);
        }
        (Err(_), Err(_)) => {
            error!("Gravity Delegate keys are not set! Please Register your Gravity delegate keys");
            error!("`gbt keys set-orchestrator-key --phrase \"orchestrator key phrase\"`");
            error!("`gbt keys set-ethereum-key --key \"eth private key\"`");
            error!("If you are seeing this error please read this documentation carefully https://github.com/Gravity-Bridge/Gravity-Docs/blob/main/docs/setting-up-a-validator.md#generate-your-delegate-keys");
            exit(1);
        }
    }
}

/// Checks if a given Coin, used for fees is in the provided address in a sufficient quantity
pub async fn check_for_fee(fee: &Coin, address: CosmosAddress, contact: &Contact) {
    // if we decide to pay no fees it doesn't matter, but we do need some coin balance
    if fee.amount == 0u8.into() {
        if let Err(CosmosGrpcError::NoToken) = contact.get_account_info(address).await {
            error!("Your Orchestrator address has no tokens of any kind. Even if you are paying zero fees this account needs to be 'initialized' by depositing tokens");
            error!(
                "Send the smallest possible unit of any token to {} to resolve this error",
                address
            );
            exit(1);
        }
        return;
    }
    let balances = get_balances_with_retry(address, contact).await;
    for balance in balances {
        if balance.denom.contains(&fee.denom) {
            if balance.amount < fee.amount {
                error!("You have specified a fee that is greater than your balance of that coin! {}{} > {}{} ", fee.amount, fee.denom, balance.amount, balance.denom);
                exit(1);
            } else {
                return;
            }
        }
    }
    error!("You have specified that fees should be paid in {} but account {} has no balance of that token!", fee.denom, address);
    exit(1);
}

/// Checks the user has some Ethereum in their address to pay for things
pub async fn check_for_eth(address: EthAddress, web3: &Web3) {
    let balance = get_eth_balances_with_retry(address, web3).await;
    if balance == 0u8.into() {
        error!("You don't have any Ethereum! You will need to send some to {} for this program to work. Dust will do for basic operations, more info about average relaying costs will be presented as the program runs", address);
        error!("You can disable relaying by editing your config file in $HOME/.gbt/config");
        error!(
            "Even if you disable relaying you still need some dust so that the oracle can function"
        );
        exit(1);
    }
}
