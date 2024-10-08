#!/bin/bash
NODES=$1
TEST_TYPE=$2
set -eu

FILE=/tmp/contracts
if test -f "$FILE"; then
echo "Contracts already deployed, running tests"
else 
echo "Testnet is not started yet, please wait before running tests"
exit 1
fi 

set +e
killall -9 test-runner
set -e

GITHUB_ACTIONS_PATH="/home/runner/work/Gravity-Bridge/Gravity-Bridge/"
DOCKER_PATH="/gravity/"

if [[ -d "$GITHUB_ACTIONS_PATH" ]]; then
    FOLDER_PATH="$GITHUB_ACTIONS_PATH"
elif [[ -d "$DOCKER_PATH" ]]; then
    FOLDER_PATH="$DOCKER_PATH"
else
    echo "Error: Neither $GITHUB_ACTIONS_PATH nor $DOCKER_PATH exists."
    exit 1
fi


pushd $FOLDER_PATH/orchestrator/test_runner
RUST_BACKTRACE=full TEST_TYPE=$TEST_TYPE RUST_LOG=INFO PATH=$PATH:$HOME/.cargo/bin cargo run --release --bin test-runner
