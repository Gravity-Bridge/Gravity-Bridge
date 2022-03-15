# MERCURY UPGRADE

The *Mercury* upgrade includes several changes, and in the code is also referred to as the v2 upgrade, because it is the upgrade in which the Gravity ConsensusVersion() result changes from "1" to "2".

## Summary of Changes

* Enable use of the upgrade module for in-place store migrations instead of JSON based genesis upgrades - no longer will our IBC channels be messed up on upgrades!
* Add bech32ibc module for IBC auto forwarding of SendToCosmos messages - big thanks to the Osmosis-Labs team for making this open source module
* Add IBC auto forwarding - when a user calls SendToCosmos on the Gravity.sol contract with a CosmosReceiver like "cosmos1abcdefg", if "cosmos1" is a prefix registered with an IBC channel in the bech32ibc module, a new Transfer will be executed to send those funds to the cosmos hub chain. In the event of error, the funds will be sent to "gravity1abcdefg" - the same address but with a local gravity prefix. The user will be able to use their "abcdefg" wallet with gravity to recover the funds and send them where they need to go. This code is inspired by Osmosis-Labs' bech32ics20 module, thanks again to them.
* Enforce validator minimum commission - big thanks to the Cosmos open source community for this feature, we grabbed some code from the Juno and Stargaze teams, see the commit history or the code for details.
* Fix distribution module invariant which was messed up by executing an airdrop which gave **GRAV** to the distribution module. Unfortunately if the distribution module receives funds it causes a community pool + validator rewards invariant to fail, these funds rightfully belong to the community so this fix will set the community pool's **GRAV** balance to the correct value.
* Update the cosmos-sdk version from v0.44.5 to v0.44.6
* Update the ibc-go version from v2.0.2 to v2.1.0
