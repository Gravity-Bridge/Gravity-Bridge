#!/bin/bash

set -e

KEY="testwallet"
CHAINID="test-1"
MONIKER="testnode"
KEYRING="test"
rm -rf ~/.gravity

if [ $# -eq 0 ];
then
	echo "$0: Input test version"
	exit 1
else	
	cd ~/Gravity-Bridge
	git checkout $1
	cd ./module
	make install
	killall gravity || true 
	rm -rf ~/.gravity
	cd ~
	gravity init $MONIKER --chain-id $CHAINID
	gravity keys add $KEY --keyring-backend $KEYRING
	gravity add-genesis-account $KEY 1000000000000stake --keyring-backend $KEYRING
	gravity gentx $KEY 1000000stake 0x1d65BCC107689Fb9c35Ae403d028E29C1C90C36A gravity18ytfr4s8lfccy048zl00y3akujxqvq75sfpuzq  --keyring-backend $KEYRING --chain-id $CHAINID
	gravity collect-gentxs
	echo -E $(jq '.app_state.gov.voting_params.voting_period = "20s"' ~/.gravity/config/genesis.json) &> ~/.gravity/config/genesis.json
	screen -S gravity -d -m gravity start
	sleep 15
	gravity tx gov submit-proposal software-upgrade $3 --upgrade-height 7 --from $KEY --deposit 10000000stake --title TEST --description TEST --keyring-backend $KEYRING --chain-id $CHAINID -y
	sleep 2
	gravity tx gov vote 1 yes --from $KEY --keyring-backend $KEYRING --chain-id $CHAINID -y
	sleep 30
	killall gravity || true
	cd ~/Gravity-Bridge
	git checkout $2
	cd ./module 
	make install
	gravity start 
fi
