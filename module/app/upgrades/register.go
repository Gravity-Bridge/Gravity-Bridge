package upgrades

import (
	bech32ibckeeper "github.com/althea-net/bech32-ibc/x/bech32ibc/keeper"
	"github.com/cosmos/cosmos-sdk/types/module"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	upgradekeeper "github.com/cosmos/cosmos-sdk/x/upgrade/keeper"
	ibctransferkeeper "github.com/cosmos/ibc-go/v4/modules/apps/transfer/keeper"

	auctionkeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/app/upgrades/antares"
	auctionmodule "github.com/Gravity-Bridge/Gravity-Bridge/module/app/upgrades/auction_module"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/app/upgrades/orion"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/app/upgrades/pleiades"
	polaris "github.com/Gravity-Bridge/Gravity-Bridge/module/app/upgrades/polaris"
	v2 "github.com/Gravity-Bridge/Gravity-Bridge/module/app/upgrades/v2"
)

// RegisterUpgradeHandlers registers handlers for all upgrades
// Note: This method has crazy parameters because of circular import issues, I didn't want to make a Gravity struct
// along with a Gravity interface
func RegisterUpgradeHandlers(
	mm *module.Manager, configurator *module.Configurator, accountKeeper *authkeeper.AccountKeeper,
	bankKeeper *bankkeeper.BaseKeeper, bech32IbcKeeper *bech32ibckeeper.Keeper, distrKeeper *distrkeeper.Keeper,
	mintKeeper *mintkeeper.Keeper, stakingKeeper *stakingkeeper.Keeper, upgradeKeeper *upgradekeeper.Keeper,
	crisisKeeper *crisiskeeper.Keeper, transferKeeper *ibctransferkeeper.Keeper, auctionkeeper *auctionkeeper.Keeper,
) {
	if mm == nil || configurator == nil || accountKeeper == nil || bankKeeper == nil || bech32IbcKeeper == nil ||
		distrKeeper == nil || mintKeeper == nil || stakingKeeper == nil || upgradeKeeper == nil {
		panic("Nil argument to RegisterUpgradeHandlers()!")
	}
	// Mercury aka v1->v2 UPGRADE HANDLER SETUP
	upgradeKeeper.SetUpgradeHandler(
		v2.V1ToV2PlanName, // Codename Mercury
		v2.GetV2UpgradeHandler(mm, configurator, accountKeeper, bankKeeper, bech32IbcKeeper, distrKeeper, mintKeeper, stakingKeeper),
	)
	// Mercury Fix aka mercury2.0 UPGRADE HANDLER SETUP
	upgradeKeeper.SetUpgradeHandler(
		v2.V2FixPlanName, // mercury2.0
		v2.GetMercury2Dot0UpgradeHandler(),
	)

	// Polaris UPGRADE HANDLER SETUP
	upgradeKeeper.SetUpgradeHandler(
		polaris.V2toPolarisPlanName,
		polaris.GetPolarisUpgradeHandler(mm, configurator, crisisKeeper, transferKeeper),
	)

	// Pleiades aka v2->v3 UPGRADE HANDLER SETUP
	upgradeKeeper.SetUpgradeHandler(
		pleiades.PolarisToPleiadesPlanName,
		pleiades.GetPleiadesUpgradeHandler(mm, configurator, crisisKeeper),
	)

	// Pleiades part 2 aka v3->v4 UPGRADE HANDLER SETUP
	upgradeKeeper.SetUpgradeHandler(
		pleiades.PleiadesPart1ToPart2PlanName,
		pleiades.GetPleiades2UpgradeHandler(mm, configurator, crisisKeeper, stakingKeeper),
	)

	// Orion upgrade handler
	upgradeKeeper.SetUpgradeHandler(
		orion.PleiadesPart2ToOrionPlanName,
		orion.GetOrionUpgradeHandler(mm, configurator, crisisKeeper),
	)

	// Antares upgrade handler
	upgradeKeeper.SetUpgradeHandler(
		antares.OrionToAntaresPlanName,
		antares.GetAntaresUpgradeHandler(mm, configurator, crisisKeeper),
	)

	// Add auction module
	upgradeKeeper.SetUpgradeHandler(
		auctionmodule.AddAuctionModulePlanName,
		auctionmodule.CreateUpgradeHandler(mm, *configurator, auctionkeeper, bankKeeper, accountKeeper),
	)
}
