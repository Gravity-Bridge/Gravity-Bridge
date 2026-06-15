#!/bin/bash
set -eux

# Script to build a single container image and run all test cases in parallel
# using docker-compose. This allows rapid testing without rebuilding the image
# for each test. Set NO_IMAGE_BUILD=1 to skip the image build when iterating.

# the directory of this script, useful for allowing this script
# to be run with any PWD
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Build the container image containing various system deps.
# Also builds Gravity once to cache Go deps. This image must be rebuilt any
# time you change the repo, unless you set NO_IMAGE_BUILD=1.
set +u
if [[ -z ${NO_IMAGE_BUILD} ]]; then
    bash "$DIR/build-container.sh"
fi
set -u

# Setup for Mac Apple Silicon compatibility
if [[ "$OSTYPE" == "darwin"* ]]; then
    if [[ -n $(sysctl -a | grep brand | grep "Apple") ]]; then
        echo "Setting DOCKER_DEFAULT_PLATFORM=linux/amd64 for Mac Apple Silicon compatibility"
        export DOCKER_DEFAULT_PLATFORM="linux/amd64"
    fi
fi

COMPOSE_FILE="$DIR/docker-compose-tests.yml"
COMPOSE_PROJECT="gravity-local-tests"

# Maximum number of test containers to run simultaneously.
# Each test container runs its own full chain stack (cosmos + geth + orchestrator),
# so running too many at once exhausts RAM/CPU quickly. Override with MAX_PARALLEL=N.
set +u
MAX_PARALLEL="${MAX_PARALLEL:-16}"
set -u

# Define all standard test types; must stay in sync with docker-compose-tests.yml
declare -a TEST_TYPES=(
    "HAPPY_PATH"
    "VALIDATOR_OUT"
    "VALSET_STRESS"
    "BATCH_STRESS"
    "HAPPY_PATH_V2"
    "ORCHESTRATOR_KEYS"
    "VALSET_REWARDS"
    "EVIDENCE"
    "TXCANCEL"
    "INVALID_EVENTS"
    "UNHALT_BRIDGE"
    "PAUSE_BRIDGE"
    "DEPOSIT_OVERFLOW"
    "ETHEREUM_BLACKLIST"
    "AIRDROP_PROPOSAL"
    "SIGNATURE_SLASHING"
    "SLASHING_DELEGATION"
    "IBC_METADATA"
    "ERC721_HAPPY_PATH"
    "IBC_AUTO_FORWARD"
    "ETHEREUM_KEYS"
    "BATCH_TIMEOUT"
    "VESTING"
    "SEND_TO_ETH_FEES"
    "V2_HAPPY_PATH_NATIVE"
    "ICA_HOST_HAPPY_PATH"
    "INFLATION_KNOCKDOWN"
    "EIP712"
    "AUCTION_STATIC"
    "AUCTION_RANDOM"
    "AUCTION_INVALID_PARAMS"
    "AUCTION_DISABLE"
    "FEEGRANT"
    "PARAM_CHANGE_PARANOIA"
    "ATTESTATION_CLAIM_VOTING"
    "ATTESTATION_HASH_INTEGRITY"
    "DENOM_VALIDATION"
    "COSMOS_BRIDGEABLE_TOKENS"
)

# Alchemy-dependent tests run only when ALCHEMY_ID is set
declare -a ALCHEMY_TESTS=(
    "RELAY_MARKET"
    "ARBITRARY_LOGIC"
)

# Clean up containers and volumes left over from previous runs
cleanup_old_tests() {
    echo "Cleaning up old test containers..."
    set +e
    docker-compose -f "$COMPOSE_FILE" -p "$COMPOSE_PROJECT" down -v 2>/dev/null
    docker-compose -f "$COMPOSE_FILE" -p "$COMPOSE_PROJECT" --profile alchemy down -v 2>/dev/null
    set -e
}

if [[ ! -f "$COMPOSE_FILE" ]]; then
    echo "Error: $COMPOSE_FILE not found."
    exit 1
fi

cleanup_old_tests

echo ""
echo "=== Gravity Bridge Local Test Suite ==="
echo "Running tests in batches of $MAX_PARALLEL..."
echo "Test types: ${TEST_TYPES[*]}"
echo ""
echo "Logs available via: docker-compose -f $COMPOSE_FILE -p $COMPOSE_PROJECT logs -f"
echo ""

# Append Alchemy tests to the run list when ALCHEMY_ID is available
set +u
declare -a ALL_TEST_TYPES=("${TEST_TYPES[@]}")
if [[ -n "${ALCHEMY_ID}" ]]; then
    export ALCHEMY_ID
    ALL_TEST_TYPES+=("${ALCHEMY_TESTS[@]}")
else
    echo "ALCHEMY_ID not set; skipping RELAY_MARKET and ARBITRARY_LOGIC"
fi
set -u

# Run tests in batches of MAX_PARALLEL, waiting for each batch to complete
# before starting the next. docker-compose up <svc1> <svc2> ... runs only
# the named services and exits when all of them have stopped.
batch_start=0
batch_num=0
while [[ $batch_start -lt ${#ALL_TEST_TYPES[@]} ]]; do
    batch=("${ALL_TEST_TYPES[@]:$batch_start:$MAX_PARALLEL}")
    batch_num=$(( batch_num + 1 ))
    echo "--- Batch $batch_num: ${batch[*]} ---"

    # Convert test type names to compose service names (test-<lowercase>)
    services=()
    for t in "${batch[@]}"; do
        services+=("test-${t,,}")
    done

    set +u
    if [[ -n "${ALCHEMY_ID}" ]]; then
        docker-compose -f "$COMPOSE_FILE" \
                       -p "$COMPOSE_PROJECT" \
                       --profile alchemy \
                       up "${services[@]}"
    else
        docker-compose -f "$COMPOSE_FILE" \
                       -p "$COMPOSE_PROJECT" \
                       up "${services[@]}"
    fi
    set -u

    batch_start=$(( batch_start + MAX_PARALLEL ))
done

echo ""
echo "=== Collecting Test Results ==="

FAILED_TESTS=()
PASSED_TESTS=()
SKIPPED_TESTS=()

for test_type in "${ALL_TEST_TYPES[@]}"; do
    container_name="gravity-test-${test_type,,}"
    if docker inspect "$container_name" &>/dev/null; then
        exit_code=$(docker inspect "$container_name" --format='{{.State.ExitCode}}')
        if [[ "$exit_code" == "0" ]]; then
            PASSED_TESTS+=("$test_type")
        else
            FAILED_TESTS+=("$test_type")
        fi
    else
        FAILED_TESTS+=("$test_type")
    fi
done

set +u
if [[ -z "${ALCHEMY_ID}" ]]; then
    SKIPPED_TESTS+=("${ALCHEMY_TESTS[@]}")
fi
set -u

TOTAL=$(( ${#TEST_TYPES[@]} + ${#ALCHEMY_TESTS[@]} ))

echo ""
echo "╔════════════════════════════════════════════════════════╗"
echo "║              TEST EXECUTION COMPLETE                   ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""

if [ ${#PASSED_TESTS[@]} -gt 0 ]; then
    echo "PASSED TESTS (${#PASSED_TESTS[@]}):"
    printf '  + %s\n' "${PASSED_TESTS[@]}"
    echo ""
fi

if [ ${#SKIPPED_TESTS[@]} -gt 0 ]; then
    echo "SKIPPED TESTS (${#SKIPPED_TESTS[@]}) - ALCHEMY_ID not set:"
    printf '  - %s\n' "${SKIPPED_TESTS[@]}"
    echo ""
fi

if [ ${#FAILED_TESTS[@]} -gt 0 ]; then
    echo "FAILED TESTS (${#FAILED_TESTS[@]}):"
    printf '  x %s\n' "${FAILED_TESTS[@]}"
    echo ""
fi

echo "════════════════════════════════════════════════════════"
echo "SUMMARY: ${#PASSED_TESTS[@]} passed, ${#FAILED_TESTS[@]} failed, ${#SKIPPED_TESTS[@]} skipped out of $TOTAL total"
echo "════════════════════════════════════════════════════════"
echo ""

if [ ${#FAILED_TESTS[@]} -gt 0 ]; then
    echo "View detailed logs:"
    echo "  All tests:"
    echo "    docker-compose -f $COMPOSE_FILE -p $COMPOSE_PROJECT logs"
    echo ""
    echo "  Specific failed tests:"
    for test_type in "${FAILED_TESTS[@]}"; do
        echo "    docker logs gravity-test-${test_type,,}"
    done
    echo ""
    exit 1
fi

echo "All tests passed successfully!"
exit 0
