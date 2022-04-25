#!/bin/bash
# the script run inside the container for all-up-test.sh
NODES=$1
TEST_TYPE=$2
ALCHEMY_ID=$3
set -eux

bash /gravity/tests/container-scripts/setup-validators.sh $NODES
bash /gravity/tests/container-scripts/setup-ibc-validators.sh $NODES
bash /gravity/tests/container-scripts/run-testnet.sh $NODES $TEST_TYPE $ALCHEMY_ID &
bash /gravity/tests/container-scripts/setup-relayer.sh

# deploy the ethereum contracts
pushd /gravity/orchestrator/test_runner
DEPLOY_CONTRACTS=1 RUST_BACKTRACE=full RUST_LOG="INFO,relayer=DEBUG,orchestrator=DEBUG" PATH=$PATH:$HOME/.cargo/bin cargo run --release --bin test-runner

# Setup and run the IBC relayer in the background
echo "Running ibc relayer in the background, directing output to /ibc-relayer-logs"
RUN_IBC_RELAYER=1 RUST_BACKTRACE=full TEST_TYPE=$TEST_TYPE NO_GAS_OPT=1 RUST_LOG="INFO,relayer=DEBUG,orchestrator=DEBUG" PATH=$PATH:$HOME/.cargo/bin cargo run --release --bin test-runner &

bash /gravity/tests/container-scripts/integration-tests.sh $NODES $TEST_TYPE