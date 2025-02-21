#!/bin/bash
# the script run inside the container for all-up-test.sh
NODES=$1
TEST_TYPE=$2
ALCHEMY_ID=$3
set -eux

bash /gravity/tests/container-scripts/setup-validators.sh $NODES
bash /gravity/tests/container-scripts/setup-ibc-validators.sh $NODES
bash /gravity/tests/container-scripts/run-testnet.sh $NODES $TEST_TYPE "/gravity/" $ALCHEMY_ID &

# deploy the ethereum contracts
pushd /gravity/orchestrator/test_runner
DEPLOY_CONTRACTS=1 RUST_BACKTRACE=full RUST_LOG="INFO,relayer=DEBUG,orchestrator=DEBUG" PATH=$PATH:$HOME/.cargo/bin cargo run --release --bin test-runner

echo "Running ibc relayer in the background, directing output to /ibc-relayer-logs"

# Create a setup complete flag file used by the integration tests
touch /gravity/test-ready-to-run

bash /gravity/tests/container-scripts/integration-tests.sh $NODES $TEST_TYPE