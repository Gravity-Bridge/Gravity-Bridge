package keeper_test

import (
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
)

func (suite KeeperTestSuite) TestAutoCreateAuction() {
	suite.SetupTest()
	ctx := suite.Ctx
	// params set
	defaultAuctionEpoch := uint64(1)
	defaultAuctionPeriod := uint64(10)
	defaultMinBidAmount := uint64(1000)
	defaultBidGap := uint64(100)
	auctionRate := sdk.NewDec(1)
	allowTokens := map[string]bool{
		"atom": true,
		"osmo": true,
		"juno": false,
	}
	params := types.NewParams(defaultAuctionEpoch, defaultAuctionPeriod, defaultMinBidAmount, defaultBidGap, auctionRate, allowTokens)
	suite.App.GetAuctionKeeper().SetParams(ctx, params)
	// set community pool
	Coins := []sdk.Coin{}
	for token := range params.AllowTokens {
		balance := suite.App.GetBankKeeper().GetBalance(ctx, suite.App.GetAccountKeeper().GetModuleAccount(ctx, distrtypes.ModuleName).GetAddress(), token)
		Coins = append(Coins, balance)
	}
	print(Coins)

	for token := range params.AllowTokens {
		sdkcoin := sdk.NewCoin(token, sdk.NewIntFromUint64(1_000_000_000))
		println(sdkcoin)
		// suite.App.GetBankKeeper()
	}

	// senderAddr := suite.App.BankKeeper.BaseSendKeeper.ak.GetModuleAddress(senderModule)
	// if senderAddr == nil {
	// 	panic(sdkerrors.Wrapf(sdkerrors.ErrUnknownAddress, "module account %s does not exist", senderModule))
	// }

	// get
}
