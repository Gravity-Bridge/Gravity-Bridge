# Gravity JSONRPC Server

Gravity Bridge is a CosmosSDK blockchain implementing EIP-712 transaction signing and Ethermint keys to enable the use of Ethereum wallets like MetaMask.
When wallets like MetaMask add a network, they query the chain provided for its Chain ID to make certain the configuration is correct.
Typically EIP-712 signing is enabled on hybrid CosmosSDK/EVM chains based off of Ethermint, which Gravity is not.
In order to facilitate the use of MetaMask with minimal user difficulty, this simple JSONRPC HTTP server exists to provide the expected responses needed
by wallets like MetaMask.

## Chain ID

In order for EIP-712 signing to work, Gravity has a EVM Chain ID (99999) in addition to its regular Cosmos-style Chain ID (gravity-bridge-3).
This service is only aware of the EVM Chain ID (99999).

## Startup

This server is expected to run with the Gravity blockchain node with `gbt`. To manually run this server:

1. First install `gbt` either by running `cargo build --release` within the orchestrator directory, or by downloading a supported release [here](https://github.com/Gravity-Bridge/Gravity-Bridge/releases) and adding it to your PATH
1. Run `gbt` with `gbt jsonrpc-server`, pass the `-h` flag for info on domain/port configuration and enabling HTTPS

## Use with MetaMask

To get MetaMask to sign Gravity Bridge transactions, first navigate to a website which generates such transactions.
Next you will need to configure MetaMask with the connection information:

1. Open MetaMask's settings and select the Networks tab
1. Select "Add a network" and then "Add a network manually"
1. For Network name put "Gravity Bridge", for New RPC URL put "\<domain\>:\<port\>" (default http://localhost:8545), for Chain ID put "999999", and for Currency symbol put "GRAV"
1. Click Save and wait for MetaMask to connect to your sever

Navigate back to the website and start signing transactions!

### Note about MetaMask UI

The information produced by this server is largely incorrect and should not be relied upon!
For example, with any account you use the reported balance by MetaMask might be 1 GRAV despite the account not existing, or having a balance much higher than 1.