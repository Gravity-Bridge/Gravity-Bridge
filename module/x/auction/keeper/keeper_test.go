package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/app/apptesting"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

type KeeperTestSuite struct {
	apptesting.AppTestHelper
	suite.Suite
	queryClient types.QueryClient
}

// Test helpers
func (suite *KeeperTestSuite) SetupTest() {
	suite.Setup()
	suite.queryClient = types.NewQueryClient(suite.QueryHelper)
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperTestSuite))
}

func (suite *KeeperTestSuite) TestParams() {
	t := suite.T()
	ctx := suite.Ctx
	ak := suite.App.AuctionKeeper

	params := ak.GetParams(ctx)
	defaultParams := types.DefaultParams()
	require.Equal(t, defaultParams, params)

	params.MinBidFee = 100
	params.BurnWinningBids = true
	params.NonAuctionableTokens = []string{"hi-there", "this", "is-not", "a-token", "ibc/abcdefg"}

	ak.SetParams(ctx, params)
	stored := ak.GetParams(ctx)
	require.Equal(t, params, stored)
}

// Tests taking from and adding to the community pool
func (suite *KeeperTestSuite) TestSendRemoveCommunityPool() {
	t := suite.T()
	ctx := suite.Ctx
	ak := suite.App.AuctionKeeper
	dk := ak.DistKeeper

	testCoins := sdk.NewCoins(
		sdk.NewCoin("test1", sdk.NewInt(1000_000000)),
		sdk.NewCoin("test2", sdk.NewInt(2000_000000)),
		sdk.NewCoin("test3", sdk.NewInt(3000_000000)),
		sdk.NewCoin("test4", sdk.NewInt(4000_000000)),
	)
	suite.FundCommunityPool(ctx, testCoins)

	removeFromPool := sdk.NewCoins(
		sdk.NewCoin("test1", sdk.NewInt(10_000000)),
		sdk.NewCoin("test4", sdk.NewInt(500_000000)),
	)
	preRemovePool := dk.GetFeePoolCommunityCoins(ctx)
	expectedPostRemovalPool := dk.GetFeePoolCommunityCoins(ctx)
	expectedPostRemovalPool = expectedPostRemovalPool.Sub(sdk.NewDecCoinsFromCoins(removeFromPool...))

	// Remove from community pool
	for _, coin := range removeFromPool {
		err := ak.RemoveFromCommunityPool(ctx, coin)
		require.NoError(t, err)
	}

	postRemovePool := dk.GetFeePoolCommunityCoins(ctx)
	require.Equal(t, expectedPostRemovalPool, postRemovePool)
	difference := preRemovePool.Sub(postRemovePool)
	require.Equal(t, sdk.NewDecCoinsFromCoins(removeFromPool...), difference)

	aucAcc := ak.AccountKeeper.GetModuleAddress(types.ModuleName)
	auctionBalances := ak.BankKeeper.GetAllBalances(ctx, aucAcc)
	require.Equal(t, removeFromPool, auctionBalances)

	// Send to pool
	preAddToPool := dk.GetFeePoolCommunityCoins(ctx)
	// Contains coins the module does not hold
	invalidAddToPool := sdk.NewCoins(
		sdk.NewCoin("ibc/abcdefg", sdk.NewInt(99990_000000)),
		sdk.NewCoin("test4", sdk.NewInt(5009900_000000)),
		sdk.NewCoin("test12", sdk.NewInt(5009900_000000)),
		sdk.NewCoin("stake", sdk.NewInt(50)),
	)
	err := ak.SendToCommunityPool(ctx, invalidAddToPool)
	require.Error(t, err)

	// Mint everything except the test4 token
	mint := sdk.NewCoins(
		sdk.NewCoin("ibc/abcdefg", sdk.NewInt(99990_000000)),
		sdk.NewCoin("test12", sdk.NewInt(5009900_000000)),
		sdk.NewCoin("stake", sdk.NewInt(50)),
	)
	err = ak.BankKeeper.MintCoins(ctx, types.ModuleName, mint)
	postMint := ak.BankKeeper.GetAllBalances(ctx, aucAcc)
	require.NoError(t, err)
	addToPool := sdk.NewCoins(
		sdk.NewCoin("ibc/abcdefg", sdk.NewInt(99990_000000)),
		sdk.NewCoin("test4", sdk.NewInt(59_000000)),
		sdk.NewCoin("test12", sdk.NewInt(5009900_000000)),
		sdk.NewCoin("stake", sdk.NewInt(50)),
	)
	expectedPostAddPool := preAddToPool.Add(sdk.NewDecCoinsFromCoins(addToPool...)...)
	err = ak.SendToCommunityPool(ctx, addToPool)
	require.NoError(t, err)
	postAddToPool := dk.GetFeePoolCommunityCoins(ctx)
	require.Equal(t, expectedPostAddPool, postAddToPool)
	difference = postAddToPool.Sub(preAddToPool)
	require.Equal(t, sdk.NewDecCoinsFromCoins(addToPool...), difference)

	auctionPostAdd := ak.BankKeeper.GetAllBalances(ctx, aucAcc)
	expectedAucBalances := postMint.Sub(addToPool)
	require.Equal(t, auctionPostAdd, expectedAucBalances)
}
