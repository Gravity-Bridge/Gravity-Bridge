//! This is a test for the Ethereum blacklist, which prevents specific addresses from depositing to or withdrawing from the bridge

use crate::airdrop_proposal::wait_for_proposals_to_execute;
use crate::utils::{
    create_gravity_params_proposal, vote_yes_on_proposals, GravityProposalParams, ValidatorKeys,
};
use crate::{get_deposit, get_fee};
use cosmos_gravity::query::get_gravity_params;
use deep_space::Contact;
use gravity_proto::gravity::v1::query_client::QueryClient as GravityQueryClient;
use tonic::transport::Channel;

pub async fn ethereum_blacklist_test(
    grpc_client: GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
) {
    let mut grpc_client = grpc_client;

    let blocked_addresses: Vec<String> =
        vec!["0x21479eB8CB1a27861c902F07A952b72b10Fd53EF".to_string()];

    // next we create a governance proposal to use the newly bridged asset as the reward
    // and vote to pass the proposal
    info!("Creating parameter change governance proposal");
    create_gravity_params_proposal(
        contact,
        keys[0].validator_key,
        get_deposit(None),
        get_fee(None),
        GravityProposalParams {
            ethereum_blacklist: Some(blocked_addresses.clone()),
            ..Default::default()
        },
    )
    .await;

    vote_yes_on_proposals(contact, &keys, None).await;

    // wait for the voting period to pass
    wait_for_proposals_to_execute(contact).await;

    let params = get_gravity_params(&mut grpc_client).await.unwrap();
    // check that params have changed
    assert_eq!(params.ethereum_blacklist, blocked_addresses);

    info!("Successfully modified the blacklist!");
}
