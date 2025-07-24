package aurora

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

var NeutrinoToAuroraPlanName = "aurora"

func GetAuroraUpgradeHandler(
	mm *module.Manager, configurator *module.Configurator, crisisKeeper *crisiskeeper.Keeper, paramsKeeper *paramskeeper.Keeper,
	consensusparamsKeeper *consensusparamskeeper.Keeper, upgradeKeeper *upgradekeeper.Keeper,
) func(
	c context.Context, plan upgradetypes.Plan, vmap module.VersionMap,
) (module.VersionMap, error) {
	if mm == nil || crisisKeeper == nil {
		panic("Nil argument to GetAuroraUpgradeHandler")
	}
	return func(c context.Context, plan upgradetypes.Plan, vmap module.VersionMap) (module.VersionMap, error) {
		ctx := sdk.UnwrapSDKContext(c)
		ctx.Logger().Info("Aurora upgrade: Enter handler")

		ctx.Logger().Info("Aurora Upgrade: Running any configured module migrations")
		out, outErr := mm.RunMigrations(ctx, *configurator, vmap)

		for moduleName, mod := range mm.Modules {
			cvMod, ok := mod.(module.HasConsensusVersion)
			if !ok {
				continue
			}
			version := cvMod.ConsensusVersion()
			ctx.Logger().Info("Aurora upgrade: Module version updated", "module", moduleName, "version", version)
		}

		ctx.Logger().Info("Aurora upgrade: Migrating consensus params from paramspace to consensus module")
		baseAppLegacySS := paramsKeeper.Subspace(baseapp.Paramspace).WithKeyTable(paramstypes.ConsensusParamsKeyTable())

		if cp := baseapp.GetConsensusParams(ctx, baseAppLegacySS); cp != nil {
			consensusparamsKeeper.ParamsStore.Set(ctx, *cp)
		} else {
			ctx.Logger().Info("warning: consensus parameters are undefined; skipping migration", "upgrade", "aurora")
		}

		ctx.Logger().Info("Asserting invariants after upgrade")
		crisisKeeper.AssertInvariants(ctx)
		ctx.Logger().Info("Aurora upgrade successful!")

		return out, outErr
	}
}
