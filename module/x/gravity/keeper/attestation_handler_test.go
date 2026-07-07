package keeper

import (
	"strings"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/require"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

var badErc20 = func() *types.EthAddress {
	addr, err := types.NewEthAddress("0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255")
	if err != nil {
		panic(err)
	}
	return addr
}()

// setCosmosOriginatedMappingUnchecked writes a bidirectional denom<->ERC20 mapping directly to
// the store without any validation. Only use this in tests or recovery code where you are
// deliberately simulating corrupted or pre-validation state.
func setCosmosOriginatedMappingUnchecked(ctx sdk.Context, k Keeper, denom string, tokenContract types.EthAddress) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.GetDenomToERC20Key(denom), tokenContract.GetAddress().Bytes())
	store.Set(types.GetERC20ToDenomKey(tokenContract), []byte(denom))
}

func TestHandleSendToCosmos_BadDenom(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	// Intentionally plant a bad ERC20 -> Denom mapping in state to test handler rejection.
	// Invariant assertion is skipped because the corrupted mapping would trigger it.

	setCosmosOriginatedMappingUnchecked(ctx, input.GravityKeeper, "ibc/gravity0xbad", *badErc20)

	attHandler := input.GravityKeeper.AttestationHandler
	//nolint: exhaustruct
	claim := types.MsgSendToCosmosClaim{
		EventNonce:     1,
		EthBlockHeight: 1,
		TokenContract:  badErc20.GetAddress().Hex(),
		Amount:         math.NewInt(1),
		CosmosReceiver: sdk.AccAddress([]byte{1}).String(),
		EthereumSender: "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7",
		Orchestrator:   OrchAddrs[0].String(),
	}

	//nolint: exhaustruct
	err := attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidDenom)
}

func TestHandleBatchSendToEth_BadDenom(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	// Intentionally plant a bad ERC20 -> Denom mapping in state to test handler rejection.
	// Invariant assertion is skipped because the corrupted mapping would trigger it.

	setCosmosOriginatedMappingUnchecked(ctx, input.GravityKeeper, "ibc/gravity0xbad", *badErc20)

	attHandler := input.GravityKeeper.AttestationHandler
	//nolint: exhaustruct
	claim := types.MsgBatchSendToEthClaim{
		TokenContract:  badErc20.GetAddress().Hex(),
		BatchNonce:     1,
		EventNonce:     1,
		EthBlockHeight: 1,
		Orchestrator:   OrchAddrs[0].String(),
	}

	//nolint: exhaustruct
	err := attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidDenom)
}

func TestHandleErc20Deployed_BadDenom(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	attHandler := input.GravityKeeper.AttestationHandler
	//nolint: exhaustruct
	claim := types.MsgERC20DeployedClaim{
		CosmosDenom:    "ibc/gravity0xbad",
		TokenContract:  "0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255",
		EventNonce:     1,
		EthBlockHeight: 1,
		Name:           "test",
		Symbol:         "TST",
		Decimals:       6,
		Orchestrator:   OrchAddrs[0].String(),
	}

	//nolint: exhaustruct
	err := attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "invalid cosmos denom"))
}

func TestHandleValsetUpdated_BadRewardDenom(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	// Intentionally plant a bad ERC20 -> Denom mapping in state to test handler rejection.
	// Invariant assertion is skipped because the corrupted mapping would trigger it.

	setCosmosOriginatedMappingUnchecked(ctx, input.GravityKeeper, "ibc/gravity0xbad", *badErc20)

	attHandler := input.GravityKeeper.AttestationHandler
	//nolint: exhaustruct
	claim := types.MsgValsetUpdatedClaim{
		RewardToken:    badErc20.GetAddress().Hex(),
		RewardAmount:   math.NewInt(1),
		EventNonce:     1,
		ValsetNonce:    0, // nonce 0 skips valset-in-store check, allowing us to reach the reward denom validation
		EthBlockHeight: 1,
		Members:        types.BridgeValidators{},
		Orchestrator:   OrchAddrs[0].String(),
	}

	//nolint: exhaustruct
	err := attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidDenom)
}

// Test rejection for Ethereum originated tokens being registered as Cosmos-originated.
func TestHandleErc20Deployed_ReverseMapping(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	// Skip invariant assertion: we intentionally create a mapping that would conflict.

	k := input.GravityKeeper
	attHandler := k.AttestationHandler

	// Set up: "footoken" is already mapped to the target ERC20 contract
	existingERC20, err := types.NewEthAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
	require.NoError(t, err)
	err = k.setCosmosOriginatedMapping(ctx, "footoken", *existingERC20)
	require.NoError(t, err)

	// Now try to register the SAME ERC20 address for a different denom ("bartoken")
	//nolint: exhaustruct
	claim := types.MsgERC20DeployedClaim{
		CosmosDenom:    "bartoken",
		TokenContract:  existingERC20.GetAddress().Hex(),
		EventNonce:     1,
		EthBlockHeight: 1,
		Name:           "Bar Token",
		Symbol:         "BAR",
		Decimals:       6,
		Orchestrator:   OrchAddrs[0].String(),
	}

	//nolint: exhaustruct
	err = attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalid)
	require.Contains(t, err.Error(), "already mapped to denom")
}

// Test rejection for re-registering a Cosmos originated token with an existing erc20 representation
func TestHandleErc20Deployed_DuplicateDenom(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	// Skip invariant assertion: we intentionally create a mapping that would conflict.

	k := input.GravityKeeper
	attHandler := k.AttestationHandler

	// Set up: "footoken" already has an ERC20 representation
	existingERC20, err := types.NewEthAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
	require.NoError(t, err)
	err = k.setCosmosOriginatedMapping(ctx, "footoken", *existingERC20)
	require.NoError(t, err)

	// Now try to register a DIFFERENT ERC20 address for the same denom ("footoken")
	//nolint: exhaustruct
	claim := types.MsgERC20DeployedClaim{
		CosmosDenom:    "footoken",
		TokenContract:  "0xdAC17F958D2ee523a2206206994597C13D831ec7",
		EventNonce:     1,
		EthBlockHeight: 1,
		Name:           "Foo Token",
		Symbol:         "FOO",
		Decimals:       6,
		Orchestrator:   OrchAddrs[0].String(),
	}

	//nolint: exhaustruct
	err = attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalid)
	require.Contains(t, err.Error(), "already exists for denom")
}

// TestHandleErc20Deployed_GravityDenom verifies that gravity0x... and gravity20x... denoms
// cannot be registered as cosmos-originated. These are Ethereum-originated.
func TestHandleErc20Deployed_GravityDenom(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	attHandler := input.GravityKeeper.AttestationHandler

	// gravity0x... denom (standard Ethereum-originated voucher)
	//nolint: exhaustruct
	claim := types.MsgERC20DeployedClaim{
		CosmosDenom:    "gravity0x0bc529c00C6401aEF6D220BE8C6Ea1667F6Ad93e",
		TokenContract:  "0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255",
		EventNonce:     1,
		EthBlockHeight: 1,
		Name:           "test",
		Symbol:         "TST",
		Decimals:       6,
		Orchestrator:   OrchAddrs[0].String(),
	}

	//nolint: exhaustruct
	err := attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalid)
	require.Contains(t, err.Error(), "cannot register Ethereum-originated gravity denom")

	// gravity20x... denom (remapped Ethereum-originated voucher)
	//nolint: exhaustruct
	claim2 := types.MsgERC20DeployedClaim{
		CosmosDenom:    "gravity20x0bc529c00C6401aEF6D220BE8C6Ea1667F6Ad93e",
		TokenContract:  "0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255",
		EventNonce:     2,
		EthBlockHeight: 1,
		Name:           "test",
		Symbol:         "TST",
		Decimals:       6,
		Orchestrator:   OrchAddrs[0].String(),
	}

	//nolint: exhaustruct
	err = attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim2)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalid)
	require.Contains(t, err.Error(), "cannot register remapped gravity2 denom")
}

// TestHandleErc20Deployed_AllowlistEnforcement verifies that handleErc20Deployed rejects
// claims for denoms that have bank metadata but are NOT on the CosmosBridgeableTokens
// allowlist.
func TestHandleErc20Deployed_AllowlistEnforcement(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	attHandler := input.GravityKeeper.AttestationHandler

	// Set up bank metadata for "footoken" (simulating a governance proposal accepted it)
	//nolint: exhaustruct
	fooMeta := banktypes.Metadata{
		Base:        "footoken",
		Display:     "FOO",
		Name:        "Foo Token",
		Symbol:      "FOO",
		Description: "A test token",
		DenomUnits: []*banktypes.DenomUnit{
			{Denom: "footoken", Exponent: 0},
			{Denom: "FOO", Exponent: 6},
		},
	}
	input.BankKeeper.SetDenomMetaData(ctx, fooMeta)

	// Ensure the allowlist is empty (default)
	require.Empty(t, input.GravityKeeper.GetAllCosmosBridgeableTokens(ctx))

	// Attempt to deploy ERC20 for "footoken" — should fail because not on allowlist
	//nolint: exhaustruct
	claim := types.MsgERC20DeployedClaim{
		CosmosDenom:    "footoken",
		TokenContract:  "0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255",
		EventNonce:     1,
		EthBlockHeight: 1,
		Name:           "Foo Token",
		Symbol:         "FOO",
		Decimals:       6,
		Orchestrator:   OrchAddrs[0].String(),
	}

	//nolint: exhaustruct
	err := attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalid)
	require.Contains(t, err.Error(), "CosmosBridgeableTokens whitelist")

	// Now add "footoken" to the allowlist
	input.GravityKeeper.SetCosmosBridgeableToken(ctx, fooMeta)

	// Retry — should now pass the allowlist check (will succeed if metadata matches)
	//nolint: exhaustruct
	err = attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	require.NoError(t, err)

	// Verify the mapping was created
	footokenOrigin, err := input.GravityKeeper.ClassifyDenom(ctx, "footoken")
	require.NoError(t, err)
	require.Equal(t, types.AssetOriginCosmos, footokenOrigin.Origin)
	require.Equal(t, "0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255", footokenOrigin.ERC20.GetAddress().Hex())
}

// TestHandleErc20Deployed_MetadataDrift verifies that handleErc20Deployed rejects a claim
// for a denom whose bank module metadata has drifted from the governance-approved
// CosmosBridgeableTokens entry (the "SECURITY VIOLATION" branch in assertMetadataWhitelisted).
func TestHandleErc20Deployed_MetadataDrift(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	attHandler := input.GravityKeeper.AttestationHandler

	// Set up bank metadata for "tokyo" and whitelist that exact metadata
	//nolint: exhaustruct
	driftMeta := banktypes.Metadata{
		Base:        "tokyo",
		Display:     "TOKYO",
		Name:        "TOKYO Token",
		Symbol:      "TOKYO",
		Description: "A test token",
		DenomUnits: []*banktypes.DenomUnit{
			{Denom: "tokyo", Exponent: 0},
			{Denom: "TOKYO", Exponent: 6},
		},
	}
	input.BankKeeper.SetDenomMetaData(ctx, driftMeta)

	input.GravityKeeper.SetCosmosBridgeableToken(ctx, driftMeta)

	// Now mutate the bank module's stored metadata out from under the allowlist entry,
	// simulating drift between the two sources of truth.
	driftedMeta := driftMeta
	driftedMeta.Name = "DRIFT"
	driftedMeta.Symbol = "DRIFT"
	driftedMeta.Display = "DRIFT"
	input.BankKeeper.SetDenomMetaData(ctx, driftedMeta)

	//nolint: exhaustruct
	claim := types.MsgERC20DeployedClaim{
		CosmosDenom:    "tokyo",
		TokenContract:  "0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255",
		EventNonce:     1,
		EthBlockHeight: 1,
		Name:           "DRIFT",
		Symbol:         "DRIFT",
		Decimals:       6,
		Orchestrator:   OrchAddrs[0].String(),
	}

	//nolint: exhaustruct
	err := attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalid)
	require.Contains(t, err.Error(), "SECURITY VIOLATION")
}
