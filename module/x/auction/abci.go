package auction

import (
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
)

func startMewAuctionPeriod(ctx sdk.Context, params types.Params, k keeper.Keeper, bk types.BankKeeper, ak types.AccountKeeper) error {
	auctionRate := params.AuctionRate

	increamentId, err := k.IncreamentAuctionPeriodId(ctx)
	if err != nil {
		panic(err)
	}

	newAuctionPeriods := types.AuctionPeriod{
		Id:               increamentId,
		StartBlockHeight: uint64(ctx.BlockHeight()),
		EndBlockHeight:   uint64(ctx.BlockHeight()) + params.AuctionPeriod,
	}

	// Set new auction period to store
	k.SetAuctionPeriod(ctx, newAuctionPeriods)

	for token := range params.AllowTokens {
		balance := bk.GetBalance(ctx, ak.GetModuleAccount(ctx, distrtypes.ModuleName).GetAddress(), token)

		// Compute auction amount to send to auction module account
		amount := sdk.NewDecFromInt(balance.Amount).Mul(auctionRate).TruncateInt()

		sdkcoin := sdk.NewCoin(token, amount)

		// Send fund from community pool to auction module
		err := k.SendFromCommunityPool(ctx, sdk.Coins{sdkcoin})
		if err != nil {
			return err
		}
		newId, err := k.IncreamentAuctionId(ctx, increamentId)
		if err != nil {
			return err
		}

		newAuction := types.Auction{
			Id:              newId,
			AuctionPeriodId: increamentId,
			AuctionAmount:   &sdkcoin,
			Status:          1,
			HighestBid:      nil,
		}

		// Update auction in auction period auction list
		err = k.AddNewAuctionToAuctionPeriod(ctx, increamentId, newAuction)
		if err != nil {
			return err
		}
	}

	k.SetEstimateAuctionPeriodBlockHeight(ctx, uint64(ctx.BlockHeight())+params.AuctionEpoch)

	return nil

}

func endAuctionPeriod(
	ctx sdk.Context,
	params types.Params,
	latestAuctionPeriod types.AuctionPeriod,
	k keeper.Keeper,
	bk types.BankKeeper,
	ak types.AccountKeeper,
) error {
	for _, auction := range k.GetAllAuctionsByPeriodID(ctx, latestAuctionPeriod.Id) {
		if auction.HighestBid == nil {
			err := k.SendToCommunityPool(ctx, sdk.Coins{*auction.AuctionAmount})
			if err != nil {
				panic(err)
			}
			continue
		}

		// Send in the winning token to the highest bidder address
		err := bk.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sdk.AccAddress(auction.HighestBid.BidderAddress), sdk.Coins{*auction.AuctionAmount})
		if err != nil {
			panic(err)
		}
	}

	balances := bk.GetAllBalances(ctx, ak.GetModuleAccount(ctx, types.ModuleName).GetAddress())

	// Empty the rest of the auction module balances back to community pool
	err := k.SendFromCommunityPool(ctx, balances)
	if err != nil {
		panic(err)
	}
	return nil
}

func BeginBlocker(ctx sdk.Context, k keeper.Keeper, bk types.BankKeeper, ak types.AccountKeeper) {
	params := k.GetParams(ctx)

	// An initial estimateNextBlockHeight need to be set as a starting point
	estimateNextBlockHeight, found := k.GetEstimateAuctionPeriodBlockHeight(ctx)
	if !found {
		return
	}

	if uint64(ctx.BlockHeight()) == estimateNextBlockHeight.Height {
		err := startMewAuctionPeriod(ctx, params, k, bk, ak)
		if err != nil {
			return
		}
	}
}

func EndBlocker(ctx sdk.Context, k keeper.Keeper, bk types.BankKeeper, ak types.AccountKeeper) {
	params := k.GetParams(ctx)

	// An initial auction period need to be set as a starting point
	lastAuctionPeriods, found := k.GetLatestAuctionPeriod(ctx)
	if !found {
		return
	}

	if lastAuctionPeriods.EndBlockHeight == params.AuctionPeriod {
		err := endAuctionPeriod(ctx, params, *lastAuctionPeriods, k, bk, ak)
		if err != nil {
			return
		}
	}
}
