package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx sdk.Context, k Keeper, genState types.GenesisState) {

	// this line is used by starport scaffolding # genesis/module/init
	k.SetParams(ctx, genState.Params)
}

// ExportGenesis returns the module's exported genesis
// func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
// 	genesis := types.DefaultGenesis()
// 	genesis.Params, _ = k.GetParams(ctx)

// 	genesis.AuctionPoolList = k.GetAllAuctionPool(ctx)
// 	genesis.AuctionPoolCount = k.GetAuctionPoolCount(ctx)
// 	// this line is used by starport scaffolding # genesis/module/export

// 	return genesis
// }
