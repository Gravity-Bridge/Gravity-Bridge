# Prerequisites

If you want to install everything locally, you should follow the below prerequisites. They have been tested using Ubuntu 20.04.

## Install gravity binary

```bash
cd module/ && make install && cd ../
```

## Install Orchestrator

### Install cmake if you dont have it already

```
sudo apt install cmake
```

```
cd orchestrator/ && cargo build --all && cd ../
```

## Install NPM & build Solidity contracts

   Change directory into the `solidity` folder and run

   ```bash

   # Install JavaScript dependencies
   HUSKY_SKIP_INSTALL=1 npm install

   # Build the Gravity bridge Solidity contract, run this after making any changes
   npm run typechain
   ```

# Start the test

Our setup is a Gravity network with one validator and two Goerli forks serving as two different EVM chains. The `genesis.json` and other private key files of the network as well as two forks have been fixed in order to repeat the tests. We also created a Dummy ERC20 token with sufficient balances so that we can test the bridge transactions

```bash
gravity start --home data/validator1
```

## Test keys:

```
cat data/orchestrator-phrases
cat data/validator-eth-keys
cat data/validator-phrases
```

## Start the e2e testing networks (can be reused by restarting all)

```bash
docker-compose up -d

# Gravity contract on Goerli: 0xa49e040d7b8F045B090306C88aEF48955404B2e8

# Dummy token on Goerli: 0xf48007ea0F3AA4d2A59DFb4473dd30f90488c8Ef
```

## Start Orchestrator

Enter the orchestrator container:

```bash
docker-compose exec orchestrator bash
```

then,

```bash
BLOCK_TO_SEARCH=100 cargo run -p gbt -- --home /root/.gbt/ --address-prefix oraib orchestrator --cosmos-grpc http://gravity:9090 --ethereum-rpc http://first_fork:8545 --fees 0uoraib --gravity-contract-address 0xa49e040d7b8F045B090306C88aEF48955404B2e8
```

## Send Dummy tokens from Goerli to gravity-test

Inside the container, run:

```bash
BLOCK_TO_SEARCH=100 cargo run -p gbt -- --home /root/.gbt/ --address-prefix oraib client eth-to-cosmos --amount 0.00000000000000001 --token-contract-address 0xf48007ea0F3AA4d2A59DFb4473dd30f90488c8Ef --ethereum-rpc http://first_fork:8545 --destination "channel-0/orai1kvx7v59g9e8zvs7e8jm2a8w4mtp9ys2s9adp2k" --ethereum-key 0xbbfb76c92cd13796899f63dc6ead6d2420e8d0bc502d42bd5773c2d4b8897f08 --gravity-contract-address 0xa49e040d7b8F045B090306C88aEF48955404B2e8
```

## Send back Dummy tokens to Goerli testnet

In side the Gravity test network, run:

```bash
gravity tx gravity send-to-eth 0xc9B6f87d637d4774EEB54f8aC2b89dBC3D38226b 9goerli-testnet0xf48007ea0F3AA4d2A59DFb4473dd30f90488c8Ef 1goerli-testnet0xf48007ea0F3AA4d2A59DFb4473dd30f90488c8Ef goerli-testnet --home data/validator1 -y --from validator1
```

# Useful commands

```bash
## Deploy new contract
npx ts-node contract-deployer.ts --cosmos-node="http://localhost:26657" --eth-node=http://localhost:8545 --eth-privkey=0xbbfb76c92cd13796899f63dc6ead6d2420e8d0bc502d42bd5773c2d4b8897f08 --contract=artifacts/contracts/Gravity.sol/Gravity.json

# fork Goerli. 8218229 is a block after the block of gravity contract & dummy token deployment. By doing this, we can re-play the network
yarn hardhat node --fork https://rpc.ankr.com/eth_goerli --fork-block-number 8218229 --port 8545

# fork BSC testnet. 25896858 is a block after the block of gravity contract & dummy token deployment. By doing this, we can re-play the network
yarn hardhat node --fork https://data-seed-prebsc-1-s1.binance.org:8545 --fork-block-number 25896858 --port 7545

# confirm Dummy balance after transferring token:
npx ts-node scripts/get-dummy-balance.ts

# Add new evm chain
gravity tx gravity add-evm-chain "goerli network 2nd" "foobar" "421" "defaultgravityid" "add goerli network 2nd" 100000000uoraib "foobar" --from validator1 --home e2e/data/validator1/ -y --keyring-backend test --chain-id gravity-test -b block --gas 2000000
gravity tx gov vote 1 yes --from validator1 --home e2e/data/validator1/ -y
```

<!-- oraid tx ibc-transfer transfer transfer channel-0 oraib1kvx7v59g9e8zvs7e8jm2a8w4mtp9ys2sjufdm4 1orai --from validator -y -->