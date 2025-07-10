#!/bin/bash
set -eux
# your gaiad binary name
BIN=gravity

CHAIN_ID="gravity-test-1"

NODES=$1
# When doing an upgrade test we need to run init using the old binary so we don't include newly added fields
set +u
if [[ ! -z ${OLD_BINARY_LOCATION} ]]; then
  echo "Replacing gravity with $OLD_BINARY_LOCATION"
  BIN=$OLD_BINARY_LOCATION
else
  echo "Old binary not set, using regular gravity"
fi
set -u

ALLOCATION="1000000000000000footoken,1000000000000000footoken2,1000000000000000ibc/nometadatatoken,1000000000000000ugraviton"

# first we start a genesis.json with validator 1
# validator 1 will also collect the gentx's once gnerated
STARTING_VALIDATOR=1
STARTING_VALIDATOR_HOME="--home /validator$STARTING_VALIDATOR"
# todo add git hash to chain name
$BIN init $STARTING_VALIDATOR_HOME --chain-id=$CHAIN_ID validator1


## Modify generated genesis.json to our liking by editing fields using jq
## we could keep a hardcoded genesis file around but that would prevent us from
## testing the generated one with the default values provided by the module.

# add in denom metadata for both native tokens
jq '.app_state.bank.denom_metadata += [{"name": "Foo Token", "symbol": "FOO", "base": "footoken", display: "mfootoken", "description": "A non-staking test token", "denom_units": [{"denom": "footoken", "exponent": 0}, {"denom": "mfootoken", "exponent": 6}]}]' /validator$STARTING_VALIDATOR/config/genesis.json > /footoken2-genesis.json
jq '.app_state.bank.denom_metadata += [{"name": "Foo Token2", "symbol": "F20", "base": "footoken2", display: "mfootoken2", "description": "A second non-staking test token", "denom_units": [{"denom": "footoken2", "exponent": 0}, {"denom": "mfootoken2", "exponent": 6}]}]' /footoken2-genesis.json > /stake-genesis.json
jq '.app_state.bank.denom_metadata += [{"name": "Stake Token", "symbol": "GRAV", "base": "ugraviton", display: "ugraviton", "description": "A staking test token", "denom_units": [{"denom": "ugraviton", "exponent": 0}, {"denom": "graviton", "exponent": 6}]}]' /stake-genesis.json > /bech32ibc-genesis.json

# Set the chain's native bech32 prefix
jq '.app_state.bech32ibc.nativeHRP = "gravity"' /bech32ibc-genesis.json > /gov-genesis.json

# a 60 second voting period to allow us to pass governance proposals in the tests
jq '.app_state.gov.voting_params.voting_period = "120s"' /gov-genesis.json > /eip712-genesis.json

# Create a user for EIP-712 testing with a reliable account number (13) so that the hardcoded transaction routinely succeeds
# 13 seems to be the first user account which can be created, but this is more reliable than waiting for test time
jq '.app_state.auth.accounts += [{"@type":"/cosmos.auth.v1beta1.BaseAccount","account_number":"13","address":"gravity1hanqss6jsq66tfyjz56wz44z0ejtyv0724h32c","pub_key":null,"sequence":"0"}]' /eip712-genesis.json > /eip712-2-genesis.json
jq '.app_state.bank.balances += [{"address": "gravity1hanqss6jsq66tfyjz56wz44z0ejtyv0724h32c", "coins": [{"amount": "1000000000", "denom": "ugraviton"}]}]' /eip712-2-genesis.json > /community-pool-genesis.json

# Add some funds to the community pool to test Airdrops, note that the gravity address here is the first 20 bytes
# of the sha256 hash of 'distribution' to create the address of the module
jq '.app_state.distribution.fee_pool.community_pool = [{"denom": "footoken", "amount": "1000000000000000000000000"},{"denom": "ugraviton", "amount": "1000000000000000000000000"}]' /community-pool-genesis.json > /community-pool2-genesis.json
jq '.app_state.auth.accounts += [{"@type": "/cosmos.auth.v1beta1.ModuleAccount", "base_account": { "account_number": "0", "address": "gravity1jv65s3grqf6v6jl3dp4t6c9t9rk99cd8r0kyvh","pub_key": null,"sequence": "0"},"name": "distribution","permissions": ["basic"]}]' /community-pool2-genesis.json > /community-pool3-genesis.json
jq '.app_state.bank.balances += [{"address": "gravity1jv65s3grqf6v6jl3dp4t6c9t9rk99cd8r0kyvh", "coins": [{"denom": "footoken", "amount": "1000000000000000000000000"},{"amount": "1000000000000000000000000", "denom": "ugraviton"}]}]' /community-pool3-genesis.json > /edited-genesis.json

# TODO: Remove after Apollo upgrade, needed to make upgrade test work while still on Antares
set +u
if [[ -z ${OLD_BINARY_LOCATION} ]]; then
  mv /edited-genesis.json /auction-pool-genesis.json
  # Add some funds to the auction pool to test the auction module, the address is the first 20 bytes of the sha256 hash of "auction_pool"
  # account_number is 14 to preserve the EIP712 test account's (gravity1hanqss6jsq66tfyjz56wz44z0ejtyv0724h32c) number as 13
  jq '.app_state.auth.accounts += [{"@type": "/cosmos.auth.v1beta1.ModuleAccount", "base_account": { "account_number": "14", "address": "gravity1vp0qhw3jqm7gxk6yu0eqkcwf5u986eahmjjnql","pub_key": null,"sequence": "0"},"name": "auction_pool","permissions": []}]' /auction-pool-genesis.json > /auction-pool2-genesis.json
  jq '.app_state.bank.balances += [{"address": "gravity1vp0qhw3jqm7gxk6yu0eqkcwf5u986eahmjjnql", "coins": [{"denom": "footoken", "amount": "1000000000000000000000000"},{"denom": "footoken2", "amount": "1000000000000000000000000"},{"amount": "1000000000000000000000000", "denom": "ugraviton"}]}]' /auction-pool2-genesis.json > /auction-genesis.json


  # Set the auction module params
  jq '.app_state.auction.params.non_auctionable_tokens = ["ugraviton"]' /auction-genesis.json > /auction2-genesis.json
  jq '.app_state.auction.params.auction_length = 40' /auction2-genesis.json > /gravity-genesis.json

  # Set the Send to Eth chain fee fraction that goes to the auction pool
  jq '.app_state.gravity.params.chain_fee_auction_pool_fraction = "0.75"' /gravity-genesis.json > /edited-genesis.json
fi
set -u

# Change the stake token to be ugraviton instead
sed -i 's/\<stake\>/ugraviton/g' /edited-genesis.json

mv /edited-genesis.json /genesis.json

VESTING_AMOUNT="1000000000ugraviton"
START_VESTING=$(expr $(date +%s) + 600) # Start vesting 10 minutes from now
END_VESTING=$(expr $START_VESTING + 900) # End vesting 15 minutes from now, giving a 5 minute window for the test to work

# Sets up an arbitrary number of validators on a single machine by manipulating
# the --home parameter on gaiad
for i in $(seq 1 $NODES);
do
GAIA_HOME="--home /validator$i"
GENTX_HOME="--home-client /validator$i"
ARGS="$GAIA_HOME --keyring-backend test"

# Generate a validator key, orchestrator key, and eth key for each validator
$BIN keys add $ARGS validator$i 2>> /validator-phrases
VALIDATOR_KEY=$($BIN keys show validator$i --address $ARGS)
$BIN keys add $ARGS orchestrator$i 2>> /orchestrator-phrases
ORCHESTRATOR_KEY=$($BIN keys show orchestrator$i --address $ARGS)
$BIN eth_keys add >> /validator-eth-keys

# move the genesis in
mkdir -p /validator$i/config/
mv /genesis.json /validator$i/config/genesis.json
$BIN add-genesis-account $ARGS $VALIDATOR_KEY $ALLOCATION
$BIN add-genesis-account $ARGS $ORCHESTRATOR_KEY $ALLOCATION

# Add a vesting account
$BIN keys add $ARGS vesting$i 2>> /vesting-phrases
VESTING_KEY=$($BIN keys show vesting$i --address $ARGS)
$BIN add-genesis-account $ARGS $VESTING_KEY --vesting-amount $VESTING_AMOUNT --vesting-start-time $START_VESTING --vesting-end-time $END_VESTING $VESTING_AMOUNT

# move the genesis back out
mv /validator$i/config/genesis.json /genesis.json
done

cp /validator1/config/genesis.json /gravity/failing-genesis.json
for i in $(seq 1 $NODES);
do
cp /genesis.json /validator$i/config/genesis.json
GAIA_HOME="--home /validator$i"
ARGS="$GAIA_HOME --keyring-backend test"
ORCHESTRATOR_KEY=$($BIN keys show orchestrator$i --address $ARGS)
ETHEREUM_KEY=$(grep address /validator-eth-keys | sed -n "$i"p | sed 's/.*://')
# the /8 containing 7.7.7.7 is assigned to the DOD and never routable on the public internet
# we're using it in private to prevent gaia from blacklisting it as unroutable
# and allow local pex
$BIN gentx $ARGS $GAIA_HOME --moniker validator$i --chain-id=$CHAIN_ID --ip 7.7.7.$i validator$i 500000000ugraviton $ETHEREUM_KEY $ORCHESTRATOR_KEY
# obviously we don't need to copy validator1's gentx to itself
if [ $i -gt 1 ]; then
cp /validator$i/config/gentx/* /validator1/config/gentx/
fi
done


$BIN collect-gentxs $STARTING_VALIDATOR_HOME
GENTXS=$(ls /validator1/config/gentx | wc -l)
cp /validator1/config/genesis.json /genesis.json
echo "Collected $GENTXS gentx"

# put the now final genesis.json into the correct folders
for i in $(seq 1 $NODES);
do
cp /genesis.json /validator$i/config/genesis.json
done
