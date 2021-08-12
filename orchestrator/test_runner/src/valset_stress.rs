use crate::happy_path::test_valset_update;
use crate::utils::create_default_test_config;
use crate::utils::start_orchestrators;
use crate::utils::ValidatorKeys;
use clarity::Address as EthAddress;
use deep_space::Contact;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use tonic::transport::Channel;
use web30::client::Web3;

pub async fn validator_set_stress_test(
    web30: &Web3,
    grpc_client: GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
) {
    let mut grpc_client = grpc_client.clone();
    let no_relay_market_config = create_default_test_config();
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;

    for _ in 0u32..10 {
        test_valset_update(web30, contact, &mut grpc_client, &keys, gravity_address).await;
    }
}
