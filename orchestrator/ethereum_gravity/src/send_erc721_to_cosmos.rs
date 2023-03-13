//! Helper functions for sending tokens to Cosmos

use clarity::abi::{encode_call, Token};
use clarity::PrivateKey as EthPrivateKey;
use clarity::{Address, Uint256};
use deep_space::address::Address as CosmosAddress;
use gravity_utils::error::GravityError;
use std::time::{Duration, Instant};
use web30::client::Web3;
use web30::types::SendTxOption;

pub const SEND_TO_COSMOS_GAS_LIMIT: u128 = 100_000;

#[allow(clippy::too_many_arguments)]
pub async fn send_erc721_to_cosmos(
    erc721: Address,
    gravityerc721_contract: Address,
    token_id: Uint256,
    cosmos_destination: CosmosAddress,
    sender_secret: EthPrivateKey,
    wait_timeout: Option<Duration>,
    web3: &Web3,
    options: Vec<SendTxOption>,
) -> Result<Uint256, GravityError> {
    let sender_address = sender_secret.to_address();
    let mut approve_nonce = None;

    info!("Now in the send_erc721_to_cosmos function!");

    for option in options.iter() {
        if let SendTxOption::Nonce(_) = option {
            return Err(GravityError::InvalidOptionsError(
                "This call sends more than one tx! Can't specify".to_string(),
            ));
        }
    }

    let mut address_approved = web3
        .check_erc721_approved(erc721, sender_address, token_id)
        .await;
    if let Some(w) = wait_timeout {
        let start = Instant::now();
        // keep trying while there's still time
        while address_approved.is_err() && Instant::now() - start < w {
            address_approved = web3
                .check_erc721_approved(erc721, sender_address, token_id)
                .await;
        }
    }
    let address_approved = address_approved?;
    if address_approved.is_none() {
        info!(
            "Approval for ERC721 contract {} token id {} is empty...approving",
            erc721, token_id
        );
        let mut options = options.clone();
        let nonce = web3.eth_get_transaction_count(sender_address).await?;
        options.push(SendTxOption::Nonce(nonce));
        approve_nonce = Some(nonce);

        let txid = web3
            .approve_erc721_transfers(
                erc721,
                sender_secret,
                gravityerc721_contract,
                token_id,
                None,
                options,
            )
            .await?;
        trace!(
            "We are not approved for ERC721 transfers, approving txid: {:#066x}",
            txid
        );
        info!("successful ERC721 approve, tx id {}", txid);
    }

    // if the user sets a gas limit we should honor it, if they don't we
    // should add the default
    let mut has_gas_limit = false;
    let mut options = options;
    for option in options.iter() {
        if let SendTxOption::GasLimit(_) = option {
            has_gas_limit = true;
            break;
        }
    }
    if !has_gas_limit {
        options.push(SendTxOption::GasLimit(SEND_TO_COSMOS_GAS_LIMIT.into()));
    }
    // if we have run an approval we should increment our nonce by one so that
    // we can be sure our actual tx can go in immediately behind
    if let Some(nonce) = approve_nonce {
        options.push(SendTxOption::Nonce(nonce + 1u8.into()));
    }

    info!(
        "sending ERC721 token id {} to cosmos dest {}",
        token_id, cosmos_destination
    );
    let encoded_destination_address = Token::String(cosmos_destination.to_string());

    let tx_hash = web3
        .send_transaction(
            gravityerc721_contract,
            encode_call(
                "sendERC721ToCosmos(address,string,uint256)",
                &[erc721.into(), encoded_destination_address, token_id.into()],
            )?,
            0u32.into(),
            sender_address,
            sender_secret,
            options,
        )
        .await?;

    if let Some(timeout) = wait_timeout {
        web3.wait_for_transaction(tx_hash, timeout, None).await?;
    }

    Ok(tx_hash)
}
