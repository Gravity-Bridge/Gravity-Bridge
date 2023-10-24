package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

/*	Auction Module Invariants
	For background: https://docs.cosmos.network/main/building-modules/invariants
	Invariants on Gravity Bridge chain will be enforced by most validators every 200 blocks, see module/cmd/root.go for
	the automatic configuration. These settings are overrideable and not consensus breaking so there are no firm
	guarantees of invariant checking no matter what is put here.
*/

// AllInvariants collects any defined invariants below
func AllInvariants(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		res, stop := ModuleBalanceInvariant(k)(ctx)
		if stop {
			return res, stop
		}
		return ValidAuctionsInvariant(k)(ctx)

		/*
			Example additional invariants:
			res, stop := FutureInvariant(k)(ctx)
			if stop {
				return res, stop
			}
			return AnotherFutureInvariant(k)(ctx)
		*/
	}
}

// ModuleBalanceInvariant is a closure (enclosing the keeper) which performs state checks at runtime
// returning an error message and true in case of failure, or an empty string and false in case of success.
// In particular this invariant checks that the auction module's balances match the active auctions and highest bids
func ModuleBalanceInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (message string, invalidState bool) {
		moduleAccount := k.AccountKeeper.GetModuleAccount(ctx, types.ModuleName).GetAddress()
		accountBalances := k.BankKeeper.GetAllBalances(ctx, moduleAccount)

		expectedBalances := ExpectedAuctionModuleBalances(ctx, k)

		if !accountBalances.IsEqual(expectedBalances) {
			return "invalid auction module balances", true
		} else {
			return "", false
		}
	}
}

func ExpectedAuctionModuleBalances(ctx sdk.Context, k Keeper) sdk.Coins {
	bidToken := k.MintKeeper.GetParams(ctx).MintDenom
	var highestBids = sdk.ZeroInt()
	var awardAmounts sdk.Coins

	k.IterateAuctions(ctx, func(_ []byte, auction types.Auction) (stop bool) {
		if awardAmounts.AmountOf(auction.Amount.Denom).IsZero() {
			awardAmounts = append(awardAmounts, auction.Amount)
		} else {
			// If the award amounts already contain this token, then we would have two auctions active on the same coin
			// This contradicts the design so we panic here
			panic(fmt.Sprintf("tallied awards already contain auction token: auction amount (%v) tally (%v)", auction.Amount, awardAmounts))
		}

		if auction.HighestBid != nil {
			highestBids = highestBids.Add(sdk.NewIntFromUint64(auction.HighestBid.BidAmount))
		}

		return false
	})

	return awardAmounts.Add(sdk.NewCoin(bidToken, highestBids))
}

// ValidAuctionsInvariant is a closure (enclosing the keeper) which performs state checks at runtime
// returning an error message and true in case of failure, or an empty string and false in case of success.
// In particular this invariant checks that the auction module's auctions do not contain the native staking token
func ValidAuctionsInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (message string, invalidState bool) {
		nativeToken := k.MintKeeper.GetParams(ctx).MintDenom
		invalid := false
		k.IterateAuctions(ctx, func(key []byte, auc types.Auction) (stop bool) {
			if auc.Amount.Denom == nativeToken {
				invalid = true
				return true
			}
			return false
		})
		if invalid {
			return "discovered auction for the native token", true
		}
		return "", false
	}
}
