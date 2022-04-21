package keeper

import (
	"testing"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestGetSetEvmChainData(t *testing.T) {
	input := CreateTestEnv(t)
	k := input.GravityKeeper
	ctx := input.Context

	evmChainsData := addEvmChainsToStore(t, ctx, k)

	for _, evmChainData := range evmChainsData {
		evmChainDataFromStore := k.GetEvmChainData(ctx, evmChainData.EvmChainPrefix)
		require.Equal(t, evmChainData.EvmChainPrefix, evmChainDataFromStore.EvmChainPrefix)
		require.Equal(t, evmChainData.EvmChainName, evmChainDataFromStore.EvmChainName)
	}
}

func TestIterateEvmChainsData(t *testing.T) {
	input := CreateTestEnv(t)
	k := input.GravityKeeper
	ctx := input.Context

	newEvmChains := addEvmChainsToStore(t, ctx, k)
	evmChainsFromTestEnv := EvmChains
	evmChainsFromStore := k.GetEvmChains(ctx)

	// Check EVM chains from test environment
	for i, cp := range evmChainsFromTestEnv {
		require.Equal(t, cp.EvmChainPrefix, evmChainsFromStore[i].EvmChainPrefix)
		require.Equal(t, cp.EvmChainName, evmChainsFromStore[i].EvmChainName)
	}

	// Check newly added EVM chains
	for i, cp := range evmChainsFromStore[len(evmChainsFromTestEnv):] {
		require.Equal(t, newEvmChains[i].EvmChainPrefix, cp.EvmChainPrefix)
		require.Equal(t, newEvmChains[i].EvmChainName, cp.EvmChainName)
	}
}

func addEvmChainsToStore(t *testing.T, ctx sdk.Context, k Keeper) []types.EvmChain {
	evmChainsData := []types.EvmChain{
		{
			EvmChainPrefix: "prefix1",
			EvmChainName:   "EVM Chain Name 1",
		},
		{
			EvmChainPrefix: "prefix2",
			EvmChainName:   "EVM Chain Name 2",
		},
	}

	for _, evmChainData := range evmChainsData {
		require.NotPanics(t, func() { k.SetEvmChainData(ctx, evmChainData) })
	}

	return evmChainsData
}
