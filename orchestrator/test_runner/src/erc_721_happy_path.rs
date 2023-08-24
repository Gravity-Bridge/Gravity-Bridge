use crate::utils::*;
use crate::MINER_ADDRESS;
use crate::MINER_PRIVATE_KEY;
use crate::OPERATION_TIMEOUT;
use crate::TOTAL_TIMEOUT;
use clarity::{Address as EthAddress, Uint256};
use deep_space::address::Address as CosmosAddress;
use deep_space::Contact;
use ethereum_gravity::send_erc721_to_cosmos::send_erc721_to_cosmos;
use ethereum_gravity::utils::get_gravity_sol_address;
use ethereum_gravity::utils::get_valset_nonce;
use gravity_utils::error::GravityError;
use std::time::Duration;
use std::time::Instant;
use web30::client::Web3;
use web30::types::SendTxOption;

pub async fn erc721_happy_path_test(
    web30: &Web3,
    contact: &Contact,
    keys: Vec<ValidatorKeys>,
    gravity_address: EthAddress,
    gravity_erc721_address: EthAddress,
    erc721_address: EthAddress,
    validator_out: bool,
) {
    let grav_sol_address_in_erc721 =
        get_gravity_sol_address(gravity_erc721_address, *MINER_ADDRESS, web30)
            .await
            .unwrap();

    info!("ERC721 address is {}", erc721_address);
    info!("Miner address is {}", *MINER_ADDRESS);
    info!("Miner pk is {}", *MINER_PRIVATE_KEY);
    info!(
        "grav_sol_address_in_erc721 is {}",
        grav_sol_address_in_erc721
    );
    info!("Gravity address is {}", gravity_address);
    info!("GravityERC721 address is {}", gravity_erc721_address);

    assert_eq!(grav_sol_address_in_erc721, gravity_address);

    let no_relay_market_config = create_default_test_config();
    start_orchestrators(
        keys.clone(),
        gravity_address,
        validator_out,
        no_relay_market_config,
    )
    .await;

    // generate an address for coin sending tests, this ensures test imdepotency
    let user_keys = get_user_key(None);

    info!("testing erc721 deposit");
    // Run test with three ERC721 tokens, 200, 201, 202
    for i in 200_i32..203_i32 {
        test_erc721_deposit_panic(
            web30,
            contact,
            user_keys.cosmos_address,
            gravity_address,
            gravity_erc721_address,
            erc721_address,
            Uint256::from_be_bytes(&i.to_be_bytes()),
            None,
        )
        .await;
    }

    info!("testing ERC721 approval utility");
    let token_id_for_approval = Uint256::from_be_bytes(&203_i32.to_be_bytes());
    test_erc721_transfer_utils(
        web30,
        gravity_erc721_address,
        erc721_address,
        token_id_for_approval,
    )
    .await;
}

// Tests an ERC721 deposit and panics on failure
#[allow(clippy::too_many_arguments)]
pub async fn test_erc721_deposit_panic(
    web30: &Web3,
    contact: &Contact,
    dest: CosmosAddress,
    gravity_address: EthAddress,
    gravity_erc721_address: EthAddress,
    erc721_address: EthAddress,
    token_id: Uint256,
    timeout: Option<Duration>,
) {
    match test_erc721_deposit_result(
        web30,
        contact,
        dest,
        gravity_address,
        gravity_erc721_address,
        erc721_address,
        token_id,
        timeout,
    )
    .await
    {
        Ok(_) => {
            info!("Successfully bridged ERC721 for token id {}!", token_id)
        }
        Err(_) => {
            panic!("Failed to bridge ERC721 for token id {}!", token_id)
        }
    }
}

/// this function tests Ethereum -> Cosmos deposits of ERC721 tokens
#[allow(clippy::too_many_arguments)]
pub async fn test_erc721_deposit_result(
    web30: &Web3,
    contact: &Contact,
    dest: CosmosAddress,
    gravity_address: EthAddress,
    gravityerc721_address: EthAddress,
    erc721_address: EthAddress,
    token_id: Uint256,
    timeout: Option<Duration>,
) -> Result<(), GravityError> {
    get_valset_nonce(gravity_address, *MINER_ADDRESS, web30)
        .await
        .expect("Incorrect Gravity Address or otherwise unable to contact Gravity");

    let val = web30
        .get_erc721_symbol(erc721_address, *MINER_ADDRESS)
        .await
        .expect("Not a valid ERC721 contract address");
    info!("In erc721_happy_path_test symbol is {}", val);

    info!(
        "Sending to Cosmos from miner adddress {} to {} with token id {}",
        *MINER_ADDRESS,
        dest,
        token_id.clone()
    );
    // we send an ERC721 to gravityERC721.sol to register a deposit
    let tx_id = send_erc721_to_cosmos(
        erc721_address,
        gravityerc721_address,
        token_id,
        dest,
        *MINER_PRIVATE_KEY,
        None,
        web30,
        vec![],
    )
    .await
    .expect("Failed to send tokens to Cosmos");

    let _tx_res = web30
        .wait_for_transaction(tx_id, OPERATION_TIMEOUT, None)
        .await
        .expect("Send to cosmos transaction failed to be included into ethereum side");

    let start = Instant::now();
    let duration = match timeout {
        Some(w) => w,
        None => TOTAL_TIMEOUT,
    };
    while Instant::now() - start < duration {
        // in this while loop wait for owner to change OR wait for event to fire
        let owner = web30
            .get_erc721_owner_of(erc721_address, *MINER_ADDRESS, token_id)
            .await;

        if owner.unwrap() == gravityerc721_address {
            info!(
                "Successfully moved token_id {} to GravityERC721 {}",
                token_id.clone(),
                gravityerc721_address
            );
            return Ok(());
        }
        contact.wait_for_next_block(TOTAL_TIMEOUT).await.unwrap();
    }
    Err(GravityError::InvalidBridgeStateError(
        "Did not complete ERC721 deposit!".to_string(),
    ))
}

pub async fn test_erc721_transfer_utils(
    web30: &Web3,
    gravity_erc721_address: EthAddress,
    erc721_address: EthAddress,
    token_id: Uint256,
) {
    let mut options = vec![];
    let nonce = web30
        .eth_get_transaction_count(*MINER_ADDRESS)
        .await
        .expect("Error retrieving nonce from eth");
    options.push(SendTxOption::Nonce(nonce));

    match web30
        .approve_erc721_transfers(
            erc721_address,
            *MINER_PRIVATE_KEY,
            gravity_erc721_address,
            token_id,
            None,
            options,
        )
        .await
    {
        Ok(_) => {
            info!("Successfully called ERC721 transfer util for {}!", token_id)
        }
        Err(_) => {
            panic!("Failed ERC721 transfer util for {}!", token_id)
        }
    }
}
