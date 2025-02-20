#!/bin/bash
# This script is used in Github actions CI to prep and run a testnet environment
TEST_TYPE=$2
set -eux
NODES=4
FILE_PATH="$GITHUB_WORKSPACE"

sudo apt-get update
sudo apt-get install -y git make gcc g++ iproute2 iputils-ping procps vim tmux net-tools htop tar jq npm libssl-dev perl rustc cargo wget

# only required for deployment script
npm install -g ts-node && npm install -g typescript
mkdir geth
pushd geth/
wget https://gethstore.blob.core.windows.net/builds/geth-linux-amd64-1.10.10-bb74230f.tar.gz
tar -xvf *
sudo mv **/geth /usr/bin/geth
popd

# Download the althea gaia fork as a IBC test chain
sudo wget https://github.com/althea-net/ibc-test-chain/releases/download/v9.1.2/gaiad-v9.1.2-linux-amd64 -O /usr/bin/gaiad

# Setup Hermes for IBC connections between chains
pushd /tmp/
wget https://github.com/informalsystems/hermes/releases/download/v1.7.0/hermes-v1.7.0-x86_64-unknown-linux-gnu.tar.gz
tar -xvf hermes-v1.7.0-x86_64-unknown-linux-gnu.tar.gz
sudo mv hermes /usr/bin/
popd

# make log dirs
sudo mkdir /ibc-relayer-logs
sudo touch /ibc-relayer-logs/hermes-logs
sudo touch /ibc-relayer-logs/channel-creation

pushd module/
GOPROXY=https://proxy.golang.org make
make install
sudo cp ~/go/bin/gravity /usr/bin/gravity
popd
pushd solidity/
HUSKY_SKIP_INSTALL=1 npm install
npm run typechain
ls -lah
ls -lah artifacts/
ls -lah artifacts/contracts/
pwd
popd

sudo bash tests/container-scripts/setup-validators.sh $NODES
sudo bash tests/container-scripts/setup-ibc-validators.sh $NODES
sudo bash tests/container-scripts/run-testnet.sh $NODES $TEST_TYPE

# deploy the ethereum contracts
pushd orchestrator/test_runner
DEPLOY_CONTRACTS=1 RUST_BACKTRACE=full RUST_LOG="INFO,relayer=DEBUG,orchestrator=DEBUG" PATH=$PATH:$HOME/.cargo/bin cargo run --release --bin test-runner
popd

# Create a setup complete flag file used by the integration tests
sudo mkdir /gravity
sudo touch /gravity/test-ready-to-run

bash tests/container-scripts/integration-tests.sh $NODES $TEST_TYPE