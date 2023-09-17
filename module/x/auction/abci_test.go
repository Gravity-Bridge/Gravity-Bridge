package auction_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"

	"reflect"
	"testing"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"

	"github.com/stretchr/testify/suite"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/app/apptesting"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
)

type TestSuite struct {
	apptesting.AppTestHelper
	suite.Suite
}

// Test helpers
func (suite *TestSuite) SetupTest() {
	suite.Setup()
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func (suite *TestSuite) TestBeginBlocker() {
	previousAuctionPeriod := types.AuctionPeriod{Id: 1, StartBlockHeight: 0, EndBlockHeight: 4}
	expectAmount := sdk.NewCoin("atom", sdk.NewInt(20_000_000))

	testCases := map[string]struct {
		ctxHeight             int64
		expectPanic           bool
		expectAuction         types.Auction
		previousAuctionPeriod *types.AuctionPeriod
		communityBalances     sdk.Coins
	}{
		"Not meet the next auction period": {
			ctxHeight:   4,
			expectPanic: false,
		},
		"Meet the next auction period, no previous auction period": {
			ctxHeight:   5,
			expectPanic: true,
		},
		"Meet the next auction period, community pool has zero balances": {
			ctxHeight:             5,
			expectPanic:           false,
			previousAuctionPeriod: &previousAuctionPeriod,
		},
		"Meet the next auction period, community pool balances truncate to zero": {
			ctxHeight:             5,
			expectPanic:           false,
			previousAuctionPeriod: &previousAuctionPeriod,
			communityBalances:     sdk.NewCoins(sdk.NewCoin("atom", sdk.NewInt(4))),
		},
		"Meet the next auction period, create new auction period": {
			ctxHeight:   5,
			expectPanic: false,
			expectAuction: types.Auction{
				Id:              1,
				AuctionAmount:   expectAmount,
				Status:          types.AuctionStatus_AUCTION_STATUS_IN_PROGRESS,
				AuctionPeriodId: 2,
			},
			previousAuctionPeriod: &previousAuctionPeriod,
			communityBalances:     sdk.NewCoins(sdk.NewCoin("atom", sdk.NewInt(100_000_000))),
		},
	}

	for name, tc := range testCases {
		suite.Run(name, func() {
			suite.SetupTest()
			ctx := suite.Ctx

			// Set params
			allowTokens := map[string]bool{
				"atom": true,
			}
			params := types.NewParams(uint64(4), uint64(2), uint64(1000), uint64(100), sdk.NewDecWithPrec(2, 1), allowTokens)
			suite.App.GetAuctionKeeper().SetParams(ctx, params)

			// Try to begin block without initial estimateNextBlockHeight set
			suite.Require().Panics(func() {
				auction.BeginBlocker(ctx, suite.App.GetAuctionKeeper(), suite.App.GetBankKeeper(), suite.App.GetAccountKeeper())
			})

			// Set next auction period at block 5
			suite.App.GetAuctionKeeper().SetEstimateAuctionPeriodBlockHeight(ctx, 5)

			ctx = ctx.WithBlockHeight(tc.ctxHeight)

			if tc.previousAuctionPeriod != nil {
				suite.App.GetAuctionKeeper().SetAuctionPeriod(ctx, *tc.previousAuctionPeriod)
			}

			if tc.communityBalances != nil {
				suite.FundModule(ctx, distrtypes.ModuleName, tc.communityBalances)
				suite.App.GetDistriKeeper().SetFeePool(ctx, distrtypes.FeePool{CommunityPool: sdk.NewDecCoinsFromCoins(tc.communityBalances...)})
			}

			if !tc.expectPanic {
				suite.Require().NotPanics(func() {
					auction.BeginBlocker(ctx, suite.App.GetAuctionKeeper(), suite.App.GetBankKeeper(), suite.App.GetAccountKeeper())
				})
				if tc.previousAuctionPeriod != nil {
					if !reflect.DeepEqual(tc.expectAuction, types.Auction{}) {
						auctions := suite.App.GetAuctionKeeper().GetAllAuctionsByPeriodID(ctx, tc.previousAuctionPeriod.Id+1)
						// Should contain 1 aution for atom token
						suite.Equal(len(auctions), 1)
						auction := auctions[0]
						suite.Equal(auction, tc.expectAuction)

					} else {
						auctions := suite.App.GetAuctionKeeper().GetAllAuctionsByPeriodID(ctx, tc.previousAuctionPeriod.Id+1)
						// Should not cotain any aution
						suite.Equal(len(auctions), 0)
					}
				}
			} else {
				suite.Require().Panics(func() {
					auction.BeginBlocker(ctx, suite.App.GetAuctionKeeper(), suite.App.GetBankKeeper(), suite.App.GetAccountKeeper())
				})
			}
		})
	}

}

func (suite *TestSuite) TestEndBlocker() {
	_, _, addr0 := testdata.KeyTestPubAddr()
	auctionAmount := sdk.NewCoin("atom", sdk.NewInt(1_000_000))

	testCases := map[string]struct {
		ctxHeight         int64
		expectPanic       bool
		currentHighestBid *types.Bid
		expectedCommunity sdk.Coins
		isAuctionUpdated  bool
	}{
		"In the auction duration": {
			ctxHeight:   1,
			expectPanic: false,
		},
		"In the process end, no winning": {
			ctxHeight:         3,
			expectPanic:       false,
			isAuctionUpdated:  true,
			expectedCommunity: sdk.NewCoins(auctionAmount),
		},
		"In the process end, has winning": {
			ctxHeight: 3,
			currentHighestBid: &types.Bid{
				AuctionId:     1,
				BidAmount:     sdk.NewCoin("stake", sdk.NewInt(1_000_000)),
				BidderAddress: addr0.String(),
			},
			expectPanic:       false,
			isAuctionUpdated:  true,
			expectedCommunity: sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(1_000_000))),
		},
		"In the process end, auction module has not enough balances": {
			ctxHeight: 3,
			currentHighestBid: &types.Bid{
				AuctionId:     1,
				BidAmount:     sdk.NewCoin("stake", sdk.NewInt(2_000_000)),
				BidderAddress: addr0.String(),
			},
			expectPanic: true,
		},
	}

	for name, tc := range testCases {
		suite.Run(name, func() {
			suite.SetupTest()
			ctx := suite.Ctx

			// Set params
			allowTokens := map[string]bool{
				"atom": true,
			}
			params := types.NewParams(uint64(4), uint64(2), uint64(1000), uint64(100), sdk.NewDecWithPrec(2, 1), allowTokens)
			suite.App.GetAuctionKeeper().SetParams(ctx, params)

			// Fund module & account
			suite.FundModule(ctx, types.ModuleName, sdk.NewCoins(auctionAmount))

			// Set auction period and auction
			newAuctionPeriods := types.AuctionPeriod{
				Id:               1,
				StartBlockHeight: uint64(suite.Ctx.BlockHeight()),
				EndBlockHeight:   uint64(suite.Ctx.BlockHeight()) + suite.App.GetAuctionKeeper().GetParams(suite.Ctx).AuctionPeriod,
			}
			suite.App.GetAuctionKeeper().SetAuctionPeriod(suite.Ctx, newAuctionPeriods)

			// Confirm that aution was set
			lastAution, found := suite.App.GetAuctionKeeper().GetLatestAuctionPeriod(suite.Ctx)
			suite.Require().True(found)
			suite.Require().Equal(uint64(1), lastAution.Id)

			atomAuction := types.Auction{
				Id:              1,
				AuctionPeriodId: 1,
				AuctionAmount:   auctionAmount,
				Status:          1,
			}
			err := suite.App.GetAuctionKeeper().AddNewAuctionToAuctionPeriod(suite.Ctx, 1, atomAuction)
			suite.Require().NoError(err)

			ctx = ctx.WithBlockHeight(tc.ctxHeight)

			if tc.currentHighestBid != nil {
				auction, _ := suite.App.GetAuctionKeeper().GetAuctionByPeriodIDAndAuctionId(ctx, 1, 1)
				auction.HighestBid = tc.currentHighestBid
				suite.App.GetAuctionKeeper().SetAuction(ctx, auction)

				// Fund auction module with stake locked
				suite.FundModule(ctx, types.ModuleName, sdk.NewCoins(tc.currentHighestBid.BidAmount))
			}

			if !tc.expectPanic {
				auction.EndBlocker(ctx, suite.App.GetAuctionKeeper(), suite.App.GetBankKeeper(), suite.App.GetAccountKeeper())
				if tc.isAuctionUpdated {
					auction, found := suite.App.GetAuctionKeeper().GetAuctionByPeriodIDAndAuctionId(ctx, 1, 1)
					suite.Require().True(found)
					suite.Require().Equal(types.AuctionStatus_AUCTION_STATUS_FINISH, auction.Status)
				}
				if tc.expectedCommunity != nil {
					communityBalances := suite.App.GetBankKeeper().GetAllBalances(ctx, suite.App.GetAccountKeeper().GetModuleAddress(distrtypes.ModuleName))
					suite.Require().Equal(tc.expectedCommunity, communityBalances)
				}
				if tc.currentHighestBid != nil {
					winnerBalances := suite.App.GetBankKeeper().GetAllBalances(ctx, sdk.MustAccAddressFromBech32(tc.currentHighestBid.BidderAddress))
					suite.Require().Equal(sdk.NewCoins(auctionAmount), winnerBalances)
				}

			}

		})
	}

}
