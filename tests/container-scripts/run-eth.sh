#!/bin/bash
# Starts the Ethereum testnet chain in the background

DOCKER_PATH="/gravity/"

if [[ -d "$GITHUB_WORKSPACE" ]]; then
    FOLDER_PATH="$GITHUB_WORKSPACE"
elif [[ -d "$DOCKER_PATH" ]]; then
    FOLDER_PATH="$DOCKER_PATH"
else
    echo "Error: Neither $GITHUB_WORKSPACE nor $DOCKER_PATH exists."
    exit 1
fi

# init the genesis block
geth --identity "GravityTestnet" \
--nodiscover \
--networkid 15 init $FOLDER_PATH/tests/assets/ETHGenesis.json

# etherbase is where rewards get sent
# private key for this address is 0xb1bab011e03a9862664706fc3bbaa1b16651528e5f0e7fbfcbfdd8be302a13e7
geth --identity "GravityTestnet" --nodiscover \
--networkid 15 \
--mine \
--http \
--http.addr="0.0.0.0" \
--http.vhosts="*" \
--http.corsdomain="*" \
--miner.threads=1 \
--nousb \
--verbosity=5 \
--miner.etherbase=0xBf660843528035a5A4921534E156a27e64B231fE &> /geth.log
