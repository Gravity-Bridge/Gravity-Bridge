#!/bin/bash
set -eux
# your gaiad binary name
BIN=gravity

CHAIN_ID=${CHAIN_ID:-"gravity-test"}

ALLOCATION="10000000000uoraib,10000000000000000000000000uairi"

# first we start a genesis.json with validator 1
# validator 1 will also collect the gentx's once gnerated
VALIDATOR=${VALIDATOR:-validator1}
ORCHESTRATOR=${ORCHESTRATOR:-orchestrator1}
STARTING_VALIDATOR_HOME="--home $GRAVITY_HOME/$VALIDATOR"


# clear previous setup
rm -rf $GRAVITY_HOME/$VALIDATOR/

# todo add git hash to chain name
$BIN init $STARTING_VALIDATOR_HOME --chain-id=$CHAIN_ID $VALIDATOR


## Modify generated genesis.json to our liking by editing fields using jq
## we could keep a hardcoded genesis file around but that would prevent us from
## testing the generated one with the default values provided by the module.

# add in denom metadata for both native tokens
jq '.app_state.bank.denom_metadata += [{"name": "ORAIB Token", "symbol": "ORAIB", "base": "oraib", display: "oraib", "description": "A native staking & minting token", "denom_units": [{"denom": "oraib", "exponent": 0}, {"denom": "uoraib", "exponent": 6}, {"denom": "uairi", "exponent": 18}]}]' $GRAVITY_HOME/$VALIDATOR/config/genesis.json > /genesis.json

# Sets up an arbitrary number of validators on a single machine by manipulating
# the --home parameter on gaiad
NODE_HOME="--home $GRAVITY_HOME/$VALIDATOR"
GENTX_HOME="--home-client $GRAVITY_HOME/$VALIDATOR"
ARGS="$NODE_HOME --keyring-backend test"

# Generate a validator key, orchestrator key, and eth key for each validator
# it means that data is not empty and there is already keyring-test to run
# can do manually via --recover flags
if ! [ -f "$GRAVITY_HOME/validator-phrases" ]; then
    $BIN keys add $ARGS $VALIDATOR 2>> $GRAVITY_HOME/validator-phrases
else 
    PASS=$(tail -1 $GRAVITY_HOME/validator-phrases)
    (echo $PASS; echo $PASS) | $BIN keys add $ARGS $VALIDATOR --recover
fi 
if ! [ -f "$GRAVITY_HOME/orchestrator-phrases" ]; then
    $BIN keys add $ARGS $ORCHESTRATOR 2>> $GRAVITY_HOME/orchestrator-phrases
else 
    PASS=$(tail -1 $GRAVITY_HOME/orchestrator-phrases)
    (echo $PASS; echo $PASS) | $BIN keys add $ARGS $ORCHESTRATOR --recover
fi 
# eth keys maybe existed
if ! [ -f "$GRAVITY_HOME/validator-eth-keys" ]; then
    $BIN eth_keys add $ARGS >> $GRAVITY_HOME/validator-eth-keys
else 
    $BIN eth_keys add $ARGS $(head -1 $GRAVITY_HOME/validator-eth-keys | awk -F': ' 'NR=2{gsub(/^[ \t]+| [ \t]+$/,"");print $2}')    
fi

VALIDATOR_KEY=$($BIN keys show $VALIDATOR -a $ARGS)
ORCHESTRATOR_KEY=$($BIN keys show $ORCHESTRATOR -a $ARGS)
# move the genesis in
mkdir -p $GRAVITY_HOME/$VALIDATOR/config/
mv /genesis.json $GRAVITY_HOME/$VALIDATOR/config/genesis.json
$BIN add-genesis-account $ARGS $VALIDATOR_KEY $ALLOCATION
$BIN add-genesis-account $ARGS $ORCHESTRATOR_KEY $ALLOCATION
# move the genesis back out
mv $GRAVITY_HOME/$VALIDATOR/config/genesis.json /genesis.json


cp /genesis.json $GRAVITY_HOME/$VALIDATOR/config/genesis.json
NODE_HOME="--home $GRAVITY_HOME/$VALIDATOR"
ARGS="$NODE_HOME --keyring-backend test"
ORCHESTRATOR_KEY=$($BIN keys show $ORCHESTRATOR -a $ARGS)
ETHEREUM_KEY=$(grep address $GRAVITY_HOME/validator-eth-keys | sed -n 1p | sed 's/.*://')
# the /8 containing 7.7.7.7 is assigned to the DOD and never routable on the public internet
# we're using it in private to prevent gaia from blacklisting it as unroutable
# and allow local pex
$BIN gentx $ARGS $NODE_HOME --moniker $VALIDATOR --chain-id=$CHAIN_ID --ip 7.7.7.1 $VALIDATOR 500000000uoraib $ETHEREUM_KEY $ORCHESTRATOR_KEY

$BIN collect-gentxs $STARTING_VALIDATOR_HOME
GENTXS=$(ls $GRAVITY_HOME/$VALIDATOR/config/gentx | wc -l)
cp $GRAVITY_HOME/$VALIDATOR/config/genesis.json /genesis.json
echo "Collected $GENTXS gentx"

# put the now final genesis.json into the correct folders
cp /genesis.json $GRAVITY_HOME/$VALIDATOR/config/genesis.json

sed -i "s/enabled-unsafe-cors *= *.*/enabled-unsafe-cors = true/g" $GRAVITY_HOME/$VALIDATOR/config/app.toml
sed -i "s/cors_allowed_origins *= *.*/cors_allowed_origins = \[\"*\"\]/g" $GRAVITY_HOME/$VALIDATOR/config/config.toml
sed -i "1,/\<laddr\>/{s/\<laddr\> *= *.*/laddr = \"tcp:\/\/0.0.0.0:26657\"/g}" $GRAVITY_HOME/$VALIDATOR/config/config.toml # replace exactly the string laddr with\< and \>

# gravity tx ibc-transfer transfer transfer channel-0 orai18hr8jggl3xnrutfujy2jwpeu0l76azprlvgrwt 10000000000000000000000uairi --from validator1 --keyring-backend test --chain-id gravity-test -y --home /gravity/data/validator1