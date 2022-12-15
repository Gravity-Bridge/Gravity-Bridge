package keeper

import (
	"fmt"
	"testing"

	"bytes"
	"sort"

	_ "github.com/Gravity-Bridge/Gravity-Bridge/module/config"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
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

func TestIterateValsetConfirms(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	k := input.GravityKeeper

	for _, cd := range k.GetEvmChains(ctx) {
		// verify that no valsets exist in the begining
		assert.Equal(t, 0, len(k.GetValsets(ctx, cd.EvmChainPrefix)))

		valset, err := k.GetCurrentValset(ctx, cd.EvmChainPrefix)
		require.NoError(t, err)

		orchestrators := OrchAddrs
		confirmsByNonce := make(map[uint64][]types.MsgValsetConfirm, 10)

		// insert 10 valsets into the store, sometimes adding confirms
		for i := 1; i <= 10; i++ {
			valset.Height = uint64(i)
			valset.Nonce = uint64(i)
			k.StoreValset(ctx, cd.EvmChainPrefix, valset)
			k.SetLatestValsetNonce(ctx, cd.EvmChainPrefix, valset.Nonce)

			confirmers := orchestrators
			if i%2 == 0 { // No claims for "even-nonce'd" valsets
				continue
			}
			if i == 3 {
				// Only claim with 2 orchestrators
				confirmers = orchestrators[0:2]
			}
			if i == 5 {
				// Create a large gap
				continue
			}

			confirms := createConfirmsForValset(ctx, k, cd.EvmChainPrefix, confirmers, valset)
			confirmsByNonce[valset.Nonce] = append(confirmsByNonce[valset.Nonce], confirms...)
		}

		// verify that N valsets exist in the store
		valsets := k.GetValsets(ctx, cd.EvmChainPrefix)
		assert.Equal(t, 10, len(valsets))

		noncesToAvoid := []uint64{2, 4, 5, 6, 8, 10}
		// verify that confirms appear in the IterateValsetClaims function
		k.IterateValsetConfirms(ctx, func(key []byte, confirms []types.MsgValsetConfirm, nonce uint64) (stop bool) {
			require.False(t, slices.Contains(noncesToAvoid, nonce), "IterateValsetConfirms returned confirms for a nonce which should not exist")

			expectedConfirms := confirmsByNonce[nonce]
			require.Equal(t, len(expectedConfirms), len(confirms))

			sort.Slice(expectedConfirms, func(i, j int) bool {
				return expectedConfirms[i].Orchestrator < expectedConfirms[j].Orchestrator
			})
			sort.Slice(confirms, func(i, j int) bool {
				return confirms[i].Orchestrator < confirms[j].Orchestrator
			})

			for i, expConf := range expectedConfirms {
				require.Equal(t, expConf, confirms[i])
			}

			return false
		})
	}
}

func createConfirmsForValset(ctx sdk.Context, k Keeper, evmChainPrefix string, orchestrators []sdk.AccAddress, valset types.Valset) []types.MsgValsetConfirm {
	// Sort the orchestrators so that checking is easy
	sort.Slice(orchestrators, func(i, j int) bool {
		return bytes.Compare(orchestrators[i].Bytes(), orchestrators[j].Bytes()) < 0
	})

	confirms := make([]types.MsgValsetConfirm, len(orchestrators))
	for i, orch := range orchestrators {
		ethAddr := EthAddrs[i]

		confirm := types.MsgValsetConfirm{
			Nonce:        valset.Nonce,
			Orchestrator: orch.String(),
			EthAddress:   ethAddr.String(),
			Signature:    "d34db33f",
		}

		k.SetValsetConfirm(ctx, evmChainPrefix, confirm)
		confirms[i] = confirm
	}

	return confirms
}
