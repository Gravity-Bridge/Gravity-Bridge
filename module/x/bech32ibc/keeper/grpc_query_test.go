package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/types"
)

func TestQuery_HrpIbcRecords(t *testing.T) {
	env := setupTestKeeper(t)

	// nolint: exhaustruct
	resp, err := env.Keeper.HrpIbcRecords(env.Ctx, &types.QueryHrpIbcRecordsRequest{})
	require.NoError(t, err)
	require.Empty(t, resp.HrpIbcRecords)

	// nolint: exhaustruct
	record := types.HrpIbcRecord{Hrp: "osmo", SourceChannel: "channel-0"}
	require.NoError(t, env.Keeper.setHrpIbcRecord(env.Ctx, record))

	// nolint: exhaustruct
	resp, err = env.Keeper.HrpIbcRecords(env.Ctx, &types.QueryHrpIbcRecordsRequest{})
	require.NoError(t, err)
	require.Equal(t, []types.HrpIbcRecord{record}, resp.HrpIbcRecords)
}

func TestQuery_HrpIbcRecord(t *testing.T) {
	env := setupTestKeeper(t)

	// nolint: exhaustruct
	_, err := env.Keeper.HrpIbcRecord(env.Ctx, nil)
	require.Error(t, err)
	s, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, s.Code())

	// nolint: exhaustruct
	_, err = env.Keeper.HrpIbcRecord(env.Ctx, &types.QueryHrpIbcRecordRequest{Hrp: "osmo"})
	require.Error(t, err)

	// nolint: exhaustruct
	record := types.HrpIbcRecord{Hrp: "osmo", SourceChannel: "channel-0"}
	require.NoError(t, env.Keeper.setHrpIbcRecord(env.Ctx, record))

	// nolint: exhaustruct
	resp, err := env.Keeper.HrpIbcRecord(env.Ctx, &types.QueryHrpIbcRecordRequest{Hrp: "osmo"})
	require.NoError(t, err)
	require.Equal(t, record, resp.HrpIbcRecord)
}

func TestQuery_NativeHrp(t *testing.T) {
	env := setupTestKeeper(t)

	// nolint: exhaustruct
	_, err := env.Keeper.NativeHrp(env.Ctx, &types.QueryNativeHrpRequest{})
	require.Error(t, err)

	require.NoError(t, env.Keeper.SetNativeHrp(env.Ctx, "gravity"))

	// nolint: exhaustruct
	resp, err := env.Keeper.NativeHrp(env.Ctx, &types.QueryNativeHrpRequest{})
	require.NoError(t, err)
	require.Equal(t, "gravity", resp.NativeHrp)
}
