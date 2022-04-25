
#!/bin/bash
set -eux
# your gaiad binary name
BIN=gaiad

chmod +x /usr/bin/gaiad
CHAIN_ID="ibc-test-1"

NODES=$1

ALLOCATION="10000000000stake,10000000000footoken"

# first we start a genesis.json with validator 1
# validator 1 will also collect the gentx's once gnerated
STARTING_VALIDATOR=1
STARTING_VALIDATOR_HOME="--home /ibc-validator$STARTING_VALIDATOR"
# todo add git hash to chain name
$BIN init $STARTING_VALIDATOR_HOME --chain-id=$CHAIN_ID ibc-validator1


## Modify generated genesis.json to our liking by editing fields using jq
## we could keep a hardcoded genesis file around but that would prevent us from
## testing the generated one with the default values provided by the module.

# add in denom metadata for both native tokens
jq '.app_state.bank.denom_metadata += [{"name": "Foo Token", "symbol": "FOO", "base": "footoken", display: "mfootoken", "description": "A non-staking test token", "denom_units": [{"denom": "footoken", "exponent": 0}, {"denom": "mfootoken", "exponent": 6}]},{"name": "Stake Token", "symbol": "STEAK", "base": "stake", display: "mstake", "description": "A staking test token", "denom_units": [{"denom": "stake", "exponent": 0}, {"denom": "mstake", "exponent": 6}]}]' /ibc-validator$STARTING_VALIDATOR/config/genesis.json > /ibc-metadata-genesis.json

# a 60 second voting period to allow us to pass governance proposals in the tests
jq '.app_state.gov.voting_params.voting_period = "60s"' /ibc-metadata-genesis.json > /ibc-edited-genesis.json

mv /ibc-edited-genesis.json /ibc-genesis.json


# Sets up an arbitrary number of validators on a single machine by manipulating
# the --home parameter on gaiad
for i in $(seq 1 $NODES);
do
GAIA_HOME="--home /ibc-validator$i"
GENTX_HOME="--home-client /ibc-validator$i"
ARGS="$GAIA_HOME --keyring-backend test"

# Generate a validator key for each validator
$BIN keys add $ARGS ibc-validator$i 2>> /ibc-validator-phrases

VALIDATOR_KEY=$($BIN keys show ibc-validator$i -a $ARGS)
# move the genesis in
mkdir -p /ibc-validator$i/config/
mv /ibc-genesis.json /ibc-validator$i/config/genesis.json
$BIN add-genesis-account $ARGS $VALIDATOR_KEY $ALLOCATION
# move the genesis back out
mv /ibc-validator$i/config/genesis.json /ibc-genesis.json
done


for i in $(seq 1 $NODES);
do
cp /ibc-genesis.json /ibc-validator$i/config/genesis.json
GAIA_HOME="--home /ibc-validator$i"
ARGS="$GAIA_HOME --keyring-backend test"
# the /8 containing 7.7.7.7 is assigned to the DOD and never routable on the public internet
# we're using it in private to prevent gaia from blacklisting it as unroutable
# and allow local pex
$BIN gentx $ARGS $GAIA_HOME --moniker ibc-validator$i --chain-id=$CHAIN_ID --ip 7.7.8.$i ibc-validator$i 500000000stake
# obviously we don't need to copy validator1's gentx to itself
if [ $i -gt 1 ]; then
cp /ibc-validator$i/config/gentx/* /ibc-validator1/config/gentx/
fi
done


$BIN collect-gentxs $STARTING_VALIDATOR_HOME
GENTXS=$(ls /ibc-validator1/config/gentx | wc -l)
cp /ibc-validator1/config/genesis.json /ibc-genesis.json
echo "Collected $GENTXS gentx"

# put the now final genesis.json into the correct folders
for i in $(seq 1 $NODES);
do
cp /ibc-genesis.json /ibc-validator$i/config/genesis.json
done
