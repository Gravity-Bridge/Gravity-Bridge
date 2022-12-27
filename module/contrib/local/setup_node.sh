#!/bin/bash
set -eu

rm -rf $HOME/.gravity

gravity init --chain-id=testing local
gravity keys add validator --keyring-backend test 2>&1 | tee account.txt
gravity add-genesis-account validator 10000000000000000000stake --keyring-backend test
gravity gentx validator 1000000000stake 0xBEF9Ec1EB861A7c050528B17ce8Ee087941bB1AA oraib1329tg05k3snr66e2r9ytkv6hcjx6fkxc2zqphe --keyring-backend test --chain-id testing
gravity collect-gentxs
jq '.app_state.gov.voting_params.voting_period = "60s"' ~/.gravity/config/genesis.json > temp && mv temp ~/.gravity/config/genesis.json
