package keeper

import (
	"encoding/binary"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// helper function to convert uint64 to []byte
func uint64ToByte(num uint64) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, num)
	return buf[:n]
}

// GetAuction returns auction by id
func (k Keeper) GetAuction(ctx sdk.Context, id uint64) (val types.Auction, found bool) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixAuction))
	bz := store.Get(uint64ToByte(id))
	if len(bz) == 0 {
		return val, false
	}
	k.cdc.MustUnmarshal(bz, &val)
	return val, true
}

// SetAuction sets an auction
func (k Keeper) SetAuction(ctx sdk.Context, auction types.Auction) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixAuction))
	bz := k.cdc.MustMarshal(&auction)
	store.Set(uint64ToByte(auction.Id), bz)
}

// UpdateAuctionStatus updates the status of an auction
func (k Keeper) UpdateAuctionStatus(ctx sdk.Context, id uint64, newStatus types.AuctionStatus) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixAuction))
	bz := store.Get(uint64ToByte(id))
	if len(bz) > 0 {
		var auction types.Auction
		k.cdc.MustUnmarshal(bz, &auction)
		auction.Status = newStatus
		newBz := k.cdc.MustMarshal(&auction)
		store.Set(uint64ToByte(id), newBz)
	}
}

// UpdateAuctionNewBid updates the new bid of an auction
func (k Keeper) UpdateAuctionNewBid(ctx sdk.Context, id uint64, newBid types.Bid) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixAuction))
	bz := store.Get(uint64ToByte(id))
	if len(bz) > 0 {
		var auction types.Auction
		k.cdc.MustUnmarshal(bz, &auction)
		auction.HighestBid = &newBid
		newBz := k.cdc.MustMarshal(&auction)
		store.Set(uint64ToByte(id), newBz)
	}
}

// TODO get auction by status? or by token? by auction period ID ?
