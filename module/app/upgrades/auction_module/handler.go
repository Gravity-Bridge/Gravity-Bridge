package auction_module

import (
	auctionkeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	auctiontypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/cosmos-sdk/types/module"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
)

// TODO: Switch to black list ( remove native token , ..etc.. )
var (
	blacklistedTokens = map[string]struct{}{
		"ugraviton": {},
	}
	NextAuctionPeriodHeightMargin = uint64(5)
)

func CreateUpgradeHandler(
	mm *module.Manager,
	configurator module.Configurator,
	auctionKeeper *auctionkeeper.Keeper,
	bankKeeper *bankkeeper.BaseKeeper,
	accountKeeper *authkeeper.AccountKeeper,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
		ctx.Logger().Info("Starting upgrade...")

		fromVM := make(map[string]uint64)
		for moduleName, module := range mm.Modules {
			fromVM[moduleName] = module.ConsensusVersion()
		}

		// Set params
		defaultParams := auctiontypes.DefaultParams()

		defaultParams.AuctionRate = sdk.NewDecWithPrec(2, 1)

		balances := bankKeeper.GetAllBalances(ctx, accountKeeper.GetModuleAccount(ctx, distrtypes.ModuleName).GetAddress())
		allowList := make(map[string]bool)

		// Remove blacklisted denom from the list
		for _, coin := range balances {
			if _, found := blacklistedTokens[coin.Denom]; !found {
				continue
			}
			allowList[coin.Denom] = true
		}
		defaultParams.AllowTokens = allowList

		auctionKeeper.SetParams(ctx, defaultParams)

		// Set EstimateNextAuctionPeriodHeight
		auctionKeeper.SetEstimateAuctionPeriodBlockHeight(ctx, uint64(ctx.BlockHeight())+NextAuctionPeriodHeightMargin)

		ctx.Logger().Info("Upgrade Complete")
		return mm.RunMigrations(ctx, configurator, fromVM)
	}
}
