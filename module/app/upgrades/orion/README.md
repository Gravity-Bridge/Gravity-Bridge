# Orion UPGRADE
The *Orion* upgrade contains the following changes.

## Summary of Changes

* Updating Cosmos SDK version to v0.45.13, which is what the Cosmos Hub is currently running.
* Fee Collection Correction: Fees collected when sending tokens to Ethereum are being moved later in the transaction processing stage, meaning that the 2 basis point fee will only be collected if your message is successful.
* Cross-Bridge Balance Monitoring: This novel end-to-end balance monitoring will dramatically improve bridge security **without daily limits**. Imposing limits hurts the community by limiting valid response to dramatic shifts in the crypto market, which happen regularly. The Cosmos side of the bridge is getting enhanced monitoring every time an Ethereum event is processed. Additionally, Gravity Bridge will require all Orchestrators to check important balances of the Ethereum side of the bridge and compare them to the balances recorded on the Cosmos side of the bridge.