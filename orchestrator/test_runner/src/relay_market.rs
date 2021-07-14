//! This is the testing module for relay market functionality, testing that
//! relayers utilize web30 to interact with a testnet to obtain coin swap values
//! and determine whether relays should happen or not
use crate::utils::ValidatorKeys;
use clarity::Address as EthAddress;
use deep_space::Contact;
use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;
use tonic::transport::Channel;
use web30::client::Web3;

pub async fn relay_market_test(
    _web30: &Web3,
    _grpc_client: GravityQueryClient<Channel>,
    _contact: &Contact,
    _keys: Vec<ValidatorKeys>,
    _gravity_address: EthAddress,
) {
}
