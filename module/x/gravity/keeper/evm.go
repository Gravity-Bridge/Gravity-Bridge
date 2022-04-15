package keeper

import (
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SetEvmChainData sets the EVM chain specific data
func (k Keeper) SetEvmChainData(ctx sdk.Context, evmChainData types.EVMChainProposal) {
	key := types.GetEvmChainKey(evmChainData.Prefix)
	store := ctx.KVStore(k.storeKey)

	if store.Has(key) {
		panic("EVM chain already in store.")
	}

	store.Set(key, k.cdc.MustMarshal(&evmChainData))
}

// GetEvmChainData returns the EVM chain specific data
func (k Keeper) GetEvmChainData(ctx sdk.Context, evmChainPrefix string) *types.EVMChainProposal {
	key := types.GetEvmChainKey(evmChainPrefix)
	store := ctx.KVStore(k.storeKey)

	bytes := store.Get(key)
	if bytes == nil {
		return nil
	}

	var evmChainData types.EVMChainProposal
	k.cdc.MustUnmarshal(bytes, &evmChainData)
	return &evmChainData
}

func (k Keeper) GetEvmChains(ctx sdk.Context) []types.EVMChainProposal {
	store := ctx.KVStore(k.storeKey)
	prefix := types.EvmChainKey
	iter := store.Iterator(prefixRange(prefix))
	defer iter.Close()

	var evmChains []types.EVMChainProposal

	for ; iter.Valid(); iter.Next() {
		value := iter.Value()
		var evmChainData types.EVMChainProposal
		k.cdc.MustUnmarshal(value, &evmChainData)

		evmChains = append(evmChains, evmChainData)
	}

	return evmChains
}
