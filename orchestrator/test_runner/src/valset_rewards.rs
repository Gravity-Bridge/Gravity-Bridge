//! This is a test for validator set relaying rewards

use crate::get_deposit;
use crate::happy_path::test_valset_update;
use crate::happy_path_v2::deploy_cosmos_representing_erc20_and_check_adoption;
use crate::utils::{
    create_parameter_change_proposal, footoken_metadata, get_erc20_balance_safe,
    vote_yes_on_proposals, ValidatorKeys,
};
use clarity::Address as EthAddress;
use cosmos_gravity::query::get_gravity_params;
use deep_space::coin::Coin;
use deep_space::Contact;
use gravity_proto::cosmos_sdk_proto::cosmos::params::v1beta1::ParamChange;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use std::time::Duration;
use tokio::time::sleep;
use tonic::transport::Channel;
use web30::client::Web3;

pub async fn valset_rewards_test(
    web30: &Web3,
    grpc_client: GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
) {
    let mut grpc_client = grpc_client;
    let token_to_send_to_eth = footoken_metadata().denom;

    // first we deploy the Cosmos asset that we will use as a reward and make sure it is adopted
    // by the Cosmos chain
    let erc20_contract = deploy_cosmos_representing_erc20_and_check_adoption(
        gravity_address,
        web30,
        Some(keys.clone()),
        &mut grpc_client,
        false,
        footoken_metadata(),
    )
    .await;

    // reward of 1 mfootoken
    let valset_reward = Coin {
        denom: token_to_send_to_eth,
        amount: 1_000_000u64.into(),
    };

    let mut params_to_change = Vec::new();
    let gravity_address_param = ParamChange {
        subspace: "gravity".to_string(),
        key: "BridgeContractAddress".to_string(),
        value: format!("\"{}\"", gravity_address),
    };
    params_to_change.push(gravity_address_param);
    let json_value = serde_json::to_string(&valset_reward).unwrap().to_string();
    let valset_reward_param = ParamChange {
        subspace: "gravity".to_string(),
        key: "ValsetReward".to_string(),
        value: json_value.clone(),
    };
    params_to_change.push(valset_reward_param);
    let chain_id = ParamChange {
        subspace: "gravity".to_string(),
        key: "BridgeChainID".to_string(),
        value: format!("\"{}\"", 1),
    };
    params_to_change.push(chain_id);

    // next we create a governance proposal to use the newly bridged asset as the reward
    // and vote to pass the proposal
    info!("Creating parameter change governance proposal");
    create_parameter_change_proposal(
        contact,
        keys[0].validator_key,
        get_deposit(),
        params_to_change,
    )
    .await;

    vote_yes_on_proposals(contact, &keys, None).await;

    // wait for the voting period to pass
    sleep(Duration::from_secs(65)).await;

    let params = get_gravity_params(&mut grpc_client).await.unwrap();
    // check that params have changed
    assert_eq!(params.bridge_chain_id, 1);
    assert_eq!(params.bridge_ethereum_address, gravity_address.to_string());

    // trigger a valset update
    test_valset_update(web30, contact, &mut grpc_client, &keys, gravity_address).await;

    // check that one of the relayers has footoken now
    let mut found = false;
    for key in keys.iter() {
        let target_address = key.eth_key.to_address();
        let balance_of_footoken = get_erc20_balance_safe(erc20_contract, web30, target_address)
            .await
            .unwrap();
        if balance_of_footoken == valset_reward.amount {
            found = true;
        }
    }
    if !found {
        panic!("No relayer was rewarded in footoken for relaying validator set!")
    }
    info!("Successfully Issued validator set reward!");
}
