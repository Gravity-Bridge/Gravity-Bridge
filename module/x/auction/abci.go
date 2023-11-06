package auction

import (
	"fmt"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/config"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EndBlocker resolves a finished AuctionPeriod and schedules a new one
func EndBlocker(ctx sdk.Context, k keeper.Keeper) {
	startSupplies := getBankSupplies(ctx, k)
	periodEnded := false
	defer func() {
		endSupplies := getBankSupplies(ctx, k)
		assertSupplyIntegrity(ctx, k, startSupplies, endSupplies, periodEnded)
	}()
	// Do nothing if the module is disabled, the current auctions must remain locked
	if enabled := k.GetParams(ctx).Enabled; !enabled {
		return
	}

	auctionPeriod := k.GetAuctionPeriod(ctx)
	if auctionPeriod == nil {
		panic("nil auction period discovered in EndBlocker - should have been initialized by now")
	}

	// The end height should only be in the past if the module was disabled through the end of the period
	// otherwise we expect to observe the exact end of the period
	if auctionPeriod.EndBlockHeight <= uint64(ctx.BlockHeight()) {
		periodEnded = true
		endAuctionPeriod(ctx, k)
		scheduleNextAuctionPeriod(ctx, k)
	}
}

// Gets the whole list of all coins that exist according to the bank module as of this moment
func getBankSupplies(ctx sdk.Context, k keeper.Keeper) sdk.Coins {
	coins := sdk.NewCoins()
	k.BankKeeper.IterateTotalSupply(ctx, func(c sdk.Coin) bool {
		coins = coins.Add(c)
		return false
	})
	return coins
}

// Checks that the bank supply only changes as expected due to the auction module
func assertSupplyIntegrity(ctx sdk.Context, k keeper.Keeper, startSupplies sdk.Coins, endSupplies sdk.Coins, periodEnded bool) {
	// Outside of the auction period changeover, no token supply should have changed
	if !periodEnded {
		if !startSupplies.IsEqual(endSupplies) {
			panic(fmt.Sprintf("unexpected supply change during auction module EndBlocker (no auctions closed) %v -> %v", startSupplies, endSupplies))
		}
	} else {
		// During an auction period changeover, only the native token supply should have changed while BurnWinningBids = true
		// Expecting a decrease if any sort of change, Sub panics on negative values
		burnWinningBids := k.GetParams(ctx).BurnWinningBids
		difference := startSupplies.Sub(endSupplies)
		if !difference.IsZero() {
			if difference.Len() != 1 {
				panic(fmt.Sprintf("auction close changed the supply of more than just the native token: %v", difference))
			}
			affectedToken := difference[0]
			if affectedToken.Denom != config.NativeTokenDenom {
				panic(fmt.Sprintf("auction close affected a token which is NOT the native token: %v", affectedToken))
			}
			if !burnWinningBids {
				panic("BurnWinningBids is false but the auction close affected native token supply")
			}
		}
	}
}

// endAuctionPeriod terminates failed auctions or awards active auctions and then emits an event
func endAuctionPeriod(ctx sdk.Context, k keeper.Keeper) {
	endingPeriod := k.GetAuctionPeriod(ctx)
	var closeError error = nil
	// Resolve the open auctions
	k.IterateAuctions(ctx, func(_ []byte, auction types.Auction) (stop bool) {
		if auction.HighestBid != nil {
			closeError = k.CloseAuctionWithWinner(ctx, auction.Id)
		} else {
			closeError = k.CloseAuctionNoWinner(ctx, auction.Id)
		}

		if closeError != nil {
			errMsg := fmt.Sprintf("unable to close auction: %v", closeError)
			ctx.Logger().Error(errMsg)
			panic(errMsg)
		} else {
			return false // Continue iterating through all of them
		}
	})

	// Clear the old auctions in preparation for new period
	k.DeleteAllAuctions(ctx)

	ctx.EventManager().EmitEvent(types.NewEventPeriodEnd(endingPeriod.StartBlockHeight, endingPeriod.EndBlockHeight))
}

// scheduleNextAuctionPeriod will create a new AuctionPeriod starting on the next block
func scheduleNextAuctionPeriod(ctx sdk.Context, k keeper.Keeper) {
	auctionPeriod, err := k.CreateNewAuctionPeriod(ctx)
	if err != nil {
		errMsg := fmt.Sprintf("unable to create new auction period: %v", err)
		ctx.Logger().Error(errMsg)
		panic(errMsg)
	}

	if err := k.CreateAuctionsForAuctionPeriod(ctx); err != nil {
		errMsg := fmt.Sprintf("unable to create auctions for new auction period: %v", err)
		ctx.Logger().Error(errMsg)
		panic(errMsg)
	}

	ctx.EventManager().EmitEvent(types.NewEventPeriodStart(auctionPeriod.StartBlockHeight, auctionPeriod.EndBlockHeight))
}
