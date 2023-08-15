package keeper

import (
	"encoding/binary"
	"fmt"

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
	store.Set([]byte(getKeyForAuction(auction)), bz)
}

// UpdateAuctionStatus updates the status of an auction
func (k Keeper) UpdateAuctionStatus(ctx sdk.Context, auction *types.Auction, newStatus types.AuctionStatus) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixAuction))
	if auction != nil {
		auction.Status = newStatus
		newBz := k.cdc.MustMarshal(auction)
		store.Set([]byte(getKeyForAuction(*auction)), newBz)
	}
}

// UpdateAuctionPeriod updates the auction period with the given id with the given auction.
func (k Keeper) AddNewAuctionToAuctionPeriod(ctx sdk.Context, periodId uint64, auction types.Auction) error {
	_, found := k.GetAuctionPeriodByID(ctx, periodId)
	if !found {
		return types.ErrAuctionPeriodNotFound
	}

	auction.AuctionPeriodId = periodId
	k.SetAuction(ctx, auction)
	return nil
}

// GetAllAuctionsByPeriodID returns all auctions for the given auction period id.
func (k Keeper) GetAllAuctionsByPeriodID(ctx sdk.Context, periodId uint64) []types.Auction {
	auctions := k.GetAllAuctions(ctx)

	auctionsFound := []types.Auction{}
	for _, auction := range auctions {
		if auction.AuctionPeriodId == periodId {
			auctionsFound = append(auctionsFound, auction)
		}
	}
	return auctionsFound
}

// GetAuctionByPeriodID returns all auctions for the given auction period id.
func (k Keeper) GetAuctionByPeriodIDAndAuctionId(ctx sdk.Context, periodId uint64, id uint64) (types.Auction, bool) {
	auctions := k.GetAllAuctions(ctx)

	auctionsFound := []types.Auction{}
	for _, auction := range auctions {
		if auction.AuctionPeriodId == periodId && auction.Id == id {
			auctionsFound = append(auctionsFound, auction)
		}
	}

	if len(auctionsFound) == 0 {
		// nolint: exhaustruct
		return types.Auction{}, false
	}
	return auctionsFound[0], true
}

func (k Keeper) IncreamentAuctionId(ctx sdk.Context, periodId uint64) (uint64, error) {
	auctions := k.GetAllAuctionsByPeriodID(ctx, periodId)
	return uint64(len(auctions)) + 1, nil
}

// TODO get auction by status? or by token? by auction period ID ?

// GetAllAuctionsByAuctionID returns all auctions for the given auction period id.
func (k Keeper) GetAllAuctionsByAuctionID(ctx sdk.Context, auctionId uint64) []types.Auction {
	auctions := k.GetAllAuctions(ctx)

	auctionsFound := []types.Auction{}
	for _, auction := range auctions {
		if auction.Id == auctionId {
			auctionsFound = append(auctionsFound, auction)
		}
	}
	return auctionsFound
}

// GetAllAuctionByBidderAndPeriodId returns all auctions for the given auction period id and bidder address.
func (k Keeper) GetAllAuctionByBidderAndPeriodId(ctx sdk.Context, bidder string, periodId uint64) []types.Auction {
	auctions := k.GetAllAuctionsByPeriodID(ctx, periodId)

	auctionsFound := []types.Auction{}
	for _, auction := range auctions {
		if auction.HighestBid.BidderAddress == bidder {
			auctionsFound = append(auctionsFound, auction)
		}
	}
	return auctionsFound
}

// GetHighestBidByAuctionIdAndPeriodID returns highest bid entry at a given auction id and period id.
func (k Keeper) GetHighestBidByAuctionIdAndPeriodID(ctx sdk.Context, auctionId uint64, periodId uint64) (types.Bid, bool) {
	auctions := k.GetAllAuctionsByPeriodID(ctx, periodId)

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

func getKeyForAuction(auction types.Auction) string {
	return fmt.Sprintf("%v-%v", auction.AuctionPeriodId, auction.Id)
}
