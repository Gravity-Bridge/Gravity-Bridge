# POLARIS UPGRADE

The *Polaris* upgrade contains the following changes.

## Summary of Changes

* Enable the use of Ethereum keys for Gravity transactions.
This change enables users to sign Gravity transactions using their Ethereum wallet (e.g. MetaMask).
Notably, this support enables IBC Auto-Forwarding functionality to work for Cosmos EVM chains (e.g. Evmos)
without the potential for loss of funds.
  * Due to the differences in cosmos-sdk's and Ethereum's Secp256k1 implementation, IBC Auto-Forwarding was not enabled
    for Evmos due to the following scenario:
    A user Auto-Forwards to Evmos while Evmos is offline for 1 month, leading to an IBC transfer failure.
    A "gravity1..." recovery address receives the funds, however this "gravity1..." address
    corresponds to a completely different private key than the one used on Evmos. This means the
    user would lose their funds. The Polaris upgrade allows the user to use their Evmos private key directly on Gravity,
    allowing them to access their funds.
