package v2

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/module"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	bech32ibckeeper "github.com/osmosis-labs/bech32-ibc/x/bech32ibc/keeper"

	gravitytypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

func GetV2UpgradeHandler(
	mm *module.Manager, configurator *module.Configurator, accountKeeper *authkeeper.AccountKeeper,
	bankKeeper *bankkeeper.BaseKeeper, bech32IbcKeeper *bech32ibckeeper.Keeper, distrKeeper *distrkeeper.Keeper,
	mintKeeper *mintkeeper.Keeper, stakingKeeper *stakingkeeper.Keeper,
) func(
	ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap,
) (module.VersionMap, error) {
	return func(ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap) (module.VersionMap, error) {
		// We previously upgraded via genesis, thus we don't want to run upgrades for all the modules
		fromVM := make(map[string]uint64)
		for moduleName, module := range mm.Modules {
			fromVM[moduleName] = module.ConsensusVersion()
		}

		// Lower the gravity module version because we want to run that upgrade
		fromVM[gravitytypes.StoreKey] = 1

		ctx.Logger().Info("v2 Upgrade: Setting up bech32ibc module's native prefix")
		err := setupBech32ibcKeeper(bech32IbcKeeper, ctx)
		if err != nil {
			panic(sdkerrors.Wrap(err, "v2 Upgrade: Unable to upgrade, bech32ibc module not initialized"))
		}

		// distribution module: Fix the issues caused by an airdrop giving distribution module some tokens
		ctx.Logger().Info("v2 Upgrade: Fixing community pool balance")
		err = fixDistributionPoolBalance(accountKeeper, bankKeeper, distrKeeper, mintKeeper, ctx)
		if err != nil {
			panic(sdkerrors.Wrap(err, "v2 Upgrade: Unable to upgrade, distribution module balance could not be corrected"))
		}

		ctx.Logger().Info("v2 Upgrade: Enforcing validator minimum commission")
		bumpMinValidatorCommissions(stakingKeeper, ctx)

		return mm.RunMigrations(ctx, *configurator, fromVM)
	}
}

// Sets up bech32ibc module by setting the native account prefix to "gravity"
// Failing to set the native prefix will cause a chain halt on init genesis or in the firstBeginBlocker assertions
func setupBech32ibcKeeper(bech32IbcKeeper *bech32ibckeeper.Keeper, ctx sdk.Context) error {
	// TODO: Implement me!
	return gravitytypes.ErrInvalid
}

// Fixes the invalid community pool balance caused by an airdrop airdropping to the distribution module account
// causing the distribution invariant checking module balance == (pool balance + validator outstanding rewards) failure
// We fix by overwriting the pool's ugraviton integer amount with the distribution module's integer amount less rewards
// If no discrepancy exists, this function does nothing
func fixDistributionPoolBalance(
	accountKeeper *authkeeper.AccountKeeper, bankKeeper *bankkeeper.BaseKeeper, distrKeeper *distrkeeper.Keeper,
	mintKeeper *mintkeeper.Keeper, ctx sdk.Context,
) error {
	// TODO: Implement me!
	return gravitytypes.ErrInvalid
}

// Enforce minimum 5% validator commission on all noncompliant validators
// The MinCommissionDecorator enforces new validators must be created with a minimum commission rate of 5%,
// but existing validators are unaffected, here we automatically bump them all to 5% if they are lower
func bumpMinValidatorCommissions(stakingKeeper *stakingkeeper.Keeper, ctx sdk.Context) {
	// TODO: Implement me!
}
