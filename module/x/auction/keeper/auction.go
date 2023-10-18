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

// GetAllAuction returns all auctions.
func (k Keeper) GetAllAuctions(ctx sdk.Context) []types.Auction {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixAuction))

	iterator := sdk.KVStorePrefixIterator(store, nil)
	defer iterator.Close()

	var auctions []types.Auction
	for ; iterator.Valid(); iterator.Next() {
		var auction types.Auction
		k.cdc.MustUnmarshal(iterator.Value(), &auction)
		auctions = append(auctions, auction)
	}

	return auctions
}

// SetAuction sets an auction
func (k Keeper) SetAuction(ctx sdk.Context, auction types.Auction) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixAuction))
	bz := k.cdc.MustMarshal(&auction)
	store.Set(uint64ToByte(auction.Id), bz)
}

// UpdateAuctionStatus updates the status of an auction
func (k Keeper) UpdateAuctionStatus(ctx sdk.Context, auction *types.Auction, newStatus types.AuctionStatus) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixAuction))
	if auction != nil {
		auction.Status = newStatus
		newBz := k.cdc.MustMarshal(auction)
		store.Set(uint64ToByte(auction.Id), newBz)
	}
}

// UpdateAuctionNewBid updates the new bid of an auction
func (k Keeper) UpdateAuctionNewBid(ctx sdk.Context, auctionId uint64, newBid types.Bid) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixAuction))
	key := uint64ToByte(auctionId)
	bz := store.Get(key)
	if len(bz) > 0 {
		var auction types.Auction
		k.cdc.MustUnmarshal(bz, &auction)
		auction.HighestBid = &newBid
		newBz := k.cdc.MustMarshal(&auction)
		store.Set(key, newBz)
	}
}

// UpdateAuctionPeriod updates the auction period with the given id with the given auction.
func (k Keeper) AddNewAuctionToAuctionPeriod(ctx sdk.Context, auction types.Auction) error {
	_, found := k.GetLatestAuctionPeriod(ctx)
	if !found {
		return types.ErrAuctionPeriodNotFound
	}

	k.SetAuction(ctx, auction)
	return nil
}

// GetAuctionById returns all auctions for the given auction id.
func (k Keeper) GetAuctionById(ctx sdk.Context, id uint64) (val types.Auction, found bool) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixAuction))
	bz := store.Get(uint64ToByte(id))
	if bz == nil {
		return val, false
	}

	k.cdc.MustUnmarshal(bz, &val)
	return val, true
}

func (k Keeper) IncreamentAuctionId(ctx sdk.Context) (uint64, error) {
	auctions := k.GetAllAuctions(ctx)
	return uint64(len(auctions)) + 1, nil
}

// GetAllAuctionByBidder returns all auctions for the given bidder address.
func (k Keeper) GetAllAuctionByBidder(ctx sdk.Context, bidder string) []types.Auction {
	auctions := k.GetAllAuctions(ctx)

	auctionsFound := []types.Auction{}
	for _, auction := range auctions {
		if auction.HighestBid.BidderAddress == bidder {
			auctionsFound = append(auctionsFound, auction)
		}
	}
	return auctionsFound
}

// GetHighestBidByAuctionId returns highest bid entry at a given auction id.
func (k Keeper) GetHighestBidByAuctionId(ctx sdk.Context, auctionId uint64) (types.Bid, bool) {
	auctions := k.GetAllAuctions(ctx)

	found := false
	var bid *types.Bid

	for _, auction := range auctions {
		if auction.Id == auctionId {
			bid = auction.HighestBid
			found = true
			break
		}
	}
	if found {
		return *bid, true
	}

	return *bid, false
}

// TODO: remove aution func
func (k Keeper) RemoveAuction(ctx sdk.Context, id uint64) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixAuction))

	store.Delete(uint64ToByte(id))
}

// TODO: remove aution func
