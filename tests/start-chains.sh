#!/bin/bash
TEST_TYPE=$1
ALCHEMY_ID=$2
set -eux

# the directory of this script, useful for allowing this script
# to be run with any PWD
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Remove existing container instance
set +e
docker rm -f gravity_test_instance
set -e

NODES=4

pushd $DIR/../

# setup for Mac M1 compatibility
PLATFORM_CMD=""
if [[ "$OSTYPE" == "darwin"* ]]; then
    if [[ -n $(sysctl -a | grep brand | grep "M1") ]]; then
       echo "Setting --platform=linux/amd64 for Mac M1 compatibility"
       PLATFORM_CMD="--platform=linux/amd64"; fi
fi
# Run new test container instance
docker run --name gravity_test_instance $PLATFORM_CMD --mount type=bind,source="$(pwd)"/,target=/gravity --cap-add=NET_ADMIN -p 9090:9090 -p 26657:26657 -p 1317:1317 -p 8545:8545 -it gravity-base /bin/bash /gravity/tests/container-scripts/reload-code.sh $NODES $TEST_TYPE $ALCHEMY_ID
