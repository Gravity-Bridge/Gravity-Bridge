package keeper

import (
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SetEvmChainData sets the EVM chain specific data
// Check if chains exists before calling this method
func (k Keeper) SetEvmChainData(ctx sdk.Context, evmChain types.EvmChain) {
	key := types.GetEvmChainKey(evmChain.EvmChainPrefix)
	ctx.KVStore(k.storeKey).Set(key, k.cdc.MustMarshal(&evmChain))
}

// GetEvmChainData returns data for the specific EVM chain
func (k Keeper) GetEvmChainData(ctx sdk.Context, evmChainPrefix string) *types.EvmChain {
	key := types.GetEvmChainKey(evmChainPrefix)
	store := ctx.KVStore(k.storeKey)

	bytes := store.Get(key)
	if bytes == nil {
		return nil
	}

	var evmChainData types.EvmChain
	k.cdc.MustUnmarshal(bytes, &evmChainData)
	return &evmChainData
}

func (k Keeper) GetEvmChains(ctx sdk.Context) []types.EvmChain {
	store := ctx.KVStore(k.storeKey)
	prefix := types.EvmChainKey
	iter := store.Iterator(prefixRange(prefix))
	defer iter.Close()

	var evmChains []types.EvmChain

	for ; iter.Valid(); iter.Next() {
		value := iter.Value()
		var evmChainData types.EvmChain
		k.cdc.MustUnmarshal(value, &evmChainData)

		evmChains = append(evmChains, evmChainData)
	}

	return evmChains
}

func (k Keeper) IterateEvmChains(ctx sdk.Context, cb func(key []byte, evmChain *types.EvmChain) (stop bool)) {
	store := ctx.KVStore(k.storeKey)
	prefix := types.EvmChainKey
	iter := store.Iterator(prefixRange(prefix))
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		evmChain := new(types.EvmChain)
		value := iter.Value()
		k.cdc.MustUnmarshal(value, evmChain)
		if cb(iter.Key(), evmChain) {
			break
		}
	}
}

func (k Keeper) GetEvmChainsWithLimit(ctx sdk.Context, limit uint64) []types.EvmChain {
	evmChains := []types.EvmChain{}

	k.IterateEvmChains(ctx, func(key []byte, evmChain *types.EvmChain) (stop bool) {
		evmChains = append(evmChains, *evmChain)
		if limit != 0 && uint64(len(evmChains)) >= limit {
			return true
		}
		return false
	})

	return evmChains
}
