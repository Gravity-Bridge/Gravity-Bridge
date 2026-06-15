package keeper

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
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

func TestHandleSendToCosmos_BadDenom(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	// Intentionally plant a bad ERC20 -> Denom mapping in state to test handler rejection.
	// Invariant assertion is skipped because the corrupted mapping would trigger it.

	input.GravityKeeper.setCosmosOriginatedDenomToERC20(ctx, "ibc/gravity0xbad", *badErc20)

	attHandler := input.GravityKeeper.AttestationHandler
	claim := types.MsgSendToCosmosClaim{
		EventNonce:     1,
		EthBlockHeight: 1,
		TokenContract:  badErc20.GetAddress().Hex(),
		Amount:         math.NewInt(1),
		CosmosReceiver: sdk.AccAddress([]byte{1}).String(),
		EthereumSender: "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7",
		Orchestrator:   OrchAddrs[0].String(),
	}

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

	input.GravityKeeper.setCosmosOriginatedDenomToERC20(ctx, "ibc/gravity0xbad", *badErc20)

	attHandler := input.GravityKeeper.AttestationHandler
	claim := types.MsgBatchSendToEthClaim{
		TokenContract:  badErc20.GetAddress().Hex(),
		BatchNonce:     1,
		EventNonce:     1,
		EthBlockHeight: 1,
		Orchestrator:   OrchAddrs[0].String(),
	}

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

	err := attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidDenom)
}

func TestHandleValsetUpdated_BadRewardDenom(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	// Intentionally plant a bad ERC20 -> Denom mapping in state to test handler rejection.
	// Invariant assertion is skipped because the corrupted mapping would trigger it.

	input.GravityKeeper.setCosmosOriginatedDenomToERC20(ctx, "ibc/gravity0xbad", *badErc20)

	attHandler := input.GravityKeeper.AttestationHandler
	claim := types.MsgValsetUpdatedClaim{
		RewardToken:    badErc20.GetAddress().Hex(),
		RewardAmount:   math.NewInt(1),
		EventNonce:     1,
		ValsetNonce:    0, // nonce 0 skips valset-in-store check, allowing us to reach the reward denom validation
		EthBlockHeight: 1,
		Members:        types.BridgeValidators{},
		Orchestrator:   OrchAddrs[0].String(),
	}

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
	k.setCosmosOriginatedDenomToERC20(ctx, "footoken", *existingERC20)

	// Now try to register the SAME ERC20 address for a different denom ("bartoken")
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
	k.setCosmosOriginatedDenomToERC20(ctx, "footoken", *existingERC20)

	// Now try to register a DIFFERENT ERC20 address for the same denom ("footoken")
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

	err = attHandler.Handle(ctx, types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(ctx.BlockHeight()),
	}, &claim)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalid)
	require.Contains(t, err.Error(), "already exists for denom")
}
