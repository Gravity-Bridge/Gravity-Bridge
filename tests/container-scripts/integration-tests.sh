#!/bin/bash
NODES=$1
LOG_LEVEL=$2
TEST_TYPE=$3
set -eu

FILE=/contracts
if test -f "$FILE"; then
echo "Contracts already deployed, running tests"
else 
echo "Testnet is not started yet, please wait before running tests"
exit 0
fi 

set +e
killall -9 test-runner
set -e

pushd /gravity/orchestrator/test_runner

RUST_LOG="INFO"
set +u
if [[! -z "${LOG_LEVEL}"]]; then
    # Construct a comma separated list like ",module_name=DEBUG,..."
    MODULES=("orchestrator", "cosmos_gravity", "ethereum_gravity", "gravity_utils", "proto_build", "test_runner", "gravity_proto", "relayer", "gbt", "metrics_exporter")
    LOG_MODULES="" # Note: It will start with a comma
    for i in ${!myArray[@]}; do
        LOG_MODULES="${LOG_MODULES},${myArray[$i]}=${LOG_LEVEL}"
    done
    RUST_LOG="${RUST_LOG}${LOG_MODULES}"
fi
set -u

RUST_BACKTRACE=full TEST_TYPE=$TEST_TYPE RUST_LOG="$RUST_LOG" PATH=$PATH:$HOME/.cargo/bin cargo run --release --bin test-runner
