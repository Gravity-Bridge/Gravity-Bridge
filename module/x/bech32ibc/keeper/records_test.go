package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/types"
)

func TestGetHrpIbcRecord_NotFound(t *testing.T) {
	env := setupTestKeeper(t)

	// nolint: exhaustruct
	_, err := env.Keeper.GetHrpIbcRecord(env.Ctx, "osmo")
	require.ErrorIs(t, err, types.ErrRecordNotFound)
}

func TestSetGetHrpIbcRecord(t *testing.T) {
	env := setupTestKeeper(t)

	// nolint: exhaustruct
	record := types.HrpIbcRecord{
		Hrp:               "osmo",
		SourceChannel:     "channel-0",
		IcsToHeightOffset: 100,
	}

	require.NoError(t, env.Keeper.setHrpIbcRecord(env.Ctx, record))

	got, err := env.Keeper.GetHrpIbcRecord(env.Ctx, "osmo")
	require.NoError(t, err)
	require.Equal(t, record, got)
}

func TestSetHrpIbcRecord_EmptySourceChannel(t *testing.T) {
	env := setupTestKeeper(t)

	// nolint: exhaustruct
	record := types.HrpIbcRecord{Hrp: "osmo", SourceChannel: ""}

	err := env.Keeper.setHrpIbcRecord(env.Ctx, record)
	require.ErrorIs(t, err, types.ErrInvalidIBCData)
}

func TestDeleteHrpIbcRecord_NotFound(t *testing.T) {
	env := setupTestKeeper(t)

	err := env.Keeper.deleteHrpIbcRecord(env.Ctx, "osmo")
	require.ErrorIs(t, err, types.ErrRecordNotFound)
}

func TestDeleteHrpIbcRecord(t *testing.T) {
	env := setupTestKeeper(t)

	// nolint: exhaustruct
	record := types.HrpIbcRecord{Hrp: "osmo", SourceChannel: "channel-0"}
	require.NoError(t, env.Keeper.setHrpIbcRecord(env.Ctx, record))

	require.NoError(t, env.Keeper.deleteHrpIbcRecord(env.Ctx, "osmo"))

	// nolint: exhaustruct
	_, err := env.Keeper.GetHrpIbcRecord(env.Ctx, "osmo")
	require.ErrorIs(t, err, types.ErrRecordNotFound)

	// deleting again fails
	err = env.Keeper.deleteHrpIbcRecord(env.Ctx, "osmo")
	require.ErrorIs(t, err, types.ErrRecordNotFound)
}

func TestGetHrpIbcRecords(t *testing.T) {
	env := setupTestKeeper(t)

	require.Empty(t, env.Keeper.GetHrpIbcRecords(env.Ctx))

	// nolint: exhaustruct
	recordA := types.HrpIbcRecord{Hrp: "osmo", SourceChannel: "channel-0"}
	// nolint: exhaustruct
	recordB := types.HrpIbcRecord{Hrp: "juno", SourceChannel: "channel-1"}

	require.NoError(t, env.Keeper.setHrpIbcRecord(env.Ctx, recordA))
	require.NoError(t, env.Keeper.setHrpIbcRecord(env.Ctx, recordB))

	records := env.Keeper.GetHrpIbcRecords(env.Ctx)
	require.ElementsMatch(t, []types.HrpIbcRecord{recordA, recordB}, records)
}

func TestGetHrpSourceChannel(t *testing.T) {
	env := setupTestKeeper(t)

	// nolint: exhaustruct
	_, err := env.Keeper.GetHrpSourceChannel(env.Ctx, "osmo")
	require.ErrorIs(t, err, types.ErrRecordNotFound)

	// nolint: exhaustruct
	record := types.HrpIbcRecord{Hrp: "osmo", SourceChannel: "channel-0"}
	require.NoError(t, env.Keeper.setHrpIbcRecord(env.Ctx, record))

	channel, err := env.Keeper.GetHrpSourceChannel(env.Ctx, "osmo")
	require.NoError(t, err)
	require.Equal(t, "channel-0", channel)
}

func TestSetHrpIbcRecords(t *testing.T) {
	env := setupTestKeeper(t)
	require.NoError(t, env.Keeper.SetNativeHrp(env.Ctx, "gravity"))

	// nolint: exhaustruct
	records := []types.HrpIbcRecord{
		{Hrp: "osmo", SourceChannel: "channel-0"},
		{Hrp: "juno", SourceChannel: "channel-1"},
	}

	env.Keeper.SetHrpIbcRecords(env.Ctx, records)

	got := env.Keeper.GetHrpIbcRecords(env.Ctx)
	require.ElementsMatch(t, records, got)
}

func TestSetHrpIbcRecords_PanicsWithoutNativeHrp(t *testing.T) {
	env := setupTestKeeper(t)

	// nolint: exhaustruct
	records := []types.HrpIbcRecord{{Hrp: "osmo", SourceChannel: "channel-0"}}

	require.Panics(t, func() {
		env.Keeper.SetHrpIbcRecords(env.Ctx, records)
	})
}

func TestValidateHrpIbcRecord(t *testing.T) {
	env := setupTestKeeper(t, "transfer/channel-0")
	require.NoError(t, env.Keeper.SetNativeHrp(env.Ctx, "gravity"))

	t.Run("valid record", func(t *testing.T) {
		// nolint: exhaustruct
		record := types.HrpIbcRecord{Hrp: "osmo", SourceChannel: "channel-0"}
		require.NoError(t, env.Keeper.ValidateHrpIbcRecord(env.Ctx, record))
	})

	t.Run("invalid hrp", func(t *testing.T) {
		// nolint: exhaustruct
		record := types.HrpIbcRecord{Hrp: "INVALID", SourceChannel: "channel-0"}
		require.Error(t, env.Keeper.ValidateHrpIbcRecord(env.Ctx, record))
	})

	t.Run("native hrp rejected", func(t *testing.T) {
		// nolint: exhaustruct
		record := types.HrpIbcRecord{Hrp: "gravity", SourceChannel: "channel-0"}
		err := env.Keeper.ValidateHrpIbcRecord(env.Ctx, record)
		require.ErrorIs(t, err, types.ErrInvalidHRP)
	})

	t.Run("channel not found", func(t *testing.T) {
		// nolint: exhaustruct
		record := types.HrpIbcRecord{Hrp: "osmo", SourceChannel: "channel-99"}
		err := env.Keeper.ValidateHrpIbcRecord(env.Ctx, record)
		require.ErrorIs(t, err, types.ErrInvalidIBCData)
	})
}
