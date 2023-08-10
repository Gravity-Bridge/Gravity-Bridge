package keeper_test

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

func (suite KeeperTestSuite) TestBeginBlockerAndEndBlockerAuction() {
	suite.SetupTest()
	ctx := suite.Ctx
	// params set
	defaultAuctionEpoch := uint64(1)
	defaultAuctionPeriod := uint64(1)
	defaultMinBidAmount := uint64(1000)
	defaultBidGap := uint64(100)
	auctionRate := sdk.NewDecWithPrec(2, 1) //20%
	allowTokens := map[string]bool{
		"atomm": true,
	}
	params := types.NewParams(defaultAuctionEpoch, defaultAuctionPeriod, defaultMinBidAmount, defaultBidGap, auctionRate, allowTokens)
	suite.App.GetAuctionKeeper().SetParams(ctx, params)

	// set community pool
	coinsSet := []sdk.Coin{}
	for token := range params.AllowTokens {
		sdkcoin := sdk.NewCoin(token, sdk.NewIntFromUint64(1_000_000_000))
		coinsSet = append(coinsSet, sdkcoin)

	}
	err := suite.App.GetBankKeeper().MintCoins(ctx, minttypes.ModuleName, coinsSet)
	suite.Require().NoError(err)

	err = suite.App.GetBankKeeper().SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, distrtypes.ModuleName, coinsSet)
	suite.Require().NoError(err)

	coins_dist := []sdk.Coin{}
	for token := range params.AllowTokens {
		// fmt.Printf("%v \n", token)
		balance := suite.App.GetBankKeeper().GetBalance(ctx, suite.App.GetAccountKeeper().GetModuleAccount(ctx, distrtypes.ModuleName).GetAddress(), token)
		coins_dist = append(coins_dist, balance)

	}
	fmt.Printf("coin dist module:%v \n", coins_dist)

	//set a Auction finish (Auction has ended.)
	CoinAuction := sdk.NewCoin("atomm", sdk.NewIntFromUint64(10))
	auctionPeriod_Set := types.AuctionPeriod{Id: 1, StartBlockHeight: 0}
	auction_Set := types.Auction{
		Id:              1,
		AuctionAmount:   &CoinAuction,
		Status:          types.AuctionStatus_AUCTION_STATUS_FINISH,
		HighestBid:      &types.Bid{AuctionId: 1, BidAmount: &CoinAuction},
		AuctionPeriodId: auctionPeriod_Set.Id,
	}
	suite.App.GetAuctionKeeper().SetAuctionPeriod(ctx, auctionPeriod_Set)
	err = suite.App.GetAuctionKeeper().AddNewAuctionToAuctionPeriod(ctx, auctionPeriod_Set.Id, auction_Set)
	suite.Require().NoError(err)
	println("-----begin block-------")
	auction.BeginBlocker(ctx, suite.App.GetAuctionKeeper(), suite.App.GetBankKeeper(), suite.App.GetAccountKeeper())
	println("++++begin block++++++++++")

	// Endblock
	increamentId, err := suite.App.GetAuctionKeeper().IncreamentAuctionPeriodId(ctx)
	if err != nil {
		panic(err)
	}
	CoinAuction2 := sdk.NewCoin("atomm", sdk.NewIntFromUint64(100))
	auctionPeriod_Set2 := types.AuctionPeriod{Id: increamentId, StartBlockHeight: 2}
	auction_Set2 := types.Auction{
		Id:              increamentId,
		AuctionAmount:   &CoinAuction2,
		Status:          types.AuctionStatus_AUCTION_STATUS_FINISH,
		HighestBid:      &types.Bid{AuctionId: 1, BidAmount: &CoinAuction},
		AuctionPeriodId: auctionPeriod_Set2.Id,
	}
	suite.App.GetAuctionKeeper().SetAuctionPeriod(ctx, auctionPeriod_Set2)
	err = suite.App.GetAuctionKeeper().AddNewAuctionToAuctionPeriod(ctx, auctionPeriod_Set2.Id, auction_Set2)
	suite.Require().NoError(err)

	au := suite.App.GetAuctionKeeper().GetAllAuctions(ctx)

	println("lllllllllll:", len(au))
	fmt.Printf("%v \n", au[0])
	fmt.Printf("%v \n", au[1])
	// fmt.Printf("%v \n", au[2])

	// auction.EndBlocker(ctx, suite.App.GetAuctionKeeper(), suite.App.GetBankKeeper(), suite.App.GetAccountKeeper())

	coins_new := []sdk.Coin{}
	for token := range params.AllowTokens {
		balance := suite.App.GetBankKeeper().GetBalance(ctx, suite.App.GetAccountKeeper().GetModuleAccount(ctx, distrtypes.ModuleName).GetAddress(), token)
		coins_new = append(coins_new, balance)

	}
	fmt.Printf("coin dist module new:%v \n", coins_new)

}
