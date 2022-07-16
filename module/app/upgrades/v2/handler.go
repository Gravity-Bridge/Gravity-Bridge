package v2

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/module"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"

	bech32ibckeeper "github.com/osmosis-labs/bech32-ibc/x/bech32ibc/keeper"

	gravitytypes "github.com/umee-network/Gravity-Bridge/module/x/gravity/types"
)

// GetMercury2Dot0UpgradeHandler Creates the handler for "mercury2.0", where we fix the mercury upgrade's
// implementation of IBC Auto Forwarding
// Note: mercury2.0 is not a consensus breaking change, as it only enables new functionality which is so far unused,
// thus it is unnecessary to change the consensus version or create a new upgrades module
func GetMercury2Dot0UpgradeHandler() func(
	ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap,
) (module.VersionMap, error) {
	return func(ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap) (module.VersionMap, error) {
		ctx.Logger().Info("Performing Fix for Mercury IBC Auto-Forwarding")
		// This upgrade introduces a new hash(PendingIbcAutoForward) key into the gravity store as IBC Auto-Forwards
		// are queued. This key will only be populated or used upon the creation of the first IBC Auto-Forward.
		// In short, there is no actual work to be performed here, but the messages make it quite clear that the upgrade
		// ran, and the new code is running as expected.
		ctx.Logger().Info("Upgrade Complete!")
		return vmap, nil
	}
}

func GetV2UpgradeHandler(
	mm *module.Manager, configurator *module.Configurator, accountKeeper *authkeeper.AccountKeeper,
	bankKeeper *bankkeeper.BaseKeeper, bech32IbcKeeper *bech32ibckeeper.Keeper, distrKeeper *distrkeeper.Keeper,
	mintKeeper *mintkeeper.Keeper, stakingKeeper *stakingkeeper.Keeper,
) func(
	ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap,
) (module.VersionMap, error) {
	if mm == nil || configurator == nil || accountKeeper == nil || bankKeeper == nil || bech32IbcKeeper == nil ||
		distrKeeper == nil || mintKeeper == nil || stakingKeeper == nil {
		panic("Nil argument to GetV2UpgradeHandler")
	}
	return func(ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap) (module.VersionMap, error) {
		ctx.Logger().Info("Mercury upgrade: Enter handler")
		// We previously upgraded via genesis, thus we don't want to run upgrades for all the modules
		fromVM := make(map[string]uint64)
		ctx.Logger().Info("Mercury upgrade: Creating version map")
		for moduleName, module := range mm.Modules {
			fromVM[moduleName] = module.ConsensusVersion()
		}

		ctx.Logger().Info("Mercury upgrade: Overwriting Gravity module version", "old", fromVM[gravitytypes.StoreKey], "new", 1)
		// Lower the gravity module version because we want to run that upgrade
		fromVM[gravitytypes.StoreKey] = 1

		ctx.Logger().Info("Mercury Upgrade: Setting up bech32ibc module's native prefix")
		err := setupBech32ibcKeeper(bech32IbcKeeper, ctx)
		if err != nil {
			panic(sdkerrors.Wrap(err, "Mercury Upgrade: Unable to upgrade, bech32ibc module not initialized"))
		}

		// distribution module: Fix the issues caused by an airdrop giving distribution module some tokens
		ctx.Logger().Info("Mercury Upgrade: Fixing community pool balance")
		err = fixDistributionPoolBalance(accountKeeper, bankKeeper, distrKeeper, mintKeeper, ctx)
		if err != nil {
			panic(sdkerrors.Wrap(err, "Mercury Upgrade: Unable to upgrade, distribution module balance could not be corrected"))
		}

		ctx.Logger().Info("Mercury Upgrade: Enforcing validator minimum commission")
		bumpMinValidatorCommissions(stakingKeeper, ctx)

		ctx.Logger().Info("Mercury Upgrade: Running all configured module migrations (Should only see Gravity run)")
		return mm.RunMigrations(ctx, *configurator, fromVM)
	}
}

// Sets up bech32ibc module by setting the native account prefix to "gravity"
// Failing to set the native prefix will cause a chain halt on init genesis or in the firstBeginBlocker assertions
func setupBech32ibcKeeper(bech32IbcKeeper *bech32ibckeeper.Keeper, ctx sdk.Context) error {
	return bech32IbcKeeper.SetNativeHrp(ctx, sdk.GetConfig().GetBech32AccountAddrPrefix())
}

// Fixes the invalid community pool balance caused by an airdrop airdropping to the distribution module account
// causing the distribution invariant checking module balance == (pool balance + validator outstanding rewards) failure
// We fix by overwriting the pool's ugraviton integer amount with the distribution module's integer amount less rewards
// If no discrepancy exists, this function does nothing
func fixDistributionPoolBalance(
	accountKeeper *authkeeper.AccountKeeper, bankKeeper *bankkeeper.BaseKeeper, distrKeeper *distrkeeper.Keeper,
	mintKeeper *mintkeeper.Keeper, ctx sdk.Context,
) error {
	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance(): Enter function, getting module account")
	distrAcc := accountKeeper.GetModuleAccount(ctx, distrtypes.ModuleName).GetAddress()
	ugraviton := mintKeeper.GetParams(ctx).MintDenom
	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance():", "distrAcc", distrAcc.String(), "chain-denom", ugraviton)

	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance(): Obtaining distribution module and community pool balances")
	distrBal := bankKeeper.GetBalance(ctx, distrAcc, ugraviton) // distr Int balance
	distrBalCoins := sdk.NewCoins(distrBal)                     // distr Int balance as Coins
	commPool := distrKeeper.GetFeePoolCommunityCoins(ctx)       // pool Dec balances

	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance():", "distrBal", distrBal.String(), "commPool", commPool.String())

	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance(): Finding community pool balances less ugraviton")
	// Collect the community pool's non-ugraviton balances which will be unmodified
	commPoolNoGrav := sdk.NewDecCoins()
	for _, c := range commPool {
		if c.Denom != ugraviton {
			ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance(): adding denom to commPoolNoGrav", "denom", c.Denom, "amount", c.Amount)
			commPoolNoGrav = commPoolNoGrav.Add(c)
		}
	}
	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance():", "commPoolNoGrav", commPoolNoGrav.String())

	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance(): Summing validator outstanding rewards")
	// Collect the current validator outstanding rewards
	var sumRewards sdk.DecCoins
	distrKeeper.IterateValidatorOutstandingRewards(ctx, func(_ sdk.ValAddress, rewards distrtypes.ValidatorOutstandingRewards) (stop bool) {
		sumRewards = sumRewards.Add(rewards.Rewards...)
		return false
	})
	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance():", "sumRewards", sumRewards.String())
	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance(): Finding outstanding validator ugraviton rewards")
	// Get only the ugraviton rewards as DecCoins
	ugravitonRewards := sdk.NewDecCoins(sdk.NewDecCoinFromDec(ugraviton, sumRewards.AmountOf(ugraviton)))
	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance():", "ugravitonRewards", ugravitonRewards.String())

	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance(): Finding distribution moduel balance less validator ugraviton rewards")
	// This is what the community pool's balance should be, as ideally distr bal = (pool bal + validator outstanding rewards)
	distrBalLessRewards, invalidBal := sdk.NewDecCoinsFromCoins(distrBalCoins...).SafeSub(ugravitonRewards)
	if invalidBal {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidCoins,
			"distribution module ugraviton balance (%+v) is lower than the validator outstanding rewards (%+v)!",
			distrBal, ugravitonRewards,
		)
	}
	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance():", "distrBalLessRewards", distrBalLessRewards.String())

	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance(): Calculating discrepancy between community pool ugraviton and (distribution module ugraviton less valdiator rewards)")
	commPoolUgraviton := sdk.NewDecCoins(sdk.NewDecCoinFromDec(ugraviton, commPool.AmountOf(ugraviton)))
	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance():", "commPoolUgraviton", commPoolUgraviton.String())
	discrepancy, invalidBal := distrBalLessRewards.SafeSub(commPoolUgraviton)
	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance():", "discrepancy", discrepancy.String(), "invalidBal", invalidBal)
	if invalidBal {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidCoins,
			"distribution module ugraviton balance less outstanding rewards (%+v) is lower than community pool ugraviton (%+v)!",
			distrBalLessRewards, commPoolUgraviton,
		)
	}

	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance(): Checking work")
	// Check our work before modifying any balances
	distrBals := sdk.NewDecCoinsFromCoins(bankKeeper.GetAllBalances(ctx, distrAcc)...)
	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance():", "distrBals", distrBals.String())

	expectedDiscrepancy := distrBals.Sub(sumRewards).Sub(commPool)
	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance():", "expectedDiscrepancy", expectedDiscrepancy.String())
	if !discrepancy.Sub(expectedDiscrepancy).AmountOf(ugraviton).IsZero() {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidCoins,
			"unexpected discrepancy (%+v) vs expected ugraviton discrepancy (%+v)",
			expectedDiscrepancy.AmountOf(ugraviton).String(), discrepancy.String(),
		)
	}

	// No discrepancy - do nothing
	if discrepancy.IsZero() && expectedDiscrepancy.IsZero() {
		ctx.Logger().Info("Mercury Upgrade: No distribution module imbalance discovered - not fixing anything!")
		return nil
	}

	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance(): Fixing the community pool balance!")
	// Final tally of all coins with ugraviton (distrBalLessRewards is only the ugraviton balance)
	fixedPool := commPoolNoGrav.Add(distrBalLessRewards...)
	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance():", "fixedPool", fixedPool.String())

	feePool := distrtypes.FeePool{CommunityPool: fixedPool}
	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance():", "feePool", feePool.String())
	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance(): Setting feePool on distribution keeper")
	distrKeeper.SetFeePool(ctx, feePool)

	// Check the invariants after our modifications
	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance(): Running distribution module invariants!")
	issueMsg, issue := distrkeeper.AllInvariants(*distrKeeper)(ctx)
	if issue {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidCoins, issueMsg)
	}

	ctx.Logger().Info("Mercury Upgrade: fixDistributionPoolBalance(): Success! Distribution invariant has been fixed")
	return nil
}

// Enforce minimum 5% validator commission on all noncompliant validators
// The MinCommissionDecorator enforces new validators must be created with a minimum commission rate of 5%,
// but existing validators are unaffected, here we automatically bump them all to 5% if they are lower
func bumpMinValidatorCommissions(stakingKeeper *stakingkeeper.Keeper, ctx sdk.Context) {
	ctx.Logger().Info("Mercury Upgrade: bumpMinValidatorCommissions(): Enter function")
	// This logic was originally included in the Juno project at github.com/CosmosContracts/juno/blob/main/app/app.go
	// This version was added to Juno by github user the-frey https://github.com/the-frey
	ctx.Logger().Info("Mercury Upgrade: bumpMinValidatorCommissions(): Getting all the validators")
	validators := stakingKeeper.GetAllValidators(ctx)
	// hard code this because we don't want
	// a) a fork or
	// b) immediate reaction with additional gov props
	minCommissionRate := sdk.NewDecWithPrec(5, 2)
	ctx.Logger().Info("Mercury Upgrade: bumpMinValidatorCommissions():", "minCommissionRate", minCommissionRate.String())
	ctx.Logger().Info("Mercury Upgrade: bumpMinValidatorCommissions(): Iterating validators")
	for _, v := range validators {
		ctx.Logger().Info("Mercury Upgrade: bumpMinValidatorCommissions(): checking validator", "validator", v.GetMoniker(), "Commission.Rate", v.Commission.Rate.String(), "Commission.MaxRate", v.Commission.MaxRate.String())
		if v.Commission.Rate.LT(minCommissionRate) {
			ctx.Logger().Info("Mercury Upgrade: bumpMinValidatorCommissions(): validator is out of compilance! Modifying their commission rate(s)", "validator ", v.GetMoniker())
			if v.Commission.MaxRate.LT(minCommissionRate) {
				ctx.Logger().Info("Mercury Upgrade: bumpMinValidatorCommissions(): Updating validator Commission.MaxRate", "validator", v.GetMoniker(), "old", v.Commission.MaxRate.String(), "new", minCommissionRate.String())
				v.Commission.MaxRate = minCommissionRate
			}

			ctx.Logger().Info("Mercury Upgrade: bumpMinValidatorCommissions(): Updating validator Commission.Rate", v.GetMoniker(), "old", v.Commission.Rate.String(), "new", minCommissionRate.String())
			v.Commission.Rate = minCommissionRate
			ctx.Logger().Info("Mercury Upgrade: bumpMinValidatorCommissions(): also UpdateTime ", "old", v.Commission.UpdateTime.String(), "new", ctx.BlockHeader().Time.String())
			v.Commission.UpdateTime = ctx.BlockHeader().Time

			ctx.Logger().Info("Mercury Upgrade: bumpMinValidatorCommissions(): calling the hook")
			// call the before-modification hook since we're about to update the commission
			stakingKeeper.BeforeValidatorModified(ctx, v.GetOperator())

			ctx.Logger().Info("Mercury Upgrade: bumpMinValidatorCommissions(): setting the validator")
			stakingKeeper.SetValidator(ctx, v)

			v, _ = stakingKeeper.GetValidator(ctx, v.GetOperator()) // Refresh since we set them in the keeper
			ctx.Logger().Info("Mercury Upgrade: bumpMinValidatorCommissions(): validator's set rate", "validator", v.GetMoniker(), "Commission", v.Commission.String())
		}
	}
}
