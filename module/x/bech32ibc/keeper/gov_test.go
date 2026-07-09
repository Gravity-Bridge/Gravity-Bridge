package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/types"
)

func TestHandleUpdateHrpIbcChannelProposal(t *testing.T) {
	env := setupTestKeeper(t, "transfer/channel-0")
	require.NoError(t, env.Keeper.SetNativeHrp(env.Ctx, "gravity"))

	t.Run("valid proposal creates a record", func(t *testing.T) {
		// nolint: exhaustruct
		p := &types.UpdateHrpIbcChannelProposal{
			Title:             "title",
			Description:       "description",
			Hrp:               "osmo",
			SourceChannel:     "channel-0",
			IcsToHeightOffset: 100,
		}
		require.NoError(t, env.Keeper.HandleUpdateHrpIbcChannelProposal(env.Ctx, p))

		record, err := env.Keeper.GetHrpIbcRecord(env.Ctx, "osmo")
		require.NoError(t, err)
		require.Equal(t, "channel-0", record.SourceChannel)
		require.Equal(t, uint64(100), record.IcsToHeightOffset)
	})

	t.Run("invalid hrp is rejected", func(t *testing.T) {
		// nolint: exhaustruct
		p := &types.UpdateHrpIbcChannelProposal{
			Title:         "title",
			Description:   "description",
			Hrp:           "INVALID",
			SourceChannel: "channel-0",
		}
		require.Error(t, env.Keeper.HandleUpdateHrpIbcChannelProposal(env.Ctx, p))
	})

	t.Run("native hrp is rejected", func(t *testing.T) {
		// nolint: exhaustruct
		p := &types.UpdateHrpIbcChannelProposal{
			Title:         "title",
			Description:   "description",
			Hrp:           "gravity",
			SourceChannel: "channel-0",
		}
		err := env.Keeper.HandleUpdateHrpIbcChannelProposal(env.Ctx, p)
		require.ErrorIs(t, err, types.ErrInvalidHRP)
	})

	t.Run("nonexistent channel is rejected", func(t *testing.T) {
		// nolint: exhaustruct
		p := &types.UpdateHrpIbcChannelProposal{
			Title:         "title",
			Description:   "description",
			Hrp:           "juno",
			SourceChannel: "channel-99",
		}
		err := env.Keeper.HandleUpdateHrpIbcChannelProposal(env.Ctx, p)
		require.ErrorIs(t, err, types.ErrInvalidIBCData)
	})
}

func TestHandleDeleteHrpIbcChannelProposal(t *testing.T) {
	env := setupTestKeeper(t, "transfer/channel-0")
	require.NoError(t, env.Keeper.SetNativeHrp(env.Ctx, "gravity"))

	// nolint: exhaustruct
	record := types.HrpIbcRecord{Hrp: "osmo", SourceChannel: "channel-0"}
	require.NoError(t, env.Keeper.setHrpIbcRecord(env.Ctx, record))

	t.Run("invalid hrp is rejected", func(t *testing.T) {
		// nolint: exhaustruct
		p := &types.DeleteHrpIbcChannelProposal{Title: "title", Description: "description", Hrp: "INVALID"}
		require.Error(t, env.Keeper.HandleDeleteHrpIbcChannelProposal(env.Ctx, p))
	})

	t.Run("native hrp is rejected", func(t *testing.T) {
		// nolint: exhaustruct
		p := &types.DeleteHrpIbcChannelProposal{Title: "title", Description: "description", Hrp: "gravity"}
		err := env.Keeper.HandleDeleteHrpIbcChannelProposal(env.Ctx, p)
		require.ErrorIs(t, err, types.ErrInvalidHRP)
	})

	t.Run("nonexistent record is rejected", func(t *testing.T) {
		// nolint: exhaustruct
		p := &types.DeleteHrpIbcChannelProposal{Title: "title", Description: "description", Hrp: "juno"}
		err := env.Keeper.HandleDeleteHrpIbcChannelProposal(env.Ctx, p)
		require.ErrorIs(t, err, types.ErrRecordNotFound)
	})

	t.Run("valid proposal deletes the record", func(t *testing.T) {
		// nolint: exhaustruct
		p := &types.DeleteHrpIbcChannelProposal{Title: "title", Description: "description", Hrp: "osmo"}
		require.NoError(t, env.Keeper.HandleDeleteHrpIbcChannelProposal(env.Ctx, p))

		// nolint: exhaustruct
		_, err := env.Keeper.GetHrpIbcRecord(env.Ctx, "osmo")
		require.ErrorIs(t, err, types.ErrRecordNotFound)
	})
}
