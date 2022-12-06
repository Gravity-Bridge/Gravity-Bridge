use crate::{CosmosAddress, ADDRESS_PREFIX, COSMOS_NODE_GRPC, STAKING_TOKEN};
use cosmos_gravity::utils::{get_current_cosmos_height, historical_grpc_query};
use deep_space::client::types::AccountType;
use deep_space::error::CosmosGrpcError;
use deep_space::{Contact, CosmosPrivateKey, PrivateKey};
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::query_client::QueryClient as BankQueryClient;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::QuerySpendableBalancesRequest;
use gravity_proto::cosmos_sdk_proto::cosmos::base::v1beta1::Coin;
use gravity_utils::num_conversion::one_atom;
use std::ops::Mul;
use std::time::{Duration, SystemTime, UNIX_EPOCH};
use tokio::time::sleep;
use tonic::transport::Channel;

/// Tests that vesting accounts receive the funds they should by querying their Spendable Balances
pub async fn vesting_test(contact: &Contact, vesting_keys: Vec<CosmosPrivateKey>) {
    let vesting_addrs: Vec<CosmosAddress> = vesting_keys
        .into_iter()
        .map(|k| k.to_address(&ADDRESS_PREFIX).expect("Invalid vesting key!"))
        .collect();
    let bank_qc = BankQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Unable to connect to Bank Client");

    let expected_amount = one_atom().mul(1000u32.into());
    let expected_vest_amt = Coin {
        denom: (*STAKING_TOKEN).clone(),
        amount: expected_amount.to_string(),
    };
    info!(
        "Expecting to see a final vested amount {} of stake",
        expected_amount.to_string()
    );

    // Check that starting vested balance is 0

    let zero_stake = Coin {
        denom: (*STAKING_TOKEN).clone(),
        amount: "0".to_string(),
    };
    let vesting_times = check_vesting_accounts_at_height(
        contact,
        &bank_qc,
        &vesting_addrs,
        1u64,
        SpendableRange {
            lo: zero_stake.clone(),
            hi: zero_stake.clone(),
        },
        expected_vest_amt.clone(),
    )
    .await
    .expect("Unable to check vesting accounts at starting height!");
    let (start, end) = vesting_times[0];

    // Check that in-progress vesting gives a partial vested amount
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("Invalid current time?");
    let until_start = start - now.as_secs() as i64;
    if until_start > 0 {
        info!("Wait for the vesting to start");
        sleep(Duration::from_secs(until_start as u64 + 30)).await;
    } else {
        info!("Vesting has already begun");
    }
    let curr_height = get_current_cosmos_height(contact)
        .await
        .expect("Couldn't get current cosmos height");
    let mut lo_in_progress = zero_stake.clone();
    lo_in_progress.amount = "1".to_string();
    let mut hi_in_progress = zero_stake.clone();
    hi_in_progress.amount = (expected_amount - 1u8.into()).to_string();
    let _ = check_vesting_accounts_at_height(
        contact,
        &bank_qc,
        &vesting_addrs,
        curr_height,
        SpendableRange {
            lo: lo_in_progress.clone(),
            hi: hi_in_progress.clone(),
        },
        expected_vest_amt.clone(),
    )
    .await
    .expect("Unable to check in-progress vesting accounts at starting height!");

    // Check that the users have a fully vested balance at the end of vesting
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("Invalid current time?");
    let until_end = end - now.as_secs() as i64;
    if until_end > 0 {
        info!("Wait for the vesting to complete");
        sleep(Duration::from_secs(until_end as u64 + 30)).await;
    } else {
        info!("Vesting has already completed");
    }
    let curr_height = get_current_cosmos_height(contact)
        .await
        .expect("Couldn't get current cosmos height");
    let mut lo_end = zero_stake.clone();
    lo_end.amount = expected_amount.to_string();
    let mut hi_end = zero_stake.clone();
    hi_end.amount = expected_amount.to_string();
    let _ = check_vesting_accounts_at_height(
        contact,
        &bank_qc,
        &vesting_addrs,
        curr_height,
        SpendableRange {
            lo: lo_end.clone(),
            hi: hi_end.clone(),
        },
        expected_vest_amt.clone(),
    )
    .await
    .expect("Unable to check in-progress vesting accounts at starting height!");

    // SUCCESS
    info!("Successfully tested vesting timelines, user's vested balance is as expected!");
}

pub struct SpendableRange {
    pub lo: Coin,
    pub hi: Coin,
}

pub async fn check_vesting_accounts_at_height(
    contact: &Contact,
    bank_qc: &BankQueryClient<Channel>,
    addresses: &Vec<CosmosAddress>,
    query_height: u64,
    spendable_range: SpendableRange,
    original_vesting: Coin,
) -> Result<Vec<(i64, i64)>, CosmosGrpcError> {
    let mut bank_qc = bank_qc.clone();
    let mut vesting_times = vec![];
    // Check the starting spendable balance was
    let lo_amount: u64 = spendable_range
        .lo
        .amount
        .parse()
        .expect("Invalid lo amount in spendable range");
    let hi_amount: u64 = spendable_range
        .hi
        .amount
        .parse()
        .expect("Invalid hi amount in spendable range");
    for a in addresses {
        let addr = a.to_string();
        let acct = contact
            .get_account_vesting_info(*a)
            .await
            .expect("Unable to fetch newly created account vesting info");

        if let AccountType::ContinuousVestingAccount(cva) = acct {
            assert!(cva.base_vesting_account.is_some());
            let bva = cva.base_vesting_account.unwrap();
            vesting_times.push((cva.start_time, bva.end_time));
            assert_eq!(bva.original_vesting, vec![original_vesting.clone()]);
            assert!(bva.base_account.is_some());
            let base = bva.base_account.unwrap();
            assert_eq!(base.address, addr.to_string());
        } else {
            panic!("Account is not a ContinuousVestingAccount! {:?}", acct);
        }

        let req = QuerySpendableBalancesRequest {
            address: addr.clone(),
            pagination: None,
        };
        // Query at the first block to see what the starting balance was
        let spendable = bank_qc
            .spendable_balances(historical_grpc_query(req, query_height))
            .await
            .unwrap_or_else(|_| {
                panic!(
                    "Unable to query vesting account {}'s spendable balances",
                    addr
                )
            })
            .into_inner()
            .balances;
        assert_eq!(spendable.len(), 1);
        let spendable = spendable[0].clone();
        assert_eq!(spendable.denom, spendable_range.lo.denom);
        assert_eq!(spendable.denom, spendable_range.hi.denom);
        let spendable_amt: u64 = spendable
            .amount
            .parse()
            .expect("Invalid spendable amount returned from bank query!");
        info!("Discovered spendable balance of {}, expecting that to be less than {} but more than {}", spendable_amt, hi_amount, lo_amount);
        assert!(lo_amount <= spendable_amt && spendable_amt <= hi_amount);

        debug!(
            "Vesting account {} discovered, these are its spendable balances: {:?}",
            addr, spendable
        );
    }
    Ok(vesting_times)
}
