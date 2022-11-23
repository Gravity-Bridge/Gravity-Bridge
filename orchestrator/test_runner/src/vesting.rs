use crate::{get_user_key, ValidatorKeys, ADDRESS_PREFIX, COSMOS_NODE_GRPC, STAKING_TOKEN};
use deep_space::client::types::AccountType;
use deep_space::{Contact, Msg, PrivateKey};
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::query_client::QueryClient as BankQueryClient;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::QuerySpendableBalancesRequest;
use gravity_proto::cosmos_sdk_proto::cosmos::base::v1beta1::Coin;
use gravity_proto::orbit::MsgCreateVestingAccount;
use gravity_utils::num_conversion::one_atom;
use std::ops::{Add, Mul};
use std::time::{Duration, SystemTime, UNIX_EPOCH};
use tokio::time::sleep;

// The Orbit module Msg TypeURL which is used to create a delayed continuous vesting account
const ORBIT_MSG_CREATE_VESTING_ACCOUNT_TYPE_URL: &str = "/orbit.v1.MsgCreateVestingAccount";

/// Tests that vesting accounts receive the funds they should by querying their Spendable Balances
pub async fn vesting_test(contact: &Contact, keys: Vec<ValidatorKeys>) {
    let now = SystemTime::now();
    let vesting_delay = Duration::from_secs(60 * 5);
    let vesting_duration = Duration::from_secs(60);
    let start_vesting = now.add(vesting_delay);
    let end_vesting = start_vesting.add(vesting_duration);
    let start_time = start_vesting.duration_since(UNIX_EPOCH).unwrap().as_secs() as i64;
    let end_time = end_vesting.duration_since(UNIX_EPOCH).unwrap().as_secs() as i64;

    let amount = one_atom().mul(1000u32.into());
    let vest_amt = Coin {
        denom: (*STAKING_TOKEN).clone(),
        amount: amount.to_string(),
    };
    info!(
        "Expecting to see a final vested amount {} of stake",
        amount.to_string()
    );

    let user = get_user_key(None);
    let to_addr = user.cosmos_address;
    let msg = MsgCreateVestingAccount {
        from_address: keys[0]
            .validator_key
            .to_address(&ADDRESS_PREFIX)
            .unwrap()
            .to_string(),
        to_address: to_addr.to_string(),
        amount: vec![vest_amt.clone()],
        end_time,
        start_time,
    };
    let msg = Msg::new(ORBIT_MSG_CREATE_VESTING_ACCOUNT_TYPE_URL, msg);

    info!(
        "Submitting MsgCreateVestingAccount for account {}",
        to_addr.to_string()
    );
    let res = contact
        .send_message(
            &[msg],
            None,
            &[],
            Some(Duration::from_secs(30)),
            keys[0].validator_key,
        )
        .await;
    info!("Sent create vesting account tx to Orbit: {:?}", res);
    let acct = contact
        .get_account_vesting_info(to_addr)
        .await
        .expect("Unable to fetch newly created account vesting info");

    if let AccountType::ContinuousVestingAccount(cva) = acct {
        assert_eq!(cva.start_time, start_time);
        assert!(cva.base_vesting_account.is_some());
        let bva = cva.base_vesting_account.unwrap();
        assert_eq!(bva.end_time, end_time);
        assert_eq!(bva.original_vesting, vec![vest_amt.clone()]);
        assert_eq!(bva.delegated_vesting, vec![]);
        assert_eq!(bva.delegated_free, vec![]);
        assert!(bva.base_account.is_some());
        let base = bva.base_account.unwrap();
        assert_eq!(base.address, to_addr.to_string());
    } else {
        panic!(
            "Account created is not a ContinuousVestingAccount! {:?}",
            acct
        );
    }
    let mut bank_qc = BankQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Unable to connect to Bank Client");

    let spendable = bank_qc
        .spendable_balances(QuerySpendableBalancesRequest {
            address: to_addr.to_string(),
            pagination: None,
        })
        .await
        .expect("Unable to query newly created vesting account spendable balances")
        .into_inner();
    let zero_stake = Coin {
        denom: (*STAKING_TOKEN).clone(),
        amount: "0".to_string(),
    };
    assert_eq!(spendable.balances, vec![zero_stake.clone()]);
    info!(
        "Vesting account successfully created, these are its spendable balances: {:?}",
        spendable.balances
    );

    info!("Wait for the vesting to start");
    sleep(vesting_delay.add(Duration::from_secs(20))).await;

    let spendable = bank_qc
        .spendable_balances(QuerySpendableBalancesRequest {
            address: to_addr.to_string(),
            pagination: None,
        })
        .await
        .expect("Unable to query newly created vesting account spendable balances")
        .into_inner();
    if spendable.balances.is_empty() {
        panic!("Discovered empty balance after vesting start!");
    } else {
        assert_eq!(spendable.balances.len(), 1);
        let spend_bal = spendable.balances[0].clone();
        info!(
            "Discovered acceptable spendable balance after vesting start {}",
            spend_bal.amount
        );
        assert!(
            spend_bal
                .amount
                .parse::<i64>()
                .expect("spendable balance is not an integer!")
                > 0
        );
        assert_eq!(spend_bal.denom, (*STAKING_TOKEN).clone());
    }

    info!("Wait for the vesting to complete");
    sleep(vesting_duration.add(Duration::from_secs(20))).await;

    let spendable = bank_qc
        .spendable_balances(QuerySpendableBalancesRequest {
            address: to_addr.to_string(),
            pagination: None,
        })
        .await
        .expect("Unable to query newly created vesting account spendable balances")
        .into_inner();
    if spendable.balances.is_empty() {
        panic!("Discovered empty balance after vesting end!");
    } else {
        assert_eq!(spendable.balances.len(), 1);
        let spend_bal = spendable.balances[0].clone();
        info!(
            "Discovered spendable balance after vesting end {}",
            spend_bal.amount
        );
        assert_eq!(spend_bal.amount, vest_amt.amount);
        assert_eq!(spend_bal.denom, (*STAKING_TOKEN).clone());
    }
    info!("Successfully tested vesting timelines, user's vested balance is as expected!");
}
