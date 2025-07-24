package next

import (
	"context"

	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	consensusparamskeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

var NeutrinoToNextPlanName = "next"

func GetNextUpgradeHandler(
	ModuleManager *module.Manager, configurator *module.Configurator, crisisKeeper *crisiskeeper.Keeper, paramsKeeper *paramskeeper.Keeper,
	consensusparamsKeeper *consensusparamskeeper.Keeper, upgradeKeeper *upgradekeeper.Keeper,
) func(
	c context.Context, plan upgradetypes.Plan, vmap module.VersionMap,
) (module.VersionMap, error) {
	if ModuleManager == nil || crisisKeeper == nil {
		panic("Nil argument to GetNextUpgradeHandler")
	}
	return func(c context.Context, plan upgradetypes.Plan, vmap module.VersionMap) (module.VersionMap, error) {
		ctx := sdk.UnwrapSDKContext(c)
		ctx.Logger().Info("Next upgrade: Enter handler")

		fromVM := make(map[string]uint64)
		ctx.Logger().Info("Next upgrade: Creating version map")
		for moduleName, mod := range ModuleManager.Modules {
			fromVM[moduleName] = mod.(module.HasConsensusVersion).ConsensusVersion()
		}

		ctx.Logger().Info("Next upgrade: Migrating consensus params from paramspace to consensus module")
		baseAppLegacySS := paramsKeeper.Subspace(baseapp.Paramspace).WithKeyTable(paramstypes.ConsensusParamsKeyTable())

		if cp := baseapp.GetConsensusParams(ctx, baseAppLegacySS); cp != nil {
			consensusparamsKeeper.ParamsStore.Set(ctx, *cp)
		} else {
			ctx.Logger().Info("warning: consensus parameters are undefined; skipping migration", "upgrade", "next")
		}

		ctx.Logger().Info("Next Upgrade: Running any configured module migrations")
		out, outErr := ModuleManager.RunMigrations(ctx, *configurator, fromVM)

		ctx.Logger().Info("Asserting invariants after upgrade")
		crisisKeeper.AssertInvariants(ctx)

		return out, outErr
	}
}
