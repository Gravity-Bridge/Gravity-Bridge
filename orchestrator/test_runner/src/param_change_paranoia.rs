use std::vec;

use gravity_proto::cosmos_sdk_proto::cosmos::auth::v1beta1::{MsgUpdateParams as MsgUpdateParamsAuth, QueryParamsRequest as QueryParamsRequestAuth};
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::{MsgUpdateParams as MsgUpdateParamsBank, QueryParamsRequest as QueryParamsRequestBank};
use gravity_proto::cosmos_sdk_proto::cosmos::base::v1beta1::Coin;
use gravity_proto::cosmos_sdk_proto::cosmos::crisis::v1beta1::{MsgUpdateParams as MsgUpdateParamsCrisis};
use gravity_proto::cosmos_sdk_proto::cosmos::consensus::v1::{MsgUpdateParams as MsgUpdateParamsConsensus, QueryParamsRequest as QueryParamsRequestConsensus};
use gravity_proto::cosmos_sdk_proto::cosmos::distribution::v1beta1::{MsgUpdateParams as MsgUpdateParamsDistribution, QueryParamsRequest as QueryParamsRequestDistribution};
use gravity_proto::cosmos_sdk_proto::cosmos::gov::v1::{MsgUpdateParams as MsgUpdateParamsGov, QueryParamsRequest as QueryParamsRequestGov};
use gravity_proto::cosmos_sdk_proto::cosmos::mint::v1beta1::{MsgUpdateParams as MsgUpdateParamsMint, QueryParamsRequest as QueryParamsRequestMint};
use gravity_proto::cosmos_sdk_proto::cosmos::slashing::v1beta1::{MsgUpdateParams as MsgUpdateParamsSlashing, QueryParamsRequest as QueryParamsRequestSlashing};
use gravity_proto::cosmos_sdk_proto::cosmos::staking::v1beta1::{MsgUpdateParams as MsgUpdateParamsStaking, QueryParamsRequest as QueryParamsRequestStaking};
use gravity_proto::gravity::v1::QueryParamsRequest as QueryParamsRequestGravity;
use gravity_proto::gravity::v2::{MsgUpdateParamsProposal as MsgUpdateParamsGravity, Param as GravityParam};
use gravity_proto::auction::{MsgUpdateParamsProposal as MsgUpdateParamsAuction, QueryParamsRequest as QueryParamsRequestAuction, Param as AuctionParam};

use gravity_proto::cosmos_sdk_proto::cosmos::auth::v1beta1::query_client::QueryClient as AuthQueryClient;
use gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::query_client::QueryClient as BankQueryClient;
use gravity_proto::cosmos_sdk_proto::cosmos::consensus::v1::query_client::QueryClient as ConsensusQueryClient;
use gravity_proto::cosmos_sdk_proto::cosmos::distribution::v1beta1::query_client::QueryClient as DistributionQueryClient;
use gravity_proto::cosmos_sdk_proto::cosmos::gov::v1::query_client::QueryClient as GovV1QueryClient;
use gravity_proto::cosmos_sdk_proto::cosmos::mint::v1beta1::query_client::QueryClient as MintQueryClient;
use gravity_proto::cosmos_sdk_proto::cosmos::slashing::v1beta1::query_client::QueryClient as SlashingQueryClient;
use gravity_proto::cosmos_sdk_proto::cosmos::staking::v1beta1::query_client::QueryClient as StakingQueryClient;
use gravity_proto::gravity::v1::query_client::QueryClient as GravityQueryClient;
use gravity_proto::auction::query_client::QueryClient as AuctionQueryClient;

use deep_space::{Coin as DSCoin, Contact, Msg, PrivateKey};

use crate::ADDRESS_PREFIX;
use crate::{utils::{get_user_key, ValidatorKeys}, OPERATION_TIMEOUT};


pub async fn param_change_paranoia_test(
    gravity_contact: &Contact,
    keys: Vec<ValidatorKeys>,
) {
    let user = get_user_key(None);
    gravity_contact.send_coins(DSCoin { amount: 1000000u64.into(), denom: "ugraviton".to_string() }, None, user.cosmos_address, Some(OPERATION_TIMEOUT), keys[0].orch_key).await.expect("Failed to fund user");
    let gov_mod_address = deep_space::address::get_module_account_address("gov", Some(&*ADDRESS_PREFIX)).expect("Unable to get gov module address");

    assert!(try_auth_module(gravity_contact, &user.cosmos_address.to_string(), user.cosmos_key, false).await, "Auth module param change went through?");
    assert!(try_bank_module(gravity_contact, &user.cosmos_address.to_string(), user.cosmos_key, false).await, "Bank module param change went through?");
    assert!(try_consensus_module(gravity_contact, &user.cosmos_address.to_string(), user.cosmos_key, false).await, "Consensus module param change went through?");
    assert!(try_crisis_module(gravity_contact, &user.cosmos_address.to_string(), user.cosmos_key, false).await, "Crisis module param change went through?");
    assert!(try_distribution_module(gravity_contact, &user.cosmos_address.to_string(), user.cosmos_key, false).await, "Distribution module param change went through?");
    assert!(try_gov_module(gravity_contact, &user.cosmos_address.to_string(), user.cosmos_key, false).await, "Gov module param change went through?");
    assert!(try_mint_module(gravity_contact, &user.cosmos_address.to_string(), user.cosmos_key, false).await, "Mint module param change went through?");
    assert!(try_slashing_module(gravity_contact, &user.cosmos_address.to_string(), user.cosmos_key, false).await, "Slashing module param change went through?");
    assert!(try_staking_module(gravity_contact, &user.cosmos_address.to_string(), user.cosmos_key, false).await, "Staking module param change went through?");
    assert!(try_gravity_module(gravity_contact, &user.cosmos_address.to_string(), user.cosmos_key, false).await, "Gravity module param change went through?");
    assert!(try_auction_module(gravity_contact, &user.cosmos_address.to_string(), user.cosmos_key, false).await, "Auction module param change went through?");


    assert!(try_auth_module(gravity_contact, &gov_mod_address.to_string(), user.cosmos_key, false).await, "Auth module param change went through?");
    assert!(try_bank_module(gravity_contact, &gov_mod_address.to_string(), user.cosmos_key, false).await, "Bank module param change went through?");
    assert!(try_crisis_module(gravity_contact, &gov_mod_address.to_string(), user.cosmos_key, false).await, "Crisis module param change went through?");
    assert!(try_consensus_module(gravity_contact, &gov_mod_address.to_string(), user.cosmos_key, false).await, "Consensus module param change went through?");
    assert!(try_distribution_module(gravity_contact, &gov_mod_address.to_string(), user.cosmos_key, false).await, "Distribution module param change went through?");
    assert!(try_gov_module(gravity_contact, &gov_mod_address.to_string(), user.cosmos_key, false).await, "Gov module param change went through?");
    assert!(try_mint_module(gravity_contact, &gov_mod_address.to_string(), user.cosmos_key, false).await, "Mint module param change went through?");
    assert!(try_slashing_module(gravity_contact, &gov_mod_address.to_string(), user.cosmos_key, false).await, "Slashing module param change went through?");
    assert!(try_staking_module(gravity_contact, &gov_mod_address.to_string(), user.cosmos_key, false).await, "Staking module param change went through?");
    assert!(try_gravity_module(gravity_contact, &gov_mod_address.to_string(), user.cosmos_key, false).await, "Gravity module param change went through?");
    assert!(try_auction_module(gravity_contact, &gov_mod_address.to_string(), user.cosmos_key, false).await, "Auction module param change went through?");
    info!("Successful param change paranoia test")
}

pub async fn try_auth_module(contact: &Contact, sender: &str, sender_key: impl PrivateKey, expect_success: bool) -> bool {
    let mut auth_qc = AuthQueryClient::connect(contact.get_url()).await.expect("Failed to connect to auth query client");
    let current_params=  auth_qc.params(QueryParamsRequestAuth{}).await.expect("Failed to get auth params").into_inner().params.unwrap();
    let mut updated_params = current_params.clone();
    // Change max_memo_characters
    updated_params.max_memo_characters += 1;
    let auth_msg = MsgUpdateParamsAuth{
        authority: sender.to_string(),
        params: Some(updated_params.clone())
    };
    let msg = Msg::new("/cosmos.auth.v1beta1.MsgUpdateParams", auth_msg);

    contact.send_message(&[msg], None, &[], Some(OPERATION_TIMEOUT), None, sender_key).await.map_or_else(
        |e|  {
            debug!("Auth module param change result: {:?}", e);
            !expect_success
        },
        |_|  expect_success ,
    )
}

pub async fn try_bank_module(contact: &Contact, sender: &str, sender_key: impl PrivateKey, expect_success: bool) -> bool {
    let mut qc = BankQueryClient::connect(contact.get_url()).await.expect("Failed to connect to bank query client");
    let current_params = qc.params(QueryParamsRequestBank{}).await.expect("Failed to get bank params").into_inner().params.unwrap();
    let mut updated_params: gravity_proto::cosmos_sdk_proto::cosmos::bank::v1beta1::Params = current_params.clone();
    // Change default_send_enabled
    updated_params.default_send_enabled = !updated_params.default_send_enabled;
    let msg = Msg::new("/cosmos.bank.v1beta1.MsgUpdateParams", MsgUpdateParamsBank {
        authority: sender.to_string(),
        params: Some(updated_params),
    });
    contact.send_message(&[msg], None, &[], Some(OPERATION_TIMEOUT), None, sender_key).await.map_or_else(
        |e| {
            debug!("Bank module param change result: {:?}", e);
            !expect_success
        },
        |_| expect_success,
    )
}

pub async fn try_consensus_module(contact: &Contact, sender: &str, sender_key: impl PrivateKey, expect_success: bool) -> bool {
    let mut qc = ConsensusQueryClient::connect(contact.get_url()).await.expect("Failed to connect to consensus query client");
    let current_params = qc.params(QueryParamsRequestConsensus{}).await.expect("Failed to get consensus params").into_inner().params.unwrap();
    let mut updated_params = current_params.clone();
    let mut block = current_params.block.unwrap_or_default();
    block.max_bytes += 1;
    // Example: increase block_max_bytes
    updated_params.block = Some(block);
    let msg = Msg::new("/cosmos.consensus.v1.MsgUpdateParams", MsgUpdateParamsConsensus {
        authority: sender.to_string(),
        block: updated_params.block,
        evidence: updated_params.evidence,
        validator: updated_params.validator,
        abci: updated_params.abci,

    });
    contact.send_message(&[msg], None, &[], Some(OPERATION_TIMEOUT), None, sender_key).await.map_or_else(
        |_| !expect_success,
        |_| expect_success,
    )
}

pub async fn try_crisis_module(contact: &Contact, sender: &str, sender_key: impl PrivateKey, expect_success: bool) -> bool {
    let constant_fee = Coin{
        denom: "ugraviton".to_string(),
        amount: "101".to_string(),
    };
    let msg = Msg::new("/cosmos.crisis.v1beta1.MsgUpdateParams", MsgUpdateParamsCrisis {
        authority: sender.to_string(),
        constant_fee: Some(constant_fee),
    });
    contact.send_message(&[msg], None, &[], Some(OPERATION_TIMEOUT), None, sender_key).await.map_or_else(
        |e| {
            debug!("Crisis module param change result: {:?}", e);
            !expect_success
        },
        |_| expect_success,
    )
}

pub async fn try_distribution_module(contact: &Contact, sender: &str, sender_key: impl PrivateKey, expect_success: bool) -> bool {
    let mut qc = DistributionQueryClient::connect(contact.get_url()).await.expect("Failed to connect to distribution query client");
    let current_params = qc.params(QueryParamsRequestDistribution{}).await.expect("Failed to get distribution params").into_inner().params.unwrap();
    let mut updated_params = current_params.clone();
    updated_params.withdraw_addr_enabled = !updated_params.withdraw_addr_enabled;
    let msg = Msg::new("/cosmos.distribution.v1beta1.MsgUpdateParams", MsgUpdateParamsDistribution {
        authority: sender.to_string(),
        params: Some(updated_params),
    });
    contact.send_message(&[msg], None, &[], Some(OPERATION_TIMEOUT), None, sender_key).await.map_or_else(
        |e| {
            debug!("Distribution module param change result: {:?}", e);
            !expect_success
        },
        |_| expect_success,
    )
}

pub async fn try_gov_module(contact: &Contact, sender: &str, sender_key: impl PrivateKey, expect_success: bool) -> bool {
    let mut qc = GovV1QueryClient::connect(contact.get_url()).await.expect("Failed to connect to gov query client");
    let current_params = qc.params(QueryParamsRequestGov{params_type: "voting".to_string()}).await.expect("Failed to get gov params").into_inner().params.unwrap();
    let mut updated_params = current_params.clone();
    // Example: increase voting_period by 1
    updated_params.burn_proposal_deposit_prevote = !updated_params.burn_proposal_deposit_prevote;
    let msg = Msg::new("/cosmos.gov.v1.MsgUpdateParams", MsgUpdateParamsGov {
        authority: sender.to_string(),
        params: Some(updated_params),
    });
    contact.send_message(&[msg], None, &[], Some(OPERATION_TIMEOUT), None, sender_key).await.map_or_else(
        |e| {
            debug!("Gov module param change result: {:?}", e);
            !expect_success
        },
        |_| expect_success,
    )
}

pub async fn try_mint_module(contact: &Contact, sender: &str, sender_key: impl PrivateKey, expect_success: bool) -> bool {
    let mut qc = MintQueryClient::connect(contact.get_url()).await.expect("Failed to connect to mint query client");
    let current_params = qc.params(QueryParamsRequestMint{}).await.expect("Failed to get mint params").into_inner().params.unwrap();
    let mut updated_params = current_params.clone();
    // Example: increase inflation_rate_change
    updated_params.inflation_rate_change = (updated_params.inflation_rate_change.parse::<f64>().unwrap_or(0.0) + 0.01).to_string();
    let msg = Msg::new("/cosmos.mint.v1beta1.MsgUpdateParams", MsgUpdateParamsMint {
        authority: sender.to_string(),
        params: Some(updated_params),
    });
    contact.send_message(&[msg], None, &[], Some(OPERATION_TIMEOUT), None, sender_key).await.map_or_else(
        |e| {
            debug!("Mint module param change result: {:?}", e);
            !expect_success
        },
        |_| expect_success,
    )
}

pub async fn try_slashing_module(contact: &Contact, sender: &str, sender_key: impl PrivateKey, expect_success: bool) -> bool {
    let mut qc = SlashingQueryClient::connect(contact.get_url()).await.expect("Failed to connect to slashing query client");
    let current_params = qc.params(QueryParamsRequestSlashing{}).await.expect("Failed to get slashing params").into_inner().params.unwrap();
    let mut updated_params = current_params.clone();
    // Example: increase signed_blocks_window
    updated_params.signed_blocks_window += 1;
    let msg = Msg::new("/cosmos.slashing.v1beta1.MsgUpdateParams", MsgUpdateParamsSlashing {
        authority: sender.to_string(),
        params: Some(updated_params),
    });
    contact.send_message(&[msg], None, &[], Some(OPERATION_TIMEOUT), None, sender_key).await.map_or_else(
        |e| {
            debug!("Slashing module param change result: {:?}", e);
            !expect_success
        },
        |_| expect_success,
    )
}

pub async fn try_staking_module(contact: &Contact, sender: &str, sender_key: impl PrivateKey, expect_success: bool) -> bool {
    let mut qc = StakingQueryClient::connect(contact.get_url()).await.expect("Failed to connect to staking query client");
    let current_params = qc.params(QueryParamsRequestStaking{}).await.expect("Failed to get staking params").into_inner().params.unwrap();
    let mut updated_params = current_params.clone();
    // Example: increase unbonding_time.seconds by 1
    if let Some(unbonding_time) = &mut updated_params.unbonding_time {
        unbonding_time.seconds += 1;
    }
    let msg = Msg::new("/cosmos.staking.v1beta1.MsgUpdateParams", MsgUpdateParamsStaking {
        authority: sender.to_string(),
        params: Some(updated_params),
    });
    contact.send_message(&[msg], None, &[], Some(OPERATION_TIMEOUT), None, sender_key).await.map_or_else(
        |e| {
            debug!("Staking module param change result: {:?}", e);
            !expect_success
        },
        |_| expect_success,
    )
}

pub async fn try_gravity_module(contact: &Contact, sender: &str, sender_key: impl PrivateKey, expect_success: bool) -> bool {
    let mut qc = GravityQueryClient::connect(contact.get_url()).await.expect("Failed to connect to gravity query client");
    let current_params = qc.params(QueryParamsRequestGravity{}).await.expect("Failed to get gravity params").into_inner().params.unwrap();
    let bridge_chain_id = current_params.bridge_chain_id;
    let msg = Msg::new("/gravity.v2.MsgUpdateParamsProposal", MsgUpdateParamsGravity {
        authority: sender.to_string(),
        param_updates: vec![GravityParam{
            key: "bridge_chain_id".to_string(),
            value: (bridge_chain_id + 1).to_string(),
        }],
    });
    contact.send_message(&[msg], None, &[], Some(OPERATION_TIMEOUT), None, sender_key).await.map_or_else(
        |e| {
            debug!("Gravity module param change result: {:?}", e);
            !expect_success
        },
        |_| expect_success,
    )
}

pub async fn try_auction_module(contact: &Contact, sender: &str, sender_key: impl PrivateKey, expect_success: bool) -> bool {
    let mut qc = AuctionQueryClient::connect(contact.get_url()).await.expect("Failed to connect to auction query client");
    let current_params = qc.params(QueryParamsRequestAuction{}).await.expect("Failed to get auction params").into_inner().params.unwrap();
    let auction_length = current_params.auction_length;
    let msg = Msg::new("/auction.v1.MsgUpdateParamsProposal", MsgUpdateParamsAuction {
        authority: sender.to_string(),
        param_updates: vec![AuctionParam{
            key: "auction_length".to_string(),
            value: (auction_length + 1).to_string(),
        }],
    });
    contact.send_message(&[msg], None, &[], Some(OPERATION_TIMEOUT), None, sender_key).await.map_or_else(
        |e| {
            debug!("Auction module param change result: {:?}", e);
            !expect_success
        },
        |_| expect_success,
    )
}