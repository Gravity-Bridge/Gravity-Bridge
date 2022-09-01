//! This is a test to ensure that the chain may be halted if an invariant is ever
//! violated by submitting a MsgVerifyInvariant to the chain
//!
use crate::{ValidatorKeys, OPERATION_TIMEOUT};
use deep_space::client::types::LatestBlock;
use deep_space::Contact;
use tokio::time::sleep;

const FAILING_INVARIANT_MODULE_NAME: &str = "orbit";
const FAILING_INVARIANT_NAME: &str = "failing-invariant";

/// Tests that a chain started with --x-crisis-verify-halts-node will halt the whole chain when
/// a failed invariant is executed
pub async fn invariants_halt_chain(contact: &Contact, keys: Vec<ValidatorKeys>) {
    let curr_block = contact.get_latest_block().await;
    info!("Got latest block: {:?}", curr_block);

    let sender = keys[0].validator_key;

    let invariant_result = contact
        .invariant_halt(
            FAILING_INVARIANT_MODULE_NAME,
            FAILING_INVARIANT_NAME,
            None,
            OPERATION_TIMEOUT,
            sender,
        )
        .await;

    info!("Got invariant result: {:?}", invariant_result);

    let curr_block = contact
        .get_latest_block()
        .await
        .expect("Could not obtain latest block");
    info!("Got latest block: {:?}", curr_block);

    sleep(OPERATION_TIMEOUT).await;

    let updated_block = contact
        .get_latest_block()
        .await
        .expect("Could not obtain latest block");

    match (curr_block, updated_block) {
        (LatestBlock::Latest { block: curr }, LatestBlock::Latest { block: updated }) => {
            assert_eq!(
                curr.header.expect("curr block has no header!").height,
                updated.header.expect("updated block has no header!").height
            );
        }
        (_, _) => panic!("Unexpected latest block response from chain"),
    }

    info!("Chain successfully halted!")
}
