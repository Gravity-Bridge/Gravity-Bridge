package keeper_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

var one_eth sdkmath.Int

func init() {
	tenTo18, ok := sdkmath.NewIntFromString("1000000000000000000") // 10^18
	if !ok {
		panic("failed to create one_eth value")
	}
	one_eth = tenTo18
}

func (suite *KeeperTestSuite) TestMsgBid() {
	testCoins := sdk.NewCoins(sdk.NewCoin("foo", sdkmath.NewInt(1000_000000)), sdk.NewCoin("bar", sdkmath.NewInt(1000_000000)), sdk.NewCoin("baz", sdkmath.NewInt(1000_000000)))
	ctx := suite.Ctx
	t := suite.T()
	ak := suite.App.AuctionKeeper
	mintParams, err := ak.MintKeeper.Params.Get(ctx)
	suite.Require().NoError(err, "failed to get mint params")

	gravDenom := mintParams.MintDenom
	// Give everyone 10 * 10^18 aka 10 Eth worth
	suite.CreateAndFundRandomAccounts(3, sdk.NewCoins(sdk.NewCoin(gravDenom, one_eth.Mul(sdkmath.NewInt(10)))))
	suite.FundAuctionPool(ctx, testCoins)

	periodEnd := ak.GetAuctionPeriod(ctx).EndBlockHeight
	ctx = ctx.WithBlockHeight(int64(periodEnd))
	// Create auctions for all of testCoins
	_, err = ak.CreateNewAuctionPeriod(ctx)
	require.NoError(t, err)
	err = ak.CreateAuctionsForAuctionPeriod(ctx)
	require.NoError(t, err)
	auctions := ak.GetAllAuctions(ctx)
	require.Equal(t, 3, len(auctions))
	params, err := ak.GetParams(ctx)
	require.NoError(t, err, "failed to get auction params")
	minFee := params.MinBidFee

	testCases := map[string]struct {
		msg          types.MsgBid
		expectedPass bool
	}{
		"Happy": {
			msg:          *types.NewMsgBid(1, suite.AppTestHelper.TestAccs[0].String(), uint64(1_000000), minFee),
			expectedPass: true,
		},
		"HappyBigFee": {
			msg:          *types.NewMsgBid(1, suite.AppTestHelper.TestAccs[1].String(), uint64(1_000000), one_eth.Mul(sdkmath.NewInt(5)).Uint64()),
			expectedPass: true,
		},
		"HappyBigAmount": {
			msg:          *types.NewMsgBid(2, suite.AppTestHelper.TestAccs[1].String(), one_eth.Mul(sdkmath.NewInt(3)).Uint64(), minFee),
			expectedPass: true,
		},
		"SadId": {
			msg:          *types.NewMsgBid(0, suite.AppTestHelper.TestAccs[0].String(), uint64(1_000000), minFee),
			expectedPass: false,
		},
		"SadAmount": {
			msg:          *types.NewMsgBid(1, suite.AppTestHelper.TestAccs[0].String(), uint64(0), minFee),
			expectedPass: false,
		},
		"SadAddress": {
			msg:          *types.NewMsgBid(1, "Bad address", uint64(0), minFee),
			expectedPass: false,
		},
		"SadFee": {
			msg:          *types.NewMsgBid(1, suite.AppTestHelper.TestAccs[0].String(), uint64(1_000000), minFee-1),
			expectedPass: false,
		},
		"SadZeroFee": {
			msg:          *types.NewMsgBid(1, suite.AppTestHelper.TestAccs[0].String(), uint64(1_000000), minFee-1),
			expectedPass: false,
		},
	}

	for name, tc := range testCases {
		suite.Run(name, func() {
			msgServer := keeper.NewMsgServerImpl(*suite.App.AuctionKeeper)
			ctx := sdk.WrapSDKContext(suite.Ctx)

			mintParams, err := suite.App.AuctionKeeper.MintKeeper.Params.Get(suite.Ctx)
			suite.Require().NoError(err, "failed to get mint params")
			bidToken := mintParams.MintDenom

			var preBal, postBal sdk.Coin

			if tc.expectedPass {
				preBal = suite.App.BankKeeper.GetBalance(suite.Ctx, sdk.MustAccAddressFromBech32(tc.msg.Bidder), bidToken)
			}
			resp, err := msgServer.Bid(ctx, &tc.msg)
			if tc.expectedPass {
				postBal = suite.App.BankKeeper.GetBalance(suite.Ctx, sdk.MustAccAddressFromBech32(tc.msg.Bidder), bidToken)
			}

			if tc.expectedPass {
				suite.Require().NoError(err)
				suite.Require().NotNil(resp)

				expDiff := sdkmath.NewIntFromUint64(tc.msg.Amount).Add(sdkmath.NewIntFromUint64(tc.msg.BidFee))
				accDiff := preBal.Sub(postBal)
				suite.Require().True(expDiff.Equal(accDiff.Amount))
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestMsgUpdateParamsProposal() {
	ctx := suite.Ctx
	t := suite.T()

	govAddress := authtypes.NewModuleAddress(govtypes.ModuleName)

	auctionLength := types.Param{
		Key:   "AuctionLength",
		Value: "1000",
	}
	minBidFee := types.Param{
		Key:   "MinBidFee",
		Value: "10",
	}
	nonAuctionableTokens := types.Param{
		Key:   "NonAuctionableTokens",
		Value: "[\"ugraviton\",\"foo\"]",
	}
	burnWinningBids := types.Param{
		Key:   "BurnWinningBids",
		Value: "true",
	}
	enabled := types.Param{
		Key:   "Enabled",
		Value: "true",
	}

	testCases := []struct {
		name           string
		msg            types.MsgUpdateParamsProposal
		expectError    bool
		expectedParams func(ctx sdk.Context, keeper keeper.Keeper) types.Params
	}{
		{
			name: "All fields set",
			msg: types.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				ParamUpdates: []*types.Param{
					&auctionLength,
					&minBidFee,
					&nonAuctionableTokens,
					&burnWinningBids,
					&enabled,
				},
			},
			expectError: false,
			expectedParams: func(ctx sdk.Context, keeper keeper.Keeper) types.Params {
				return types.Params{
					AuctionLength:        1000,
					MinBidFee:            10,
					NonAuctionableTokens: []string{"ugraviton", "foo"},
					BurnWinningBids:      true,
					Enabled:              true,
				}
			},
		},
		{
			name: "Update only AuctionLength",
			msg: types.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				ParamUpdates: []*types.Param{
					&auctionLength,
				},
			},
			expectError: false,
			expectedParams: func(ctx sdk.Context, keeper keeper.Keeper) types.Params {
				params, err := keeper.GetParams(ctx)
				require.NoError(t, err)
				params.AuctionLength = 1000
				require.NoError(t, err)
				return params
			},
		},
		{
			name: "Update only MinBidFee",
			msg: types.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				ParamUpdates: []*types.Param{
					&minBidFee,
				},
			},
			expectError: false,
			expectedParams: func(ctx sdk.Context, keeper keeper.Keeper) types.Params {
				params, err := keeper.GetParams(ctx)
				require.NoError(t, err)
				params.MinBidFee = 10
				return params
			},
		},
		{
			name: "Update only NonAuctionableTokens",
			msg: types.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				ParamUpdates: []*types.Param{
					&nonAuctionableTokens,
				},
			},
			expectError: false,
			expectedParams: func(ctx sdk.Context, keeper keeper.Keeper) types.Params {
				params, err := keeper.GetParams(ctx)
				require.NoError(t, err)
				var tokens []string
				err = json.Unmarshal([]byte(nonAuctionableTokens.Value), &tokens)
				require.NoError(t, err)
				params.NonAuctionableTokens = tokens
				return params
			},
		},
		{
			name: "Update only BurnWinningBids",
			msg: types.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				ParamUpdates: []*types.Param{
					&burnWinningBids,
				},
			},
			expectError: false,
			expectedParams: func(ctx sdk.Context, keeper keeper.Keeper) types.Params {
				params, err := keeper.GetParams(ctx)
				require.NoError(t, err)
				params.BurnWinningBids = true
				return params
			},
		},
		{
			name: "Update only Enabled",
			msg: types.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				ParamUpdates: []*types.Param{
					&enabled,
				},
			},
			expectError: false,
			expectedParams: func(ctx sdk.Context, keeper keeper.Keeper) types.Params {
				params, err := keeper.GetParams(ctx)
				require.NoError(t, err)
				params.Enabled = true
				return params
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cacheCtx, _ := ctx.CacheContext()
			msgServer := keeper.NewMsgServerImpl(*suite.App.AuctionKeeper)
			expectedParams := tc.expectedParams(cacheCtx, *suite.App.AuctionKeeper)
			_, err := msgServer.UpdateParamsProposal(cacheCtx, &tc.msg)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				params, err := suite.App.AuctionKeeper.GetParams(cacheCtx)
				require.NoError(t, err)

				fmt.Println("Expected Params:", expectedParams)
				fmt.Println("Actual Params:", params)
				require.Equal(t, expectedParams.AuctionLength, params.AuctionLength, "Expected auction length to match after proposal execution")
				require.Equal(t, expectedParams.MinBidFee, params.MinBidFee, "Expected min bid fee to match after proposal execution")
				require.Equal(t, expectedParams.NonAuctionableTokens, params.NonAuctionableTokens, "Expected non-auctionable tokens to match after proposal execution")
				require.Equal(t, expectedParams.BurnWinningBids, params.BurnWinningBids, "Expected burn winning bids to match after proposal execution")
				require.Equal(t, expectedParams.Enabled, params.Enabled, "Expected enabled to match after proposal execution")
			}
		})
	}
}
