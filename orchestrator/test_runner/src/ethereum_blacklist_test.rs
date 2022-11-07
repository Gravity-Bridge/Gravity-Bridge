//! This is a test for the Ethereum blacklist, which prevents specific addresses from depositing to or withdrawing from the bridge

use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::utils::{create_parameter_change_proposal, vote_yes_on_proposals, ValidatorKeys};
use crate::{get_fee, get_validator_private_keys};
use cosmos_gravity::query::get_gravity_params;
use deep_space::Contact;
use gravity_proto::cosmos_sdk_proto::cosmos::params::v1beta1::ParamChange;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use tonic::transport::Channel;

pub async fn ethereum_blacklist_test(
    grpc_client: GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
) {
    let val_priv_keys = get_validator_private_keys(&keys);
    let mut grpc_client = grpc_client;

    let blocked_addresses: Vec<String> =
        vec!["0x21479eB8CB1a27861c902F07A952b72b10Fd53EF".to_string()];
    let json_value = serde_json::to_string(&blocked_addresses).unwrap();

    let mut params_to_change = Vec::new();
    let blocked_address_param = ParamChange {
        subspace: "gravity".to_string(),
        key: "EthereumBlacklist".to_string(),
        value: json_value,
    };
    params_to_change.push(blocked_address_param);

    // next we create a governance proposal to use the newly bridged asset as the reward
    // and vote to pass the proposal
    info!("Creating parameter change governance proposal");
    create_parameter_change_proposal(
        contact,
        keys[0].validator_key,
        params_to_change,
        get_fee(None),
    )
    .await;

    vote_yes_on_proposals(contact, &val_priv_keys, None).await;

    // wait for the voting period to pass
    wait_for_proposals_to_execute(contact).await;

    let params = get_gravity_params(&mut grpc_client).await.unwrap();
    // check that params have changed
    assert_eq!(params.ethereum_blacklist, blocked_addresses);

    info!("Successfully modified the blacklist!");
}
