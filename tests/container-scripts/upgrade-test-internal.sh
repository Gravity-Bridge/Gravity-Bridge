#!/bin/bash
set -eux
# Number of validators to start
NODES=$1
# old binary version to run
OLD_VERSION=$2

echo "Downloading old gravity version at https://github.com/Gravity-Bridge/Gravity-Bridge/releases/download/${OLD_VERSION}/gravity-linux-amd64"
wget https://github.com/Gravity-Bridge/Gravity-Bridge/releases/download/${OLD_VERSION}/gravity-linux-amd64
mv gravity-linux-amd64 oldgravity
# Make old gravity executable
chmod +x oldgravity

# This must sync with the --mount argument passed to docker in run-upgrade-test.sh
export OLD_BINARY_LOCATION=/oldgravity

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
tests/container-scripts/run-testnet.sh $NODES

# deploy the ethereum contracts
pushd /gravity/orchestrator/test_runner
DEPLOY_CONTRACTS=1 RUST_BACKTRACE=full NO_GAS_OPT=1 RUST_LOG="INFO,relayer=DEBUG,orchestrator=DEBUG" PATH=$PATH:$HOME/.cargo/bin cargo run --release --bin test-runner
popd

# Run the pre-upgrade tests
pushd /gravity/
tests/container-scripts/integration-tests.sh $NODES UPGRADE_PART_1
popd

# Run the new binary
pkill gravity || true # allowed to fail
tests/container-scripts/run-testnet.sh $NODES

# Run the post-upgrade test
pushd /gravity/
tests/container-scripts/integration-tests.sh $NODES UPGRADE_PART_2
popd
