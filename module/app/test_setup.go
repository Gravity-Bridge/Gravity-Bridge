package app

import (
	"encoding/json"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/config"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/simapp"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"
)

const Bech32Prefix = "gravity"

// Initializes a new GravityApp without IBC functionality
func InitGravityTestApp(initChain bool) *Gravity {
	db := dbm.NewMemDB()
	app := NewGravityApp(
		log.NewNopLogger(),
		db,
		nil,
		true,
		map[int64]bool{},
		DefaultNodeHome,
		5,
		MakeEncodingConfig(),
		simapp.EmptyAppOptions{},
	)
	if initChain {
		genesisState := NewDefaultGenesisState()
		replaceStakeWithGrav(&genesisState, app.AppCodec)
		stateBytes, err := json.MarshalIndent(genesisState, "", " ")
		if err != nil {
			panic(err)
		}

		app.InitChain(
			// nolint: exhaustruct
			abci.RequestInitChain{
				Validators:      []abci.ValidatorUpdate{},
				ConsensusParams: simapp.DefaultConsensusParams,
				AppStateBytes:   stateBytes,
			},
		)
	}

	return app
}

func replaceStakeWithGrav(genState *GenesisState, cdc codec.Codec) {
	stake := "stake"

	bankGenesis := (*genState)[banktypes.ModuleName]
	if bankGenesis == nil {
		panic("Nil bank genesis")
	}
	var bankGenState banktypes.GenesisState
	cdc.MustUnmarshalJSON(bankGenesis, &bankGenState)
	for _, mdta := range bankGenState.DenomMetadata {
		if mdta.Base == stake {
			mdta.Base = config.NativeTokenDenom
			mdta.DenomUnits = []*banktypes.DenomUnit{{
				Denom:    config.NativeTokenDenom,
				Exponent: 0,
			}}
			mdta.Display = config.NativeTokenDenom
		}
	}
	(*genState)[banktypes.ModuleName] = cdc.MustMarshalJSON(&bankGenState)

	govGenesis := (*genState)[govtypes.ModuleName]
	if govGenesis == nil {
		panic("Nil gov genesis")
	}
	var govGenState govtypes.GenesisState
	cdc.MustUnmarshalJSON(govGenesis, &govGenState)
	govGenState.DepositParams.MinDeposit[0].Denom = config.NativeTokenDenom
	(*genState)[govtypes.ModuleName] = cdc.MustMarshalJSON(&govGenState)

	mintGenesis := (*genState)[minttypes.ModuleName]
	if mintGenesis == nil {
		panic("Nil mint genesis")
	}
	var mintGenState minttypes.GenesisState
	cdc.MustUnmarshalJSON(mintGenesis, &mintGenState)
	mintGenState.Params.MintDenom = config.NativeTokenDenom
	(*genState)[minttypes.ModuleName] = cdc.MustMarshalJSON(&mintGenState)

	stakingGenesis := (*genState)[stakingtypes.ModuleName]
	if stakingGenesis == nil {
		panic("Nil staking genesis")
	}
	var stakingGenState stakingtypes.GenesisState
	cdc.MustUnmarshalJSON(stakingGenesis, &stakingGenState)
	stakingGenState.Params.BondDenom = config.NativeTokenDenom
	(*genState)[stakingtypes.ModuleName] = cdc.MustMarshalJSON(&stakingGenState)
}
