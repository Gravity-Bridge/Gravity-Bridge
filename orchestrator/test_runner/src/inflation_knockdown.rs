//! This is a test for inflation param changes governance handler

use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::utils::{
    create_mint_params_proposal, vote_yes_on_proposals, MintProposalParams, ValidatorKeys,
};
use crate::{get_deposit, get_fee};
use deep_space::Contact;
use gravity_proto::cosmos_sdk_proto::cosmos::mint::v1beta1::query_client::QueryClient as MintQueryClient;
use gravity_proto::cosmos_sdk_proto::cosmos::mint::v1beta1::{Params, QueryParamsRequest};

// Module params have moved to be held by their modules, so this is the new method we use to get mint params
async fn get_mint_params(contact: &Contact) -> Result<Params, tonic::Status> {
    let mut mint_qc = MintQueryClient::connect(contact.get_url()).await.unwrap();
    mint_qc
        .params(QueryParamsRequest {})
        .await
        .map(|v| v.into_inner().params.unwrap_or_default())
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

    create_mint_params_proposal(
        contact,
        keys[0].validator_key,
        get_deposit(None),
        get_fee(None),
        MintProposalParams {
            inflation_max: Some("0.01".to_string()),
            inflation_min: Some("0.01".to_string()),
            inflation_rate_change: Some("1".to_string()),
            ..Default::default()
        },
    )
    .await;
    vote_yes_on_proposals(contact, &keys, None).await;
    wait_for_proposals_to_execute(contact).await;

    let params = get_mint_params(contact).await;
    info!("Mint Params query result: {params:?}");
    match params {
        Ok(params) => {
            assert_eq!(
                params.inflation_rate_change.parse::<f64>().unwrap() / 10f64.powi(18),
                1.0
            );
            assert_eq!(
                params.inflation_min.parse::<f64>().unwrap() / 10f64.powi(18),
                0.01
            );
            assert_eq!(
                params.inflation_max.parse::<f64>().unwrap() / 10f64.powi(18),
                0.01
            );
        }
        Err(_) => {
            assert_eq!(
                fallback_get_mint_param_as_float("InflationRateChange", contact).await,
                -1.0
            );
            assert_eq!(
                fallback_get_mint_param_as_float("InflationMin", contact).await,
                -2.01
            );
            assert_eq!(
                fallback_get_mint_param_as_float("InflationMax", contact).await,
                -2.01
            );
        }
    };

    create_mint_params_proposal(
        contact,
        keys[0].validator_key,
        get_deposit(None),
        get_fee(None),
        MintProposalParams {
            inflation_max: Some("0.07".to_string()),
            ..Default::default()
        },
    )
    .await;
    vote_yes_on_proposals(contact, &keys, None).await;
    wait_for_proposals_to_execute(contact).await;

    let params = get_mint_params(contact).await;
    match params {
        Ok(params) => {
            assert_eq!(
                params.inflation_max.parse::<f64>().unwrap() / 10f64.powi(18),
                0.07
            );
        }
        Err(_) => {
            assert_eq!(
                fallback_get_mint_param_as_float("InflationMax", contact).await,
                0.07
            );
        }
    };

    info!("Successfully passed inflation knockdown test!")
}
