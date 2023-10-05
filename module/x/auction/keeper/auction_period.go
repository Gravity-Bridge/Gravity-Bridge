package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

// UpdateAuctionPeriod replaces the current auction period with the given one
func (k Keeper) UpdateAuctionPeriod(ctx sdk.Context, auctionPeriod types.AuctionPeriod) error {
	lastPeriod := k.GetAuctionPeriod(ctx)
	if lastPeriod != nil && lastPeriod.EndBlockHeight > uint64(ctx.BlockHeight()) {
		return sdkerrors.Wrap(types.ErrInvalidAuctionPeriod, "cannot update auction period during the current auction period")
	}
	if lastPeriod != nil && (lastPeriod.EndBlockHeight > auctionPeriod.StartBlockHeight || lastPeriod.EndBlockHeight >= auctionPeriod.EndBlockHeight) {
		return sdkerrors.Wrapf(types.ErrInvalidAuctionPeriod, "new auction period (%v) conflicts with the last one (%v)", auctionPeriod, lastPeriod)
	}

	k.updateAuctionPeriodUnsafe(ctx, auctionPeriod)

	return nil
}

// updateAuctionPeriodUnsafe forces the auction period update with minimal safety checks
// in particular it checks to see if the end block height is
func (k Keeper) updateAuctionPeriodUnsafe(ctx sdk.Context, auctionPeriod types.AuctionPeriod) {
	if auctionPeriod.StartBlockHeight > uint64(ctx.BlockHeight()+1) { // This is invalid in all situations, so the check is used here
		panic("cannot update auction end to start after the next block")
	}
	if auctionPeriod.EndBlockHeight < uint64(ctx.BlockHeight()) { // This is invalid in all situations, so the check is used here
		panic("cannot update auction end to be in the past")
	}
	store := ctx.KVStore(k.storeKey)
	key := []byte(types.KeyAuctionPeriod)

	bz := k.cdc.MustMarshal(&auctionPeriod)
	store.Set(key, bz)

}

// GetAuctionPeriod returns the current auction period, if any has been stored yet
func (k Keeper) GetAuctionPeriod(ctx sdk.Context) *types.AuctionPeriod {
	store := ctx.KVStore(k.storeKey)
	key := []byte(types.KeyAuctionPeriod)

	bz := store.Get(key)
	if len(bz) == 0 {
		return nil
	}

	var auctionPeriod types.AuctionPeriod
	k.cdc.MustUnmarshal(bz, &auctionPeriod)

	return &auctionPeriod
}

// CreateNewAuctionPeriod creates an auction period starting on the next block according to the module params,
// and then creates auctions for the new period
func (k Keeper) CreateNewAuctionPeriod(ctx sdk.Context) (types.AuctionPeriod, error) {
	params := k.GetParams(ctx)
	startBlock := uint64(ctx.BlockHeight()) + 1
	auctionPeriod := k.initializeAuctionPeriodFromParams(startBlock, params)
	err := k.UpdateAuctionPeriod(ctx, auctionPeriod)
	if err != nil {
		return types.AuctionPeriod{}, sdkerrors.Wrapf(err, "unable to create new auction period with start height %d and end height %d", auctionPeriod.StartBlockHeight, auctionPeriod.EndBlockHeight)
	}

	return auctionPeriod, nil
}

func (k Keeper) initializeAuctionPeriodFromParams(startBlock uint64, params types.Params) types.AuctionPeriod {
	endBlock := startBlock + params.AuctionLength

	return types.AuctionPeriod{
		StartBlockHeight: startBlock,
		EndBlockHeight:   endBlock,
	}
}

// CreateAuctionsForActivePeriod will iterate through all acceptable community pool balances and store auctions for them
// Returns an error if the module is disabled, an active period is detected, or on failure
func (k Keeper) CreateAuctionsForAuctionPeriod(ctx sdk.Context) error {
	params := k.GetParams(ctx)
	var foundAuctions = false
	k.IterateAuctions(ctx, func(_ []byte, _ types.Auction) (stop bool) {
		foundAuctions = true
		return true
	})
	// Auctions should have been deleted after they were closed
	if foundAuctions {
		return sdkerrors.Wrapf(types.ErrInvalidAuction, "attempted to create auctions without removing old auctions from store")
	}
	period := k.GetAuctionPeriod(ctx)
	nextBlock := uint64(ctx.BlockHeight() + 1)
	// The only valid call is when a new period begins the next block, and ends in the future
	if period.StartBlockHeight != nextBlock || period.EndBlockHeight <= nextBlock {
		return sdkerrors.Wrapf(types.ErrInvalidAuction, "attempted to create auctions in the middle of an active period")
	}

	auctionBlacklist := params.NonAuctionableTokens
	blacklistMap := listToMap(auctionBlacklist)
	communityPool := k.DistKeeper.GetFeePoolCommunityCoins(ctx)

	// For all elligible coins, remove them from the community pool and create an auction for them
	for _, poolCoin := range communityPool {
		if poolCoin.Amount.TruncateInt().LT(sdk.OneInt()) {
			continue // Skip coins which are only fractional
		}
		if blacklistMap[poolCoin.Denom] { // Coin in blacklist
			continue // Do nothing with that coin
		}

		coin := sdk.NewCoin(poolCoin.Denom, poolCoin.Amount.TruncateInt())
		id := k.GetNextAuctionId(ctx)
		auction := types.NewAuction(id, coin)

		if err := k.RemoveFromCommunityPool(ctx, coin); err != nil {
			return sdkerrors.Wrapf(err, "unable to take auction amount out of pool")
		}
		if err := k.StoreAuction(ctx, auction); err != nil {
			return sdkerrors.Wrapf(err, "unable to store auction")
		}

		ctx.EventManager().EmitEvent(types.NewEventAuction(id, coin.Denom, coin.Amount))
	}

	return nil
}

// Constructs a quick-access map to check if the list contains an input string
// using this, checking that list contains "abc" is equivalent to map["abc"] == true
func listToMap(list []string) map[string]bool {
	retMap := make(map[string]bool, len(list))
	for _, v := range list {
		retMap[v] = true
	}

	return retMap
}
