package bech32ibc_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	bech32ibc "github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/types"
)

// nolint: exhaustruct
type unrecognizedProposalContent struct{}

func (unrecognizedProposalContent) GetTitle() string       { return "title" }
func (unrecognizedProposalContent) GetDescription() string { return "description" }
func (unrecognizedProposalContent) ProposalRoute() string  { return types.RouterKey }
func (unrecognizedProposalContent) ProposalType() string   { return "Unrecognized" }
func (unrecognizedProposalContent) ValidateBasic() error   { return nil }
func (unrecognizedProposalContent) String() string         { return "unrecognized" }

func TestNewBech32IBCProposalHandler(t *testing.T) {
	k, ctx := setupTestKeeper(t, "transfer/channel-0")
	require.NoError(t, k.SetNativeHrp(ctx, "gravity"))

	handler := bech32ibc.NewBech32IBCProposalHandler(k)

	t.Run("update proposal is dispatched", func(t *testing.T) {
		// nolint: exhaustruct
		p := &types.UpdateHrpIbcChannelProposal{
			Title:         "title",
			Description:   "description",
			Hrp:           "osmo",
			SourceChannel: "channel-0",
		}
		require.NoError(t, handler(ctx, p))

		_, err := k.GetHrpIbcRecord(ctx, "osmo")
		require.NoError(t, err)
	})

	t.Run("delete proposal is dispatched", func(t *testing.T) {
		// nolint: exhaustruct
		p := &types.DeleteHrpIbcChannelProposal{Title: "title", Description: "description", Hrp: "osmo"}
		require.NoError(t, handler(ctx, p))

		_, err := k.GetHrpIbcRecord(ctx, "osmo")
		require.ErrorIs(t, err, types.ErrRecordNotFound)
	})

	t.Run("unrecognized proposal type errors", func(t *testing.T) {
		err := handler(ctx, unrecognizedProposalContent{})
		require.Error(t, err)
	})
}
