package keeper_test

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	helpers "github.com/Gravity-Bridge/Gravity-Bridge/module/app/apptesting"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

const (
	TestDenom1 = "foocoin"
	TestDenom2 = "ibc/18DB4F18E0C631514AFA67261BCC5FA62F46B2E453778D0CE5AE5234D3B7C1CF"
)

var TestBalances sdk.Coins
var TestAccounts []sdk.AccAddress
var GravDenom string
var ModuleAccount sdk.AccAddress

func (suite *KeeperTestSuite) TestEndBlockerAuction() {
	InitPoolAndAuctionTokens(suite)

	ctx := suite.Ctx
	auctionKeeper := suite.App.AuctionKeeper
	auctionParams := auctionKeeper.GetParams(ctx)
	auctionParams.BurnWinningBids = false
	auctionKeeper.SetParams(ctx, auctionParams)

	ctx = ctx.WithBlockHeight(int64(auctionParams.AuctionLength) + ctx.BlockHeight())
	auction.EndBlocker(ctx, *auctionKeeper)
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)

	// Create an auction period
	block := uint64(ctx.BlockHeight())
	period := auctionKeeper.GetAuctionPeriod(ctx)
	require.Equal(suite.T(), &types.AuctionPeriod{
		StartBlockHeight: block,
		EndBlockHeight:   block + auctionParams.AuctionLength,
	}, period)

	// Observe created auctions
	auctions := auctionKeeper.GetAllAuctions(ctx)
	auctionCoins := sdk.NewCoins()
	for _, auction := range auctions {
		auctionCoins = append(auctionCoins, auction.Amount)
	}

	require.Equal(suite.T(), TestBalances, auctionCoins)

	// Bid on some of the auctions
	Bid(suite, TestAccounts[0], 1000, 3000, 0, false) // Fail to bid 1000 - insufficient fee
	Bid(suite, TestAccounts[0], 1000, 3500, 0, true)  // Bid 1000 on first
	AdvanceBlock(&ctx, auctionKeeper)
	Bid(suite, TestAccounts[0], 1000, 3110, 1, true) // Bid 1000 on second
	AdvanceBlock(&ctx, auctionKeeper)

	// Rebid, outbid, rebid, ...
	Bid(suite, TestAccounts[0], 2000, 3109, 0, false) // Rebid 2000 - insufficient fee
	Bid(suite, TestAccounts[0], 2000, 3500, 0, false) // Rebid 2000 - rebids not allowed
	AdvanceBlock(&ctx, auctionKeeper)
	Bid(suite, TestAccounts[1], 2000, 999999, 0, true) // Bid 2000 on first
	Bid(suite, TestAccounts[0], 1500, 10, 0, false)    // Insufficient fee, insufficient amount
	Bid(suite, TestAccounts[0], 1500, 3500, 0, false)  // Outbid the initial bid but not the current highest
	Bid(suite, TestAccounts[0], 2500, 5500, 0, true)   // Outbid the the current highest
	AdvanceBlock(&ctx, auctionKeeper)
	Bid(suite, TestAccounts[0], 2300, 9999000, 0, false)  // Rebid but for too low
	Bid(suite, TestAccounts[0], 2500, 70000700, 0, false) // Rebid but at current highest
	Bid(suite, TestAccounts[0], 2500, 9000, 0, false)     // Fail to outbid

	// End the auction period
	ctx = ctx.WithBlockHeight(int64(period.EndBlockHeight))
	auction.EndBlocker(ctx, *auctionKeeper)
	VerifyAuctionPayout(suite, TestAccounts[0], auctions[0], 2500, true)
	VerifyAuctionPayout(suite, TestAccounts[0], auctions[1], 1000, true)

	AssertModuleBalanceStrict(suite, sdk.NewCoins())

	// Observe no new auctions
	auctions = auctionKeeper.GetAllAuctions(ctx)
	require.Empty(suite.T(), auctions)

	// Create one auction this time
	auctionCoins = sdk.NewCoins(TestBalances[0])
	suite.FundAuctionPool(ctx, auctionCoins)

	// Run endblocker with no active auctions, observe one auction created
	period = auctionKeeper.GetAuctionPeriod(ctx)
	ctx = ctx.WithBlockHeight(int64(period.EndBlockHeight))
	auction.EndBlocker(ctx, *auctionKeeper)
	AssertModuleBalanceStrict(suite, auctionCoins)

	auctions = auctionKeeper.GetAllAuctions(ctx)
	require.Equal(suite.T(), len(auctions), 1)

	// Let this auction fail
	require.Nil(suite.T(), auctions[0].HighestBid)
	period = auctionKeeper.GetAuctionPeriod(ctx)
	ctx = ctx.WithBlockHeight(int64(period.EndBlockHeight))
	auction.EndBlocker(ctx, *auctionKeeper)

	// ensure the same auction has been created but with a new ID
	newAuctions := auctionKeeper.GetAllAuctions(ctx)
	require.Equal(suite.T(), len(newAuctions), 1)
	require.True(suite.T(), newAuctions[0].Id != auctions[0].Id)
	require.Equal(suite.T(), newAuctions[0].Amount, auctions[0].Amount)
	require.Nil(suite.T(), newAuctions[0].HighestBid)
}

// Increments the ctx block height and runs EndBlocker
func AdvanceBlock(ctx *sdk.Context, k *keeper.Keeper) {
	*ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
	auction.EndBlocker(*ctx, *k)
}

func (suite *KeeperTestSuite) TestWinningBidBurning() {
	InitPoolAndAuctionTokens(suite)

	ctx := suite.Ctx
	auctionKeeper := suite.App.AuctionKeeper
	auctionParams := auctionKeeper.GetParams(ctx)

	ctx = ctx.WithBlockHeight(int64(auctionParams.AuctionLength) + ctx.BlockHeight())
	auction.EndBlocker(ctx, *auctionKeeper)
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)

	// Create an auction period
	params := auctionKeeper.GetParams(ctx)

	// Update to burn the winning bids
	params.BurnWinningBids = true
	auctionKeeper.SetParams(ctx, params)

	block := uint64(ctx.BlockHeight())
	period := auctionKeeper.GetAuctionPeriod(ctx)
	require.Equal(suite.T(), &types.AuctionPeriod{
		StartBlockHeight: block,
		EndBlockHeight:   block + params.AuctionLength,
	}, period)

	// Observe created auctions
	auctions := auctionKeeper.GetAllAuctions(ctx)
	auctionCoins := sdk.NewCoins()
	for _, auction := range auctions {
		auctionCoins = append(auctionCoins, auction.Amount)
	}

	require.Equal(suite.T(), TestBalances, auctionCoins)

	// Bid on some of the auctions
	Bid(suite, TestAccounts[0], 1000, 3500, 0, true) // Bid 1000 on first
	Bid(suite, TestAccounts[0], 1000, 4000, 1, true) // Bid 1000 on second
	Bid(suite, TestAccounts[1], 2000, 3500, 0, true) // Bid 2000 on first

	// Expecting to have 3k burned
	preBurn := auctionKeeper.BankKeeper.GetSupply(ctx, GravDenom)
	// End the auction period
	ctx = ctx.WithBlockHeight(int64(period.EndBlockHeight))
	auction.EndBlocker(ctx, *auctionKeeper)
	postBurn := auctionKeeper.BankKeeper.GetSupply(ctx, GravDenom)
	VerifyAuctionPayout(suite, TestAccounts[1], auctions[0], 2500, false)
	VerifyAuctionPayout(suite, TestAccounts[0], auctions[1], 1000, false)

	require.Equal(suite.T(), int64(3000), preBurn.Sub(postBurn).Amount.Int64())
}

// Initializes the auction pool, funds several accounts and populates some tokens to be used in future auctions
func InitPoolAndAuctionTokens(suite *KeeperTestSuite) {
	ctx := suite.Ctx

	GravDenom = suite.App.MintKeeper.GetParams(ctx).MintDenom // Native Token: Use the mint denom for flexibility
	fmt.Printf("Grav in test env is %s\n", GravDenom)

	// Create test balances for the auction pool
	testAmount := sdk.NewInt(helpers.OneAtom() * 1000)
	TestBalances = sdk.NewCoins(
		sdk.NewCoin(TestDenom1, testAmount),
		sdk.NewCoin(TestDenom2, testAmount),
	)
	suite.FundAuctionPool(ctx, TestBalances)

	// Fund some users with the native coin (aka GRAV)
	testGrav := sdk.NewCoin(GravDenom, testAmount)
	TestAccounts = suite.CreateAndFundRandomAccounts(3, sdk.NewCoins(testGrav))

	params := suite.App.AuctionKeeper.GetParams(ctx)
	params.NonAuctionableTokens = []string{GravDenom}
	suite.App.AuctionKeeper.SetParams(ctx, params)

	ModuleAccount = suite.App.AccountKeeper.GetModuleAddress(types.ModuleName)
}

// Performs a bid and checks the balance changes of the bidder and the auction module
// whichAuction is the index of the currently active auction to consider
// expNewHighestBid should be true when the bid will cause `account` to be the new highest bidder on that auction
func Bid(suite *KeeperTestSuite, account sdk.AccAddress, amount int64, fee int64, whichAuction int64, expNewHighestBid bool) {
	ctx := suite.Ctx
	t := suite.T()
	auctionKeeper := suite.App.AuctionKeeper
	msgServer := keeper.NewMsgServerImpl(*auctionKeeper)
	auction := auctionKeeper.GetAllAuctions(ctx)[whichAuction]
	rebid := auction.HighestBid != nil && auction.HighestBid.BidderAddress == account.String()

	preGravBalAcc := suite.App.BankKeeper.GetBalance(ctx, account, GravDenom)
	preGravBalMod := suite.App.BankKeeper.GetBalance(ctx, ModuleAccount, GravDenom)

	_, err := msgServer.Bid(sdk.WrapSDKContext(ctx), types.NewMsgBid(auction.Id, account.String(), uint64(amount), uint64(fee)))
	if expNewHighestBid {
		require.NoError(t, err)
	}

	postGravBalAcc := suite.App.BankKeeper.GetBalance(ctx, account, GravDenom)
	postGravBalMod := suite.App.BankKeeper.GetBalance(ctx, ModuleAccount, GravDenom)

	if expNewHighestBid {
		var expectedUserDifference, expectedModDifference uint64
		if rebid {
			// We will expect the user's difference to be the rebid amount
			expectedUserDifference = uint64(amount) - auction.HighestBid.BidAmount
			// We expect the module difference to be the rebid amount
			expectedModDifference = expectedUserDifference
		} else {
			// The user's difference should be the whole bid amount
			expectedUserDifference = uint64(amount)
			// The module's difference is only the amount on top of the old bid, or the whole amount if there was no previous
			var oldBidAmount uint64 = 0
			if auction.HighestBid != nil {
				oldBidAmount = auction.HighestBid.BidAmount
			}
			expectedModDifference = uint64(amount) - oldBidAmount
		}

		actualUserDifference := preGravBalAcc.Amount.Sub(postGravBalAcc.Amount)
		actualModDifference := postGravBalMod.Amount.Sub(preGravBalMod.Amount)
		require.True(t, actualModDifference.IsPositive(), "Module balance increased")
		require.True(t, actualUserDifference.IsPositive(), "User balance decreased")
		if rebid {
			fmt.Printf("Expecting mod increase of %v - %v = %v \n", amount, auction.HighestBid.BidAmount, expectedModDifference)
			require.Equal(t, expectedModDifference, actualModDifference.Uint64(), "module balance did not increase by exactly the rebid amount")
			require.True(t, actualUserDifference.Uint64() >= expectedUserDifference, "user balance did not decrease by rebid amount + fee")
		} else {
			require.Equal(t, expectedModDifference, actualModDifference.Uint64())
			require.True(t, actualUserDifference.Uint64() >= expectedUserDifference, "user balance did not decrease by amount + fee")
		}
	}
}

// Verifies that the given `auction` was paid to `expWinner` and their `winningBid` has been paid to the community pool or burned
// Optionally verifies that the community pool has received the bid amounts if `verifyPoolGrav` is true
func VerifyAuctionPayout(suite *KeeperTestSuite, expWinner sdk.AccAddress, auction types.Auction, winningBid uint64, verifyPoolGrav bool) {
	ctx := suite.Ctx
	t := suite.T()
	auctionKeeper := suite.App.AuctionKeeper

	shouldBeNil := auctionKeeper.GetAuctionById(ctx, auction.Id)
	require.Nil(t, shouldBeNil, "Discovered auction lingering in store after period ended")

	awardBalance := auctionKeeper.BankKeeper.GetBalance(ctx, expWinner, auction.Amount.Denom)
	require.True(t, awardBalance.IsGTE(auction.Amount), "winner %v has insufficient award token balance", expWinner.String())

	auctionPool := auctionKeeper.GetAuctionPoolBalances(ctx)
	poolCoin := auctionPool.AmountOf(auction.Amount.Denom)
	require.True(t, poolCoin.IsZero(), "Positive auction pool balance of reward token after auction success")

	if verifyPoolGrav {
		communityPool, _ := auctionKeeper.DistKeeper.GetFeePoolCommunityCoins(ctx).TruncateDecimal()
		poolGrav := communityPool.AmountOf(GravDenom)
		expGrav := sdk.NewIntFromUint64(winningBid)
		require.True(t, poolGrav.GTE(expGrav), "community pool does not have the bidders tokens")
	}
}

// Asserts that the module's balances exactly match `coins`, and that no other coins are held by the module
func AssertModuleBalanceStrict(suite *KeeperTestSuite, coins sdk.Coins) {
	require.True(suite.T(), checkModuleBalanceStrict(suite, coins), "module balance does not match coins")
}
func checkModuleBalanceStrict(suite *KeeperTestSuite, coins sdk.Coins) bool {
	bankKeeper := suite.App.AuctionKeeper.BankKeeper
	balances := bankKeeper.GetAllBalances(suite.Ctx, ModuleAccount)

	return coins.IsEqual(balances)
}

// Asserts that the pool contains the exact same amount of each coin provided in `coins`
// Makes no assertions about other coins that may exist in the pool
func AssertPoolBalanceRelaxed(suite *KeeperTestSuite, coins sdk.Coins) {
	require.True(suite.T(), checkPoolBalanceRelaxed(suite, coins))
}
func checkPoolBalanceRelaxed(suite *KeeperTestSuite, coins sdk.Coins) bool {
	poolCoins := suite.App.AuctionKeeper.GetAuctionPoolBalances(suite.Ctx)

	for _, coin := range coins {
		poolAmt := poolCoins.AmountOf(coin.Denom)
		if !poolAmt.Equal(coin.Amount) {
			fmt.Printf("Pool Balance (%v) != Coin balance (%v)\n", poolAmt, coin)
			return false
		}
	}
	return true
}

// Tests the helper functions to make sure the above tests are actually checking what is expected
func (suite *KeeperTestSuite) TestHelpers() {
	InitPoolAndAuctionTokens(suite)

	ctx := suite.Ctx
	t := suite.T()
	bankKeeper := suite.App.BankKeeper
	auctionKeeper := suite.App.AuctionKeeper

	expectedAuctionAccount := suite.App.AccountKeeper.GetModuleAccount(ctx, types.ModuleName).GetAddress()
	require.Equal(t, ModuleAccount, expectedAuctionAccount) // The ModuleAccount is cached, ensure it is correct

	// Check the empty balances first
	actualBalances := bankKeeper.GetAllBalances(ctx, ModuleAccount)
	require.Equal(t, sdk.NewCoins(), actualBalances)
	require.True(t, checkModuleBalanceStrict(suite, actualBalances))

	// Check the base pool coins
	actualCoins := bankKeeper.GetAllBalances(ctx, auctionKeeper.GetAuctionPoolAccount(ctx))
	require.True(t, checkPoolBalanceRelaxed(suite, actualCoins))
	require.True(t, checkPoolBalanceRelaxed(suite, sdk.NewCoins()))
	require.False(t, checkPoolBalanceRelaxed(suite, sdk.NewCoins(sdk.NewCoin("fakecoin", sdk.OneInt()))))
	require.False(t, checkPoolBalanceRelaxed(suite, sdk.NewCoins(actualCoins[0], sdk.NewCoin("fakecoin", sdk.OneInt()))))

	// Produce some auctions
	period := auctionKeeper.GetAuctionPeriod(ctx)
	ctx = ctx.WithBlockHeight(int64(period.EndBlockHeight))
	auction.EndBlocker(ctx, *suite.App.AuctionKeeper)

	// Ensure the auction now holds the amounts from the auction pool
	auctionBalances := bankKeeper.GetAllBalances(ctx, ModuleAccount)
	require.Equal(t, actualCoins, auctionBalances)
	require.True(t, checkModuleBalanceStrict(suite, auctionBalances))

	// Ensure the pool is empty now
	require.False(t, checkPoolBalanceRelaxed(suite, actualCoins))
	require.True(t, checkPoolBalanceRelaxed(suite, sdk.NewCoins()))
	actualCoins = bankKeeper.GetAllBalances(ctx, auctionKeeper.GetAuctionPoolAccount(ctx))
	require.Equal(t, sdk.NewCoins(), actualCoins)

	// Make a bid and ensure balance updates
	Bid(suite, TestAccounts[0], 10_000000, 50000, 0, true) // Bid 10 stake on first
	auctionWithBid := auctionKeeper.GetAllAuctions(ctx)[0]
	expectedCoins := auctionBalances.Add(sdk.NewCoin(GravDenom, sdk.NewInt(10_000000)))
	require.True(t, checkModuleBalanceStrict(suite, expectedCoins))

	// After the auction ends, check balance changes
	ctx = ctx.WithBlockHeight(int64(auctionKeeper.GetAuctionPeriod(ctx).EndBlockHeight))
	auction.EndBlocker(ctx, *auctionKeeper)
	require.False(t, checkModuleBalanceStrict(suite, expectedCoins))
	require.False(t, checkModuleBalanceStrict(suite, sdk.NewCoins()))
	expectedCoins = auctionBalances.Sub(auctionWithBid.Amount)
	require.True(t, checkModuleBalanceStrict(suite, expectedCoins))
	require.True(t, checkPoolBalanceRelaxed(suite, sdk.NewCoins()))
}
