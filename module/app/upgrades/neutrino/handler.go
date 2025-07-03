package neutrino

import (
	"context"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"

	auctionkeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
)

var ApolloToNeutrinoPlanName = "neutrino"

func GetNeutrinoUpgradeHandler(
	ModuleManager *module.Manager, configurator *module.Configurator, crisisKeeper *crisiskeeper.Keeper, auctionKeeper *auctionkeeper.Keeper,
) func(
	c context.Context, plan upgradetypes.Plan, vmap module.VersionMap,
) (module.VersionMap, error) {
	if ModuleManager == nil {
		panic("Nil argument to GetNeutrinoUpgradeHandler")
	}
	return func(c context.Context, plan upgradetypes.Plan, vmap module.VersionMap) (module.VersionMap, error) {
		ctx := sdk.UnwrapSDKContext(c)
		ctx.Logger().Info("Module Consensus Version Map", "vmap", vmap)

		ctx.Logger().Info("Neutrino Upgrade: Running any configured module migrations")
		out, outErr := ModuleManager.RunMigrations(ctx, *configurator, vmap)

		ctx.Logger().Info("Asserting invariants after upgrade")
		crisisKeeper.AssertInvariants(ctx)

		ctx.Logger().Info("Neutrino Upgrade Successful")
		return out, outErr
	}
}
