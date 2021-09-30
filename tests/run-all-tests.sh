#!/bin/bash
set -eux
# the directory of this script, useful for allowing this script
# to be run with any PWD
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $DIR
bash all-up-test.sh
bash all-up-test.sh VALSET_STRESS
bash all-up-test.sh VALSET_REWARDS
bash all-up-test.sh VALIDATOR_OUT
bash all-up-test.sh BATCH_STRESS
bash all-up-test.sh HAPPY_PATH_V2
bash all-up-test.sh ORCHESTRATOR_KEYS
bash all-up-test.sh EVIDENCE
bash all-up-test.sh TXCANCEL
bash all-up-test.sh INVALID_EVENTS
if [ ! -z "$ALCHEMY_ID" ]; then
    bash all-up-test.sh ARBITRARY_LOGIC $ALCHEMY_ID
    bash all-up-test.sh RELAY_MARKET $ALCHEMY_ID
else
    echo "Alchemy API key not set under variable ALCHEMY_ID, not running ARBITRARY_LOGIC nor RELAY_MARKET"
fi
bash all-up-test.sh UNHALT_BRIDGE
echo "All tests succeeded!"
