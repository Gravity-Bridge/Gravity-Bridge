package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	"github.com/stretchr/testify/require"
)

func (suite *KeeperTestSuite) TestModuleBalanceInvariant() {
	InitPoolAndAuctionTokens(suite)

	ctx := suite.Ctx
	auctionKeeper := suite.App.AuctionKeeper
	period := auctionKeeper.GetAuctionPeriod(ctx)

	// Initialize the auctions by calling endblocker at the current period's height
	ctx = ctx.WithBlockHeight(int64(period.EndBlockHeight))
	auction.EndBlocker(ctx, *auctionKeeper)
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)

	// At this point the TestBalances variable has been populated with the auction tokens
	expBalances := keeper.ExpectedAuctionModuleBalances(ctx, *auctionKeeper)
	AssertCorrectInvariant(suite.T(), ctx, *auctionKeeper, expBalances)

	Bid(suite, TestAccounts[0], 1000, 3500, 0, true) // Bid 1000 on first
	firstBidBalances := TestBalances.Add(sdk.NewInt64Coin(GravDenom, 1000))
	AssertCorrectInvariant(suite.T(), ctx, *auctionKeeper, firstBidBalances)

	Bid(suite, TestAccounts[0], 1000, 4000, 1, true) // Bid 1000 on second
	secondBidBalances := firstBidBalances.Add(sdk.NewInt64Coin(GravDenom, 1000))
	AssertCorrectInvariant(suite.T(), ctx, *auctionKeeper, secondBidBalances)

	Bid(suite, TestAccounts[0], 2000, 3500, 0, false) // Rebid 2000 - rebids not allowed
	AssertCorrectInvariant(suite.T(), ctx, *auctionKeeper, secondBidBalances)

	Bid(suite, TestAccounts[1], 2000, 4500, 0, true) // Bid 2000 on first
	thirdBidBalances := secondBidBalances.Add(sdk.NewInt64Coin(GravDenom, 1000))
	AssertCorrectInvariant(suite.T(), ctx, *auctionKeeper, thirdBidBalances)

	// End the period
	period = auctionKeeper.GetAuctionPeriod(ctx)
	ctx = ctx.WithBlockHeight(int64(period.EndBlockHeight))
	auction.EndBlocker(ctx, *auctionKeeper)

	// Ensure the module does not hold any balances after the period ends (no new auctions)
	postPeriodBalances := sdk.NewCoins()
	AssertCorrectInvariant(suite.T(), ctx, *auctionKeeper, postPeriodBalances)

	// Fund the auction pool with a new coin during the current period
	heyCoin := sdk.NewCoins(sdk.NewCoin("Hey", sdk.NewInt(1000000000000000000)))
	suite.FundAuctionPool(ctx, heyCoin)
	period = auctionKeeper.GetAuctionPeriod(ctx)
	ctx = ctx.WithBlockHeight(int64(period.EndBlockHeight))

	// Test the invariant when the new auctions start
	auction.EndBlocker(ctx, *auctionKeeper)
	AssertCorrectInvariant(suite.T(), ctx, *auctionKeeper, heyCoin)
}

func AssertCorrectInvariant(t *testing.T, ctx sdk.Context, k keeper.Keeper, expectedBalances sdk.Coins) {
	// Check the helper function calculates the right result
	actualBalances := k.BankKeeper.GetAllBalances(ctx, ModuleAccount)
	require.Equal(t, expectedBalances, actualBalances)

	// Test that the invariant finds the same result
	res, failure := keeper.ModuleBalanceInvariant(k)(ctx)
	require.Equal(t, "", res)
	require.False(t, failure)
}
