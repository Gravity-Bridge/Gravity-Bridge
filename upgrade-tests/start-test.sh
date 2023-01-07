#!/bin/bash

GRAVITY_HOME=${1:-data}
NODE_ID=${2-a079edb265cbde6f8eeecd6f22a57dfdf3ab7b34}

rm -rf $GRAVITY_HOME

cp -r "$GRAVITY_HOME-before-upgrade" $GRAVITY_HOME

gravity start --home $GRAVITY_HOME --p2p.persistent_peers $NODE_ID@0.0.0.0:26656