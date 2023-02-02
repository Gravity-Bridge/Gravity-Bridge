# Validator setup

## Clone repo

```bash
git clone https://github.com/oraichain/Gravity-Bridge.git

# checkout current production branch
git checkout v1.0.1

# enter module/ dir and build gravity binary
cd module/ && make install && cd ../
```

## Restore keys

```bash
gravity keys add $moniker --recover
gravity keys add $orchestrator --recover
gravity eth_keys add <your evm account private key>
```

## Start node

Init your node, use genesis.json file given by the team and start it with the given persistent peers. Wait until it is synced then proceed to the next step

## Create validator

You should have some oraib in your validator account for staking and tx fee.

> **_NOTE:_** Don't use more uoraib than you have!

```bash
gravity tx staking create-validator \
   --amount=<staking-amount>uoraib \
   --pubkey=$(gravity tendermint show-validator --home $GRAVITY_HOME) \
   --moniker=$moniker \
   --chain-id=$CHAIN_ID \
   --commission-rate="0.10" \
   --commission-max-rate="0.20" \
   --commission-max-change-rate="0.01" \
   --min-self-delegation="1" \
   --gas="auto" \
   --gas-prices="0uoraib" \
   --gas-adjustment=1.3 \
   --from=$moniker \
   -y
```

## Set Orchestrator

```bash
export validator=$(gravity keys show $moniker --bech val -a)

export orchestrator_address=$(gravity keys show $orchestrator --bech acc -a)

export ethereum_address=<your-ethereum-address>

gravity tx gravity set-orchestrator-address $validator $orchestrator_address $ethereum_address \
   --chain-id=$CHAIN_ID \
   --gas="200000" \
   --gas-prices="0uoraib" \
   --from=$moniker \
   -y
```

## Run Orchestrator

```yml
# docker-compose file

version: "3.3"
services:
  orchestrator:
    image: oraichain/foundation-oraibridge-orchestrator:0.0.1 # docker build -t oraichain/foundation-oraibridge-orchestrator:0.0.1 -f orchestrator/Dockerfile ./orchestrator
    # apk add make upx
    # upx --best --lzma
    working_dir: /workspace
    tty: true
    volumes:
      - ./orchestrator:/orchestrator
      - ./e2e/.gbt/:/root/.gbt
    entrypoint: tail -f /dev/null
```

Start the orchestrator inside docker

```bash

# goerli-testnet fork
BLOCK_TO_SEARCH=100 cargo run -p gbt -- --home /root/.gbt/ --address-prefix oraib orchestrator --cosmos-grpc http://<local-node-ip>:9090 --ethereum-rpc http://localhost:8545 --fees 0uoraib

# bsc mainnet fork

BLOCK_TO_SEARCH=100 cargo run -p gbt -- --home /root/.gbt/ --address-prefix oraib orchestrator --cosmos-grpc http://<local-node-ip>:9090 --ethereum-rpc http://localhost:7545 --fees 0uoraib
```
