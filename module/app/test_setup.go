package app

import (
	"encoding/json"

	"github.com/cosmos/cosmos-sdk/simapp"
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
