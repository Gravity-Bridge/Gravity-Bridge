use crate::happy_path::send_erc20_deposit;
use crate::happy_path_v2::send_to_eth_and_confirm;
use crate::ibc_auto_forward::{get_channel_id, test_ibc_transfer};
use crate::{
    create_default_test_config, create_parameter_change_proposal, delegate_and_confirm,
    get_ethermint_key, get_fee, get_ibc_chain_id, one_eth, send_eth_bulk, start_orchestrators,
    wait_for_balance, EthAddress, EthermintUserKey, GravityQueryClient, ValidatorKeys,
    ADDRESS_PREFIX, COSMOS_NODE_GRPC, IBC_ADDRESS_PREFIX, IBC_NODE_GRPC, OPERATION_TIMEOUT,
    STAKING_TOKEN, TOTAL_TIMEOUT,
};
use deep_space::{Coin, Contact, CosmosPrivateKey, PrivateKey};
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::query_client::QueryClient as BankQueryClient;
use gravity_proto::cosmos_sdk_proto::cosmos::base::v1beta1::Coin as ProtoCoin;
use gravity_proto::cosmos_sdk_proto::cosmos::params::v1beta1::ParamChange;
use gravity_proto::cosmos_sdk_proto::ibc::applications::transfer::v1::query_client::QueryClient as IbcTransferQueryClient;
use gravity_proto::cosmos_sdk_proto::ibc::core::channel::v1::query_client::QueryClient as IbcChannelQueryClient;
use num256::Uint256;
use std::time::Duration;
use tonic::transport::Channel;
use web30::client::Web3;

// Tests creation and example usage of an EthermintPrivateKey on gravity bridge chain
// returns a boolean indicating success (true) or failure (false)
pub async fn ethereum_keys_test(
    web30: &Web3,
    gravity_client: GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    ibc_keys: Vec<CosmosPrivateKey>,
    gravity_address: EthAddress,
    erc20_address: EthAddress,
) -> bool {
    let no_relay_market_config = create_default_test_config();
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;

    // Generate an Ethermint user account
    let ethermint_user = get_ethermint_key(None);
    setup_ethermint_test(
        contact,
        web30,
        gravity_client,
        gravity_address,
        keys.clone(),
        ethermint_user,
        erc20_address,
    )
    .await;

    // Test the use of the ethermint key
    example_ethermint_key_usage(
        contact,
        web30,
        keys,
        ibc_keys,
        ethermint_user,
        erc20_address,
    )
    .await
}

// Gives the account under ethermint_key some STAKE and erc20_address
pub async fn setup_ethermint_test(
    contact: &Contact,
    web30: &Web3,
    gravity_grpc: GravityQueryClient<Channel>,
    gravity_address: EthAddress,
    validator_keys: Vec<ValidatorKeys>,
    ethermint_key: EthermintUserKey,
    erc20_address: EthAddress,
) {
    let mut gravity_grpc = gravity_grpc.clone();
    let user_address = ethermint_key.ethermint_address;
    let user_eth_address = ethermint_key.eth_address;

    // Send the user account some coins
    let send_amount: Uint256 = 2000000000u32.into();
    let denom: String = STAKING_TOKEN.clone().to_string();
    let output = contact
        .send_coins(
            Coin {
                amount: send_amount,
                denom: denom.clone(),
            },
            None,
            user_address,
            Some(OPERATION_TIMEOUT),
            validator_keys[0].validator_key,
        )
        .await;
    info!("Output is {:?}", output);
    let balance = contact
        .get_balance(user_address, denom.clone())
        .await
        .expect("Could not get cosmos erc20 balance");
    assert_eq!(
        balance,
        Some(Coin {
            denom,
            amount: send_amount
        })
    );
    info!("User {} has {}", user_address, balance.unwrap());

    // Send the user a bit of eth for future queries
    send_eth_bulk(one_eth(), &[user_eth_address], web30).await;

    let erc20_denom = "gravity".to_string() + &erc20_address.to_string();
    let send_amount: Uint256 = one_eth() * 10u8.into();
    send_erc20_deposit(
        web30,
        &mut gravity_grpc,
        user_address,
        gravity_address,
        erc20_address,
        send_amount,
    )
    .await
    .unwrap();
    let expected_balance = Coin {
        denom: erc20_denom,
        amount: send_amount,
    };
    wait_for_balance(contact, user_address, expected_balance, None).await;
}

// Tests example usage of an EthermintPrivateKey on gravity bridge chain
// returns a boolean indicating success (true) or failure (false)
pub async fn example_ethermint_key_usage(
    contact: &Contact,
    web30: &Web3,
    validator_keys: Vec<ValidatorKeys>,
    ibc_keys: Vec<CosmosPrivateKey>,
    ethermint_key: EthermintUserKey,
    erc20_address: EthAddress,
) -> bool {
    let user_key = ethermint_key.ethermint_key;
    let user_cosmos_address = ethermint_key.ethermint_address;
    let user_eth_address = ethermint_key.eth_address;
    let denom: String = STAKING_TOKEN.clone().to_string();
    let erc20_denom = "gravity".to_string() + &erc20_address.to_string();
    // BANK Module
    // Send some tokens out of the Ethermint account
    let send_amount: Uint256 = 1u8.into();
    let start_balance = contact
        .get_balance(user_cosmos_address, denom.clone())
        .await
        .unwrap()
        .expect("No starting balance!");
    let user_send = contact
        .send_coins(
            Coin {
                amount: send_amount,
                denom: denom.clone(),
            },
            None,
            validator_keys[0]
                .validator_key
                .to_address(&ADDRESS_PREFIX)
                .unwrap(),
            Some(OPERATION_TIMEOUT),
            user_key,
        )
        .await;
    debug!("user_send is {:?}", user_send);

    let expected_balance = start_balance.amount - send_amount;
    let balance = contact
        .get_balance(user_cosmos_address, denom.clone())
        .await
        .unwrap();
    info!(
        "User {} start balance {} and end balance {:?}",
        user_cosmos_address, start_balance, balance
    );
    let success = balance
        == Some(deep_space::Coin {
            amount: expected_balance,
            denom: denom.clone(),
        });
    match success {
        true => info!(
            "Successfully used bank module with ethermint account {}, sent {} {}",
            user_cosmos_address.to_string(),
            send_amount,
            denom,
        ),
        false => {
            error!("Failed Ethereum Keys test!");
            return false;
        }
    };

    // --^v^v^v^v^v^-- STAKING Module --^v^v^v^v^v^--
    // Delegate 50% of our tokens to a validator
    let delegate_amt = 1000u32.into();
    let delegate_amount = Coin {
        denom: denom.clone(),
        amount: delegate_amt,
    };
    let valoper_prefix = ADDRESS_PREFIX.to_string() + "valoper";
    let delegate_to = validator_keys[0]
        .validator_key
        .to_address(&valoper_prefix)
        .unwrap();
    let delegation_res = delegate_and_confirm(
        contact,
        user_key,
        user_cosmos_address,
        delegate_to,
        delegate_amount.clone(),
        get_fee(Some(denom.clone())),
    )
    .await
    .unwrap();
    if delegation_res.is_none() {
        error!("Could not confirm delegation!");
        return false;
    }
    info!(
        "Successfully used staking module with ethermint account {}, delegated {} to {}",
        user_cosmos_address, delegate_amount, delegate_to,
    );

    // --^v^v^v^v^v^-- GRAVITY Module --^v^v^v^v^v^--
    // Send to Eth
    let send_to_eth_amount = 1_000u32.into();
    let send_to_eth_coin = Coin {
        denom: erc20_denom.clone(),
        amount: send_to_eth_amount,
    };
    let success = send_to_eth_and_confirm(
        web30,
        contact,
        user_key,
        user_eth_address,
        send_to_eth_coin.clone(),
        get_fee(Some(send_to_eth_coin.denom.clone())),
        None, // Sending so little that the minimum chain fee does not apply
        get_fee(Some(send_to_eth_coin.denom.clone())),
        erc20_address,
    )
    .await;
    if !success {
        error!("Could not send to eth!");
        return false;
    }

    // --^v^v^v^v^v^-- IBC Module --^v^v^v^v^v^--
    // Send to Ibc user
    let gravity_channel_qc = IbcChannelQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect channel query client");
    let ibc_bank_qc = BankQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect bank query client");
    let ibc_transfer_qc = IbcTransferQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Could not connect ibc-transfer query client");

    // Wait for the ibc channel to be created and find the channel ids
    let channel_id_timeout = Duration::from_secs(60);
    let gravity_channel_id = get_channel_id(
        gravity_channel_qc,
        get_ibc_chain_id(),
        Some(channel_id_timeout),
    )
    .await
    .expect("Could not find gravity-test-1 channel");

    // Test an IBC transfer of 1 stake from gravity-test-1 to ibc-test-1
    let receiver = ibc_keys[0].to_address(&IBC_ADDRESS_PREFIX).unwrap();
    let success = test_ibc_transfer(
        contact,
        ibc_bank_qc.clone(),
        ibc_transfer_qc.clone(),
        user_key,
        receiver,
        Some(ProtoCoin {
            denom: denom.clone(),
            amount: "1".to_string(),
        }),
        Some(Coin {
            denom: denom.clone(),
            amount: 1u8.into(),
        }),
        gravity_channel_id.clone(),
        Duration::from_secs(60 * 3),
    )
    .await;
    if !success {
        info!("Failed to use ibc-transfer module");
        return false;
    }
    info!(
        "Successfully used ibc-transfer module with ethermint account {}, transferred {} to {}",
        user_cosmos_address, "1", receiver,
    );

    // --^v^v^v^v^v^-- GOV Module --^v^v^v^v^v^--
    // Start and a gov proposal from the ethermint key
    let signed_valsets_window = ParamChange {
        subspace: "gravity".to_string(),
        key: "SignedValsetsWindow".to_string(),
        value: r#""10""#.to_string(),
    };
    let pre_proposals = contact
        .get_governance_proposals_in_voting_period()
        .await
        .expect("Did not find any proposals in voting status");
    create_parameter_change_proposal(
        contact,
        user_key,
        vec![signed_valsets_window],
        get_fee(Some(denom.clone())),
    )
    .await;
    let post_proposals = contact
        .get_governance_proposals_in_voting_period()
        .await
        .expect("Did not find any proposals in voting status");
    assert_eq!(
        post_proposals.proposals.len(),
        pre_proposals.proposals.len() + 1,
        "Did not observe a new proposal creation"
    );
    info!("Successfully used gov module with ethermint account {}, created a parameter change proposal", user_cosmos_address);

    // --^v^v^v^v^v^-- DISTRIBUTION Module --^v^v^v^v^v^--
    // Claim staking rewards
    let rewards = contact
        .query_delegation_rewards(user_cosmos_address, delegate_to)
        .await
        .expect("Could not get staking rewards!");
    info!("Got delegation rewards: {:?}", rewards);
    let mut stake_rewards = None;
    for reward in &rewards {
        if reward.denom == *STAKING_TOKEN {
            stake_rewards = Some(reward.clone());
        }
    }
    let _reward = stake_rewards
        .unwrap_or_else(|| panic!("Could not find a reward in stake from {:?}", rewards));
    let stake_balance = contact
        .get_balance(user_cosmos_address, STAKING_TOKEN.to_string())
        .await
        .expect("Could not get stake balance");
    let _res = contact
        .withdraw_delegator_rewards(
            delegate_to,
            Coin {
                denom: STAKING_TOKEN.to_string(),
                amount: 0u8.into(),
            },
            user_key,
            Some(TOTAL_TIMEOUT),
        )
        .await;
    let rewarded_stake_balance = contact
        .get_balance(user_cosmos_address, STAKING_TOKEN.to_string())
        .await
        .expect("Could not get stake balance");
    match (stake_balance, rewarded_stake_balance) {
        (None, Some(rewarded)) => {
            assert!(
                rewarded.amount > 0u8.into(),
                "Unexpected reward amount {}",
                rewarded
            )
        }
        (Some(unrewarded), Some(rewarded)) => {
            let positive_reward = rewarded.amount > unrewarded.amount;
            assert!(positive_reward,
                "Unexpected negative reward amount: balance after withdrawal {}, balance before withdrawal {}",
                rewarded, unrewarded,
            );
        }
        (_, _) => {
            error!("Unexpected balance of stake before and after withdrawing staking rewards!");
            return false;
        }
    };
    info!("Successfully used distribution module with ethermint account {}, withdrew staking rewards!", user_cosmos_address);

    info!("Successfully tested example usage of an ethermint account!");
    true
}
