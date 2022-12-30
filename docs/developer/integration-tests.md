# Prerequisites

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

# Start the network in one terminal

gravity start --home data/validator1

## Test keys:

cat data/orchestrator-phrases
cat data/validator-eth-keys
cat data/validator-phrases

## Deploy new contract

npx ts-node contract-deployer.ts --cosmos-node="http://localhost:26657" --eth-node=https://rpc.ankr.com/eth_goerli --eth-privkey=0xbbfb76c92cd13796899f63dc6ead6d2420e8d0bc502d42bd5773c2d4b8897f08 --contract=artifacts/contracts/Gravity.sol/Gravity.json

Gravity contract on Goerli: 0xa49e040d7b8F045B090306C88aEF48955404B2e8

Dummy token on Goerli: 0xf48007ea0F3AA4d2A59DFb4473dd30f90488c8Ef

## Start Orchestrator

cargo run -p gbt -- --home data/.gbt/ --address-prefix oraib orchestrator --cosmos-grpc http://localhost:9090 --ethereum-rpc https://rpc.ankr.com/eth_goerli --fees 0uoraib --gravity-contract-address 0xa49e040d7b8F045B090306C88aEF48955404B2e8

## Send Dummy tokens from Goerli to gravity-test

cargo run -p gbt -- --home data/.gbt/ --address-prefix oraib client eth-to-cosmos --amount 0.00000000000000001 --token-contract-address 0xf48007ea0F3AA4d2A59DFb4473dd30f90488c8Ef --ethereum-rpc https://rpc.ankr.com/eth_goerli --destination "oraib1kvx7v59g9e8zvs7e8jm2a8w4mtp9ys2sjufdm4" --ethereum-key 0xbbfb76c92cd13796899f63dc6ead6d2420e8d0bc502d42bd5773c2d4b8897f08 --gravity-contract-address 0xa49e040d7b8F045B090306C88aEF48955404B2e8

gravity tx gravity send-to-eth 0xc9B6f87d637d4774EEB54f8aC2b89dBC3D38226b 9goerli-testnet0xf48007ea0F3AA4d2A59DFb4473dd30f90488c8Ef 1goerli-testnet0xf48007ea0F3AA4d2A59DFb4473dd30f90488c8Ef goerli-testnet --home data/validator1 -y --from validator1

# BSC testnet interaction

npx ts-node contract-deployer.ts --cosmos-node="http://localhost:26657" --eth-node=https://data-seed-prebsc-1-s1.binance.org:8545 --eth-privkey=0xbbfb76c92cd13796899f63dc6ead6d2420e8d0bc502d42bd5773c2d4b8897f08 --contract=artifacts/contracts/Gravity.sol/Gravity.json

Gravity contract on BSC testnet: 0xFab35f1C870923596A2f8F691F74b082A275D404

Dummy token on BSC testnet: 0xb0deA83b2360019979A5c5C6537478DB7Bf60CAE

cargo run -p gbt -- --home data/.gbt/ --address-prefix oraib orchestrator --cosmos-grpc http://localhost:9090 --ethereum-rpc https://data-seed-prebsc-1-s1.binance.org:8545 --fees 0uoraib --gravity-contract-address 0xFab35f1C870923596A2f8F691F74b082A275D404

## Add new chains

gravity tx gravity add-evm-chain "goerli network" foobar "add goerli network" 100000000uoraib "foobar" --from validator1 --home data/validator1/ -y

gravity tx gov vote 1 yes --from validator1 --home data/validator1/ -y