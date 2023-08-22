package keeper_test

import (
	"fmt"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (suite *KeeperTestSuite) TestMsgBid() {

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
	osmoAmount := sdk.NewCoin("osmo", sdk.NewInt(1000000))

	atomAuction := types.Auction{
		Id:              1,
		AuctionPeriodId: 1,
		AuctionAmount:   &atomAmount,
		Status:          1,
		HighestBid:      &types.Bid{
			AuctionId: 1,
			BidAmount: &sdk.Coin{Denom: "stake", Amount: sdk.NewInt(100000)},
		},
	}
	err := suite.App.GetAuctionKeeper().AddNewAuctionToAuctionPeriod(suite.Ctx, 1, atomAuction)
	suite.Require().NoError(err)
	suite.App.GetAuctionKeeper().CreateNewBidQueue(suite.Ctx, 1)

	// With osmo auction, dont set the queue
	osmoAuction := types.Auction{
		Id:              2,
		AuctionPeriodId: 1,
		AuctionAmount:   &osmoAmount,
		Status:          1,
		HighestBid:      nil,
	}
	err = suite.App.GetAuctionKeeper().AddNewAuctionToAuctionPeriod(suite.Ctx, 1, osmoAuction)
	suite.Require().NoError(err)

	// Fund a account
	suite.FundAccount(suite.Ctx, suite.TestAccs[0], sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(1000000))))
	balance := suite.App.GetBankKeeper().GetAllBalances(suite.Ctx, suite.TestAccs[0])
	fmt.Println("balance", balance)

	// Set allowlist tokens
	params := suite.App.GetAuctionKeeper().GetParams(suite.Ctx)
	params.AllowTokens["atom"] = true
	params.AllowTokens["osmo"] = true
	suite.App.GetAuctionKeeper().SetParams(suite.Ctx, params)

	testCases := map[string]struct {
		msg          types.MsgBid
		expectedPass bool
	}{
		"Not using native denom": {
			msg:          *types.NewMsgBid(1, suite.TestAccs[0].String(), sdk.NewCoin("invalid", sdk.NewInt(500000))),
			expectedPass: false,
		},
		"Insurfficient balance": {
			msg:          *types.NewMsgBid(1, suite.TestAccs[0].String(), sdk.NewCoin("stake", sdk.NewInt(15000000))),
			expectedPass: false,
		},
		"Bid amount less than min bid": {
			msg:          *types.NewMsgBid(1, suite.TestAccs[0].String(), sdk.NewCoin("stake", sdk.NewInt(100))),
			expectedPass: false,
		},
		"Bid queue not found": {
			msg:          *types.NewMsgBid(2, suite.TestAccs[0].String(), sdk.NewCoin("stake", sdk.NewInt(500000))),
			expectedPass: false,
		},
		"Auction not found": {
			msg:          *types.NewMsgBid(3, suite.TestAccs[0].String(), sdk.NewCoin("stake", sdk.NewInt(500000))),
			expectedPass: false,
		},
		// test fail here, should be an error returns
		"Bid amount less than the highest bid": {
			msg:          *types.NewMsgBid(1, suite.TestAccs[0].String(), sdk.NewCoin("stake", sdk.NewInt(50000))),
			expectedPass: false,
		},
		"new bid sub highest bid less than gap": {
			msg:          *types.NewMsgBid(1, suite.TestAccs[0].String(), sdk.NewCoin("stake", sdk.NewInt(100010))),
			expectedPass: false,
		},
		"Bid successfully": {
			msg:          *types.NewMsgBid(1, suite.TestAccs[0].String(), sdk.NewCoin("stake", sdk.NewInt(500000))),
			expectedPass: true,
		},
	}

	for name, tc := range testCases {
		suite.Run(name, func() {
			msgServer := keeper.NewMsgServerImpl(suite.App.GetAuctionKeeper())
			ctx := sdk.WrapSDKContext(suite.Ctx)

			resp, err := msgServer.Bid(ctx, &tc.msg)

			if tc.expectedPass {
				suite.Require().NoError(err)
				suite.Require().NotNil(resp)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}
