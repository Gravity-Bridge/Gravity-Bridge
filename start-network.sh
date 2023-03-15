#!/bin/bash
set -eux
# first we start a genesis.json with validator 1
# validator 1 will also collect the gentx's once gnerated
GRAVITY_HOME=${GRAVITY_HOME:-$PWD/data}
VALIDATOR=${VALIDATOR:-validator1}
DATA_HOME=$GRAVITY_HOME/$VALIDATOR
STARTING_VALIDATOR_HOME="--home $DATA_HOME"

if [ ! -f $DATA_HOME/data/priv_validator_state.json ]
then
    echo '{"height":"0","round":0,"step":0}' > $DATA_HOME/data/priv_validator_state.json
fi

gravity start $STARTING_VALIDATOR_HOME

# GRAVITY_HOME=./upgrade-tests/data-local VALIDATOR=local ./start-network.sh