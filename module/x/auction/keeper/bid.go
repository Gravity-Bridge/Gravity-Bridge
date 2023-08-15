package keeper

import (
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) GetBidsQueue(ctx sdk.Context, auctionId uint64) (val types.BidsQueue, found bool) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixBidsQueue))

	bz := store.Get(uint64ToBytes(auctionId))
	if bz == nil {
		return val, false
	}

	k.cdc.MustUnmarshal(bz, &val)
	return val, true
}

func (k Keeper) SetBidsQueue(ctx sdk.Context, bidsQueue types.BidsQueue, autionId uint64) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixBidsQueue))

	bz := k.cdc.MustMarshal(&bidsQueue)
	store.Set(uint64ToBytes(autionId), bz)
}

func (k Keeper) CreateNewBidQueue(ctx sdk.Context, autionId uint64) {
	bidQueue := types.BidsQueue{
		Queue: []*types.Bid{},
	}
	k.SetBidsQueue(ctx, bidQueue, autionId)
}

func (k Keeper) AddBidToQueue(ctx sdk.Context, bid types.Bid, bidsQueue *types.BidsQueue) {
	bidsQueue.Queue = append(bidsQueue.Queue, &bid)

	k.SetBidsQueue(ctx, *bidsQueue, bid.AuctionId)
}

func (k Keeper) ClearQueue(ctx sdk.Context, auctionId uint64) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixBidsQueue))
	store.Delete(uint64ToByte(auctionId))
}
