use crate::utils::create_default_test_config;
use crate::utils::start_orchestrators;
use crate::utils::ValidatorKeys;
use clarity::Address as EthAddress;

pub async fn orch_only_test(keys: Vec<ValidatorKeys>, gravity_address: EthAddress) {
    let no_relay_market_config = create_default_test_config();
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;
}
