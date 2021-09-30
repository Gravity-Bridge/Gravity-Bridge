//! This is a test for validator set relaying rewards

use crate::happy_path::test_valset_update;
use crate::happy_path_v2::deploy_cosmos_representing_erc20_and_check_adoption;
use crate::utils::{create_parameter_change_proposal, vote_yes_on_proposals, ValidatorKeys};
use crate::STAKING_TOKEN;
use clarity::Address as EthAddress;
use cosmos_gravity::query::get_gravity_params;
use deep_space::coin::Coin;
use deep_space::Contact;
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
    validator_out: bool,
) {
    let mut grpc_client = grpc_client;
    let token_to_send_to_eth = "footoken".to_string();
    let token_to_send_to_eth_display_name = "mfootoken".to_string();

    // first we deploy the Cosmos asset that we will use as a reward and make sure it is adopted
    // by the Cosmos chain
    let erc20_contract = deploy_cosmos_representing_erc20_and_check_adoption(
        gravity_address,
        web30,
        Some(keys.clone()),
        &mut grpc_client,
        validator_out,
        token_to_send_to_eth.clone(),
        token_to_send_to_eth_display_name.clone(),
    )
    .await;

    // reward of 1 mfootoken
    let valset_reward = Coin {
        denom: token_to_send_to_eth,
        amount: 1_000_000u64.into(),
    };
    // 1000 altg deposit
    let deposit = Coin {
        denom: STAKING_TOKEN.to_string(),
        amount: 1_000_000_000u64.into(),
    };
    // next we create a governance proposal to use the newly bridged asset as the reward
    // and vote to pass the proposal
    info!("Creating parameter change governance proposal");
    create_parameter_change_proposal(
        contact,
        keys[0].validator_key,
        deposit,
        gravity_address,
        valset_reward.clone(),
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
        let target_address = key.eth_key.to_public_key().unwrap();
        let balance_of_footoken = web30
            .get_erc20_balance(erc20_contract, target_address)
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
