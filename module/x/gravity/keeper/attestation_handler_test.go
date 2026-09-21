package keeper

import (
	"context"
	"errors"
	"strings"
	"testing"

	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
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

func requireBridgePaused(t *testing.T, ctx sdk.Context, k Keeper) {
	t.Helper()
	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.False(t, params.BridgeActive, "expected the handler to pause the bridge")
}

func TestHandleSendToCosmos_CommunityPoolFailureRollsBack(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	gravityKeeper := input.GravityKeeper
	denom := "poolrollback"
	require.NoError(t, gravityKeeper.setCosmosOriginatedMapping(ctx, denom, *badErc20))
	coins := sdk.NewCoins(sdk.NewInt64Coin(denom, 1))
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, coins))
	require.NoError(t, input.DistKeeper.FeePool.Remove(ctx))
	moduleAddr := input.AccountKeeper.GetModuleAddress(types.ModuleName)
	distributionAddr := input.AccountKeeper.GetModuleAddress(distrtypes.ModuleName)
	distributionBalance := input.BankKeeper.GetBalance(ctx, distributionAddr, denom)

	probeCtx, _ := ctx.CacheContext()
	require.ErrorContains(t, gravityKeeper.SendToCommunityPool(probeCtx, coins), "failed to get fee pool")
	require.Equal(t, distributionBalance.Add(coins[0]), input.BankKeeper.GetBalance(probeCtx, distributionAddr, denom))

	claim := &types.MsgSendToCosmosClaim{
		EventNonce: 1, EthBlockHeight: 1, TokenContract: badErc20.GetAddress().Hex(),
		Amount: math.NewInt(1), CosmosReceiver: AccAddrs[0].String(),
		EthereumSender: EthAddrs[0].String(), Orchestrator: OrchAddrs[0].String(),
	}
	eventsBefore := len(ctx.EventManager().Events())
	gravityKeeper.processAttestation(ctx, new(types.Attestation), claim)

	requireBridgePaused(t, ctx, gravityKeeper)
	require.Equal(t, coins[0], input.BankKeeper.GetBalance(ctx, moduleAddr, denom))
	require.Equal(t, distributionBalance, input.BankKeeper.GetBalance(ctx, distributionAddr, denom))
	require.True(t, input.BankKeeper.GetBalance(ctx, AccAddrs[0], denom).IsZero())
	require.Equal(t, eventsBefore, len(ctx.EventManager().Events()))
}

func TestHandleSendToCosmos_MintFailureRollsBack(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	gravityKeeper := input.GravityKeeper
	mintAttempted := false
	bankKeeper := input.BankKeeper.WithMintCoinsRestriction(func(mintCtx context.Context, coins sdk.Coins) error {
		mintAttempted = true
		require.NoError(t, input.BankKeeper.MintCoins(mintCtx, types.ModuleName, coins))
		return errors.New("injected failure after mint writes")
	})
	gravityKeeper.bankKeeper = &bankKeeper
	gravityKeeper.AttestationHandler = AttestationHandler{keeper: &gravityKeeper}
	claim := &types.MsgSendToCosmosClaim{
		EventNonce: 1, EthBlockHeight: 1, TokenContract: badErc20.GetAddress().Hex(),
		Amount: math.NewInt(1), CosmosReceiver: AccAddrs[0].String(),
		EthereumSender: EthAddrs[0].String(), Orchestrator: OrchAddrs[0].String(),
	}
	eventsBefore := len(ctx.EventManager().Events())
	gravityKeeper.processAttestation(ctx, new(types.Attestation), claim)

	require.True(t, mintAttempted)
	requireBridgePaused(t, ctx, gravityKeeper)
	denom := types.GravityDenom(*badErc20)
	require.True(t, input.BankKeeper.GetSupply(ctx, denom).IsZero())
	require.True(t, input.BankKeeper.GetBalance(ctx, input.AccountKeeper.GetModuleAddress(types.ModuleName), denom).IsZero())
	require.True(t, input.BankKeeper.GetBalance(ctx, AccAddrs[0], denom).IsZero())
	require.Equal(t, eventsBefore, len(ctx.EventManager().Events()))
	input.AssertInvariants()
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
	// The deposit cannot be classified, so nothing is minted and the bridge stops.
	require.NoError(t, err)
	requireBridgePaused(t, ctx, input.GravityKeeper)
	require.True(t, input.BankKeeper.GetSupply(ctx, "ibc/gravity0xbad").IsZero())
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
	// Vouchers cannot be burned for an unclassifiable token, so the batch is left for governance.
	require.NoError(t, err)
	requireBridgePaused(t, ctx, input.GravityKeeper)
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
	require.Contains(t, err.Error(), "collides with an eth-originated gravity denom")

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
	require.Contains(t, err.Error(), "collides with an eth-originated gravity2 denom")
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
	const driftedName = "DRIFT"
	driftedMeta := driftMeta
	driftedMeta.Name = driftedName
	driftedMeta.Symbol = driftedName
	driftedMeta.Display = driftedName
	input.BankKeeper.SetDenomMetaData(ctx, driftedMeta)

	//nolint: exhaustruct
	claim := types.MsgERC20DeployedClaim{
		CosmosDenom:    "tokyo",
		TokenContract:  "0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255",
		EventNonce:     1,
		EthBlockHeight: 1,
		Name:           driftedName,
		Symbol:         driftedName,
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

	// Restore bank metadata so the deferred invariant assertion does not fail
	input.BankKeeper.SetDenomMetaData(ctx, driftMeta)
}

// deleteBankMetadata removes the x/bank denom metadata entry for denom directly from the store.
// The bank keeper does not expose a delete method, so tests use this to simulate missing metadata.
func deleteBankMetadata(ctx sdk.Context, bankStoreKey *storetypes.KVStoreKey, denom string) {
	store := prefix.NewStore(ctx.KVStore(bankStoreKey), banktypes.DenomMetadataPrefix)
	store.Delete([]byte(denom))
}

// TestHandleSendToCosmos_MetadataDrift verifies that handleSendToCosmos diverts a
// cosmos-originated deposit and pauses the bridge when the bank module metadata has drifted
// from the governance-approved CosmosBridgeableTokens entry.
// nolint: exhaustruct
func TestHandleSendToCosmos_MetadataDrift(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	k := input.GravityKeeper
	attHandler := k.AttestationHandler

	denom := "sendcosmosdrift"
	erc20, err := types.NewEthAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
	require.NoError(t, err)
	err = k.setCosmosOriginatedMapping(ctx, denom, *erc20)
	require.NoError(t, err)

	meta := minMeta(denom)
	input.BankKeeper.SetDenomMetaData(ctx, meta)
	k.SetCosmosBridgeableToken(ctx, meta)

	// Fund the gravity module so the sanity-check deposit can send coins.
	sendCoins := sdk.NewCoins(sdk.NewInt64Coin(denom, 1000))
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, sendCoins))

	claim := types.MsgSendToCosmosClaim{
		EventNonce:     1,
		EthBlockHeight: 1,
		TokenContract:  erc20.GetAddress().Hex(),
		Amount:         math.NewInt(1),
		CosmosReceiver: AccAddrs[0].String(),
		EthereumSender: "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7",
		Orchestrator:   OrchAddrs[0].String(),
	}

	// Sanity check: succeeds while bank metadata matches the allowlist entry.
	err = attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	require.NoError(t, err)

	// Drift the bank metadata out from under the allowlist entry.
	driftedMeta := meta
	driftedMeta.Name = "Drifted"
	input.BankKeeper.SetDenomMetaData(ctx, driftedMeta)

	err = attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	// The escrowed coins still have to go somewhere, so the deposit is diverted to the community
	// pool rather than credited, and the bridge stops.
	require.NoError(t, err)
	requireBridgePaused(t, ctx, k)
	require.Equal(t, math.NewInt(1), input.BankKeeper.GetBalance(ctx, AccAddrs[0], denom).Amount)

	// Restore bank metadata so the deferred invariant assertion does not fail.
	input.BankKeeper.SetDenomMetaData(ctx, meta)
}

// TestHandleSendToCosmos_MissingBankMetadata verifies that handleSendToCosmos diverts
// a cosmos-originated deposit and pauses the bridge when the bank module metadata has been
// deleted while the CosmosBridgeableTokens entry still exists.
// nolint: exhaustruct
func TestHandleSendToCosmos_MissingBankMetadata(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	k := input.GravityKeeper
	attHandler := k.AttestationHandler

	denom := "sendcosmosmissing"
	erc20, err := types.NewEthAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
	require.NoError(t, err)
	err = k.setCosmosOriginatedMapping(ctx, denom, *erc20)
	require.NoError(t, err)

	meta := minMeta(denom)
	input.BankKeeper.SetDenomMetaData(ctx, meta)
	k.SetCosmosBridgeableToken(ctx, meta)

	// Delete the bank metadata entry, leaving the allowlist entry in place.
	deleteBankMetadata(ctx, input.BankStoreKey, denom)

	// Fund the gravity module so the diverted deposit can reach the community pool.
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(sdk.NewInt64Coin(denom, 1000))))

	claim := types.MsgSendToCosmosClaim{
		EventNonce:     1,
		EthBlockHeight: 1,
		TokenContract:  erc20.GetAddress().Hex(),
		Amount:         math.NewInt(1),
		CosmosReceiver: AccAddrs[0].String(),
		EthereumSender: "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7",
		Orchestrator:   OrchAddrs[0].String(),
	}

	err = attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	require.NoError(t, err)
	requireBridgePaused(t, ctx, k)
	require.True(t, input.BankKeeper.GetBalance(ctx, AccAddrs[0], denom).IsZero())

	// Restore bank metadata so the deferred invariant assertion does not fail.
	input.BankKeeper.SetDenomMetaData(ctx, meta)
}

// TestHandleBatchSendToEth_MetadataDrift verifies that handleBatchSendToEth still clears the
// executed batch but pauses the bridge when bank metadata has drifted from the
// governance-approved CosmosBridgeableTokens entry.
// nolint: exhaustruct
func TestHandleBatchSendToEth_MetadataDrift(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	k := input.GravityKeeper
	attHandler := k.AttestationHandler

	denom := "batchdrift"
	erc20, err := types.NewEthAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
	require.NoError(t, err)
	err = k.setCosmosOriginatedMapping(ctx, denom, *erc20)
	require.NoError(t, err)

	meta := minMeta(denom)
	input.BankKeeper.SetDenomMetaData(ctx, meta)
	k.SetCosmosBridgeableToken(ctx, meta)

	// Store a batch so the sanity-check claim can reach OutgoingTxBatchExecuted.
	batch := types.InternalOutgoingTxBatch{
		BatchNonce:         1,
		BatchTimeout:       100,
		Transactions:       []*types.InternalOutgoingTransferTx{},
		TokenContract:      *erc20,
		CosmosBlockCreated: uint64(ctx.BlockHeight()),
	}
	k.StoreBatch(ctx, batch)

	claim := types.MsgBatchSendToEthClaim{
		TokenContract:  erc20.GetAddress().Hex(),
		BatchNonce:     1,
		EventNonce:     1,
		EthBlockHeight: 1,
		Orchestrator:   OrchAddrs[0].String(),
	}

	// Sanity check: succeeds while bank metadata matches the allowlist entry.
	err = attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	require.NoError(t, err)

	// Recreate a fresh batch for the drifted-metadata call
	k.StoreBatch(ctx, batch)

	// Drift the bank metadata out from under the allowlist entry.
	driftedMeta := meta
	driftedMeta.Name = "Drifted"
	input.BankKeeper.SetDenomMetaData(ctx, driftedMeta)

	err = attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	// Ethereum already paid the batch out, so the batch is still cleared and the bridge stops.
	require.NoError(t, err)
	requireBridgePaused(t, ctx, k)
	require.Nil(t, k.GetOutgoingTXBatch(ctx, *erc20, 1))

	// Restore bank metadata so the deferred invariant assertion does not fail.
	input.BankKeeper.SetDenomMetaData(ctx, meta)
}

// TestHandleBatchSendToEth_MissingBankMetadata verifies that handleBatchSendToEth still
// clears the executed batch but pauses the bridge when the bank module metadata has been
// deleted while the CosmosBridgeableTokens entry still exists.
// nolint: exhaustruct
func TestHandleBatchSendToEth_MissingBankMetadata(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	k := input.GravityKeeper
	attHandler := k.AttestationHandler

	denom := "batchmissing"
	erc20, err := types.NewEthAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
	require.NoError(t, err)
	err = k.setCosmosOriginatedMapping(ctx, denom, *erc20)
	require.NoError(t, err)

	meta := minMeta(denom)
	input.BankKeeper.SetDenomMetaData(ctx, meta)
	k.SetCosmosBridgeableToken(ctx, meta)

	// Store a batch so the executed-batch bookkeeping has something to clear.
	batch := types.InternalOutgoingTxBatch{
		BatchNonce:         1,
		BatchTimeout:       100,
		Transactions:       []*types.InternalOutgoingTransferTx{},
		TokenContract:      *erc20,
		CosmosBlockCreated: uint64(ctx.BlockHeight()),
	}
	k.StoreBatch(ctx, batch)

	// Delete the bank metadata entry, leaving the allowlist entry in place.
	deleteBankMetadata(ctx, input.BankStoreKey, denom)

	claim := types.MsgBatchSendToEthClaim{
		TokenContract:  erc20.GetAddress().Hex(),
		BatchNonce:     1,
		EventNonce:     1,
		EthBlockHeight: 1,
		Orchestrator:   OrchAddrs[0].String(),
	}

	err = attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	require.NoError(t, err)
	requireBridgePaused(t, ctx, k)
	require.Nil(t, k.GetOutgoingTXBatch(ctx, *erc20, 1))

	// Restore bank metadata so the deferred invariant assertion does not fail.
	input.BankKeeper.SetDenomMetaData(ctx, meta)
}

// TestHandleErc20Deployed_MissingBankMetadata verifies that handleErc20Deployed rejects
// an ERC20 deployment claim when the bank module metadata has been deleted while the
// CosmosBridgeableTokens entry still exists.
// nolint: exhaustruct
func TestHandleErc20Deployed_MissingBankMetadata(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	k := input.GravityKeeper
	attHandler := k.AttestationHandler

	denom := "deploymissing"
	meta := minMeta(denom)
	input.BankKeeper.SetDenomMetaData(ctx, meta)
	k.SetCosmosBridgeableToken(ctx, meta)

	// Delete the bank metadata entry, leaving the allowlist entry in place.
	deleteBankMetadata(ctx, input.BankStoreKey, denom)

	claim := types.MsgERC20DeployedClaim{
		CosmosDenom:    denom,
		TokenContract:  "0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255",
		EventNonce:     1,
		EthBlockHeight: 1,
		Name:           meta.Name,
		Symbol:         meta.Symbol,
		Decimals:       6,
		Orchestrator:   OrchAddrs[0].String(),
	}

	err := attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	require.Error(t, err)
	require.Contains(t, err.Error(), "SECURITY VIOLATION")
	require.Contains(t, err.Error(), "not found")

	// Restore bank metadata so the deferred invariant assertion does not fail.
	input.BankKeeper.SetDenomMetaData(ctx, meta)
}
