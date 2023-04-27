//! Monitors balances across both sides of the bridge, enabling the orchestrator
//! to halt in the event of a balance discrepancy

use clarity::{Address as EthAddress, Uint256};
use deep_space::error::CosmosGrpcError;
use futures::future::join_all;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_proto::gravity::{
    BridgeBalanceSnapshot, QueryBridgeBalanceSnapshots, QueryMonitoredErc20Addresses,
};
use std::collections::HashMap;
use std::str::FromStr;
use tonic::transport::Channel;
use web30::client::Web3;

use super::erc20::Erc20Token;
use crate::error::GravityError::InvalidBridgeBalances;
use crate::error::{GravityError, ETHEREUM_MISSING_NODE};

/// Collects balances on both sides of the bridge (supply snapshots on the Gravity Bridge Chain side, Gravity.sol
/// holdings on the Ethereum side) and asserts that the Ethereum side is not insolvent.
/// Note that it is possible to inflate the balances on the Ethereum side by sending tokens to Gravity.sol as part of an
/// ERC20 transfer(), but it should not be possible to create supply on the Gravity Bridge Chain side.
pub async fn check_cross_bridge_balances(
    grpc_client: &GravityQueryClient<Channel>,
    web30: &Web3,
    querier_address: EthAddress,
    gravity_contract_address: EthAddress,
) -> Result<(), GravityError> {
    let mut grpc_client = grpc_client.clone();
    let erc20s: Vec<EthAddress> = grpc_client
        .get_monitored_erc20_addresses(QueryMonitoredErc20Addresses {})
        .await?
        .into_inner()
        .addresses
        .into_iter()
        .map(|a| {
            EthAddress::from_str(&a).unwrap_or_else(|_| {
                panic!(
                    "received invalid monitored erc20 address from gravity module {}",
                    a
                )
            })
        })
        .collect();

    if erc20s.is_empty() {
        // When the monitored token response is empty, no need to check balances
        return Ok(());
    }

    // Collect the erc20s, more snapshots than desired
    let snapshot = gravity_chain_balance_data(&grpc_client).await?;
    let height = snapshot.ethereum_block_height;

    // Nothing to check, no new snapshots have been committed
    if height == 0 {
        return Ok(());
    }

    let eth_balances = collect_eth_balances_at_height(
        web30,
        querier_address,
        gravity_contract_address,
        &erc20s,
        height.into(),
    )
    .await?;

    match eth_balances {
        HistEthBalances::Missing => {
            info!("Unable to get historical ethereum balances at height {} - skipping this balance check", height);
            Ok(())
        }
        HistEthBalances::Found(eth_bals) => {
            let cosmos_balances: Vec<Erc20Token> = snapshot
                .balances
                .into_iter()
                .map(|b| Erc20Token {
                    amount: Uint256::from_str(&b.amount)
                        .expect("Invalid balance amount obtained from gravity module"),
                    token_contract_address: EthAddress::from_str(&b.contract)
                        .expect("invalid balance contract obtained from gravity module"),
                })
                .collect();

            let res = valid_bridge_balances(eth_bals, cosmos_balances);
            if res.is_err() {
                error!("!!!!!!!!!!!!!!!!!!!!!!!!!!!! INVALID CROSS BRIDGE BALANCES !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!");
                let err = res.err();
                error!("Error is {:?}", err);
                return Err(err.unwrap());
            }

            Ok(())
        }
    }
}

// BalanceEntry is a helper struct used to populate a HashMap in valid_bridge_balances, these are
// 0 initialized and populated if balances are found. They are then iterated over to make assertions
#[derive(Clone, Copy, Debug)]
pub struct BalanceEntry {
    pub c: Uint256,
    pub e: Uint256,
}

/// Checks the inputs to determine if the bridge has an unexplained difference
/// in balances. Returns Ok(()) if the balances are normal or Err(InvalidBridgeBalances) otherwise
pub fn valid_bridge_balances(
    ethereum_balances: Vec<Erc20Token>,
    cosmos_snapshot_balances: Vec<Erc20Token>,
) -> Result<(), GravityError> {
    // Add the cosmos and ethereum side entries to a map by contract address
    let mut balances_by_contract: HashMap<String, BalanceEntry> = HashMap::new();
    for balance in ethereum_balances {
        let key = balance.token_contract_address.to_string();
        let entry = balances_by_contract.get(&key);
        let entry = match entry {
            None => BalanceEntry {
                c: 0u8.into(),
                e: balance.amount,
            },
            Some(e) => {
                let mut copy = *e;
                copy.e = balance.amount;
                copy
            }
        };
        balances_by_contract.insert(key.clone(), entry);
    }

    for balance in cosmos_snapshot_balances {
        let key = balance.token_contract_address.to_string();
        let entry = balances_by_contract.get_mut(&key);
        if entry.is_none() {
            // Cosmos reports *all* bridged tokens, skip unmonitored tokens
            continue;
        }
        let entry = entry.unwrap();
        entry.c = balance.amount;
    }

    // Assert that any recorded balances are appropriate
    for (k, BalanceEntry { e: eth, c: cos }) in balances_by_contract {
        // A balance is appropriate iff the ehtereum balance is greater than or equal to the cosmos balance
        // Note that only one of eth or cos may be populated, but since they were initialized with 0 it all works out
        if eth.lt(&cos) {
            return Err(InvalidBridgeBalances(format!(
                "The balance of contract {} does not match: (ethereum {} != cosmos {})",
                k, eth, cos
            )));
        }
    }

    Ok(())
}

// Collects the monitored ERC20 tokens, relevant bridge balance snapshots, and the list of ethereum heights to monitor
pub async fn gravity_chain_balance_data(
    grpc_client: &GravityQueryClient<Channel>,
) -> Result<BridgeBalanceSnapshot, CosmosGrpcError> {
    let mut grpc_client = grpc_client.clone();

    let snapshots_res = grpc_client
        .get_bridge_balance_snapshots(QueryBridgeBalanceSnapshots {
            limit: 1,
            newest_first: true,
        })
        .await?
        .into_inner()
        .snapshots;
    if snapshots_res.is_empty() {
        info!("No snapshots returned - cross bridge balances will not be checked. This log should occur only once or twice!");
        Ok(BridgeBalanceSnapshot {
            cosmos_block_height: 0,
            ethereum_block_height: 0,
            balances: vec![],
            event_nonce: 0,
        })
    } else {
        return Ok(snapshots_res.get(0).unwrap().clone());
    }
}

// An enum used to describe the acceptable results an ethereum endpoint can respond with for a historical request
// if the node is not archival it is likely to have pruned some history the orchestrator requires, in which case
// the best case scenario is the orchestrator skips this issue and tries to check later
#[derive(Debug)]
pub enum HistEthBalances {
    Missing,
    Found(Vec<Erc20Token>),
}

/// Fetches the balances of the Gravity.sol contract at the provided ethereum block height
/// Returns an error if any of the underlying queries return an error
pub async fn collect_eth_balances_at_height(
    web3: &Web3,
    querier_address: EthAddress,
    gravity_contract: EthAddress,
    erc20s: &[EthAddress],
    height: Uint256,
) -> Result<HistEthBalances, GravityError> {
    let mut futs = vec![];
    for e in erc20s {
        futs.push(web3.get_erc20_balance_at_height_as_address(
            Some(querier_address),
            *e,
            gravity_contract,
            Some(height),
        ));
    }
    let res = join_all(futs).await;
    let mut results = vec![];
    // Order of res is preserved, so we can assign the erc20 by index
    for (i, r) in res.into_iter().enumerate() {
        match r {
            Ok(bal) => {
                let erc20 = erc20s[i];
                results.push(Erc20Token {
                    token_contract_address: erc20,
                    amount: bal,
                });
            }
            Err(error) => {
                if error.to_string().contains(ETHEREUM_MISSING_NODE) {
                    return Ok(HistEthBalances::Missing);
                }
            }
        }
    }

    Ok(HistEthBalances::Found(results))
}

#[cfg(test)]
mod tests {
    use crate::types::cross_bridge_balances::valid_bridge_balances;
    use std::str::FromStr;
    use clarity::Address;
    use crate::types::erc20::Erc20Token;
    #[test]
    fn test_valid_bridge_balances() {
        // This configuration implies that 0x0412C7c846bb6b7DC462CF6B453f76D8440b2609 and 0x30dA8589BFa1E509A319489E014d384b87815D89
        // are the monitored ERC20s, thus despite 0x9676519d99E390A180Ab1445d5d857E3f6869065 having a lesser balance on Ethereum
        // we should not expect an error.

        let eth_bals = vec![
            Erc20Token{
                amount: 100u8.into(),
                token_contract_address: Address::from_str("0x0412C7c846bb6b7DC462CF6B453f76D8440b2609").unwrap(),
            },
            Erc20Token{
                amount: 10u8.into(),
                token_contract_address: Address::from_str("0x30dA8589BFa1E509A319489E014d384b87815D89").unwrap(),
            },
        ];
        let cos_bals = vec![
            Erc20Token{
                amount: 10u8.into(),
                token_contract_address: Address::from_str("0x30dA8589BFa1E509A319489E014d384b87815D89").unwrap(),
            },
            Erc20Token{
                amount: 100u8.into(),
                token_contract_address: Address::from_str("0x9676519d99E390A180Ab1445d5d857E3f6869065").unwrap(),
            },
        ];

        let res = valid_bridge_balances(eth_bals, cos_bals);
        println!("Got valid_bridge_balances: {res:?}");
        assert!(res.is_ok());
    }

    #[test]
    fn test_invalid_bridge_balances() {
        // This configuration implies that 0x0412C7c846bb6b7DC462CF6B453f76D8440b2609 and 0x9676519d99E390A180Ab1445d5d857E3f6869065
        // are monitored ERC20 tokens, and 0x0412C7c846bb6b7DC462CF6B453f76D8440b2609 has been erroneously sent to Gravity.sol.
        // Notably, 0x9676519d99E390A180Ab1445d5d857E3f6869065 has a low balance and should cause a bridge halt

        let eth_bals = vec![
            Erc20Token{
                amount: 100u8.into(),
                token_contract_address: Address::from_str("0x0412C7c846bb6b7DC462CF6B453f76D8440b2609").unwrap(),
            },
            Erc20Token{
                amount: 99u8.into(),
                token_contract_address: Address::from_str("0x9676519d99E390A180Ab1445d5d857E3f6869065").unwrap(),
            },
        ];
        let cos_bals = vec![
            Erc20Token{
                amount: 10u8.into(),
                token_contract_address: Address::from_str("0x30dA8589BFa1E509A319489E014d384b87815D89").unwrap(),
            },
            Erc20Token{
                amount: 100u8.into(),
                token_contract_address: Address::from_str("0x9676519d99E390A180Ab1445d5d857E3f6869065").unwrap(),
            },
        ];

        let res = valid_bridge_balances(eth_bals, cos_bals);
        println!("Got valid_bridge_balances: {res:?}");
        assert!(res.is_err());
    }
}