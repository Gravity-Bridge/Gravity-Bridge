//! This is a test for inflation param changes governance handler

use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::get_fee;
use crate::utils::{create_parameter_change_proposal, vote_yes_on_proposals, ValidatorKeys};
use deep_space::Contact;
use gravity_proto::cosmos_sdk_proto::cosmos::params::v1beta1::ParamChange;

async fn get_mint_param_as_float(param: &str, contact: &Contact) -> f32 {
    let param = contact.get_param("mint", param).await.unwrap();
    param
        .param
        .unwrap()
        .value
        .replace(['\\', '"'], "")
        .parse()
        .unwrap()
}

/// Produce and send a number of valid and invalid airdrops in order to demonstrate
/// correct behavior.
pub async fn inflation_knockdown_test(contact: &Contact, keys: Vec<ValidatorKeys>) {
    info!("Submitting and passing a proposal to zero out inflation");

    let mut params_to_change = Vec::new();
    let change = ParamChange {
        subspace: "mint".to_string(),
        key: "InflationRateChange".to_string(),
        value: r#""1""#.to_string(),
    };
    params_to_change.push(change);
    let change = ParamChange {
        subspace: "mint".to_string(),
        key: "InflationMin".to_string(),
        value: r#""0.01""#.to_string(),
    };
    params_to_change.push(change);
    let change = ParamChange {
        subspace: "mint".to_string(),
        key: "InflationMax".to_string(),
        value: r#""0.01""#.to_string(),
    };
    params_to_change.push(change);
    create_parameter_change_proposal(
        contact,
        keys[0].validator_key,
        params_to_change,
        get_fee(None),
    )
    .await;
    vote_yes_on_proposals(contact, &keys, None).await;
    wait_for_proposals_to_execute(contact).await;

    assert_eq!(
        get_mint_param_as_float("InflationRateChange", contact).await,
        1.0
    );
    assert_eq!(get_mint_param_as_float("InflationMin", contact).await, 0.01);
    assert_eq!(get_mint_param_as_float("InflationMax", contact).await, 0.01);

    let mut params_to_change = Vec::new();
    let change = ParamChange {
        subspace: "mint".to_string(),
        key: "InflationMax".to_string(),
        value: r#""0.07""#.to_string(),
    };
    params_to_change.push(change);
    create_parameter_change_proposal(
        contact,
        keys[0].validator_key,
        params_to_change,
        get_fee(None),
    )
    .await;
    vote_yes_on_proposals(contact, &keys, None).await;
    wait_for_proposals_to_execute(contact).await;

    assert_eq!(get_mint_param_as_float("InflationMax", contact).await, 0.07);

    info!("Successfully passed inflation knockdown test!")
}
