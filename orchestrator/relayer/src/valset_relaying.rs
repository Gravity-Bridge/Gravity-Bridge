//! This module contains code for the validator update lifecycle. Functioning as a way for this validator to observe
//! the state of both chains and perform the required operations.

use clarity::address::Address as EthAddress;
use clarity::PrivateKey as EthPrivateKey;
use cosmos_gravity::query::get_latest_valsets;
use cosmos_gravity::query::{get_all_valset_confirms, get_valset};
use ethereum_gravity::message_signatures::encode_valset_confirm_hashed;
use ethereum_gravity::{
    utils::get_valset_nonce, utils::GasCost, valset_update::send_eth_valset_update,
};
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use gravity_utils::error::GravityError;
use gravity_utils::num_conversion::{print_eth, print_gwei};
use gravity_utils::prices::get_weth_price_with_retries;
use gravity_utils::types::{RelayerConfig, Valset};
use gravity_utils::types::{ValsetConfirmResponse, ValsetRelayingMode};
use tonic::transport::Channel;
use web30::client::Web3;

use crate::batch_relaying::get_cost_with_margin;
use crate::main_loop::ETH_SUBMIT_WAIT_TIME;

#[allow(clippy::too_many_arguments)]
/// High level entry point for valset relaying, this function starts by finding
/// what validator set is valid, then evaluating if it should be relayed according
/// to the users preferences and finally relaying a validator set that is valid at
/// this moment in time
pub async fn relay_valsets(
    // the validator set currently in the contract on Ethereum
    current_valset: Valset,
    ethereum_key: EthPrivateKey,
    web3: &Web3,
    grpc_client: &mut GravityQueryClient<Channel>,
    gravity_contract_address: EthAddress,
    gravity_id: String,
    config: RelayerConfig,
) {
    // we have to start with the current valset, we need to know what's currently
    // in the contract in order to determine if a new validator set is valid.
    // For example the contract has set A which contains validators x/y/z the
    // latest valset has set C which has validators z/e/f in order to have enough
    // power we actually need to submit validator set B with validators x/y/e in
    // order to know exactly which one we must iterate over the history in find_latest_valid_valset()

    let latest_cosmos_valset_nonce = match get_latest_cosmos_valset_nonce(grpc_client).await {
        Some(n) => n,
        None => return,
    };

    // the latest cosmos validator set that it is possible to submit given the constraints
    // of the validator set currently in the bridge
    let (latest_submittable_valset, confirms) = match find_latest_valid_valset(
        latest_cosmos_valset_nonce,
        &current_valset,
        grpc_client,
        &gravity_id,
    )
    .await
    {
        Ok(v) => v,
        Err(GravityError::ValsetUpToDate) => return,
        Err(e) => {
            error!(
                "We were unable to find a valid validator set update to submit! {:?}",
                e
            );
            return;
        }
    };

    relay_valid_valset(
        latest_cosmos_valset_nonce,
        latest_submittable_valset,
        current_valset,
        confirms,
        web3,
        gravity_contract_address,
        gravity_id,
        ethereum_key,
        config,
    )
    .await;
}

#[allow(clippy::too_many_arguments)]
/// determines if the provided validator set should be relayed according to user preferences
async fn relay_valid_valset(
    latest_cosmos_valset_nonce: u64,
    valset_to_relay: Valset,
    current_valset: Valset,
    conformations: Vec<ValsetConfirmResponse>,
    web3: &Web3,
    gravity_contract_address: EthAddress,
    gravity_id: String,
    ethereum_key: EthPrivateKey,
    config: RelayerConfig,
) {
    let cost = ethereum_gravity::valset_update::estimate_valset_cost(
        &valset_to_relay,
        &current_valset,
        &conformations,
        web3,
        gravity_contract_address,
        gravity_id.clone(),
        ethereum_key.to_address(),
    )
    .await;
    if cost.is_err() {
        valset_cost_error(
            cost,
            &ethereum_key,
            &gravity_contract_address,
            web3,
            valset_to_relay.clone(),
            current_valset,
        )
        .await;
        return;
    }
    let cost = cost.unwrap();

    info!(
       "We have detected that valset {} is valid to submit. Latest on Ethereum is {} This update is estimated to cost {} Gas @ {} Gwei/ {:.4} ETH to submit",
        valset_to_relay.nonce, current_valset.nonce,
        cost.gas.clone(),
        print_gwei(cost.gas_price),
        print_eth(cost.get_total())
    );

    let should_relay = should_relay_valset(
        latest_cosmos_valset_nonce,
        &valset_to_relay,
        ethereum_key.to_address(),
        cost,
        web3,
        &config.valset_relaying_mode,
    )
    .await;

    if should_relay {
        let res = send_eth_valset_update(
            valset_to_relay,
            current_valset,
            &conformations,
            web3,
            ETH_SUBMIT_WAIT_TIME,
            gravity_contract_address,
            gravity_id,
            ethereum_key,
        )
        .await;
        if let Err(e) = res {
            error!("Failed to relay validator set with {:?}", e);
        }
    }
}

// Locates the latest valid valset which can be moved to ethereum
// Due to the disparity between the ethereum valset and the actual current cosmos valset
// we may need to move multiple valsets over to update ethereum, based on how much voting power change has ocurred
async fn find_latest_valid_valset(
    latest_nonce_on_cosmos: u64,
    current_valset: &Valset,
    grpc_client: &mut GravityQueryClient<Channel>,
    gravity_id: &str,
) -> Result<(Valset, Vec<ValsetConfirmResponse>), GravityError> {
    // we only use the latest valsets endpoint to get a starting point, from there we will iterate
    // backwards until we find the newest validator set that we can submit to the bridge. So if we
    // have sets A-Z and it's possible to submit only A, L, and Q before reaching Z this code will do
    // so.
    let mut latest_nonce = latest_nonce_on_cosmos;
    let mut latest_confirms = None;
    let mut latest_valset = None;
    // this is used to display the state of the last validator set to fail signature checks
    let mut last_error = None;
    while latest_nonce > current_valset.nonce {
        let valset = get_valset(grpc_client, latest_nonce).await;
        if let Ok(Some(valset)) = valset {
            // check that we got the right valset, should never occur
            assert_eq!(valset.nonce, latest_nonce);
            let confirms = get_all_valset_confirms(grpc_client, latest_nonce).await;
            if let Ok(confirms) = confirms {
                // search for invalid confirms, should never occur
                for confirm in confirms.iter() {
                    assert_eq!(valset.nonce, confirm.nonce);
                }
                let hash = encode_valset_confirm_hashed(gravity_id.to_string(), valset.clone());
                // order valset sigs prepares signatures for submission, notice we compare
                // them to the 'current' set in the bridge, this confirms for us that the validator set
                // we have here can be submitted to the bridge in it's current state
                let res = current_valset.order_sigs(&hash, &confirms);
                if res.is_ok() {
                    if !valset.enough_power() {
                        warn!("Validator set {} can not be executed, power is too low to pass following measures. How was this generated?", valset.nonce);
                    } else {
                        latest_confirms = Some(confirms);
                        latest_valset = Some(valset);
                        // once we have the latest validator set we can submit exit
                        break;
                    }
                } else if let Err(e) = res {
                    // this error prints details about why the valset is not valid, look at it
                    // if you are confused
                    last_error = Some(e);
                }
            }
        }

        latest_nonce -= 1
    }

    if let (Some(v), Some(c)) = (latest_valset, latest_confirms) {
        Ok((v, c))
    } else if let Some(e) = last_error {
        Err(e)
    } else {
        Err(GravityError::ValsetUpToDate)
    }
}

/// determines if the provided valset is profitable
async fn should_relay_valset(
    latest_cosmos_valset_nonce: u64,
    valset: &Valset,
    pubkey: EthAddress,
    cost: GasCost,
    web3: &Web3,
    config: &ValsetRelayingMode,
) -> bool {
    match config {
        // if the user has configured only profitable relaying then it is our only consideration
        ValsetRelayingMode::ProfitableOnly { margin } => match valset.reward_token {
            Some(reward_token) => {
                let price =
                    get_weth_price_with_retries(pubkey, reward_token, valset.reward_amount, web3)
                        .await;
                let cost_with_margin = get_cost_with_margin(cost.get_total(), *margin);
                // we need to see how much WETH we can get for the reward token amount,
                // and compare that value to the gas cost times the margin
                match price {
                    Ok(price) => price > cost_with_margin,
                    Err(e) => {
                        info!(
                            "Unable to determine swap price of token {} for WETH \n
                             it may just not be on Uniswap - Will not be relaying valset {:?}",
                            reward_token, e
                        );
                        false
                    }
                }
            }
            None => false,
        },

        // if the user has requested to relay every single valset, we do so
        ValsetRelayingMode::EveryValset => true,
        // user is an altruistic relayer, so we'll do our best to balance not spending
        // all their money with keeping the validator set up to date.
        //
        // This logic is very financially conservative and will only relay valsets when find_latest_valid_valset() does not
        // return the latest valset. This means that voting power has sufficiently changed that a two step update
        // is required, where one older valset is played back, and then the latest one. At this stage it is critical an update be
        // performed, or relaying batches will be impossible.
        //
        // remember the philosophy of valset creation is to create valsets such that there is always a valid chain of updates to submit
        // since we store all the required signatures for as long as we may need them on the cosmos chain it's not fatal to wait, we can always play
        // them back later when we need them. Since 2/3 of voting power is required to spend funds and only 1/3 of voting power must change over
        // before this condition is triggered it should not risk a stale validator set in the Ethereum side of the bridge sending funds.
        ValsetRelayingMode::Altruistic => latest_cosmos_valset_nonce != valset.nonce,
    }
}

/// due to some API design quirks we get the latest 5 validator sets from the Gravity module
/// this is actually usually enough to relay, but not always, find_latest_valid_valset contains
/// the logic that actually answers this difficult question, but it needs a starting place to work
/// backwards from, that is provided by getting the latest 5 and going from there
async fn get_latest_cosmos_valset_nonce(
    grpc_client: &mut GravityQueryClient<Channel>,
) -> Option<u64> {
    let latest_valsets = get_latest_valsets(grpc_client).await;
    match latest_valsets {
        Ok(latest_valsets) => {
            if latest_valsets.is_empty() {
                None
            } else {
                Some(latest_valsets[0].nonce)
            }
        }
        Err(_) => None,
    }
}

// Handles errors that occur when estimating valset cost
async fn valset_cost_error(
    cost: Result<GasCost, GravityError>,
    ethereum_key: &EthPrivateKey,
    gravity_contract_address: &EthAddress,
    web3: &Web3,
    latest_cosmos_valset: Valset,
    current_valset: Valset,
) {
    let our_address = ethereum_key.to_address();
    let current_valset_from_eth =
        get_valset_nonce(*gravity_contract_address, our_address, web3).await;
    if let Ok(current_valset_from_eth) = current_valset_from_eth {
        error!(
            "Valset cost estimate for Nonce {} failed with {:?}, current valset {} / {}",
            latest_cosmos_valset.nonce, cost, current_valset.nonce, current_valset_from_eth
        );
    }
}
