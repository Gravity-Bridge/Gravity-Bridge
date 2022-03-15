#!/bin/bash
# Number of validators to start
NODES=$1
# what test to execute
TEST_TYPE=$2
ALCHEMY_ID=$3
set -eux

# This must sync with the --mount argument passed to docker in run-upgrade-test.sh
OLD_BINARY_LOCATION=/oldgravity

# Stop any currently running gravity and eth processes
pkill gravity || true # allowed to fail
pkill geth || true # allowed to fail

# Wipe filesystem changes
for i in $(seq 1 $NODES);
do
    rm -rf "/validator$i"
done


cd /gravity/module/
export PATH=$PATH:/usr/local/go/bin
make
make install
cd /gravity/
tests/container-scripts/setup-validators.sh $NODES
# Run the old binary
tests/container-scripts/run-testnet.sh $NODES $OLD_BINARY_LOCATION $TEST_TYPE $ALCHEMY_ID

# deploy the ethereum contracts
pushd /gravity/orchestrator/test_runner
DEPLOY_CONTRACTS=1 RUST_BACKTRACE=full TEST_TYPE=$TEST_TYPE NO_GAS_OPT=1 RUST_LOG="INFO,relayer=DEBUG,orchestrator=DEBUG" PATH=$PATH:$HOME/.cargo/bin cargo run --release --bin test-runner
popd

# Prompt for upgrade
read -p "Old gravity binary running: Press Return once ready to switch to the new binary..."

# Run the new binary
pkill gravity || true # allowed to fail
tests/container-scripts/run-testnet.sh $NODES $TEST_TYPE $ALCHEMY_ID


# This keeps the script open to prevent Docker from stopping the container
# immediately if the nodes are killed by a different process
read -p "Press Return to Close...(don't press enter if you want Docker to keep running)"
