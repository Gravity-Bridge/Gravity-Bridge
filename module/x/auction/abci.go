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
	// Take a snapshot of the total token supply and Auction account balances for assertions at the end of EndBlocker
	startSupplies := getBankSupplies(ctx, k)
	startModuleBalance := k.BankKeeper.GetAllBalances(ctx, k.AccountKeeper.GetModuleAddress(types.ModuleName))
	startPoolBalance := k.BankKeeper.GetAllBalances(ctx, k.AccountKeeper.GetModuleAddress(types.AuctionPoolAccountName))
	var closingAuctions []types.Auction
	affectedAccs := make(map[string]sdk.Coins)
	periodEnded := false

	// Schedule token supply and account balance assertions after EndBlocker finishes
	defer func() {
		// defer will execute this func() just before any `return` from EndBlocker executes

		endSupplies := getBankSupplies(ctx, k)
		endModuleBalance := k.BankKeeper.GetAllBalances(ctx, k.AccountKeeper.GetModuleAddress(types.ModuleName))
		endPoolBalance := k.BankKeeper.GetAllBalances(ctx, k.AccountKeeper.GetModuleAddress(types.AuctionPoolAccountName))

		assertSupplyIntegrity(ctx, k, startSupplies, endSupplies, periodEnded)
		assertBalanceChanges(ctx, k, periodEnded, closingAuctions, affectedAccs, startModuleBalance, startPoolBalance, endModuleBalance, endPoolBalance)
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
		closingAuctions = k.GetAllAuctions(ctx)
		// Store the winning bidder Accs and their balances before their tokens are modified for later verification
		k.IterateAuctions(ctx, func(_ []byte, auction types.Auction) (stop bool) {
			if auction.HighestBid != nil {
				acc := sdk.MustAccAddressFromBech32(auction.HighestBid.BidderAddress)
				balances := k.BankKeeper.GetAllBalances(ctx, acc)
				affectedAccs[auction.HighestBid.BidderAddress] = balances
			}
			return false // Continue iterating through all auctions
		})

		endAuctionPeriod(ctx, k)
		scheduleNextAuctionPeriod(ctx, k)
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
		}

		return false // Continue iterating through all of them
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

// getBankSupplies gets the whole list of all coins that exist according to the bank module as of this moment
func getBankSupplies(ctx sdk.Context, k keeper.Keeper) sdk.Coins {
	coins := sdk.NewCoins()
	k.BankKeeper.IterateTotalSupply(ctx, func(c sdk.Coin) bool {
		coins = coins.Add(c)
		return false
	})
	return coins
}

// assertSupplyIntegrity checks that the bank supply only changes as expected due to the auction module, potentially panicking
// If the auction period ended this block then only GRAV is allowed to decrease in supply when BurnWinningBids is true
// otherwise no token supply is allowed to change.
// WARNING: Only call after the EndBlocker has made all of its state changes
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
		difference := startSupplies.Sub(endSupplies...)
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

// assertBalanceChanges checks that the module, pool, and user accounts have balance changes which make sense for the auctions that closed and the new ones that opened
// WARNING: Only call after the EndBlocker has made all of its state changes
func assertBalanceChanges(
	ctx sdk.Context,
	k keeper.Keeper,
	periodEnded bool,
	auctions []types.Auction,
	affectedAccs map[string]sdk.Coins,
	startModuleBalances, startPoolBalances, endModuleBalances, endPoolBalances sdk.Coins,
) {
	// If the period did not end, then EndBlocker should not trigger any balance changes
	if !periodEnded {
		// There should be no affected Accs
		if len(affectedAccs) != 0 {
			panic(fmt.Sprintf("No auctions closed but there were user accounts affected by the auction module: %v", affectedAccs))
		}
		// The module balances should not have changed as a result of the EndBlocker
		if !startModuleBalances.IsEqual(endModuleBalances) {
			panic(fmt.Sprintf("No auctions closed but the module balance changed during EndBlocker: %v -> %v", startModuleBalances, endModuleBalances))

		}
		// The pool balances should not have changed as a result of the EndBlocker
		if !startPoolBalances.IsEqual(endPoolBalances) {
			panic(fmt.Sprintf("No auctions closed but the pool balance changed during EndBlocker: %v -> %v", startPoolBalances, endPoolBalances))
		}
		return
	}

	// Otherwise, there were auctions that closed and new ones that opened

	bidAmounts := sdk.ZeroInt()
	for _, auction := range auctions {
		startModAmount := startModuleBalances.AmountOf(auction.Amount.Denom)
		if !startModAmount.Equal(auction.Amount.Amount) {
			panic(fmt.Sprintf("Auction EndBlocker: Expected auction module account to only hold the closing auction amount %v, instead it held %v", auction.Amount, startModAmount))
		}
		endModAmount := endModuleBalances.AmountOf(auction.Amount.Denom)
		startPoolAmount := startPoolBalances.AmountOf(auction.Amount.Denom)

		// If there is a HighestBid recorded, that Acc wins the auction. Otherwise the balance is recycled for the next auction.
		if auction.HighestBid != nil {
			// Auction Paid Out: Amount sent from Module to Bidder, Bid removed from Module (user does not receive it, bid is burned/paid to stakers outside of the EndBlocker's scope)
			startBidderBals := affectedAccs[auction.HighestBid.BidderAddress]
			startBidderGrav := startBidderBals.AmountOf(config.NativeTokenDenom)
			endBidderGrav := k.BankKeeper.GetBalance(ctx, sdk.MustAccAddressFromBech32(auction.HighestBid.BidderAddress), config.NativeTokenDenom)
			if !endBidderGrav.Amount.Equal(startBidderGrav) {
				panic(fmt.Sprintf("Auction EndBlocker: Unexpected change to bidder %v's GRAV: %v -> %v\n\n%v", auction.HighestBid.BidderAddress, startBidderGrav, endBidderGrav, startBidderBals))
			}

			startBidderAmount := sdk.NewCoin(auction.Amount.Denom, startBidderBals.AmountOf(auction.Amount.Denom))
			endBidderAmount := k.BankKeeper.GetBalance(ctx, sdk.MustAccAddressFromBech32(auction.HighestBid.BidderAddress), auction.Amount.Denom)
			bidderAmountDiff := endBidderAmount.Sub(startBidderAmount)
			if !bidderAmountDiff.Equal(auction.Amount) {
				panic(fmt.Sprintf("Auction EndBlocker: Bidder gained %v by winning auction, but expected to receive %v", bidderAmountDiff, auction.Amount))
			}

			// Since we're at the end of EndBlocker, expecting the module to have lost all its auction balance (paid out to bidder) and to have gained the starting pool balance
			if !endModAmount.Equal(startPoolAmount) {
				panic(fmt.Sprintf("Auction EndBlocker: Expected auction module to receive auction pool balance (%v) of %v, instead received %v", startPoolAmount, auction.Amount.Denom, endModAmount))
			}

			// Tally the bids for later checking
			bidAmounts = bidAmounts.Add(sdk.NewIntFromUint64(auction.HighestBid.BidAmount))
		} else {
			// Auction Not Paid Out: Amount sent from Module to Pool, then full Pool balance sent back to Module for new auction
			expectedModAmount := startModAmount.Add(startPoolAmount)
			if !endModAmount.Equal(expectedModAmount) {
				panic(fmt.Sprintf("Auction EndBlocker: Expected module account to add the pool balance (%v) to its own balance (%v), instead it held %v", startPoolAmount, startModAmount, endModAmount))
			}
		}
	}

	// The pool should not hold any balances after the creation of new auctions, they are either locked in auctions or sent to the Community Pool (non auctionable tokens)
	if !endPoolBalances.IsZero() {
		panic(fmt.Sprintf("Auction EndBlocker: Expected auction pool to lose all balances at the end of EndBlocker, instead it held %v", endPoolBalances))
	}

	// Assert that the bid amounts were removed from the module account
	startModGrav := startModuleBalances.AmountOf(config.NativeTokenDenom)
	endModGrav := endModuleBalances.AmountOf(config.NativeTokenDenom)
	if !endModGrav.Equal(sdk.ZeroInt()) {
		panic(fmt.Sprintf("Auction EndBlocker: Expected module to lose its balance of Grav after auctions close, instead it has %v", endModGrav))
	}
	if !startModGrav.Equal(bidAmounts) {
		panic(fmt.Sprintf("Auction EndBlocker: Expected the module account to hold only the bid amounts in Grav (%v) at the start of EndBlocker, instead it held %v", bidAmounts, startModGrav))
	}

	// The module account should now only hold balances for the new auctions, which may include new tokens not previously auctioned
	k.IterateAuctions(ctx, func(_ []byte, auction types.Auction) (stop bool) {
		modAmount := endModuleBalances.AmountOf(auction.Amount.Denom)
		if !modAmount.Equal(auction.Amount.Amount) {
			panic(fmt.Sprintf("Auction EndBlocker: Expected module account to hold the new auction amount %v, instead it held %v", auction.Amount, modAmount))
		}
		return false // Continue iterating through all auctions
	})
}
