package antares

import (
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"

	ica "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts"
	icacontrollertypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/types"
	icahosttypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/types"
)

func GetAntaresUpgradeHandler(
	ModuleManager *module.Manager, configurator *module.Configurator, crisisKeeper *crisiskeeper.Keeper,
) func(
	ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap,
) (module.VersionMap, error) {
	if ModuleManager == nil {
		panic("Nil argument to GetAntaresUpgradeHandler")
	}
	return func(ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap) (module.VersionMap, error) {
		ctx.Logger().Info("Antares upgrade: Starting upgrade")

		vmap[icatypes.ModuleName] = ModuleManager.Modules[icatypes.ModuleName].(module.HasConsensusVersion).ConsensusVersion()
		icaHostParams := icahosttypes.Params{
			HostEnabled:   false,
			AllowMessages: []string{},
		}
		icaControllerParams := icacontrollertypes.Params{
			ControllerEnabled: false,
		}

		icaModule, ok := ModuleManager.Modules[icatypes.ModuleName].(ica.AppModule)
		if !ok {
			panic("module manager's ica module is not an ica.AppModule")
		}
		icaModule.InitModule(ctx, icaControllerParams, icaHostParams)
		ctx.Logger().Info("Antares Upgrade: Running any configured module migrations")
		out, outErr := ModuleManager.RunMigrations(ctx, *configurator, vmap)
		if outErr != nil {
			return out, outErr
		}
		ctx.Logger().Info("Asserting invariants after upgrade")
		crisisKeeper.AssertInvariants(ctx)

		ctx.Logger().Info("Antares Upgrade Successful")
		return out, nil
	}
}
