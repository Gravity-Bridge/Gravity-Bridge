# Orion UPGRADE
The *Orion* upgrade contains the following changes.

## Summary of Changes

* Updating Cosmos SDK version to v0.45.13, which is what the Cosmos Hub is currently running.
* Fee Collection Correction: Fees collected when sending tokens to Ethereum are being moved later in the transaction processing stage, meaning that the 2 basis point fee will only be collected if your message is successful.
* Upgrades to GBT dependencies including H2, Openssl, and Clarity which now uses the bnum library for uint256 math
