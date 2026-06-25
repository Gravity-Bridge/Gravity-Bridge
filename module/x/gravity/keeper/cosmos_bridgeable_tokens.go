package keeper

import (
	"fmt"

	"cosmossdk.io/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// SetCosmosBridgeableToken writes the given metadata into the CosmosBridgeableTokens
// allowlist store, keyed by the metadata's base denom. If an entry already exists for
// this denom it is overwritten.
func (k Keeper) SetCosmosBridgeableToken(ctx sdk.Context, metadata banktypes.Metadata) {
	store := ctx.KVStore(k.storeKey)
	bz := k.cdc.MustMarshal(&metadata)
	store.Set(types.GetCosmosBridgeableTokenKey(metadata.Base), bz)
}

// GetCosmosBridgeableToken returns the CosmosBridgeableTokens allowlist entry for the
// given base denom, and whether it was found.
func (k Keeper) GetCosmosBridgeableToken(ctx sdk.Context, denom string) (banktypes.Metadata, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.GetCosmosBridgeableTokenKey(denom))
	if bz == nil {
		//nolint: exhaustruct
		return banktypes.Metadata{}, false
	}
	var metadata banktypes.Metadata
	k.cdc.MustUnmarshal(bz, &metadata)
	return metadata, true
}

// DeleteCosmosBridgeableToken removes the CosmosBridgeableTokens allowlist entry for the
// given base denom.
func (k Keeper) DeleteCosmosBridgeableToken(ctx sdk.Context, denom string) {
	store := ctx.KVStore(k.storeKey)
	store.Delete(types.GetCosmosBridgeableTokenKey(denom))
}

// IterateCosmosBridgeableTokens calls cb for each entry in the CosmosBridgeableTokens
// allowlist. cb should return true to stop iteration early.
func (k Keeper) IterateCosmosBridgeableTokens(ctx sdk.Context, cb func(metadata banktypes.Metadata) bool) {
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.CosmosBridgeableTokensKey)
	iter := prefixStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var metadata banktypes.Metadata
		if err := k.cdc.Unmarshal(iter.Value(), &metadata); err != nil {
			panic(fmt.Sprintf("invalid Metadata in CosmosBridgeableTokens store under denom %q: %v", string(iter.Key()), err))
		}
		if cb(metadata) {
			break
		}
	}
}

// GetAllCosmosBridgeableTokens returns every entry currently in the CosmosBridgeableTokens
// allowlist.
func (k Keeper) GetAllCosmosBridgeableTokens(ctx sdk.Context) []banktypes.Metadata {
	tokens := []banktypes.Metadata{}
	k.IterateCosmosBridgeableTokens(ctx, func(metadata banktypes.Metadata) bool {
		tokens = append(tokens, metadata)
		return false
	})
	return tokens
}
