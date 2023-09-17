package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	abci "github.com/tendermint/tendermint/abci/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx sdk.Context, k Keeper, genState types.GenesisState) []abci.ValidatorUpdate {

	// this line is used by starport scaffolding # genesis/module/init
	k.SetParams(ctx, genState.Params)

	k.AccountKeeper.GetModuleAccount(ctx, types.ModuleName)

	return []abci.ValidatorUpdate{}
}

// ExportGenesis returns the module's exported genesis
func ExportGenesis(ctx sdk.Context, k Keeper) *types.GenesisState {
	genesis := types.DefaultGenesis()
	genesis.Params = k.GetParams(ctx)

	return genesis
}
