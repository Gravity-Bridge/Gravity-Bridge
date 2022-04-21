package keeper

import (
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SetEvmChainData sets the EVM chain specific data
func (k Keeper) SetEvmChainData(ctx sdk.Context, evmChain types.EvmChain) {
	key := types.GetEvmChainKey(evmChain.EvmChainPrefix)
	store := ctx.KVStore(k.storeKey)

	if store.Has(key) {
		return
		//panic("EVM chain already in store.")
	}

	store.Set(key, k.cdc.MustMarshal(&evmChain))
}

// GetEvmChainData returns the EVM chain specific data
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
