package antares

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"

	ica "github.com/cosmos/ibc-go/v6/modules/apps/27-interchain-accounts"
	icacontrollertypes "github.com/cosmos/ibc-go/v6/modules/apps/27-interchain-accounts/controller/types"
	icahosttypes "github.com/cosmos/ibc-go/v6/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v6/modules/apps/27-interchain-accounts/types"
)

func GetAntaresUpgradeHandler(
	mm *module.Manager, configurator *module.Configurator, crisisKeeper *crisiskeeper.Keeper,
) func(
	ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap,
) (module.VersionMap, error) {
	if mm == nil {
		panic("Nil argument to GetAntaresUpgradeHandler")
	}
	return func(ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap) (module.VersionMap, error) {
		ctx.Logger().Info("Antares upgrade: Starting upgrade")

		vmap[icatypes.ModuleName] = mm.Modules[icatypes.ModuleName].ConsensusVersion()
		icaHostParams := icahosttypes.Params{
			HostEnabled:   false,
			AllowMessages: []string{},
		}
		icaControllerParams := icacontrollertypes.Params{
			ControllerEnabled: false,
		}

		icaModule, ok := mm.Modules[icatypes.ModuleName].(ica.AppModule)
		if !ok {
			panic("module manager's ica module is not an ica.AppModule")
		}
		icaModule.InitModule(ctx, icaControllerParams, icaHostParams)
		ctx.Logger().Info("Antares Upgrade: Running any configured module migrations")
		out, outErr := mm.RunMigrations(ctx, *configurator, vmap)
		if outErr != nil {
			return out, outErr
		}
		ctx.Logger().Info("Asserting invariants after upgrade")
		crisisKeeper.AssertInvariants(ctx)

		ctx.Logger().Info("Antares Upgrade Successful")
		return out, nil
	}
}
