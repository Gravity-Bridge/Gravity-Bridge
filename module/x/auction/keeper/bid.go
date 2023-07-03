package keeper

import (
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetBidByID returns the bids by auction ID
func (k Keeper) GetBidByID(ctx sdk.Context, auctionID uint64) (val types.Bid, found bool) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixBid))
	bidKey := uint64ToByte(auctionID)
	bz := store.Get(bidKey)
	if len(bz) == 0 {
		return val, false
	}
	k.cdc.MustUnmarshal(bz, &val)
	return val, true
}

// SetBid sets the bid for a specific auction and bidder.
func (k Keeper) SetBid(ctx sdk.Context, auctionID uint64, bid types.Bid) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixBid))
	bidKey := append(uint64ToByte(auctionID), []byte(bid.BidderAddress)...)
	bz := k.cdc.MustMarshal(&bid)
	store.Set(bidKey, bz)
}

// UpdateBidAmount updates the bid amount for a specific auction and bidder.
func (k Keeper) UpdateBidAmount(ctx sdk.Context, auctionID uint64, newAmount sdk.Coin) bool {
	bid, found := k.GetBidByID(ctx, auctionID)
	if !found {
		return false
	}
	bid.BidAmount = &newAmount
	k.SetBid(ctx, auctionID, bid)
	return true
}

// DeleteBid deletes the bid for a specific auction and bidder.
func (k Keeper) DeleteBid(ctx sdk.Context, auctionID uint64, bidderAddress string) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixBid))
	bidKey := append(uint64ToByte(auctionID), []byte(bidderAddress)...)
	store.Delete(bidKey)
}
