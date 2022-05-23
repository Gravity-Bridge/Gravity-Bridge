package keeper

import (
	"fmt"
	"testing"

	_ "github.com/Gravity-Bridge/Gravity-Bridge/module/config"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	ValsetCount      = 200
	LastSlashedNonce = 10
)

func TestValsets(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	k := input.GravityKeeper

	for _, cd := range k.GetEvmChains(ctx) {
		// verify that no valsets exist in the begining
		assert.Equal(t, 0, len(k.GetValsets(ctx, cd.EvmChainPrefix)))

		valset, err := k.GetCurrentValset(ctx, cd.EvmChainPrefix)
		require.NoError(t, err)

		// insert N valsets into the store
		for i := 1; i <= ValsetCount; i++ {
			valset.Height = uint64(i)
			valset.Nonce = uint64(i)
			k.StoreValset(ctx, cd.EvmChainPrefix, valset)
			k.SetLatestValsetNonce(ctx, cd.EvmChainPrefix, valset.Nonce)
		}

		// verify that N valsets exist in the store
		valsets := k.GetValsets(ctx, cd.EvmChainPrefix)
		assert.Equal(t, ValsetCount, len(valsets))

		// verify that valset for each inserterd nonce can be read from the store
		for i := 1; i <= ValsetCount; i++ {
			require.Equal(t, true, k.HasValsetRequest(ctx, cd.EvmChainPrefix, uint64(i)))
			require.NotNil(t, k.GetValset(ctx, cd.EvmChainPrefix, uint64(i)))
		}

		// verify that latest valset nonce and latest valset are as expected
		require.Equal(t, uint64(ValsetCount), k.GetLatestValsetNonce(ctx, cd.EvmChainPrefix))
		require.Equal(t, uint64(ValsetCount), k.GetLatestValset(ctx, cd.EvmChainPrefix).Nonce)

		// verify that non exisiting valset retrieval is handled properly
		require.Nil(t, k.GetValset(ctx, cd.EvmChainPrefix, uint64(ValsetCount+1)))

		// verify that panic is triggered when tring to store valset with the same nonce
		valset.Nonce = uint64(ValsetCount)
		require.Panics(t, func() { k.StoreValset(ctx, cd.EvmChainPrefix, valset) })

		// verify that last slashed valset nonce is stored/loaded as expected
		require.Equal(t, uint64(0), k.GetLastSlashedValsetNonce(ctx, cd.EvmChainPrefix))
		k.SetLastSlashedValsetNonce(ctx, cd.EvmChainPrefix, LastSlashedNonce)
		require.Equal(t, uint64(LastSlashedNonce), k.GetLastSlashedValsetNonce(ctx, cd.EvmChainPrefix))

		ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 100)

		// verify that only valsets with higher nonce than LastSlashedValsetNonce are returned by GetUnSlashedValsets()
		unslashedValsets := k.GetUnSlashedValsets(ctx, cd.EvmChainPrefix, 10)
		for _, vs := range unslashedValsets {
			require.Greater(t, vs.Nonce, uint64(LastSlashedNonce),
				fmt.Sprintf("got valset with nonce: %d, but expected only valsets with nonce higher than %d", vs.Nonce, LastSlashedNonce))
		}

		// create valset confirmations for each valset and store them
		for i := 1; i <= ValsetCount; i++ {
			for j, orch := range OrchAddrs {
				ethAddr, err := types.NewEthAddress(EthAddrs[j].String())
				require.NoError(t, err)

				conf := types.NewMsgValsetConfirm(uint64(i), *ethAddr, orch, "dummysig")
				k.SetValsetConfirm(ctx, cd.EvmChainPrefix, *conf)

				// verify that valset confirm was stored successfully
				require.NotNil(t, k.GetValsetConfirm(ctx, cd.EvmChainPrefix, conf.Nonce, orch))
			}
		}

		// verify that GetValsetConfirms() returns expected number of confirmations
		for i := 1; i <= ValsetCount; i++ {
			confirms := k.GetValsetConfirms(ctx, cd.EvmChainPrefix, uint64(i))
			require.Equal(t, len(OrchAddrs), len(confirms))
		}

		// delete all valsets and their confirmations
		for i := 1; i <= ValsetCount; i++ {
			k.DeleteValset(ctx, cd.EvmChainPrefix, uint64(i))
			k.DeleteValsetConfirms(ctx, cd.EvmChainPrefix, uint64(i))
		}

		// verify that no valset and confirmations exist in the store
		require.Equal(t, 0, len(k.GetValsets(ctx, cd.EvmChainPrefix)))
		for i := 1; i <= ValsetCount; i++ {
			require.Equal(t, 0, len(k.GetValsetConfirms(ctx, cd.EvmChainPrefix, uint64(i))))
		}
	}
}
