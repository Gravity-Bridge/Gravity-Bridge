package polaris

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	ibctransferkeeper "github.com/cosmos/ibc-go/v6/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v6/modules/apps/transfer/types"
)

func GetPolarisUpgradeHandler(
	mm *module.Manager, configurator *module.Configurator, crisisKeeper *crisiskeeper.Keeper, transferKeeper *ibctransferkeeper.Keeper,
) func(
	ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap,
) (module.VersionMap, error) {
	if mm == nil || transferKeeper == nil {
		panic("Nil argument to GetPolarisUpgradeHandler")
	}
	return func(ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap) (module.VersionMap, error) {
		ctx.Logger().Info("Polaris upgrade: Enter handler")
		// We previously upgraded via genesis, thus we don't want to run upgrades for all the modules
		fromVM := make(map[string]uint64)
		ctx.Logger().Info("Polaris upgrade: Creating version map")
		for moduleName, module := range mm.Modules {
			fromVM[moduleName] = module.ConsensusVersion()
		}

		/* On the ibc-go v2 -> v3 migration guide, they mention the following is needed to enable Interchain Accounts.
		We are not enabling Interchain Accounts at this time, so the upgrade info has been commented here for reference.

		"If the chain will adopt ICS27, it must set the appropriate params during the execution of the upgrade handler in app.go:"

		app.UpgradeKeeper.SetUpgradeHandler("v3",
		    func(ctx sdk.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		        // set the ICS27 consensus version so InitGenesis is not run
		        fromVM[icatypes.ModuleName] = icamodule.ConsensusVersion()

		        // create ICS27 Controller submodule params
		        controllerParams := icacontrollertypes.Params{
		            ControllerEnabled: true,
		        }

		        // create ICS27 Host submodule params
		        hostParams := icahosttypes.Params{
		            HostEnabled: true,
		            AllowMessages: []string{"/cosmos.bank.v1beta1.MsgSend", ...},
		        }

		        // initialize ICS27 module
		        icamodule.InitModule(ctx, controllerParams, hostParams)

		        ...

		        return app.mm.RunMigrations(ctx, app.configurator, fromVM)
		    })
		*/

		// Fix any denoms with /'s in their names

		// list of traces that must replace the old traces in store
		var newTraces []ibctransfertypes.DenomTrace
		transferKeeper.IterateDenomTraces(ctx,
			func(dt ibctransfertypes.DenomTrace) bool {
				// check if the new way of splitting FullDenom
				// into Trace and BaseDenom passes validation and
				// is the same as the current DenomTrace.
				// If it isn't then store the new DenomTrace in the list of new traces.
				newTrace := ibctransfertypes.ParseDenomTrace(dt.GetFullDenomPath())
				if err := newTrace.Validate(); err == nil && !equalTraces(newTrace, dt) {
					newTraces = append(newTraces, newTrace)
				}

				return false
			})

		// replace the outdated traces with the new trace information
		for _, nt := range newTraces {
			transferKeeper.SetDenomTrace(ctx, nt)
		}

		ctx.Logger().Info("Polaris Upgrade: Running any configured module migrations")
		out, outErr := mm.RunMigrations(ctx, *configurator, fromVM)

		ctx.Logger().Info("Asserting invariants after upgrade")
		crisisKeeper.AssertInvariants(ctx)

		return out, outErr
	}
}

// Used to fix denoms with /'s in their names
func equalTraces(dtA, dtB ibctransfertypes.DenomTrace) bool {
	return dtA.BaseDenom == dtB.BaseDenom && dtA.Path == dtB.Path
}
