package keeper

import (
	"testing"

	"cosmossdk.io/math"
	bech32ibctypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/stretchr/testify/require"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

func TestValidatePendingIbcAutoForward_BadDenom(t *testing.T) {
	input, ctx := SetupFiveValChain(t)

	// Register a foreign HRP so the address is valid and foreign.
	foreignHrp := "astro"
	rec := bech32ibctypes.HrpIbcRecord{
		Hrp:               foreignHrp,
		SourceChannel:     "channel-0",
		IcsToHeightOffset: 1000,
		IcsToTimeOffset:   1000,
	}
	input.GravityKeeper.bech32IbcKeeper.SetHrpIbcRecords(ctx, []bech32ibctypes.HrpIbcRecord{rec})

	// Provide module balance so funds check passes.
	coins := sdk.NewCoins(sdk.NewCoin("ugraviton", math.NewInt(1000)))
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, coins))

	foreignAddr, err := bech32.ConvertAndEncode(foreignHrp, []byte{1})
	require.NoError(t, err)

	fwd := types.PendingIbcAutoForward{
		ForeignReceiver: foreignAddr,
		Token:           &sdk.Coin{Denom: "ibc/gravity0xbad", Amount: math.NewInt(1)},
		EventNonce:      1,
		IbcChannel:      "channel-0",
	}

	err = input.GravityKeeper.ValidatePendingIbcAutoForward(ctx, fwd)
	require.ErrorIs(t, err, types.ErrInvalidDenom)
}

func TestProcessNextPendingIbcAutoForward_BadDenom(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	// Invariant assertion is skipped because we intentionally persist a bad denom to state.

	foreignHrp := "astro"
	rec := bech32ibctypes.HrpIbcRecord{
		Hrp:               foreignHrp,
		SourceChannel:     "channel-0",
		IcsToHeightOffset: 1000,
		IcsToTimeOffset:   1000,
	}
	input.GravityKeeper.bech32IbcKeeper.SetHrpIbcRecords(ctx, []bech32ibctypes.HrpIbcRecord{rec})

	foreignAddr, err := bech32.ConvertAndEncode(foreignHrp, []byte{1})
	require.NoError(t, err)

	// Manually store a forward with an invalid denom to test the panic path.
	store := ctx.KVStore(input.GravityKeeper.storeKey)
	key := types.GetPendingIbcAutoForwardKey(1)
	fwd := types.PendingIbcAutoForward{
		ForeignReceiver: foreignAddr,
		Token:           &sdk.Coin{Denom: "ibc/gravity0xbad", Amount: math.NewInt(1)},
		EventNonce:      1,
		IbcChannel:      "channel-0",
	}
	store.Set(key, input.GravityKeeper.cdc.MustMarshal(&fwd))

	require.Panics(t, func() {
		_, err = input.GravityKeeper.ProcessNextPendingIbcAutoForward(ctx)
		require.Error(t, err)
	})
}
