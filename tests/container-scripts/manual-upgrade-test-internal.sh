#!/bin/bash
set -eux
# Number of validators to start
NODES=$1
# old binary version to run
OLD_VERSION=$2

chmod -R 777 /root/

set +e
rm /gravity/test-ready-to-run
set -e

echo "Downloading old gravity version at https://github.com/Gravity-Bridge/Gravity-Bridge/releases/download/${OLD_VERSION}/gravity-linux-amd64"
wget https://github.com/Gravity-Bridge/Gravity-Bridge/releases/download/${OLD_VERSION}/gravity-linux-amd64
mv gravity-linux-amd64 oldgravity
# Make old gravity executable
chmod +x oldgravity

export OLD_BINARY_LOCATION=/oldgravity

# Prepare the contracts for later deployment
pushd /gravity/solidity/
HUSKY_SKIP_INSTALL=1 npm install
npm run typechain

cd /gravity/module/
export PATH=$PATH:/usr/local/go/bin
make
make install
cd /gravity/
tests/container-scripts/setup-validators.sh $NODES
tests/container-scripts/setup-ibc-validators.sh $NODES

# Run the old binary
tests/container-scripts/run-testnet.sh $NODES


# deploy the ethereum contracts
pushd /gravity/orchestrator/test_runner
DEPLOY_CONTRACTS=1 RUST_BACKTRACE=full NO_GAS_OPT=1 RUST_LOG="INFO,relayer=DEBUG,orchestrator=DEBUG" PATH=$PATH:$HOME/.cargo/bin cargo run --release --bin test-runner
popd

touch /gravity/test-ready-to-run

# This allows the tester to run the first part of the test
# immediately if the nodes are killed by a different process

read -p "Old binary is running, use tests/run-tests.sh to run tests/populate pre-upgrade state! Hit Enter to continue to part 2..."


unset OLD_BINARY_LOCATION
# Run the new binary
pkill gravity || true # allowed to fail
tests/container-scripts/run-testnet.sh $NODES

# This allows the tester to run the first part of the test
# immediately if the nodes are killed by a different process

read -p "New binary is running, use tests/run-tests.sh to run tests on the upgraded chain! Hit Enter to close the container and end all tests..."