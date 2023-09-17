package keeper_test

import (
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (suite *KeeperTestSuite) TestMsgBid() {

	_, _, addr0 := testdata.KeyTestPubAddr()
	_, _, addr1 := testdata.KeyTestPubAddr()

	testCases := map[string]struct {
		height            int64
		msg               types.MsgBid
		expectedPass      bool
		currentHighestBid *types.Bid
	}{
		"Not using native denom": {
			msg:          *types.NewMsgBid(1, addr0.String(), sdk.NewCoin("invalid", sdk.NewInt(500000))),
			expectedPass: false,
		},
		"Insurfficient balance": {
			msg:          *types.NewMsgBid(1, addr0.String(), sdk.NewCoin("stake", sdk.NewInt(15000000))),
			expectedPass: false,
		},
		"Bid amount less than min bid": {
			msg:          *types.NewMsgBid(1, addr0.String(), sdk.NewCoin("stake", sdk.NewInt(100))),
			expectedPass: false,
		},
		"Auction not found": {
			msg:          *types.NewMsgBid(3, addr0.String(), sdk.NewCoin("stake", sdk.NewInt(500000))),
			expectedPass: false,
		},
		"Insufficient balances": {
			msg:          *types.NewMsgBid(1, addr1.String(), sdk.NewCoin("stake", sdk.NewInt(500000))),
			expectedPass: false,
		},
		"Auction end": {
			height:       15,
			msg:          *types.NewMsgBid(1, addr0.String(), sdk.NewCoin("stake", sdk.NewInt(500000))),
			expectedPass: false,
		},
		"Bid amount less than the highest bid": {
			msg:          *types.NewMsgBid(1, addr0.String(), sdk.NewCoin("stake", sdk.NewInt(50000))),
			expectedPass: true,
			currentHighestBid: &types.Bid{
				AuctionId:     1,
				BidAmount:     sdk.NewCoin("stake", sdk.NewInt(1_000_000)),
				BidderAddress: addr0.String(),
			},
		},
		"new bid sub highest bid less than gap": {
			msg:          *types.NewMsgBid(1, addr0.String(), sdk.NewCoin("stake", sdk.NewInt(1_000_010))),
			expectedPass: false,
			currentHighestBid: &types.Bid{
				AuctionId:     1,
				BidAmount:     sdk.NewCoin("stake", sdk.NewInt(1_000_000)),
				BidderAddress: addr0.String(),
			},
		},
		"Bid successfully, no exist highest bid": {
			msg:          *types.NewMsgBid(1, addr0.String(), sdk.NewCoin("stake", sdk.NewInt(500000))),
			expectedPass: true,
		},
		"Bid successfully, exist highest bid": {
			msg:          *types.NewMsgBid(1, addr0.String(), sdk.NewCoin("stake", sdk.NewInt(2_000_000))),
			expectedPass: true,
			currentHighestBid: &types.Bid{
				AuctionId:     1,
				BidAmount:     sdk.NewCoin("stake", sdk.NewInt(1_000_000)),
				BidderAddress: addr1.String(),
			},
		},
		"Bid successfully, exist highest bid with same addr": {
			msg:          *types.NewMsgBid(1, addr0.String(), sdk.NewCoin("stake", sdk.NewInt(2_000_000))),
			expectedPass: true,
			currentHighestBid: &types.Bid{
				AuctionId:     1,
				BidAmount:     sdk.NewCoin("stake", sdk.NewInt(1_000_000)),
				BidderAddress: addr0.String(),
			},
		},
	}

	for name, tc := range testCases {
		suite.Run(name, func() {
			// Set up test app
			suite.SetupTest()

			// Set new auction period to store
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

			// Set auction for tokens

			atomAmount := sdk.NewCoin("atom", sdk.NewInt(1000000))

			atomAuction := types.Auction{
				Id:              1,
				AuctionPeriodId: 1,
				AuctionAmount:   atomAmount,
				Status:          1,
			}
			err := suite.App.GetAuctionKeeper().AddNewAuctionToAuctionPeriod(suite.Ctx, 1, atomAuction)
			suite.Require().NoError(err)

			// Fund a account & auction module
			suite.FundAccount(suite.Ctx, addr0, sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(2_000_000))))
			suite.FundModule(suite.Ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(1_000_000))))

			// Set allowlist tokens
			params := suite.App.GetAuctionKeeper().GetParams(suite.Ctx)
			params.AllowTokens["atom"] = true
			suite.App.GetAuctionKeeper().SetParams(suite.Ctx, params)

			if tc.currentHighestBid != nil {
				auction, _ := suite.App.GetAuctionKeeper().GetAuctionByPeriodIDAndAuctionId(suite.Ctx, 1, 1)
				auction.HighestBid = tc.currentHighestBid
				suite.App.GetAuctionKeeper().SetAuction(suite.Ctx, auction)
			}

			if tc.height != 0 {
				suite.Ctx = suite.Ctx.WithBlockHeight(tc.height)
			}

			msgServer := keeper.NewMsgServerImpl(suite.App.GetAuctionKeeper())
			ctx := sdk.WrapSDKContext(suite.Ctx)

			resp, err := msgServer.Bid(ctx, &tc.msg)

			if tc.expectedPass {
				suite.Require().NoError(err)
				suite.Require().NotNil(resp)
				auction, _ := suite.App.GetAuctionKeeper().GetAuctionByPeriodIDAndAuctionId(suite.Ctx, 1, 1)
				newHighestBid := auction.HighestBid
				suite.Require().Equal(tc.msg.Amount, newHighestBid.BidAmount)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}
