# Antares UPGRADE
The *Antares* upgrade contains the following changes.

## Summary of Changes

* Updating Cosmos SDK version to v0.45.16, which is the end of the v0.45 line. A migration to v0.46 will happen in the next upgrade.
* Updating IBC version to v4.3.1, since IBC v3 is no longer supported.
* Adding Interchain Accounts Host module:
    * By enabling ICA it will be possible to send tokens to and from chains without interacting with your Gravity Bridge account. Combining Interchain Accounts and IBC Auto Forwarding, users on an ICA Controller chain will be able to send tokens to Cosmos via Gravity or transfer tokens to Ethereum through Gravity while only submitting messages to Ethereum and the Controller chain. That means a sophisticated UI can present all transactions through Metamask for convenience!
    * ICA will also enable Liquid Staking using platforms like Persistence, Quicksilver, Stride, and more!