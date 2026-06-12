package keeper

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

var badErc20, _ = types.NewEthAddress("0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255")

func TestHandleSendToCosmos_BadDenom(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	// Intentionally plant a bad ERC20 -> Denom mapping in state to test handler rejection.
	// Invariant assertion is skipped because the corrupted mapping would trigger it.

	input.GravityKeeper.setCosmosOriginatedDenomToERC20(ctx, "ibc/gravity0xbad", *badErc20)

	attHandler := input.GravityKeeper.AttestationHandler
	claim := types.MsgSendToCosmosClaim{
		TokenContract:  badErc20.GetAddress().Hex(),
		Amount:         math.NewInt(1),
		CosmosReceiver: sdk.AccAddress([]byte{1}).String(),
		EthereumSender: "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7",
	}

	err := attHandler.Handle(ctx, types.Attestation{}, &claim)
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
		TokenContract: badErc20.GetAddress().Hex(),
		BatchNonce:    1,
		EventNonce:    1,
	}

	err := attHandler.Handle(ctx, types.Attestation{}, &claim)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidDenom)
}

func TestHandleErc20Deployed_BadDenom(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	attHandler := input.GravityKeeper.AttestationHandler
	claim := types.MsgERC20DeployedClaim{
		CosmosDenom:   "ibc/gravity0xbad",
		TokenContract: "0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255",
		EventNonce:    1,
		Name:          "test",
		Symbol:        "TST",
		Decimals:      6,
	}

	err := attHandler.Handle(ctx, types.Attestation{}, &claim)
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
		RewardToken:  badErc20.GetAddress().Hex(),
		RewardAmount: math.NewInt(1),
		EventNonce:   1,
		ValsetNonce:  0, // nonce 0 skips valset-in-store check, allowing us to reach the reward denom validation
		Members:      types.BridgeValidators{},
	}

	err := attHandler.Handle(ctx, types.Attestation{}, &claim)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidDenom)
}
