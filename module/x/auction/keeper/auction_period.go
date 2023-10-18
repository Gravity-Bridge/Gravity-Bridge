package keeper

import (
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetAuctionPeriodByID returns the auction period with the given id.
func (k Keeper) GetLatestAuctionPeriod(ctx sdk.Context) (val types.AuctionPeriod, found bool) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixAuctionPeriod))
	bz := store.Get([]byte(types.KeyPrefixAuctionPeriod))
	if bz == nil {
		return val, false
	}

	k.cdc.MustUnmarshal(bz, &val)
	return val, true
}

// SetAuctionPeriod sets the given auction period.
func (k Keeper) SetAuctionPeriod(ctx sdk.Context, auctionPeriod types.AuctionPeriod) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixAuctionPeriod))

	bz := k.cdc.MustMarshal(&auctionPeriod)
	store.Set([]byte(types.KeyPrefixAuctionPeriod), bz)
}

// SetLastAuctionPeriodBlockHeight sets the block height for given height.
func (k Keeper) SetEstimateAuctionPeriodBlockHeight(ctx sdk.Context, blockHeight uint64) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixEstimateNextAuctionPeriodBlockHeight))

	nextBlockHeight := types.EstimateNextAuctionPeriodHeight{
		Height: blockHeight,
	}

	bz := k.cdc.MustMarshal(&nextBlockHeight)
	store.Set([]byte(types.KeyAuctionPeriodBlockHeight), bz)
}

func (k Keeper) GetEstimateAuctionPeriodBlockHeight(ctx sdk.Context) (blockHeight types.EstimateNextAuctionPeriodHeight, found bool) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixEstimateNextAuctionPeriodBlockHeight))
	bz := store.Get([]byte(types.KeyAuctionPeriodBlockHeight))
	if bz == nil {
		return blockHeight, false
	}

	k.cdc.MustUnmarshal(bz, &blockHeight)
	return blockHeight, true
}

func (k Keeper) DeleteAutionPeriod(ctx sdk.Context, id uint64) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.KeyPrefixAuctionPeriod))

	store.Delete([]byte(types.KeyPrefixAuctionPeriod))
}
