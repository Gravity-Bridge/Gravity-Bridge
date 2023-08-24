#!/bin/bash
# Number of validators to start
NODES=$1
# what test to execute
TEST_TYPE=$2
ALCHEMY_ID=$3
set -eux

# Stop any currently running gravity and eth processes
pkill gravityd || true # allowed to fail
pkill geth || true # allowed to fail

# Wipe filesystem changes
for i in $(seq 1 $NODES);
do
    rm -rf "/validator$i"
done


pushd /gravity/module/
export PATH=$PATH:/usr/local/go/bin
make install-no-verify
popd
pushd /gravity/
tests/container-scripts/setup-validators.sh $NODES
tests/container-scripts/setup-ibc-validators.sh $NODES
tests/container-scripts/run-testnet.sh $NODES $TEST_TYPE $ALCHEMY_ID
popd

# Setup relayer files to avoid permissions issues later
set +e
mkdir /ibc-relayer-logs
touch /ibc-relayer-logs/hermes-logs
touch /ibc-relayer-logs/channel-creation
set -e

# deploy the ethereum contracts
pushd /gravity/orchestrator/test_runner
DEPLOY_CONTRACTS=1 RUST_BACKTRACE=full TEST_TYPE=$TEST_TYPE NO_GAS_OPT=1 RUST_LOG="INFO,relayer=DEBUG,orchestrator=DEBUG" PATH=$PATH:$HOME/.cargo/bin cargo run --release --bin test-runner
popd

# This keeps the script open to prevent Docker from stopping the container
# immediately if the nodes are killed by a different process
read -p "Press Return to Close...(don't press enter if you want Docker to keep running)"
