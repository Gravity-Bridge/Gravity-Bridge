package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	abci "github.com/tendermint/tendermint/abci/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx sdk.Context, k Keeper, genState types.GenesisState) []abci.ValidatorUpdate {
	k.SetParams(ctx, genState.Params)

	// Previous module state
	if genState.ActivePeriod != nil {
		k.updateAuctionPeriodUnsafe(ctx, *genState.ActivePeriod)
		for _, auction := range genState.ActiveAuctions {
			k.StoreAuction(ctx, auction)
		}
	} else {
		// Initialize the first auction period
		k.CreateNewAuctionPeriod(ctx)
	}

	// Test that the module was correctly initialized by running auction module invariants
	if errMsg, failure := AllInvariants(k)(ctx); failure {
		panic(fmt.Sprintf("Invariant failure after auction init genesis: %v", errMsg))
	}

	return []abci.ValidatorUpdate{}
}

// ExportGenesis returns the module's exported genesis
func ExportGenesis(ctx sdk.Context, k Keeper) *types.GenesisState {
	genesis := types.DefaultGenesis()
	genesis.Params = k.GetParams(ctx)

	auctionPeriod := k.GetAuctionPeriod(ctx)
	genesis.ActivePeriod = auctionPeriod

	auctions := k.GetAllAuctions(ctx)
	genesis.ActiveAuctions = auctions

	return genesis
}
