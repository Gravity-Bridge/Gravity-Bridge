package pleiades

import (
	gravitykeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/module"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	bech32ibckeeper "github.com/osmosis-labs/bech32-ibc/x/bech32ibc/keeper"
)

func GetPleiadesUpgradeHandler(
	mm *module.Manager, configurator *module.Configurator, crisisKeeper *crisiskeeper.Keeper, gravityKeeper *gravitykeeper.Keeper, bech32ibckeeper *bech32ibckeeper.Keeper,
) func(
	ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap,
) (module.VersionMap, error) {
	if mm == nil {
		panic("Nil argument to GetPleiadesUpgradeHandler")
	}
	return func(ctx sdk.Context, plan upgradetypes.Plan, vmap module.VersionMap) (module.VersionMap, error) {
		ctx.Logger().Info("Pleiades upgrade: Enter handler")

		evmChains := []types.EvmChain{
			{
				EvmChainPrefix:     EthereumChainPrefix,
				EvmChainName:       "Binance Smart Chain",
				EvmChainNetVersion: 5,
			},
		}

		for _, evmChain := range evmChains {
			gravityKeeper.SetEvmChainData(ctx, evmChain)
		}

		// just in case the new version uses default native hrp which is osmo
		err := bech32ibckeeper.SetNativeHrp(ctx, sdk.GetConfig().GetBech32AccountAddrPrefix())
		if err != nil {
			panic(sdkerrors.Wrap(err, "Pleiades Upgrade: Unable to upgrade, bech32ibc module not initialized properly"))
		}

		ctx.Logger().Info("Pleiades Upgrade: Running any configured module migrations")
		out, outErr := mm.RunMigrations(ctx, *configurator, vmap)

		ctx.Logger().Info("Asserting invariants after upgrade")
		crisisKeeper.AssertInvariants(ctx)

		return out, outErr
	}
}

func GetPleiades2UpgradeHandler(
	mm *module.Manager, configurator *module.Configurator, crisisKeeper *crisiskeeper.Keeper,
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

		ctx.Logger().Info("Asserting invariants after upgrade")
		crisisKeeper.AssertInvariants(ctx)

		return out, outErr
	}
}
