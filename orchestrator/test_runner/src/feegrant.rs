use std::vec;

use deep_space::client::type_urls::MSG_SEND_TYPE_URL;
use deep_space::utils::encode_any;
use deep_space::{Coin as DSCoin, Contact, Fee, Msg};
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::MsgSend;
use gravity_proto::cosmos_sdk_proto::cosmos::base::v1beta1::Coin;
use gravity_proto::cosmos_sdk_proto::cosmos::feegrant::v1beta1::query_client::QueryClient as FeeGrantQueryClient;
use gravity_proto::cosmos_sdk_proto::cosmos::feegrant::v1beta1::{MsgGrantAllowance, BasicAllowance, QueryAllowanceRequest};
use gravity_proto::gravity::v1::query_client::QueryClient as GravityQueryClient;
use tonic::transport::Channel;
use web30::client::Web3;

use crate::{utils::{get_user_key, ValidatorKeys}, OPERATION_TIMEOUT};

pub async fn feegrant_test(
    _web30: &Web3,
    _grpc_client: GravityQueryClient<Channel>,
    gravity_contact: &Contact,
    keys: Vec<ValidatorKeys>,
) {
    let granter = get_user_key(None);
    let grantee = get_user_key(None);

    info!("Granter address: {}, grantee address: {}", granter.cosmos_address, grantee.cosmos_address);

    // Send the granter some coins to use
    let send_res = gravity_contact.send_coins(
        DSCoin {
            denom: "ugraviton".to_string(),
            amount: 10_000000u32.into(),
        },
        Some(DSCoin {
            denom: "ugraviton".to_string(),
            amount: 100u32.into(),
        }),
        granter.cosmos_address.clone(),
        Some(OPERATION_TIMEOUT),
        keys[0].validator_key,
    ).await;
    assert!(send_res.is_ok(), "Failed to send coins to granter: {:?}", send_res.err());

    // Send the grantee 1 ugraviton to test feegrant
    let send_res = gravity_contact.send_coins(
        DSCoin {
            denom: "ugraviton".to_string(),
            amount: 10u32.into(),
        },
        None,
        grantee.cosmos_address.clone(),
        Some(OPERATION_TIMEOUT),
        granter.cosmos_key.clone(),
    ).await;
    assert!(send_res.is_ok(), "Failed to send coins to grantee: {:?}", send_res.err());

    let allowance = BasicAllowance {
        spend_limit: vec![Coin {
            denom: "ugraviton".to_string(),
            amount: "1000000".to_string(),
        }],
        expiration: None,
    };
    let allowance_any = encode_any(allowance, "/cosmos.feegrant.v1beta1.BasicAllowance");
    let msg = MsgGrantAllowance {
        granter: granter.cosmos_address.to_string(),
        grantee: grantee.cosmos_address.to_string(),
        allowance: Some(allowance_any),
    };
    let msg = Msg::new("/cosmos.feegrant.v1beta1.MsgGrantAllowance", msg);
    
    let res = gravity_contact.send_message(&[msg], None, &[], Some(OPERATION_TIMEOUT), None, granter.cosmos_key).await;
    assert!(res.is_ok(), "Failed to grant fee allowance: {:?}", res.err());

    let mut fgqc = FeeGrantQueryClient::connect(gravity_contact.get_url()).await.unwrap();
    let allowance = fgqc.allowance(QueryAllowanceRequest {
        granter: granter.cosmos_address.to_string(),
        grantee: grantee.cosmos_address.to_string(),
    }).await.unwrap().into_inner();
    assert!(allowance.allowance.is_some(), "Allowance should exist after granting");

    let amount = 10u32;
    let fee_amount = 1_000000u64;
    let grantee_balance = gravity_contact.get_balance(grantee.cosmos_address.clone(), "ugraviton".to_string()).await.expect("Failed to get grantee balance").unwrap().amount;
    let granter_balance = gravity_contact.get_balance(granter.cosmos_address.clone(), "ugraviton".to_string()).await.expect("Failed to get granter balance").unwrap().amount;
    let expect_grantee_balance = grantee_balance - amount.into();
    let expect_granter_balance = granter_balance + amount.into() - fee_amount.into();
    info!("About to send {amount} ugraviton from grantee to granter using feegrant, expect grantee balance {grantee_balance} -> {expect_grantee_balance} and granter balance {granter_balance} -> {expect_granter_balance} after the transaction");
    let send = MsgSend {
        amount: vec![Coin {
            denom: "ugraviton".to_string(),
            amount: amount.to_string(),
        }],
        from_address: grantee.cosmos_address.to_string(),
        to_address: granter.cosmos_address.to_string(),
    };
    let msg = Msg::new(MSG_SEND_TYPE_URL, send);
    let fee = Fee {
        amount: vec![DSCoin {
            denom: "ugraviton".to_string(),
            amount: fee_amount.into(), // Should be covered by feegrant
        }],
        gas_limit: 200_000u64,
        payer: None,
        granter: Some(granter.cosmos_address.to_string()),
    };
    let args = gravity_contact.get_message_args(grantee.cosmos_address.clone(), fee, None).await.expect("Failed to get message args");
    let result = gravity_contact.send_message_with_args(
        &[msg],
        None,
        args,
        Some(OPERATION_TIMEOUT),
        grantee.cosmos_key,
    )
    .await;
    assert!(result.is_ok(), "Failed to send message with feegrant: {:?}", result.err());
    info!("Tx hash: {:?}", result.unwrap().txhash());

    let grantee_balance = gravity_contact.get_balance(grantee.cosmos_address.clone(), "ugraviton".to_string()).await.expect("Failed to get grantee balance").unwrap().amount;
    let granter_balance = gravity_contact.get_balance(granter.cosmos_address.clone(), "ugraviton".to_string()).await.expect("Failed to get granter balance").unwrap().amount;
    assert_eq!(grantee_balance, expect_grantee_balance, "Grantee balance incorrect");
    assert_eq!(granter_balance, expect_granter_balance, "Granter balance incorrect");

    info!("Successful feegrant test")
}
