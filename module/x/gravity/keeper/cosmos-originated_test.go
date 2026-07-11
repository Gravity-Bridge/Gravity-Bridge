package keeper

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// TestClassify_EthOriginatedBankMetadataErrors verifies that ClassifyERC20 and ClassifyDenom
// both return an error if an eth-originated denom (gravity0x or gravity20x) is ever found to have
// bank metadata set. x/gravity never sets metadata for eth-originated assets, so its presence
// indicates a serious inconsistency and all four call sites (ClassifyERC20/ClassifyDenom x
// gravity/gravity2) must refuse to proceed rather than silently returning a bad result.
// nolint: exhaustruct
func TestClassify_EthOriginatedBankMetadataErrors(t *testing.T) {
	// Non-remapped ERC20: gravity0x denom
	//nolint: goconst
	nonRemappedContract := "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
	nonRemappedAddr, err := types.NewEthAddress(nonRemappedContract)
	require.NoError(t, err)
	gravityDenom := types.GravityDenom(*nonRemappedAddr)

	// Remapped ERC20: gravity20x denom
	remappedContract := "0xF815240800ddf3E0be80e0d848B13ecaa504BF37"
	remappedAddr, err := types.NewEthAddress(remappedContract)
	require.NoError(t, err)
	gravity2Denom := types.Gravity2Denom(*remappedAddr)

	t.Run("ClassifyERC20 errors for gravity denom with bank metadata", func(t *testing.T) {
		input, ctx := SetupFiveValChain(t)
		gk := input.GravityKeeper
		input.BankKeeper.SetDenomMetaData(ctx, minMeta(gravityDenom))

		origin, err := gk.ClassifyERC20(ctx, *nonRemappedAddr)
		require.Nil(t, origin, "ClassifyERC20 must not return an AssetOrigin when a gravity denom has bank metadata set")
		require.ErrorIs(t, err, types.ErrInvalid, "ClassifyERC20 must return ErrInvalid when a gravity denom has bank metadata set")
		require.ErrorContains(t, err,
			fmt.Sprintf("ClassifyERC20: Eth-originated gravity denom %s has bank metadata %s, which is not allowed", gravityDenom, gravityDenom))
	})

	t.Run("ClassifyERC20 errors for gravity2 denom with bank metadata", func(t *testing.T) {
		input, ctx := SetupFiveValChain(t)
		gk := input.GravityKeeper
		gk.SetRemappedERC20(ctx, *remappedAddr)
		input.BankKeeper.SetDenomMetaData(ctx, minMeta(gravity2Denom))

		origin, err := gk.ClassifyERC20(ctx, *remappedAddr)
		require.Nil(t, origin, "ClassifyERC20 must not return an AssetOrigin when a gravity2 denom has bank metadata set")
		require.ErrorIs(t, err, types.ErrInvalid, "ClassifyERC20 must return ErrInvalid when a gravity2 denom has bank metadata set")
		require.ErrorContains(t, err,
			fmt.Sprintf("ClassifyERC20: Eth-originated gravity2 denom %s has bank metadata %s, which is not allowed", gravity2Denom, gravity2Denom))
	})

	t.Run("ClassifyDenom errors for gravity denom with bank metadata", func(t *testing.T) {
		input, ctx := SetupFiveValChain(t)
		gk := input.GravityKeeper
		input.BankKeeper.SetDenomMetaData(ctx, minMeta(gravityDenom))

		origin, err := gk.ClassifyDenom(ctx, gravityDenom)
		require.Nil(t, origin, "ClassifyDenom must not return an AssetOrigin when a gravity denom has bank metadata set")
		require.ErrorIs(t, err, types.ErrInvalid, "ClassifyDenom must return ErrInvalid when a gravity denom has bank metadata set")
		require.ErrorContains(t, err,
			fmt.Sprintf("ClassifyDenom: Eth-originated gravity denom %s has bank metadata %s, which is not allowed", gravityDenom, gravityDenom))
	})

	t.Run("ClassifyDenom errors for gravity2 denom with bank metadata", func(t *testing.T) {
		input, ctx := SetupFiveValChain(t)
		gk := input.GravityKeeper
		gk.SetRemappedERC20(ctx, *remappedAddr)
		input.BankKeeper.SetDenomMetaData(ctx, minMeta(gravity2Denom))

		origin, err := gk.ClassifyDenom(ctx, gravity2Denom)
		require.Nil(t, origin, "ClassifyDenom must not return an AssetOrigin when a gravity2 denom has bank metadata set")
		require.ErrorIs(t, err, types.ErrInvalid, "ClassifyDenom must return ErrInvalid when a gravity2 denom has bank metadata set")
		require.ErrorContains(t, err,
			fmt.Sprintf("ClassifyDenom: Eth-originated gravity2 denom %s has bank metadata %s, which is not allowed", gravity2Denom, gravity2Denom))
	})
}

// TestClassifyRoundTrip verifies that ClassifyERC20 and ClassifyDenom agree with each other
// across all three asset classes (cosmos-originated, eth-originated, eth-originated remapped).
// Because both entry points now delegate to the shared classifyCosmosOriginated /
// classifyEthOriginated helpers, classifying an ERC20 and then re-classifying the resulting denom
// (and vice versa) must always round-trip back to the same asset.
// nolint: exhaustruct
func TestClassifyRoundTrip(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()
	gk := input.GravityKeeper

	// Cosmos-originated: a registered denom<->ERC20 mapping
	cosmosERC20, err := types.NewEthAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
	require.NoError(t, err)
	require.NoError(t, gk.setCosmosOriginatedMapping(ctx, "footoken", *cosmosERC20))

	// Eth-originated (not remapped)
	ethERC20, err := types.NewEthAddress("0xdAC17F958D2ee523a2206206994597C13D831ec7")
	require.NoError(t, err)

	// Eth-originated remapped
	remappedERC20, err := types.NewEthAddress("0xF815240800ddf3E0be80e0d848B13ecaa504BF37")
	require.NoError(t, err)
	gk.SetRemappedERC20(ctx, *remappedERC20)

	cases := []struct {
		name       string
		erc20      types.EthAddress
		wantOrigin types.AssetOriginChain
		wantRemap  bool
	}{
		{"cosmos-originated", *cosmosERC20, types.AssetOriginCosmos, false},
		{"eth-originated", *ethERC20, types.AssetOriginEthereum, false},
		{"eth-originated remapped", *remappedERC20, types.AssetOriginEthereum, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// ERC20 -> origin
			fromERC20, err := gk.ClassifyERC20(ctx, tc.erc20)
			require.NoError(t, err)
			require.Equal(t, tc.wantOrigin, fromERC20.Origin)
			require.Equal(t, tc.wantRemap, fromERC20.IsRemapped)
			require.Equal(t, tc.erc20.GetAddress(), fromERC20.ERC20.GetAddress())

			// denom (from ERC20 classification) -> origin, must match
			fromDenom, err := gk.ClassifyDenom(ctx, fromERC20.Denom)
			require.NoError(t, err)
			require.Equal(t, fromERC20.Origin, fromDenom.Origin)
			require.Equal(t, fromERC20.IsRemapped, fromDenom.IsRemapped)
			require.Equal(t, fromERC20.Denom, fromDenom.Denom)
			require.Equal(t, fromERC20.ERC20.GetAddress(), fromDenom.ERC20.GetAddress())

			// and back to ERC20 again, closing the loop
			reERC20, err := gk.ClassifyERC20(ctx, *fromDenom.ERC20)
			require.NoError(t, err)
			require.Equal(t, fromERC20.Denom, reERC20.Denom)
			require.Equal(t, fromERC20.Origin, reERC20.Origin)
			require.Equal(t, fromERC20.IsRemapped, reERC20.IsRemapped)
		})
	}
}

// TestValidateCosmosOriginatedMapping_Negative exercises every rejection branch of the shared
// validateCosmosOriginatedMapping predicate directly, plus the happy path.
// nolint: exhaustruct
func TestValidateCosmosOriginatedMapping_Negative(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context
	k := input.GravityKeeper

	erc20, err := types.NewEthAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
	require.NoError(t, err)

	remapped, err := types.NewEthAddress("0xF815240800ddf3E0be80e0d848B13ecaa504BF37")
	require.NoError(t, err)
	k.SetRemappedERC20(ctx, *remapped)

	t.Run("valid mapping passes", func(t *testing.T) {
		require.NoError(t, k.validateCosmosOriginatedMapping(ctx, "footoken", *erc20))
	})

	t.Run("gravity-prefixed denom is rejected", func(t *testing.T) {
		err := k.validateCosmosOriginatedMapping(ctx, types.GravityDenom(*erc20), *erc20)
		require.ErrorIs(t, err, types.ErrInvalid)
		require.Contains(t, err.Error(), "collides with an eth-originated gravity denom")
	})

	t.Run("gravity2-prefixed denom is rejected", func(t *testing.T) {
		err := k.validateCosmosOriginatedMapping(ctx, types.Gravity2Denom(*erc20), *erc20)
		require.ErrorIs(t, err, types.ErrInvalid)
		require.Contains(t, err.Error(), "collides with an eth-originated gravity2 denom")
	})

	t.Run("denom with an embedded eth address is rejected", func(t *testing.T) {
		denom := "footoken" + erc20.GetAddress().Hex()
		err := k.validateCosmosOriginatedMapping(ctx, denom, *erc20)
		require.ErrorIs(t, err, types.ErrInvalid)
		require.Contains(t, err.Error(), "contains an embedded Ethereum address")
	})

	t.Run("ERC20 already in the remapped set is rejected", func(t *testing.T) {
		err := k.validateCosmosOriginatedMapping(ctx, "footoken", *remapped)
		require.ErrorIs(t, err, types.ErrInvalid)
		require.Contains(t, err.Error(), "is also in the remapped eth-originated set")
	})
}

// TestSetCosmosOriginatedMapping_Duplicates verifies the write-path duplicate prevention that
// setCosmosOriginatedMapping layers on top of the shared predicate.
// nolint: exhaustruct
func TestSetCosmosOriginatedMapping_Duplicates(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context
	k := input.GravityKeeper

	erc20A, err := types.NewEthAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
	require.NoError(t, err)
	erc20B, err := types.NewEthAddress("0xdAC17F958D2ee523a2206206994597C13D831ec7")
	require.NoError(t, err)

	require.NoError(t, k.setCosmosOriginatedMapping(ctx, "footoken", *erc20A))

	t.Run("same denom mapped to a different ERC20 is rejected", func(t *testing.T) {
		err := k.setCosmosOriginatedMapping(ctx, "footoken", *erc20B)
		require.ErrorIs(t, err, types.ErrDuplicate)
		require.Contains(t, err.Error(), "is already mapped to cosmos-originated ERC20")
	})

	t.Run("same ERC20 mapped to a different denom is rejected", func(t *testing.T) {
		err := k.setCosmosOriginatedMapping(ctx, "bartoken", *erc20A)
		require.ErrorIs(t, err, types.ErrDuplicate)
		require.Contains(t, err.Error(), "is already mapped to cosmos-originated denom")
	})
}

// TestClassifyDenom_Negative covers the ClassifyDenom rejection branches that are not tied to
// bank metadata: unknown denoms and gravity0x denoms whose ERC20 has been remapped.
// nolint: exhaustruct
func TestClassifyDenom_Negative(t *testing.T) {
	t.Run("unknown denom returns ErrInvalidDenom", func(t *testing.T) {
		input := CreateTestEnv(t)
		ctx := input.Context
		origin, err := input.GravityKeeper.ClassifyDenom(ctx, "footoken")
		require.Nil(t, origin)
		require.ErrorIs(t, err, types.ErrInvalidDenom)
		require.Contains(t, err.Error(), "not registered as a known bridged asset")
	})

	t.Run("gravity0x denom of a remapped ERC20 is rejected", func(t *testing.T) {
		input, ctx := SetupFiveValChain(t)
		// remapped-only state is left in the store, so we do not assert invariants here
		k := input.GravityKeeper
		erc20, err := types.NewEthAddress("0xF815240800ddf3E0be80e0d848B13ecaa504BF37")
		require.NoError(t, err)
		k.SetRemappedERC20(ctx, *erc20)

		origin, err := k.ClassifyDenom(ctx, types.GravityDenom(*erc20))
		require.Nil(t, origin)
		require.ErrorIs(t, err, types.ErrInvalid)
		require.Contains(t, err.Error(), "was remapped; new deposits use")
	})
}

// TestClassifyCosmosOriginated_CorruptState verifies that both ClassifyERC20 and ClassifyDenom
// reject corrupted cosmos-originated index state: broken bidirectional mappings, an ERC20 that is
// simultaneously remapped, and a stored denom that fails strict validation. These states can only
// arise from a bug or manual tampering, so the classifiers must refuse to produce a result.
// nolint: exhaustruct
func TestClassifyCosmosOriginated_CorruptState(t *testing.T) {
	erc20, err := types.NewEthAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
	require.NoError(t, err)

	t.Run("ClassifyDenom: forward entry with no reverse entry", func(t *testing.T) {
		input, ctx := SetupFiveValChain(t)
		k := input.GravityKeeper
		// Write only the denom->ERC20 direction, leaving the reverse missing.
		store := ctx.KVStore(k.storeKey)
		store.Set(types.GetDenomToERC20Key("footoken"), erc20.GetAddress().Bytes())

		origin, err := k.ClassifyDenom(ctx, "footoken")
		require.Nil(t, origin)
		require.ErrorIs(t, err, types.ErrInvalid)
		require.Contains(t, err.Error(), "not bidirectionally consistent")
	})

	t.Run("ClassifyERC20: reverse entry with no forward entry", func(t *testing.T) {
		input, ctx := SetupFiveValChain(t)
		k := input.GravityKeeper
		// Write only the ERC20->denom direction, leaving the forward missing.
		store := ctx.KVStore(k.storeKey)
		store.Set(types.GetERC20ToDenomKey(*erc20), []byte("footoken"))

		origin, err := k.ClassifyERC20(ctx, *erc20)
		require.Nil(t, origin)
		require.ErrorIs(t, err, types.ErrInvalid)
		require.Contains(t, err.Error(), "not bidirectionally consistent")
	})

	t.Run("ClassifyDenom: forward and reverse point at different partners", func(t *testing.T) {
		input, ctx := SetupFiveValChain(t)
		k := input.GravityKeeper
		store := ctx.KVStore(k.storeKey)
		// footoken -> erc20, but erc20 -> "bartoken" (mismatched reverse)
		store.Set(types.GetDenomToERC20Key("footoken"), erc20.GetAddress().Bytes())
		store.Set(types.GetERC20ToDenomKey(*erc20), []byte("bartoken"))

		origin, err := k.ClassifyDenom(ctx, "footoken")
		require.Nil(t, origin)
		require.ErrorIs(t, err, types.ErrInvalid)
		require.Contains(t, err.Error(), "not bidirectionally consistent")
	})

	t.Run("ClassifyERC20: cosmos-originated ERC20 also in the remapped set", func(t *testing.T) {
		input, ctx := SetupFiveValChain(t)
		k := input.GravityKeeper
		require.NoError(t, k.setCosmosOriginatedMapping(ctx, "footoken", *erc20))
		// Corrupt the state: mark the same ERC20 as remapped.
		k.SetRemappedERC20(ctx, *erc20)

		origin, err := k.ClassifyERC20(ctx, *erc20)
		require.Nil(t, origin)
		require.ErrorIs(t, err, types.ErrInvalid)
		require.Contains(t, err.Error(), "is also in the remapped eth-originated set")

		denomOrigin, denomErr := k.ClassifyDenom(ctx, "footoken")
		require.Nil(t, denomOrigin)
		require.ErrorIs(t, denomErr, types.ErrInvalid)
		require.Contains(t, denomErr.Error(), "is also in the remapped eth-originated set")
	})

	t.Run("ClassifyDenom: stored denom fails strict validation", func(t *testing.T) {
		input, ctx := SetupFiveValChain(t)
		k := input.GravityKeeper
		// A backslash is forbidden by ValidateStrictDenom but is not an embedded eth address,
		// so it can only be reached via the cosmos-originated index lookup.
		badDenom := `foo\bar`
		setCosmosOriginatedMappingUnchecked(ctx, k, badDenom, *erc20)

		origin, err := k.ClassifyDenom(ctx, badDenom)
		require.Nil(t, origin)
		require.ErrorIs(t, err, types.ErrInvalidDenom)
		require.Contains(t, err.Error(), "is invalid")
	})

	t.Run("ClassifyERC20: stored denom fails strict validation", func(t *testing.T) {
		input, ctx := SetupFiveValChain(t)
		k := input.GravityKeeper
		// Same corruption reached from the ERC20 side: the reverse index resolves the ERC20 to a
		// strictly-invalid denom, so ClassifyERC20 must reject it in classifyCosmosOriginated.
		badDenom := `foo\bar`
		setCosmosOriginatedMappingUnchecked(ctx, k, badDenom, *erc20)

		origin, err := k.ClassifyERC20(ctx, *erc20)
		require.Nil(t, origin)
		require.ErrorIs(t, err, types.ErrInvalidDenom)
		require.Contains(t, err.Error(), "ClassifyERC20: the denom")
		require.Contains(t, err.Error(), "is invalid")
	})
}

// TestClassifyEthOriginated_RoundTripMismatch forces the eth-originated round-trip guard in the
// shared classifyEthOriginated helper. This branch is unreachable through ClassifyERC20 /
// ClassifyDenom, which always derive or parse a denom that is consistent with the ERC20, so the
// helper is invoked directly with deliberately mismatched inputs to prove the defensive check
// fires rather than returning a bogus AssetOrigin.
// nolint: exhaustruct
func TestClassifyEthOriginated_RoundTripMismatch(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context
	k := input.GravityKeeper

	erc20A, err := types.NewEthAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
	require.NoError(t, err)
	erc20B, err := types.NewEthAddress("0xdAC17F958D2ee523a2206206994597C13D831ec7")
	require.NoError(t, err)

	t.Run("denom resolves to a different ERC20 than claimed", func(t *testing.T) {
		// gravity denom for erc20A, but we claim it belongs to erc20B
		origin, err := k.classifyEthOriginated(ctx, "ClassifyTest", types.GravityDenom(*erc20A), *erc20B, false)
		require.Nil(t, origin)
		require.ErrorIs(t, err, types.ErrInvalid)
		require.Contains(t, err.Error(), "failed round-trip validation")
	})

	t.Run("denom is not parseable under the selected namespace", func(t *testing.T) {
		// a gravity2 denom passed with isRemapped=false will not parse as a gravity denom
		origin, err := k.classifyEthOriginated(ctx, "ClassifyTest", types.Gravity2Denom(*erc20A), *erc20A, false)
		require.Nil(t, origin)
		require.ErrorIs(t, err, types.ErrInvalid)
		require.Contains(t, err.Error(), "failed round-trip validation")
	})

	t.Run("remapped namespace: denom resolves to a different ERC20 than claimed", func(t *testing.T) {
		origin, err := k.classifyEthOriginated(ctx, "ClassifyTest", types.Gravity2Denom(*erc20A), *erc20B, true)
		require.Nil(t, origin)
		require.ErrorIs(t, err, types.ErrInvalid)
		require.Contains(t, err.Error(), "failed round-trip validation")
	})
}
