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
