use std::str::FromStr;

use clarity::Address as EthAddress;
use deep_space::{
    client::type_urls::MSG_SEND_TYPE_URL, utils::decode_bytes, Address as CosmosAddress, Coin,
    Contact, CosmosPrivateKey, EthermintPrivateKey, PrivateKey,
};
use gravity_proto::{
    cosmos_sdk_proto::cosmos::{
        bank::v1beta1::MsgSend,
        tx::v1beta1::{BroadcastMode, TxRaw},
    },
    gravity::query_client::QueryClient as GravityQueryClient,
};
use gravity_utils::num_conversion::one_atom;
use num256::Uint256;
use prost::Message;
use reqwest::Client;
use tonic::transport::Channel;
use web30::client::Web3;

use crate::{
    get_fee,
    utils::{get_user_key, BridgeUserKey, ValidatorKeys},
    ADDRESS_PREFIX, OPERATION_TIMEOUT, STAKING_TOKEN,
};

pub const TX_POST_PORT: &str = "1317";
pub const TX_POST_ENDPOINT: &str = "/cosmos/tx/v1beta1/txs";
/*
This is a valid transaction containing one MsgSend from USER to RECEIVER, signed with EIP-712, expected to be the first transaction from USER
Tx
    Account_number: 0
    Chain_id: gravity-test-1
    Fee:
        Amount:
            0:
                Amount: 1
                Denom: ugraviton
        Gas: 200000
        FeePayer: gravity1hanqss6jsq66tfyjz56wz44z0ejtyv0724h32c
    Memo: Test EIP-712
    Msgs:
        0:
            Type: cosmos-sdk/MsgSend
            Value:
                Amount:
                    0:
                        Amount: 1
                        Denom: ugraviton
                From_address: gravity1hanqss6jsq66tfyjz56wz44z0ejtyv0724h32c
                To_address: gravity190e5l0kyddd4ljl262fxefnnrfg22h2rts2v4k
    Sequence: 0

*/
pub const SUCCESSFUL_EIP712_MSG_SEND: &str = "10,202,2,10,144,1,10,28,47,99,111,115,109,111,115,46,98,97,110,107,46,118,49,98,101,116,97,49,46,77,115,103,83,101,110,100,18,112,10,46,103,114,97,118,105,116,121,49,104,97,110,113,115,115,54,106,115,113,54,54,116,102,121,106,122,53,54,119,122,52,52,122,48,101,106,116,121,118,48,55,50,52,104,51,50,99,18,46,103,114,97,118,105,116,121,49,57,48,101,53,108,48,107,121,100,100,100,52,108,106,108,50,54,50,102,120,101,102,110,110,114,102,103,50,50,104,50,114,116,115,50,118,52,107,26,14,10,9,117,103,114,97,118,105,116,111,110,18,1,49,18,12,84,101,115,116,32,69,73,80,45,55,49,50,250,63,165,1,10,42,47,101,116,104,101,114,109,105,110,116,46,116,121,112,101,115,46,118,49,46,69,120,116,101,110,115,105,111,110,79,112,116,105,111,110,115,87,101,98,51,84,120,18,119,8,191,132,61,18,46,103,114,97,118,105,116,121,49,104,97,110,113,115,115,54,106,115,113,54,54,116,102,121,106,122,53,54,119,122,52,52,122,48,101,106,116,121,118,48,55,50,52,104,51,50,99,26,65,193,94,28,168,124,188,200,245,19,70,189,30,146,96,156,10,193,235,21,113,33,106,23,1,183,186,103,62,21,203,46,200,46,55,89,159,92,165,106,195,75,29,175,191,105,170,163,56,214,62,181,237,223,227,128,141,68,175,58,9,205,126,237,213,28,18,111,10,87,10,79,10,40,47,101,116,104,101,114,109,105,110,116,46,99,114,121,112,116,111,46,118,49,46,101,116,104,115,101,99,112,50,53,54,107,49,46,80,117,98,75,101,121,18,35,10,33,2,96,22,221,190,65,131,210,254,152,203,219,14,220,50,31,66,250,207,36,246,56,139,39,117,192,120,199,253,189,89,254,2,18,4,10,2,8,127,18,20,10,14,10,9,117,103,114,97,118,105,116,111,110,18,1,49,16,192,154,12,26,0";
// This is an invalid transaction containing one MsgSend from USER to RECEIVER, generated correctly but with the 6th byte of the signature replaced with 0, expected to be the second transaction submitted by USER
pub const BAD_SECOND_MESSAGE: &str =         "10,202,2,10,144,1,10,28,47,99,111,115,109,111,115,46,98,97,110,107,46,118,49,98,101,116,97,49,46,77,115,103,83,101,110,100,18,112,10,46,103,114,97,118,105,116,121,49,104,97,110,113,115,115,54,106,115,113,54,54,116,102,121,106,122,53,54,119,122,52,52,122,48,101,106,116,121,118,48,55,50,52,104,51,50,99,18,46,103,114,97,118,105,116,121,49,57,48,101,53,108,48,107,121,100,100,100,52,108,106,108,50,54,50,102,120,101,102,110,110,114,102,103,50,50,104,50,114,116,115,50,118,52,107,26,14,10,9,117,103,114,97,118,105,116,111,110,18,1,49,18,12,84,101,115,116,32,69,73,80,45,55,49,50,250,63,165,1,10,42,47,101,116,104,101,114,109,105,110,116,46,116,121,112,101,115,46,118,49,46,69,120,116,101,110,115,105,111,110,79,112,116,105,111,110,115,87,101,98,51,84,120,18,119,8,191,132,61,18,46,103,114,97,118,105,116,121,49,104,97,110,113,115,115,54,106,115,113,54,54,116,102,121,106,122,53,54,119,122,52,52,122,48,101,106,116,121,118,48,55,50,52,104,51,50,99,26,65,188,96,41,47,129,24,180,159,182,253,127,173,35,196,212,246,115,120,59,74,18,106,215,127,41,29,169,57,79,125,243,54,58,54,175,41,11,98,223,162,105,28,189,14,34,118,251,93,32,10,215,185,37,243,222,207,35,113,180,208,141,72,60,4,28,18,113,10,89,10,79,10,40,47,101,116,104,101,114,109,105,110,116,46,99,114,121,112,116,111,46,118,49,46,101,116,104,115,101,99,112,50,53,54,107,49,46,80,117,98,75,101,121,18,35,10,33,2,96,22,221,190,65,131,210,254,152,203,219,14,220,50,31,66,250,207,36,246,56,139,39,117,192,120,199,253,189,89,254,2,18,4,10,2,8,127,24,1,18,20,10,14,10,9,117,103,114,97,118,105,116,111,110,18,1,49,16,192,154,12,26,0";
pub const USER: &str = "gravity1hanqss6jsq66tfyjz56wz44z0ejtyv0724h32c"; // This is the MINER_ADDRESS interpreted as a Bech32 address
pub const RECEIVER: &str = "gravity190e5l0kyddd4ljl262fxefnnrfg22h2rts2v4k";
pub const AMOUNT: u64 = 1;
pub const FEE: u64 = 1;

// Submits a hardcoded EIP712-signed message to the chain, only works once and must be the first test run
// due to the user's account_number depending on the previous tests and sequence depending on
// previous executions of this test
pub async fn eip_712_test(
    _web30: &Web3,
    _gravity_client: GravityQueryClient<Channel>,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    _ibc_keys: Vec<CosmosPrivateKey>,
    _gravity_address: EthAddress,
    _erc20_address: EthAddress,
) {
    let sender = CosmosAddress::from_bech32(USER.to_string()).expect("invalid successful user?");
    let receiver =
        CosmosAddress::from_bech32(RECEIVER.to_string()).expect("invalid successful receiver?");
    // Grant the user the tokens needed for the message to succeed
    setup(contact, &keys, sender).await;

    // First, test that a bad deep_space transaction (invalid signature) produces a similar error

    // Create a user and send them some funds
    let user = get_user_key(None);
    contact
        .send_coins(
            Coin {
                amount: one_atom() * 100u8.into(),
                denom: STAKING_TOKEN.clone(),
            },
            Some(get_fee(None)),
            user.cosmos_address,
            Some(OPERATION_TIMEOUT),
            keys.first().unwrap().validator_key,
        )
        .await
        .expect("Could not send tokens to new user");

    send_bad_deep_space_tx(contact, user, &receiver).await;

    // Next, send a successful EIP712 transaction as the miner

    let http_client = Client::new();
    // Get the URL from contact, remove the port, add the tx POST port
    let tx_post_host =
        contact.get_url().rsplit_once(':').unwrap().0.to_string() + ":" + TX_POST_PORT;
    let tx_post_url = tx_post_host + TX_POST_ENDPOINT;
    debug!("Sending Tx to {tx_post_url}");
    let req_body = wrap_transaction(SUCCESSFUL_EIP712_MSG_SEND);
    debug!("Tx request is {req_body}");
    send_eip712_tx(
        &http_client,
        tx_post_url.clone(),
        req_body,
        contact,
        &sender,
        &receiver,
        true, // Expect success
    )
    .await;

    // Send a second EIP712 transaction which has an invalid signature

    let second_req = wrap_transaction(BAD_SECOND_MESSAGE);
    debug!("Tx request is {second_req}");
    send_eip712_tx(
        &http_client,
        tx_post_url.clone(),
        second_req,
        contact,
        &sender,
        &receiver,
        false, // Expect failure
    )
    .await;

    // Finally, send a valid transaction using ethermint key support with deep_space

    let miner_ethermint = EthermintPrivateKey::from_str(
        "0xb1bab011e03a9862664706fc3bbaa1b16651528e5f0e7fbfcbfdd8be302a13e7",
    )
    .expect("Could not interpret miner private key as Ethermint key");
    let miner_bech32 = miner_ethermint.to_address(&ADDRESS_PREFIX).unwrap();
    let pre_balance = contact
        .get_balance(miner_bech32, STAKING_TOKEN.to_string())
        .await
        .expect("Unable to get miner staking token balance")
        .expect("Miner has no staking token balance");
    contact
        .send_coins(
            Coin {
                amount: 1u8.into(),
                denom: "ugraviton".to_string(),
            },
            Some(get_fee(Some(STAKING_TOKEN.to_string()))),
            CosmosAddress::from_bech32(RECEIVER.to_string()).unwrap(),
            Some(OPERATION_TIMEOUT),
            miner_ethermint,
        )
        .await
        .expect("Unable to send 1 ugraviton from miner address as ethermint key");
    let post_balance = contact
        .get_balance(miner_bech32, STAKING_TOKEN.to_string())
        .await
        .expect("Unable to get miner staking token balance")
        .expect("Miner has no staking token balance");
    assert!(pre_balance.amount.gt(&post_balance.amount));

    info!("Successful EIP-712 Signature verification!")
}

// Gives the SUCCESSFUL_USER enough stake to submit the hard-coded message
pub async fn setup(contact: &Contact, keys: &[ValidatorKeys], addr: CosmosAddress) {
    contact
        .send_coins(
            Coin {
                amount: 1_000000u64.into(),
                denom: STAKING_TOKEN.to_string(),
            },
            None,
            addr,
            Some(OPERATION_TIMEOUT),
            keys[0].validator_key,
        )
        .await
        .expect("Failed to send user tokens");
}

pub fn wrap_transaction(transaction: &str) -> String {
    format!(
        "{{ \"tx_bytes\": [{}], \"mode\": \"BROADCAST_MODE_BLOCK\" }}",
        transaction
    )
}

pub async fn send_eip712_tx(
    http_client: &reqwest::Client,
    url: String,
    body: String,
    contact: &Contact,
    sender: &CosmosAddress,
    receiver: &CosmosAddress,
    expect_signature_success: bool,
) -> String {
    let sender_prebal = contact
        .get_balance(*sender, STAKING_TOKEN.to_string())
        .await
        .expect("Could not get sender's balance")
        .expect("Sender has no balance");
    let receiver_prebal = contact
        .get_balance(*receiver, STAKING_TOKEN.to_string())
        .await
        .expect("Could not get receiver's balance")
        .expect("receiver has no balance");

    // Send the transaction
    let res = http_client
        .post(url.to_string())
        .header(reqwest::header::CONTENT_TYPE, "application/json")
        .body(body.to_string())
        .send()
        .await
        .expect("Transaction submission failure")
        .text()
        .await;
    debug!("Tx submission res: {res:?}");

    let sender_postbal = contact
        .get_balance(*sender, STAKING_TOKEN.to_string())
        .await
        .expect("Could not get sender's balance")
        .expect("Sender has no balance");
    let receiver_postbal = contact
        .get_balance(*receiver, STAKING_TOKEN.to_string())
        .await
        .expect("Could not get receiver's balance")
        .expect("receiver has no balance");

    assert!(res.is_ok());
    let res = res.unwrap();
    if expect_signature_success {
        assert!(!res.contains("signature verification failed")); // The signature must be invalid

        let transferred: Uint256 = AMOUNT.into();
        let total_paid: Uint256 = transferred + FEE.into();

        assert!(sender_postbal.amount < sender_prebal.amount);
        let sender_diff = sender_prebal.amount - sender_postbal.amount;
        assert_eq!(sender_diff, total_paid);
        assert!(receiver_postbal.amount > receiver_prebal.amount);
        let receiver_diff = receiver_postbal.amount - receiver_prebal.amount;
        assert_eq!(receiver_diff, transferred);
    } else {
        assert!(res.contains("signature verification failed")); // The signature must be invalid
        assert!(
            res.contains("EthPubKeySecp256k1{026016DDBE4183D2FE98CBDB0EDC321F42FACF24F6388B2775C078C7FDBD59FE02} is different from transaction pubkey") ||
            res.contains("failed to recover delegated fee payer from sig")
        );
        let sender_postbal = contact
            .get_balance(*sender, STAKING_TOKEN.to_string())
            .await
            .expect("Could not get sender's balance")
            .expect("Sender has no balance");
        let receiver_postbal = contact
            .get_balance(*receiver, STAKING_TOKEN.to_string())
            .await
            .expect("Could not get receiver's balance")
            .expect("receiver has no balance");

        // Also check that the balances did not change
        assert_eq!(sender_prebal, sender_postbal);
        assert_eq!(receiver_prebal, receiver_postbal);
    }

    res
}

pub async fn send_bad_deep_space_tx(
    contact: &Contact,
    user: BridgeUserKey,
    receiver: &CosmosAddress,
) {
    // Manually generate and sign a transaction from that user, but mess with the signature
    let msg = MsgSend {
        from_address: user.cosmos_address.to_string(),
        to_address: receiver.to_string(),
        amount: vec![Coin {
            amount: one_atom(),
            denom: STAKING_TOKEN.clone(),
        }
        .into()],
    };
    let ds_msg = deep_space::Msg::new(MSG_SEND_TYPE_URL, msg);
    let fee = contact
        .get_fee_info(
            &[ds_msg.clone()],
            &[get_fee(Some(STAKING_TOKEN.clone()))],
            user.cosmos_key,
        )
        .await
        .expect("Could not get fee info");
    let args = contact
        .get_message_args(user.cosmos_address, fee, None)
        .await
        .expect("Could not get message args");
    let memo = "Bad message, bad!".to_string();
    let msg_bytes = user
        .cosmos_key
        .sign_std_msg(&[ds_msg], args, &memo)
        .expect("Could not get signed TxRaw bytes");

    // Decode the signed message back to a TxRaw, since deep_space will not give us the type
    let mut tx_raw = decode_bytes::<TxRaw>(msg_bytes).expect("Failed to decode TxRaw");

    // Change the
    let mut signature = tx_raw.signatures[0].clone();
    signature[0] += 1u8;

    tx_raw.signatures = vec![signature];

    let mut bad_tx_buf = vec![];
    tx_raw
        .encode(&mut bad_tx_buf)
        .expect("Unable to re-encode tx_raw");

    let sender_prebal = contact
        .get_balance(user.cosmos_address, STAKING_TOKEN.to_string())
        .await
        .expect("Could not get sender's balance")
        .expect("Sender has no balance");
    let receiver_prebal = contact
        .get_balance(*receiver, STAKING_TOKEN.to_string())
        .await
        .expect("Could not get receiver's balance")
        .expect("receiver has no balance");

    let err_response = contact
        .send_transaction(bad_tx_buf, BroadcastMode::Sync)
        .await
        .expect_err("bad tx was successful?");
    debug!("Got error response: {err_response:?}");

    tokio::time::sleep(OPERATION_TIMEOUT).await;

    let sender_postbal = contact
        .get_balance(user.cosmos_address, STAKING_TOKEN.to_string())
        .await
        .expect("Could not get sender's balance")
        .expect("Sender has no balance");
    let receiver_postbal = contact
        .get_balance(*receiver, STAKING_TOKEN.to_string())
        .await
        .expect("Could not get receiver's balance")
        .expect("receiver has no balance");

    // Also check that the balances did not change
    assert_eq!(sender_prebal, sender_postbal);
    assert_eq!(receiver_prebal, receiver_postbal);
}
