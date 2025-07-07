package app

import (
	"encoding/json"

	"cosmossdk.io/log"
	simappparams "cosmossdk.io/simapp/params"
	dbm "github.com/cosmos/cosmos-db"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
)

// The genesis state of the blockchain is represented here as a map of raw json
// messages key'd by a identifier string.
// The identifier is used to determine which module genesis information belongs
// to so it may be appropriately routed during init chain.
// Within this application default genesis information is retrieved from
// the ModuleBasicManager which populates json from each BasicModule
// object provided to it during init.
type GenesisState map[string]json.RawMessage

// NewDefaultGenesisState generates the default state for the application.
func NewDefaultGenesisState() GenesisState {
	tempApp := tempGravity()
	return tempApp.DefaultGenesis()
}

func NewEncodingConfig() simappparams.EncodingConfig {
	tempApp := tempGravity()
	return tempApp.EncodingConfig
}

func tempGravity() *Gravity {
	return NewGravityApp(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		map[int64]bool{},
		DefaultNodeHome,
		0,
		simtestutil.NewAppOptionsWithFlagHome(DefaultNodeHome),
	)
}
