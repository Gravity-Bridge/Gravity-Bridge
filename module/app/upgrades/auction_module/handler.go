package auction_module

import (
	auctionkeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	auctiontypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
)

// TODO: Switch to black list ( remove native token , ..etc.. )
var (
	allowTokens = map[string]bool{
		// USDC
		"gravity0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48": true,
		// USDT
		"gravity0xdAC17F958D2ee523a2206206994597C13D831ec7": true,
	}
	NextAuctionPeriodHeightMargin = uint64(5)
)

func CreateUpgradeHandler(
	mm *module.Manager,
	configurator module.Configurator,
	auctionKeeper *auctionkeeper.Keeper,
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
		defaultParams.AllowTokens = allowTokens

		auctionKeeper.SetParams(ctx, defaultParams)

		// Set EstimateNextAuctionPeriodHeight
		auctionKeeper.SetEstimateAuctionPeriodBlockHeight(ctx, uint64(ctx.BlockHeight())+NextAuctionPeriodHeightMargin)

		ctx.Logger().Info("Upgrade Complete")
		return mm.RunMigrations(ctx, configurator, fromVM)
	}
}
