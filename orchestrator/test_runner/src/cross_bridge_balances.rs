use crate::ValidatorKeys;
use clarity::Address as EthAddress;
use deep_space::{Contact, CosmosPrivateKey};
use gravity_proto::gravity::query_client::QueryClient;
use tonic::transport::Channel;
use web30::client::Web3;

pub async fn cross_bridge_balance_test(
    _web30: &Web3,
    _grpc: QueryClient<Channel>,
    _contact: &Contact,
    _keys: Vec<ValidatorKeys>,
    _ibc_keys: Vec<CosmosPrivateKey>,
    _gravity_address: EthAddress,
    _erc20_addresses: Vec<EthAddress>,
) {
    // FIRST: Set the MonitoredTokenAddresses governance parameter, create bridge activity, run the happy_path functions (SHOULD NOT HALT)

    // SECOND: Try to mess up the balances by sending to Gravity.sol + Gravity Module (SHOULD NOT HALT)

    // THIRD: Submit a false claim with all validators where at least one ERC20 balance is lower than it should be (SHOULD HALT)
    // MANUALLY INSPECT THAT THE OTHER NODES ALSO HALTED
}
