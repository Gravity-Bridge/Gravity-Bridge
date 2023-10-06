package apollo

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"

	auctionkeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	auctiontypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

func GetApolloUpgradeHandler(
	mm *module.Manager, configurator *module.Configurator, crisisKeeper *crisiskeeper.Keeper, auctionKeeper *auctionkeeper.Keeper,
) func(
	ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap,
) (module.VersionMap, error) {
	if mm == nil {
		panic("Nil argument to GetApolloUpgradeHandler")
	}
	return func(ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap) (module.VersionMap, error) {
		vmap[auctiontypes.ModuleName] = mm.Modules[auctiontypes.ModuleName].ConsensusVersion()
		ctx.Logger().Info("Module Consensus Version Map", "vmap", vmap)

		ctx.Logger().Info("Apollo Upgrade: Running any configured module migrations")
		out, outErr := mm.RunMigrations(ctx, *configurator, vmap)

		ctx.Logger().Info("Setting the auction module Params")
		auctionParams := auctiontypes.DefaultParams()
		auctionKeeper.SetParams(ctx, auctionParams)

		ctx.Logger().Info("Creating the initial auction period (with no active auctions)")
		if _, err := auctionKeeper.CreateNewAuctionPeriod(ctx); err != nil {
			panic(fmt.Sprintf("Encountered error initializing auction module: %v", err))
		}

		ctx.Logger().Info("Asserting invariants after upgrade")
		crisisKeeper.AssertInvariants(ctx)

		ctx.Logger().Info("Apollo Upgrade Successful")
		return out, outErr
	}
}
