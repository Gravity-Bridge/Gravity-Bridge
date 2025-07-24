#!/bin/bash
set -eux
# your gaiad binary name
BIN=gravity

NODES=$1
set +u
TEST_TYPE=$2
ALCHEMY_ID=$3

DOCKER_PATH="/gravity"

if [[ -d "$GITHUB_WORKSPACE" ]]; then
    FOLDER_PATH="$GITHUB_WORKSPACE"
elif [[ -d "$DOCKER_PATH" ]]; then
    FOLDER_PATH="$DOCKER_PATH"
else
    echo "Error: Neither $GITHUB_WORKSPACE nor $DOCKER_PATH exists."
    exit 1
fi

if [[ ! -z ${OLD_BINARY_LOCATION} ]]; then
    BIN=$OLD_BINARY_LOCATION
fi
set -u
# Setup the gravity test chain (chain id gravity-test-1) using gravity as the binary
# Creates $NODES number of validators, with their home directories at /validator#
for i in $(seq 1 $NODES);
do
    # add this ip for loopback dialing
    ip addr add 7.7.7.$i/32 dev eth0 || true # allowed to fail

    GAIA_HOME="--home /validator$i"
    # this implicitly caps us at ~6000 nodes for this sim
    # note that we start on 26656 the idea here is that the first
    # node (node 1) is at the expected contact address from the gentx
    # faciliating automated peer exchange
    if [[ "$i" -eq 1 ]]; then
        # node one gets localhost so we can easily shunt these ports
        # to the docker host
        RPC_ADDRESS="--rpc.laddr tcp://0.0.0.0:26657"
        GRPC_ADDRESS="--grpc.address 0.0.0.0:9090"
        # TODO: Remove this after we upgrade past SDK v0.50, at which point this will break upgrade tests
        if [[ "$BIN" != "gravity" ]]; then
            echo "Setting --grpc-web.address for old upgrade test compatibility"
            GRPC_ADDRESS="${GRPC_ADDRESS} --grpc-web.address 0.0.0.0:9092"
        fi

        sed -i 's/enable-unsafe-cors = false/enable-unsafe-cors = true/g' /validator$i/config/app.toml
        sed -i 's/enabled-unsafe-cors = false/enabled-unsafe-cors = true/g' /validator$i/config/app.toml
        sed -i 's/enable = false/enable = true/g' /validator$i/config/app.toml #enables more than we want, but will work for now
    else
        # move these to another port and address, not becuase they will
        # be used there, but instead to prevent them from causing problems
        # you also can't duplicate the port selection against localhost
        # for reasons that are not clear to me right now.
        RPC_ADDRESS="--rpc.laddr tcp://7.7.7.$i:26658"
        GRPC_ADDRESS="--grpc.address 7.7.7.$i:9091"
        # TODO: Remove this after we upgrade past SDK v0.50, at which point this will break upgrade tests
        if [[ "$BIN" != "gravity" ]]; then
            echo "Setting --grpc-web.address for old upgrade test compatibility"
            GRPC_ADDRESS="${GRPC_ADDRESS} --grpc-web.address 7.7.7.$i:9093"
        fi
    fi
    LISTEN_ADDRESS="--address tcp://7.7.7.$i:26655"
    P2P_ADDRESS="--p2p.laddr tcp://7.7.7.$i:26656"
    LOG_LEVEL="--log_level info"
    INVARIANTS_CHECK="--inv-check-period 1"
    MIN_GAS_PRICES="--minimum-gas-prices 0stake"
    ARGS="$GAIA_HOME $LISTEN_ADDRESS $RPC_ADDRESS $GRPC_ADDRESS $LOG_LEVEL $INVARIANTS_CHECK $P2P_ADDRESS $MIN_GAS_PRICES"
    $BIN $ARGS start &> /validator$i/logs &
done

# Setup the IBC test chain (chain id ibc-test-1) using gaiad as the binary
# Creates the same number of validators as the gravity chain above, with their home directories at /ibc-validator#
BIN=gaiad
for i in $(seq 1 $NODES);
do
    ip addr add 7.7.8.$i/32 dev eth0 || true # allowed to fail

    GAIA_HOME="--home /ibc-validator$i"
    if [[ "$i" -eq 1 ]]; then
        # node one gets localhost so we can easily shunt these ports
        # to the docker host
        RPC_ADDRESS="--rpc.laddr tcp://0.0.0.0:27657"
        GRPC_ADDRESS="--grpc.address 0.0.0.0:9190"
        # Must remap the grpc-web address because it conflicts with what we want to use
        GRPC_WEB_ADDRESS="--grpc-web.address 0.0.0.0:9192"
    else
        RPC_ADDRESS="--rpc.laddr tcp://7.7.8.$i:26658"
        GRPC_ADDRESS="--grpc.address 7.7.8.$i:9091"
        # Must remap the grpc-web address because it conflicts with what we want to use
        GRPC_WEB_ADDRESS="--grpc-web.address 7.7.8.$i:9093"
    fi
    LISTEN_ADDRESS="--address tcp://7.7.8.$i:26655"
    P2P_ADDRESS="--p2p.laddr tcp://7.7.8.$i:26656"
    LOG_LEVEL="--log_level info"
    ARGS="$GAIA_HOME $LISTEN_ADDRESS $RPC_ADDRESS $GRPC_ADDRESS $GRPC_WEB_ADDRESS $LOG_LEVEL $P2P_ADDRESS"
    $BIN $ARGS start &> /ibc-validator$i/logs &
done

# let the cosmos chain settle before starting eth as it
# consumes a lot of processing power
sleep 10
# GETH and TEST_TYPE may be unbound
set +u

# Starts a hardhat RPC backend that is based off of a fork of Ethereum mainnet. This is useful in that we take
# over the account of a major Uniswap liquidity provider and from there we can test many things that are infeasible
# to do with a Geth backend, simply becuase reproducting that state on our testnet would be far too complex to consider
# The tradeoff here is that hardhat is an ETH dev environment and not an actual ETH implementation, as such the outputs
# may be different. These two tests have different fork block heights they rely on
if [[ $TEST_TYPE == *"ARBITRARY_LOGIC"* ]] && [[ ! -z ${ALCHEMY_ID} ]]; then
    export ALCHEMY_ID=$ALCHEMY_ID
    pushd ${FOLDER_PATH}/solidity
    npm run solidity_test_fork &
    popd
elif [[ $TEST_TYPE == *"RELAY_MARKET"* ]] && [[ ! -z ${ALCHEMY_ID} ]]; then
    export ALCHEMY_ID=$ALCHEMY_ID
    pushd ${FOLDER_PATH}/solidity
    npm run evm_fork &
    popd
# This starts a hardhat test environment with no pre-seeded state, faster to run, not accurate
elif [[ ! -z "$HARDHAT" ]]; then
    pushd ${FOLDER_PATH}/solidity
    npm run evm &
    popd
# This starts the Geth backed testnet with no pre-seeded in state.
# Geth is what we run in CI and in general, but developers frequently
# perfer a faster experience provided by HardHat, also Mac's do not
# work correctly with the Geth backend, there is some issue where the Docker VM on Mac platforms can't get
# the right number of cpu cores and Geth goes crazy consuming all the processing power, on the other hand
# hardhat doesn't work for some tests that depend on transactions waiting for blocks, so Geth is the default
else
    if [[ -z "$(ps -e | grep geth)" ]]; then # Only run-eth if it's not running, which it would be with upgrade tests
      bash ${FOLDER_PATH}/tests/container-scripts/run-eth.sh &
    fi
fi
sleep 10
