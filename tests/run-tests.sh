#!/bin/bash
TEST_TYPE=$1
set -e

if [[ -z "${LOG_LEVEL}" ]]; then
  echo "Setting log level to the default of INFO"
  LOG_LEVEL="INFO"
fi

if [[ -z "${TEST_TYPE}" ]]; then
  echo "No TEST_TYPE provided, HAPPY_PATH should run"
fi

OPTIONAL_KEY=""
if [ ! -z $2 ];
    then OPTIONAL_KEY="$2"
fi

# Run test entry point script
docker exec gravity_test_instance /bin/sh -c "pushd /gravity/ && tests/container-scripts/integration-tests.sh 1 $TEST_TYPE $LOG_LEVEL $OPTIONAL_KEY"
