package keeper

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// IterateAuctions executes the given callback `cb` over every discovered Auction in the store
// To exit early, return true from `cb` otherwise return false to continue iteration
func (k Keeper) IterateAuctions(ctx sdk.Context, cb func(key []byte, auction types.Auction) (stop bool)) {
	// Iterate over all items under the Auction key prefix
	store := ctx.KVStore(k.storeKey)
	prefixKey := types.KeyAuction

	iterator := store.Iterator(prefixRange([]byte(prefixKey)))
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		value := iterator.Value()
		// nolint: exhaustruct
		var auction = types.Auction{}
		if err := k.cdc.Unmarshal(value, &auction); err != nil {
			panic(fmt.Sprintf("Got error when unmarshaling bytes %x: %v", value, err))
		}

		if cb(iterator.Key(), auction) {
			return
		}
	}
}

// GetAllAuctions returns all auctions in the store
func (k Keeper) GetAllAuctions(ctx sdk.Context) []types.Auction {
	var auctions []types.Auction
	k.IterateAuctions(ctx, func(_ []byte, auction types.Auction) (stop bool) {
		auctions = append(auctions, auction)
		return false
	})

	return auctions
}

// UpdateHighestBidder will update an auction and replace the `HighestBid` on the auction with `highestBidder`
func (k Keeper) UpdateHighestBidder(ctx sdk.Context, auctionid uint64, highestBidder types.Bid) error {
	auction := k.GetAuctionById(ctx, auctionid)
	if auction == nil {
		return types.ErrAuctionNotFound
	}

	auction.HighestBid = &highestBidder
	return k.UpdateAuction(ctx, *auction)
}

// UpdateAuction overwrites an auction
// Returns an error if the auction does not exist, or if the denom/amount has been updated.
// Panics if `auction` cannot be marshaled
// Note that it is impossible to "update" an auction's id, since that is the key under which it is stored
func (k Keeper) UpdateAuction(ctx sdk.Context, auction types.Auction) error {
	if err := auction.ValidateBasic(); err != nil {
		return err
	}

	if enabled := k.GetParams(ctx).Enabled; !enabled {
		return types.ErrDisabledModule
	}

	// Check that the previous auction exists
	previousAuction := k.GetAuctionById(ctx, auction.Id)
	if previousAuction == nil {
		return errorsmod.Wrap(types.ErrAuctionNotFound, "could not find auction to update")
	}
	// And that the denom/amount has not changed
	if !previousAuction.Amount.Equal(auction.Amount) {
		return errorsmod.Wrap(types.ErrInvalidAuction, "cannot update auction amount or denom")
	}

	auctionKey := []byte(types.GetAuctionKey(auction.Id))
	store := ctx.KVStore(k.storeKey)
	store.Set(auctionKey, k.cdc.MustMarshal(&auction))

	return nil
}

// DeleteAllAuctions will clear the current auctions to prepare for storing the next period's auctions
func (k Keeper) DeleteAllAuctions(ctx sdk.Context) {
	if enabled := k.GetParams(ctx).Enabled; !enabled {
		panic("cannot delete auctions while the module is disabled")
	}
	auctionPeriod := k.GetAuctionPeriod(ctx)
	if auctionPeriod != nil && auctionPeriod.EndBlockHeight > uint64(ctx.BlockHeight()) {
		panic(fmt.Sprintf("attempted to delete all auctions during active auction period %v", auctionPeriod))
	}
	// Collect the keys to avoid deleting while iterating
	keys := [][]byte{} // Each key is a []byte
	k.IterateAuctions(ctx, func(key []byte, _ types.Auction) (stop bool) {
		keys = append(keys, key)
		return false
	})

	// Delete every collected key
	store := ctx.KVStore(k.storeKey)
	for _, k := range keys {
		store.Delete(k)
	}
}

// StoreAuction stores the given `auction`
// Returns an error if the auction is invalid or if an auction with the same Id is already stored
// Panics if auction cannot be marshaled
// Note: This function does not check the Enabled param because it may be used by InitGenesis to restore a disabled module with active auctions
func (k Keeper) StoreAuction(ctx sdk.Context, auction types.Auction) error {
	if err := auction.ValidateBasic(); err != nil {
		return err
	}
	expectedId := k.GetNextAuctionId(ctx)
	if auction.Id != expectedId {
		return errorsmod.Wrapf(types.ErrInvalidAuction, "expected next auction to have id %v, received %v", expectedId, auction.Id)
	}

	if !k.IsDenomAuctionable(ctx, auction.Amount.Denom) {
		return errorsmod.Wrapf(types.ErrInvalidAuction, "denom %v is on the NonAuctionableTokens list", auction.Amount.Denom)
	}

	store := ctx.KVStore(k.storeKey)
	auctionKey := []byte(types.GetAuctionKey(auction.Id))

	if store.Has(auctionKey) {
		return types.ErrDuplicateAuction
	}

	auctionBz := k.cdc.MustMarshal(&auction)
	store.Set(auctionKey, auctionBz)

	// Update the nonce too
	k.IncrementAuctionNonce(ctx)

	return nil
}

// GetAuctionById returns the auction with the given `id`, if any exists
func (k Keeper) GetAuctionById(ctx sdk.Context, id uint64) *types.Auction {
	// This prefix store only contains items under the Auction key
	store := ctx.KVStore(k.storeKey)
	auctionBz := store.Get([]byte(types.GetAuctionKey(id)))

	if len(auctionBz) == 0 {
		return nil
	}
	var auction types.Auction
	k.cdc.MustUnmarshal(auctionBz, &auction)

	return &auction
}

// GetAuctionByDenom returns the auction with the given `denom`, if any exists
func (k Keeper) GetAuctionByDenom(ctx sdk.Context, denom string) *types.Auction {
	var foundAuction *types.Auction = nil
	k.IterateAuctions(ctx, func(_ []byte, auction types.Auction) (stop bool) {
		if auction.Amount.Denom == denom {
			foundAuction = &auction
			return true
		}
		return false
	})

	return foundAuction
}

// GetAllAuctionByBidder returns all auctions with `bidder` as their current highest bidder
func (k Keeper) GetAllAuctionsByBidder(ctx sdk.Context, bidder string) []types.Auction {
	auctions := []types.Auction{}
	k.IterateAuctions(ctx, func(_ []byte, auction types.Auction) (stop bool) {
		if auction.HighestBid != nil && auction.HighestBid.BidderAddress == bidder {
			auctions = append(auctions, auction)
		}
		return false
	})

	return auctions
}

// GetNextAuctionId computes the next usable auction id by looking at the store's AuctionNonce
func (k Keeper) GetNextAuctionId(ctx sdk.Context) uint64 {
	nonce := k.GetAuctionNonce(ctx)
	// Return the next id: stored auction nonce + 1
	return nonce.Id + 1
}

// GetAuctionNonce returns the internal value used to keep track of the most recent auction
// Use GetNextAuctionId for an auto-incremented value when creating new auctions
func (k Keeper) GetAuctionNonce(ctx sdk.Context) types.AuctionId {
	store := ctx.KVStore(k.storeKey)

	if !store.Has([]byte(types.KeyAuctionNonce)) {
		return types.AuctionId{Id: 0}
	}
	lastAuctionIdBz := store.Get([]byte(types.KeyAuctionNonce))
	if len(lastAuctionIdBz) == 0 {
		panic("Unexpected 0 length value in KeyAuctionNonce")
	}
	var auctionId types.AuctionId
	k.cdc.MustUnmarshal(lastAuctionIdBz, &auctionId)

	return auctionId
}

// Adds 1 to the internal nonce used to keep track of the most recently created auction.
// Returns the previously stored value
func (k Keeper) IncrementAuctionNonce(ctx sdk.Context) (lastNonce types.AuctionId) {
	nonce := k.GetAuctionNonce(ctx)
	newNonce := types.AuctionId{Id: nonce.Id + uint64(1)}

	k.unsafeSetAuctionNonce(ctx, newNonce)

	return nonce
}

// Updates the auction nonce directly, do not use outside of InitGenesis
func (k Keeper) unsafeSetAuctionNonce(ctx sdk.Context, nonce types.AuctionId) {
	store := ctx.KVStore(k.storeKey)

	idBz := k.cdc.MustMarshal(&nonce)
	store.Set([]byte(types.KeyAuctionNonce), idBz)
}

// CloseAuctionWithWinner will transfer auction funds to the highest bidder,
// send their bid to the auction pool or burn it,
// and emits a related event
// Panics if the auction had no winning bid
// Note this function takes the auction_id instead of the auction itself to ensure
// correct payouts of auctions
func (k Keeper) CloseAuctionWithWinner(ctx sdk.Context, auction_id uint64) error {
	enabled := k.GetParams(ctx).Enabled
	if !enabled {
		return errorsmod.Wrap(types.ErrDisabledModule, "unable to close an auction when the module is not enabled")
	}
	// Ensure this auction is currently stored
	auction := k.GetAuctionById(ctx, auction_id)
	if auction == nil {
		return types.ErrAuctionNotFound
	}
	if auction.HighestBid == nil {
		panic(fmt.Sprintf("unexpected unsuccessful auction: %s", auction.String()))
	}

	burnWinningBids := k.GetParams(ctx).BurnWinningBids

	highestBidAmount := auction.HighestBid.BidAmount
	highestBidInt := sdk.NewIntFromUint64(highestBidAmount)
	bidToken := k.MintKeeper.GetParams(ctx).MintDenom
	highestBidCoin := sdk.NewCoin(bidToken, highestBidInt)
	highestBidder := sdk.MustAccAddressFromBech32(auction.HighestBid.BidderAddress)

	if burnWinningBids {
		// Burn the winning bid
		if err := k.BankKeeper.BurnCoins(ctx, types.ModuleName, sdk.NewCoins(highestBidCoin)); err != nil {
			return errorsmod.Wrapf(err, "unable to burn highest bid (%v)", highestBidCoin)
		}
	} else {
		// Send bid to community pool
		if err := k.SendFromAuctionAccountToCommunityPool(ctx, highestBidCoin); err != nil {
			return errorsmod.Wrapf(err, "unable to send highest bid (%v) to community pool", highestBidCoin)
		}
	}

	if err := k.AwardAuction(ctx, highestBidder, auction.Amount); err != nil {
		return errorsmod.Wrapf(err, "unable to award auction to highest bidder (%s)", auction.HighestBid.BidderAddress)
	}

	ctx.EventManager().EmitEvent(types.NewEventAuctionAward(auction.Id, highestBidInt, highestBidder, auction.Amount.Denom, auction.Amount.Amount))

	return nil
}

// CloseAuctionNoWinner will transfer auction funds to the auction pool, and emit a related event
// Panics if the auction actually had a winning bid
// Note this function takes the auction_id instead of the auction itself to ensure
// correct payouts of auctions
func (k Keeper) CloseAuctionNoWinner(ctx sdk.Context, auction_id uint64) error {
	enabled := k.GetParams(ctx).Enabled
	if !enabled {
		return errorsmod.Wrap(types.ErrDisabledModule, "unable to close an auction when the module is not enabled")
	}
	// Ensure this auction is currently stored
	auction := k.GetAuctionById(ctx, auction_id)
	if auction == nil {
		return types.ErrAuctionNotFound
	}
	if auction.HighestBid != nil {
		panic(fmt.Sprintf("unexpected successful auction: %v", auction))
	}

	// Send amount back to auction pool for future auctions
	if err := k.SendToAuctionPool(ctx, sdk.NewCoins(auction.Amount)); err != nil {
		return errorsmod.Wrapf(err, "unable to send auction amount (%v) to auction pool", auction.Amount)
	}

	ctx.EventManager().EmitEvent(types.NewEventAuctionFailure(auction.Id, auction.Amount.Denom, auction.Amount.Amount))

	return nil
}
