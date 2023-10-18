package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	abci "github.com/tendermint/tendermint/abci/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx sdk.Context, k Keeper, genState types.GenesisState) []abci.ValidatorUpdate {
	fmt.Println("Creating auction pool account (", types.AuctionPoolAccountName, ")")
	k.AccountKeeper.GetModuleAccount(ctx, types.AuctionPoolAccountName)
	k.SetParams(ctx, genState.Params)

	// Previous module state
	if genState.ActivePeriod != nil {
		k.updateAuctionPeriodUnsafe(ctx, *genState.ActivePeriod)
		for _, auction := range genState.ActiveAuctions {
			err := k.StoreAuction(ctx, auction)
			if err != nil {
				panic(fmt.Sprintf("Unable to store auction: %v", err))
			}
		}
	} else {
		// Initialize the first auction period
		// even if the module is not enabled, a previous auction period is expected
		_, err := k.CreateNewAuctionPeriod(ctx)
		if err != nil {
			panic(fmt.Sprintf("Unable to create a new auction period: %v", err))
		}

		// If the module is starting enabled, create auctions for it
		if genState.Params.Enabled {
			if err := k.CreateAuctionsForAuctionPeriod(ctx); err != nil {
				errMsg := fmt.Sprintf("unable to create auctions for genesis auction period: %v", err)
				ctx.Logger().Error(errMsg)
				panic(errMsg)
			}
		}
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
