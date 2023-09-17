package auction

import (
	"fmt"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
)

func startNewAuctionPeriod(ctx sdk.Context, params types.Params, k keeper.Keeper, bk types.BankKeeper, ak types.AccountKeeper) error {
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

		// For zero balance skip creation of auction for this token
		if balance.IsZero() {
			ctx.Logger().Info(fmt.Sprintf("Token with denom %s is empty in community pool", token))
			continue
		}

		// Compute auction amount to send to auction module account
		amount := sdk.NewDecFromInt(balance.Amount).Mul(auctionRate).TruncateInt()

		// For zero amount skip creation of auction for this token
		if amount.IsZero() {
			ctx.Logger().Info(fmt.Sprintf("Auction amount with denom %s is empty", token))
			continue
		}

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
			AuctionAmount:   sdkcoin,
			Status:          1,
			HighestBid:      nil,
		}

		// Update auction in auction period auction list
		err = k.AddNewAuctionToAuctionPeriod(ctx, increamentId, newAuction)
		if err != nil {
			return err
		}
	}

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
		// Update auction status to finished
		auction.Status = types.AuctionStatus_AUCTION_STATUS_FINISH
		k.SetAuction(ctx, auction)

		// If no bid continue
		if auction.HighestBid == nil {
			ctx.Logger().Info("No bid entry for this auction")
			continue
		}

		// Send in the winning token to the highest bidder address
		err := bk.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sdk.MustAccAddressFromBech32(auction.HighestBid.BidderAddress), sdk.Coins{auction.AuctionAmount})
		if err != nil {
			panic(err)
		}

	}

	balances := bk.GetAllBalances(ctx, ak.GetModuleAccount(ctx, types.ModuleName).GetAddress())

	// Empty the rest of the auction module balances back to community pool
	err := k.SendToCommunityPool(ctx, balances)
	if err != nil {
		ctx.Logger().Error("Fail to return fund to community pool, will try again in the end of the next auction period")
	}
	return nil
}

func BeginBlocker(ctx sdk.Context, k keeper.Keeper, bk types.BankKeeper, ak types.AccountKeeper) {
	params := k.GetParams(ctx)

	// An initial estimateNextBlockHeight need to be set as a starting point
	estimateNextBlockHeight, found := k.GetEstimateAuctionPeriodBlockHeight(ctx)
	if !found {
		panic("Cannot find estimate block height for next auction period")
	}

	if uint64(ctx.BlockHeight()) == estimateNextBlockHeight.Height {
		// Set estimate block height for next auction periods
		k.SetEstimateAuctionPeriodBlockHeight(ctx, uint64(ctx.BlockHeight())+params.AuctionEpoch)

		err := startNewAuctionPeriod(ctx, params, k, bk, ak)
		if err != nil {
			ctx.Logger().Error(fmt.Sprintf("Fail to initialize a new auction period at height %v, detail log: %s", ctx.BlockHeight(), err.Error()))
			return
		}
	}
}

func EndBlocker(ctx sdk.Context, k keeper.Keeper, bk types.BankKeeper, ak types.AccountKeeper) {
	params := k.GetParams(ctx)

	lastestAuctionPeriods, found := k.GetLatestAuctionPeriod(ctx)
	if !found {
		return
	}

	if lastestAuctionPeriods.EndBlockHeight == uint64(ctx.BlockHeight()) {
		err := endAuctionPeriod(ctx, params, *lastestAuctionPeriods, k, bk, ak)
		if err != nil {
			ctx.Logger().Error(fmt.Sprintf("Fail to end the current auction period at height %v, detail log: %s", ctx.BlockHeight(), err.Error()))
			return
		}
	}
}
