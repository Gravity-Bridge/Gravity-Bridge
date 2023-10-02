package keeper_test

import (
	"github.com/Gravity-Bridge/Gravity-Bridge/module/config"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func (suite *KeeperTestSuite) TestGenesis() {
	t := suite.T()
	ctx := suite.Ctx
	ak := suite.App.AuctionKeeper
	// Test import and export

	genesis := types.GenesisState{
		Params: types.DefaultParams(),
		ActivePeriod: &types.AuctionPeriod{
			StartBlockHeight: 1,
			EndBlockHeight:   1000,
		},
		ActiveAuctions: []types.Auction{},
	}
	keeper.InitGenesis(ctx, *ak, genesis)

	exported := keeper.ExportGenesis(ctx, *ak)
	require.NotNil(t, exported)
	require.Equal(t, genesis.Params, exported.Params)
	require.Equal(t, genesis.ActivePeriod, exported.ActivePeriod)
	require.Equal(t, len(genesis.ActiveAuctions), len(exported.ActiveAuctions))
	require.ElementsMatch(t, genesis.ActiveAuctions, exported.ActiveAuctions)
}

// Checks that the invalid InitGenesis conditions successfully trigger a panic and that with the correct setup an invalid genesis
// may become valid
func (suite *KeeperTestSuite) TestInvalidGenesis() {
	t := suite.T()
	ctx := suite.Ctx
	ak := suite.App.AuctionKeeper
	// Test import and export

	nonAuctionable := TestDenom1
	nonAuctionableCoin := sdk.NewCoin(nonAuctionable, sdk.NewInt(100))

	genesis := types.GenesisState{
		Params: types.DefaultParams(),
		ActivePeriod: &types.AuctionPeriod{
			StartBlockHeight: 1,
			EndBlockHeight:   1000,
		},
		ActiveAuctions: []types.Auction{{
			Id:         1,
			Amount:     nonAuctionableCoin,
			HighestBid: nil,
		}},
	}
	genesis.Params.NonAuctionableTokens = append(genesis.Params.NonAuctionableTokens, nonAuctionable)

	// This could panic because of the NonAuctionableTokens list or due to the bank balance of the module
	cacheCtx, _ := ctx.CacheContext() // cache so future tests work correctly
	require.Panics(t, func() {
		keeper.InitGenesis(cacheCtx, *ak, genesis)
	})

	err := ak.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(nonAuctionableCoin))
	require.NoError(t, err)

	// This must still panic because of the NonAuctionableTokens list
	cacheCtx, _ = ctx.CacheContext() // cache since we expect the init not to panic
	require.Panics(t, func() {
		keeper.InitGenesis(cacheCtx, *ak, genesis)
	})

	genesis.Params.NonAuctionableTokens = []string{config.NativeTokenDenom, TestDenom2}
	// This must not panic because we removed the denom from the NonAuctionbleTokens list
	cacheCtx, _ = ctx.CacheContext() // cache so future tests work correctly
	require.NotPanics(t, func() {
		keeper.InitGenesis(cacheCtx, *ak, genesis)
	})

	err = ak.BankKeeper.BurnCoins(ctx, types.ModuleName, sdk.NewCoins(nonAuctionableCoin))
	require.NoError(t, err)
	// This must panic because of the bank balance of the module
	cacheCtx, _ = ctx.CacheContext() // cache so future tests work correctly
	require.Panics(t, func() {
		keeper.InitGenesis(cacheCtx, *ak, genesis)
	})

	oldHeight := ctx.BlockHeight()
	ctx = ctx.WithBlockHeight(10000)
	genesis.ActivePeriod = &types.AuctionPeriod{
		StartBlockHeight: uint64(oldHeight + 1),
		EndBlockHeight:   uint64(oldHeight + 1000),
	}
	genesis.ActiveAuctions = []types.Auction{}
	// This must panic because the period is in the past
	cacheCtx, _ = ctx.CacheContext() // cache so future tests work correctly
	require.Panics(t, func() {
		keeper.InitGenesis(cacheCtx, *ak, genesis)
	})

	genesis.ActivePeriod = &types.AuctionPeriod{
		StartBlockHeight: uint64(ctx.BlockHeight() - 10),
		EndBlockHeight:   uint64(ctx.BlockHeight() - 11),
	}
	// This must panic because the period starts after it begins
	cacheCtx, _ = ctx.CacheContext() // cache so future tests work correctly
	require.Panics(t, func() {
		keeper.InitGenesis(cacheCtx, *ak, genesis)
	})
	genesis.ActivePeriod = &types.AuctionPeriod{
		StartBlockHeight: uint64(ctx.BlockHeight() - 10),
		EndBlockHeight:   uint64(ctx.BlockHeight() + 1000),
	}
	// This must not panic because the period is valid
	cacheCtx, _ = ctx.CacheContext() // cache so future tests work correctly
	require.NotPanics(t, func() {
		keeper.InitGenesis(cacheCtx, *ak, genesis)
	})
	genesis.ActivePeriod = &types.AuctionPeriod{
		StartBlockHeight: uint64(ctx.BlockHeight() - 10),
		EndBlockHeight:   uint64(ctx.BlockHeight()),
	}
	// This must not panic because the period is valid, even if it ends on this block
	cacheCtx, _ = ctx.CacheContext() // cache so future tests work correctly
	require.NotPanics(t, func() {
		keeper.InitGenesis(cacheCtx, *ak, genesis)
	})
}
