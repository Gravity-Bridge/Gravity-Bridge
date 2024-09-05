package keeper_test

import (
	"github.com/stretchr/testify/require"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

// Creates auction periods and tests their retrieval
func (suite *KeeperTestSuite) TestAuctionPeriodStorage() {
	ctx := suite.Ctx
	ak := suite.App.AuctionKeeper
	params := ak.GetParams(ctx)
	// Create and store multiple AuctionPeriods
	startAP := ak.GetAuctionPeriod(ctx)
	// The first period should start on the first block
	require.Equal(suite.T(), startAP.StartBlockHeight, uint64(1))
	// And end auctionLength blocks later
	require.Equal(suite.T(), startAP.EndBlockHeight, params.AuctionLength+1)

	newAP := types.AuctionPeriod{
		StartBlockHeight: startAP.EndBlockHeight + 1,
		EndBlockHeight:   startAP.EndBlockHeight + 2,
	}
	// Fail to update in the middle of an auction period
	err := ak.UpdateAuctionPeriod(ctx, newAP)
	require.Error(suite.T(), err)

	// Update at the end of the auction period
	ctx = ctx.WithBlockHeight(int64(startAP.EndBlockHeight))
	err = ak.UpdateAuctionPeriod(ctx, newAP)
	require.NoError(suite.T(), err)

	updatedAP := ak.GetAuctionPeriod(ctx)
	require.Equal(suite.T(), newAP, *updatedAP)

	ctx = ctx.WithBlockHeight(int64(updatedAP.EndBlockHeight))

	// Automatically create a new period off the custom state (e.g. the auction length param changed at some earlier point)
	createdAP, err := ak.CreateNewAuctionPeriod(ctx)
	require.NoError(suite.T(), err)
	// It should start after the last period did
	require.Equal(suite.T(), updatedAP.EndBlockHeight+1, createdAP.StartBlockHeight)
	// And end auctionLength blocks after that
	require.Equal(suite.T(), updatedAP.EndBlockHeight+1+params.AuctionLength, createdAP.EndBlockHeight)

	ctx = ctx.WithBlockHeight(int64(createdAP.EndBlockHeight))

	// Now actually update the auctionLength parameter
	params.AuctionLength = uint64(10000)
	ak.SetParams(ctx, params)

	// Generate a new period and verify it follows the updated param
	finalAP, err := ak.CreateNewAuctionPeriod(ctx)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), createdAP.EndBlockHeight+1, finalAP.StartBlockHeight)
	require.Equal(suite.T(), createdAP.EndBlockHeight+1+params.AuctionLength, finalAP.EndBlockHeight)
}

// Tests auction period function behavior when the module is disabled
func (suite *KeeperTestSuite) TestAuctionPeriodWhileDisabled() {
	ctx := suite.Ctx
	ak := suite.App.AuctionKeeper
	t := suite.T()
	params := ak.GetParams(ctx)
	// Create and store multiple AuctionPeriods
	startAP := ak.GetAuctionPeriod(ctx)
	// The first period should start on the first block
	require.Equal(t, startAP.StartBlockHeight, uint64(1))
	// And end auctionLength blocks later
	require.Equal(t, startAP.EndBlockHeight, params.AuctionLength+1)

	newAP := types.AuctionPeriod{
		StartBlockHeight: startAP.EndBlockHeight + 1,
		EndBlockHeight:   startAP.EndBlockHeight + 2,
	}

	// Disable the module
	params.Enabled = false
	ak.SetParams(ctx, params)
	ctx = ctx.WithBlockHeight(int64(startAP.EndBlockHeight))

	// Fail to update the auction period to the new auction
	err := ak.UpdateAuctionPeriod(ctx, newAP)
	require.Error(t, err)

	// Fail to create new auctions (at the wrong time but still should fail)
	suite.AppTestHelper.FundAuctionPool(ctx, TestBalances)
	err = ak.CreateAuctionsForAuctionPeriod(ctx)
	require.Error(t, err)

	// Ensure that the attempted behavior works as normal when enabled
	params.Enabled = true
	ak.SetParams(ctx, params)
	err = ak.UpdateAuctionPeriod(ctx, newAP)
	require.NoError(t, err)
	err = ak.CreateAuctionsForAuctionPeriod(ctx)
	require.NoError(t, err)
}
