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
	distrAcc := accountKeeper.GetModuleAccount(ctx, distrtypes.ModuleName).GetAddress()
	ugraviton := mintKeeper.GetParams(ctx).MintDenom

	distrBal := bankKeeper.GetBalance(ctx, distrAcc, ugraviton) // distr Int balance
	distrBalCoins := sdk.NewCoins(distrBal)                     // distr Int balance as Coins
	commPool := distrKeeper.GetFeePoolCommunityCoins(ctx)       // pool Dec balances

	// Collect the community pool's non-ugraviton balances which will be unmodified
	commPoolNoGrav := sdk.NewDecCoins()
	for _, c := range commPool {
		if c.Denom != ugraviton {
			commPoolNoGrav.Add(c)
		}
	}

	// Collect the current validator outstanding rewards
	var sumRewards sdk.DecCoins
	distrKeeper.IterateValidatorOutstandingRewards(ctx, func(_ sdk.ValAddress, rewards distrtypes.ValidatorOutstandingRewards) (stop bool) {
		sumRewards = sumRewards.Add(rewards.Rewards...)
		return false
	})
	// Get only the ugraviton rewards as DecCoins
	ugravitonRewards := sdk.NewDecCoins(sdk.NewDecCoinFromDec(ugraviton, sumRewards.AmountOf(ugraviton)))

	// This is what the community pool's balance should be, as ideally distr bal = (pool bal + validator outstanding rewards)
	distrBalLessRewards, invalidBal := sdk.NewDecCoinsFromCoins(distrBalCoins...).SafeSub(ugravitonRewards)
	if invalidBal {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidCoins,
			"distribution module ugraviton balance (%+v) is lower than the validator outstanding rewards (%+v)!",
			distrBal, ugravitonRewards,
		)
	}

	commPoolUgraviton := sdk.NewDecCoins(sdk.NewDecCoinFromDec(ugraviton, commPool.AmountOf(ugraviton)))
	discrepancy, invalidBal := distrBalLessRewards.SafeSub(commPoolUgraviton)
	if invalidBal {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidCoins,
			"distribution module ugraviton balance less outstanding rewards (%+v) is lower than community pool ugraviton (%+v)!",
			distrBalLessRewards, commPoolUgraviton,
		)
	}

	// Check our work before modifying any balances
	distrBals := sdk.NewDecCoinsFromCoins(bankKeeper.GetAllBalances(ctx, distrAcc)...)
	expectedDiscrepancy := distrBals.Sub(sumRewards).Sub(commPool)
	if !discrepancy.Sub(expectedDiscrepancy).IsZero() {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidCoins,
			"unexpected discrepancy (%+v) vs expected ugraviton discrepancy (%+v)",
			expectedDiscrepancy, discrepancy,
		)
	}

	// No discrepancy - do nothing
	if discrepancy.IsZero() && expectedDiscrepancy.IsZero() {
		ctx.Logger().Info("v2 Upgrade: No distribution module imbalance discovered")
		return nil
	}

	// Final tally of all coins with ugraviton (distrBalLessRewards is only the ugraviton balance)
	fixedPool := commPoolNoGrav.Add(distrBalLessRewards...)

	feePool := distrKeeper.GetFeePool(ctx)
	feePool = distrtypes.FeePool{CommunityPool: fixedPool}
	distrKeeper.SetFeePool(ctx, feePool)

	// Check the invariants after our modifications
	issueMsg, issue := distrkeeper.AllInvariants(*distrKeeper)(ctx)
	if issue != false {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidCoins, issueMsg)
	}

	return nil
}

// Enforce minimum 5% validator commission on all noncompliant validators
// The MinCommissionDecorator enforces new validators must be created with a minimum commission rate of 5%,
// but existing validators are unaffected, here we automatically bump them all to 5% if they are lower
func bumpMinValidatorCommissions(stakingKeeper *stakingkeeper.Keeper, ctx sdk.Context) {
	// TODO: Implement me!
}
