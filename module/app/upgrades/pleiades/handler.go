package pleiades

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
)

func GetPleiadesUpgradeHandler(
	mm *module.Manager, configurator *module.Configurator, crisisKeeper *crisiskeeper.Keeper,
) func(
	ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap,
) (module.VersionMap, error) {
	if mm == nil {
		panic("Nil argument to GetPleiadesUpgradeHandler")
	}
	return func(ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap) (module.VersionMap, error) {
		ctx.Logger().Info("Pleiades upgrade: Enter handler")

		ctx.Logger().Info("Pleiades Upgrade: Running any configured module migrations")
		out, outErr := mm.RunMigrations(ctx, *configurator, vmap)

		ctx.Logger().Info("Asserting invariants after upgrade")
		crisisKeeper.AssertInvariants(ctx)

		return out, outErr
	}
}

func GetPleiades2UpgradeHandler(
	mm *module.Manager, configurator *module.Configurator, crisisKeeper *crisiskeeper.Keeper, stakingKeeper *stakingkeeper.Keeper,
) func(
	ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap,
) (module.VersionMap, error) {
	if mm == nil {
		panic("Nil argument to GetPleiadesUpgradeHandler")
	}
	return func(ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap) (module.VersionMap, error) {
		ctx.Logger().Info("Pleiades Upgrade part 2: Enter handler")

		ctx.Logger().Info("Pleiades Upgrade part 2: Running any configured module migrations")
		out, outErr := mm.RunMigrations(ctx, *configurator, vmap)

		ctx.Logger().Info("Pleiades Upgrade part 2: Enforcing validator minimum comission")
		bumpMinValidatorCommissions(stakingKeeper, ctx)

		ctx.Logger().Info("Asserting invariants after upgrade")
		crisisKeeper.AssertInvariants(ctx)

		return out, outErr
	}
}

// Enforce minimum 10% validator commission on all noncompliant validators
// The MinCommissionDecorator enforces new validators must be created with a minimum commission rate of 10%,
// but existing validators are unaffected, here we automatically bump them all to 10% if they are lower
func bumpMinValidatorCommissions(stakingKeeper *stakingkeeper.Keeper, ctx sdk.Context) {
	ctx.Logger().Info("Pleiades Upgrade part 2: bumpMinValidatorCommissions(): Enter function")
	// This logic was originally included in the Juno project at github.com/CosmosContracts/juno/blob/main/app/app.go
	// This version was added to Juno by github user the-frey https://github.com/the-frey
	ctx.Logger().Info("Pleiades Upgrade part 2: bumpMinValidatorCommissions(): Getting all the validators")
	validators := stakingKeeper.GetAllValidators(ctx)
	minCommissionRate := sdk.NewDecWithPrec(10, 2)
	ctx.Logger().Info("Pleiades Upgrade part 2: bumpMinValidatorCommissions():", "minCommissionRate", minCommissionRate.String())
	ctx.Logger().Info("Pleiades Upgrade part 2: bumpMinValidatorCommissions(): Iterating validators")
	for _, v := range validators {
		ctx.Logger().Info("Pleiades Upgrade part 2: bumpMinValidatorCommissions(): checking validator", "validator", v.GetMoniker(), "Commission.Rate", v.Commission.Rate.String(), "Commission.MaxRate", v.Commission.MaxRate.String())
		if v.Commission.Rate.LT(minCommissionRate) {
			ctx.Logger().Info("Pleiades Upgrade part 2: bumpMinValidatorCommissions(): validator is out of compilance! Modifying their commission rate(s)", "validator ", v.GetMoniker())
			if v.Commission.MaxRate.LT(minCommissionRate) {
				ctx.Logger().Info("Pleiades Upgrade part 2: bumpMinValidatorCommissions(): Updating validator Commission.MaxRate", "validator", v.GetMoniker(), "old", v.Commission.MaxRate.String(), "new", minCommissionRate.String())
				v.Commission.MaxRate = minCommissionRate
			}

			ctx.Logger().Info("Pleiades Upgrade part 2: bumpMinValidatorCommissions(): Updating validator Commission.Rate", v.GetMoniker(), "old", v.Commission.Rate.String(), "new", minCommissionRate.String())
			v.Commission.Rate = minCommissionRate
			ctx.Logger().Info("Pleiades Upgrade part 2: bumpMinValidatorCommissions(): also UpdateTime ", "old", v.Commission.UpdateTime.String(), "new", ctx.BlockHeader().Time.String())
			v.Commission.UpdateTime = ctx.BlockHeader().Time

			ctx.Logger().Info("Pleiades Upgrade part 2: bumpMinValidatorCommissions(): calling the hook")
			// call the before-modification hook since we're about to update the commission
			stakingKeeper.BeforeValidatorModified(ctx, v.GetOperator())

			ctx.Logger().Info("Pleiades Upgrade part 2: bumpMinValidatorCommissions(): setting the validator")
			stakingKeeper.SetValidator(ctx, v)

			v, _ = stakingKeeper.GetValidator(ctx, v.GetOperator()) // Refresh since we set them in the keeper
			ctx.Logger().Info("Pleiades Upgrade part 2: bumpMinValidatorCommissions(): validator's set rate", "validator", v.GetMoniker(), "Commission", v.Commission.String())
		}
	}
}
