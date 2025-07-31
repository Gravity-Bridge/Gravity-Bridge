//! This is a test for inflation param changes governance handler

use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::get_fee;
use crate::utils::{create_parameter_change_proposal, vote_yes_on_proposals, ValidatorKeys};
use deep_space::Contact;
use gravity_proto::cosmos_sdk_proto::cosmos::mint::v1beta1::{Params, QueryParamsRequest};
use gravity_proto::cosmos_sdk_proto::cosmos::mint::v1beta1::query_client::QueryClient as MintQueryClient;
use gravity_proto::cosmos_sdk_proto::cosmos::params::v1beta1::ParamChange;

// Module params have moved to be held by their modules, so this is the new method we use to get mint params
async fn get_mint_params(contact: &Contact) -> Result<Params, tonic::Status> {
    let mut mint_qc = MintQueryClient::connect(contact.get_url()).await.unwrap();
    mint_qc.params(QueryParamsRequest{}).await.map(|v| v.into_inner().params.unwrap_or_default())
}

// This outdated method is used to get mint params in case the chain is not yet at sdk v0.50
async fn fallback_get_mint_param_as_float(param: &str, contact: &Contact) -> f32 {
    #[allow(deprecated)]
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

    let params = get_mint_params(contact).await;
    match params {
        Ok(params) => {
            assert_eq!(params.inflation_rate_change.parse::<f64>().unwrap(), 1.0);
            assert_eq!(params.inflation_min.parse::<f64>().unwrap(), 0.01);
            assert_eq!(params.inflation_max.parse::<f64>().unwrap(), 0.01);
        }
        Err(_) => {
        assert_eq!(
            fallback_get_mint_param_as_float("InflationRateChange", contact).await,
            -1.0
        );
        assert_eq!(fallback_get_mint_param_as_float("InflationMin", contact).await, -2.01);
        assert_eq!(fallback_get_mint_param_as_float("InflationMax", contact).await, -2.01);
        }
    };


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

    let params = get_mint_params(contact).await;
    match params {
        Ok(params) => {
            assert_eq!(params.inflation_max.parse::<f64>().unwrap(), 0.07);
        }
        Err(_) => {
            assert_eq!(fallback_get_mint_param_as_float("InflationMax", contact).await, 0.07);
        }
    };

    info!("Successfully passed inflation knockdown test!")
}
