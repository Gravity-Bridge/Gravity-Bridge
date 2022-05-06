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

	evmChainsFromTestEnv := EvmChains
	newEvmChains := addEvmChainsToStore(t, ctx, k)
	evmChainsFromStore := k.GetEvmChains(ctx)
	evmChainsFromGet := []types.EvmChain{}

	// Check EVM chains from test environment
	for _, cp := range evmChainsFromTestEnv {
		chain := k.GetEvmChainData(ctx, cp.EvmChainPrefix)
		evmChainsFromGet = append(evmChainsFromGet, *chain)
		require.Equal(t, cp.EvmChainPrefix, chain.EvmChainPrefix)
		require.Equal(t, cp.EvmChainName, chain.EvmChainName)
	}

	// Check newly added EVM chains
	for _, cp := range newEvmChains {
		chain := k.GetEvmChainData(ctx, cp.EvmChainPrefix)
		evmChainsFromGet = append(evmChainsFromGet, *chain)
		require.Equal(t, cp.EvmChainPrefix, chain.EvmChainPrefix)
		require.Equal(t, cp.EvmChainName, chain.EvmChainName)
	}

	// Check if GetEvmChains matches
	require.ElementsMatch(t, evmChainsFromGet, evmChainsFromStore)
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
