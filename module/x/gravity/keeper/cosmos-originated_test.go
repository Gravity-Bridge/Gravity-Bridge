package keeper

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// TestClassify_EthOriginatedBankMetadataPanics verifies that ClassifyERC20 and ClassifyDenom
// both panic if an eth-originated denom (gravity0x or gravity20x) is ever found to have bank
// metadata set. x/gravity never sets metadata for eth-originated assets, so its presence
// indicates a serious inconsistency and all four call sites (ClassifyERC20/ClassifyDenom x
// gravity/gravity2) must refuse to proceed rather than silently returning a bad result.
// nolint: exhaustruct
func TestClassify_EthOriginatedBankMetadataPanics(t *testing.T) {
	// Non-remapped ERC20: gravity0x denom
	nonRemappedContract := "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
	nonRemappedAddr, err := types.NewEthAddress(nonRemappedContract)
	require.NoError(t, err)
	gravityDenom := types.GravityDenom(*nonRemappedAddr)

	// Remapped ERC20: gravity20x denom
	remappedContract := "0xF815240800ddf3E0be80e0d848B13ecaa504BF37"
	remappedAddr, err := types.NewEthAddress(remappedContract)
	require.NoError(t, err)
	gravity2Denom := types.Gravity2Denom(*remappedAddr)

	t.Run("ClassifyERC20 panics for gravity denom with bank metadata", func(t *testing.T) {
		input, ctx := SetupFiveValChain(t)
		gk := input.GravityKeeper
		input.BankKeeper.SetDenomMetaData(ctx, minMeta(gravityDenom))

		require.PanicsWithValue(t,
			fmt.Sprintf("ClassifyERC20: Eth-originated gravity denom %s has bank metadata %s, which is not allowed", gravityDenom, gravityDenom),
			func() { gk.ClassifyERC20(ctx, *nonRemappedAddr) },
			"ClassifyERC20 must panic when a gravity denom has bank metadata set",
		)
	})

	t.Run("ClassifyERC20 panics for gravity2 denom with bank metadata", func(t *testing.T) {
		input, ctx := SetupFiveValChain(t)
		gk := input.GravityKeeper
		gk.SetRemappedERC20(ctx, *remappedAddr)
		input.BankKeeper.SetDenomMetaData(ctx, minMeta(gravity2Denom))

		require.PanicsWithValue(t,
			fmt.Sprintf("ClassifyERC20: Eth-originated gravity2 denom %s has bank metadata %s, which is not allowed", gravity2Denom, gravity2Denom),
			func() { gk.ClassifyERC20(ctx, *remappedAddr) },
			"ClassifyERC20 must panic when a gravity2 denom has bank metadata set",
		)
	})

	t.Run("ClassifyDenom panics for gravity denom with bank metadata", func(t *testing.T) {
		input, ctx := SetupFiveValChain(t)
		gk := input.GravityKeeper
		input.BankKeeper.SetDenomMetaData(ctx, minMeta(gravityDenom))

		require.PanicsWithValue(t,
			fmt.Sprintf("ClassifyDenom: Eth-originated gravity denom %s has bank metadata %s, which is not allowed", gravityDenom, gravityDenom),
			func() { gk.ClassifyDenom(ctx, gravityDenom) },
			"ClassifyDenom must panic when a gravity denom has bank metadata set",
		)
	})

	t.Run("ClassifyDenom panics for gravity2 denom with bank metadata", func(t *testing.T) {
		input, ctx := SetupFiveValChain(t)
		gk := input.GravityKeeper
		gk.SetRemappedERC20(ctx, *remappedAddr)
		input.BankKeeper.SetDenomMetaData(ctx, minMeta(gravity2Denom))

		require.PanicsWithValue(t,
			fmt.Sprintf("ClassifyDenom: Eth-originated gravity2 denom %s has bank metadata %s, which is not allowed", gravity2Denom, gravity2Denom),
			func() { gk.ClassifyDenom(ctx, gravity2Denom) },
			"ClassifyDenom must panic when a gravity2 denom has bank metadata set",
		)
	})
}
