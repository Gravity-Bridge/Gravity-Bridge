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
		evmChainDataFromStore := k.GetEvmChainData(ctx, evmChainData.Prefix)
		require.Equal(t, evmChainData.Prefix, evmChainDataFromStore.Prefix)
		require.Equal(t, evmChainData.ChainName, evmChainDataFromStore.ChainName)
	}
}

func TestIterateEvmChainsData(t *testing.T) {
	input := CreateTestEnv(t)
	k := input.GravityKeeper
	ctx := input.Context

	evmChainsData := addEvmChainsToStore(t, ctx, k)
	evmChainsDataFromStore := k.GetEvmChains(ctx)

	for i := 0; i < len(evmChainsDataFromStore); i++ {
		require.Equal(t, evmChainsData[i].Prefix, evmChainsDataFromStore[i].Prefix)
		require.Equal(t, evmChainsData[i].ChainName, evmChainsDataFromStore[i].ChainName)
	}
}

func addEvmChainsToStore(t *testing.T, ctx sdk.Context, k Keeper) []types.EVMChainProposal {
	evmChainsData := []types.EVMChainProposal{
		{
			Prefix:    "prefix1",
			ChainName: "EVM Chain Name 1",
		},
		{
			Prefix:    "prefix2",
			ChainName: "EVM Chain Name 2",
		},
		{
			Prefix:    "prefix3",
			ChainName: "EVM Chain Name 3",
		},
	}

	for _, evmChainData := range evmChainsData {
		require.NotPanics(t, func() { k.SetEvmChainData(ctx, evmChainData) })
	}

	return evmChainsData
}
