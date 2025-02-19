#!/bin/bash
TEST_TYPE=$1
set -eu

if [[ -z "${TEST_TYPE}" ]]; then
  echo "No TEST_TYPE provided, HAPPY_PATH should run"
fi

set +u
OPTIONAL_KEY=""
if [ ! -z $2 ];
    then OPTIONAL_KEY="$2"
fi
set -u

CONTAINER=$(docker ps | grep gravity_test_instance | awk '{print $1}')
echo "Waiting for container to start before starting the test"
while [ -z "$CONTAINER" ];
do
    CONTAINER=$(docker ps | grep gravity_test_instance | awk '{print $1}')
    sleep 1
done
echo "Container started, running tests"

# Run test entry point script
docker exec gravity_test_instance /bin/sh -c "pushd /gravity/ && tests/container-scripts/integration-tests.sh 1 $TEST_TYPE $OPTIONAL_KEY"
