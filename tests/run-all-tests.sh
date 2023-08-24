#!/bin/bash
set -eux
# the directory of this script, useful for allowing this script
# to be run with any PWD
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $DIR
bash all-up-test.sh # Happy path
export NO_IMAGE_BUILD=1
bash all-up-test.sh VALIDATOR_OUT
bash all-up-test.sh VALSET_STRESS
bash all-up-test.sh BATCH_STRESS
bash all-up-test.sh HAPPY_PATH_V2
bash all-up-test.sh ORCHESTRATOR_KEYS
bash all-up-test.sh VALSET_REWARDS
bash all-up-test.sh EVIDENCE
bash all-up-test.sh TXCANCEL
bash all-up-test.sh INVALID_EVENTS
bash all-up-test.sh UNHALT_BRIDGE
bash all-up-test.sh PAUSE_BRIDGE
bash all-up-test.sh DEPOSIT_OVERFLOW
bash all-up-test.sh ETHEREUM_BLACKLIST
bash all-up-test.sh AIRDROP_PROPOSAL
bash all-up-test.sh SIGNATURE_SLASHING
bash all-up-test.sh SLASHING_DELEGATION
bash all-up-test.sh IBC_METADATA
bash all-up-test.sh ERC721_HAPPY_PATH
bash all-up-test.sh IBC_AUTO_FORWARD
bash all-up-test.sh ETHEREUM_KEYS
bash all-up-test.sh BATCH_TIMEOUT
bash all-up-test.sh VESTING
bash all-up-test.sh SEND_TO_ETH_FEES
if [ ! -z "$ALCHEMY_ID" ]; then
    bash all-up-test.sh RELAY_MARKET $ALCHEMY_ID
    bash all-up-test.sh ARBITRARY_LOGIC $ALCHEMY_ID
else
    echo "Alchemy API key not set under variable ALCHEMY_ID, not running ARBITRARY_LOGIC nor RELAY_MARKET"
fi
echo "All tests succeeded!"
