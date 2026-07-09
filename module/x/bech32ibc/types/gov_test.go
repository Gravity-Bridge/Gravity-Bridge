package types_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/types"
)

func TestNewUpdateHrpIBCRecordProposal(t *testing.T) {
	content := types.NewUpdateHrpIBCRecordProposal("title", "description", "osmo", "channel-0", 100, time.Hour)

	proposal, ok := content.(*types.UpdateHrpIbcChannelProposal)
	require.True(t, ok)

	require.Equal(t, "title", proposal.GetTitle())
	require.Equal(t, "description", proposal.GetDescription())
	require.Equal(t, types.RouterKey, proposal.ProposalRoute())
	require.Equal(t, types.ProposalTypeUpdateHrpIbcChannel, proposal.ProposalType())
	require.Equal(t, "osmo", proposal.Hrp)
	require.Equal(t, "channel-0", proposal.SourceChannel)
	require.Equal(t, uint64(100), proposal.IcsToHeightOffset)
	require.Equal(t, time.Hour, proposal.IcsToTimeOffset)

	str := proposal.String()
	require.Contains(t, str, "title")
	require.Contains(t, str, "description")
	require.Contains(t, str, "osmo")
	require.Contains(t, str, "channel-0")
}

func TestUpdateHrpIbcChannelProposal_ValidateBasic(t *testing.T) {
	validProposal := func() *types.UpdateHrpIbcChannelProposal {
		return &types.UpdateHrpIbcChannelProposal{
			Title:             "title",
			Description:       "description",
			Hrp:               "osmo",
			SourceChannel:     "channel-0",
			IcsToHeightOffset: 100,
			IcsToTimeOffset:   0,
		}
	}

	t.Run("valid proposal", func(t *testing.T) {
		require.NoError(t, validProposal().ValidateBasic())
	})

	t.Run("valid proposal with only time offset set", func(t *testing.T) {
		p := validProposal()
		p.IcsToHeightOffset = 0
		p.IcsToTimeOffset = time.Hour
		require.NoError(t, p.ValidateBasic())
	})

	t.Run("empty title fails abstract validation", func(t *testing.T) {
		p := validProposal()
		p.Title = ""
		require.Error(t, p.ValidateBasic())
	})

	t.Run("empty description fails abstract validation", func(t *testing.T) {
		p := validProposal()
		p.Description = ""
		require.Error(t, p.ValidateBasic())
	})

	t.Run("empty source channel", func(t *testing.T) {
		p := validProposal()
		p.SourceChannel = ""
		err := p.ValidateBasic()
		require.ErrorIs(t, err, types.ErrInvalidIBCData)
	})

	t.Run("blank source channel", func(t *testing.T) {
		p := validProposal()
		p.SourceChannel = "   "
		err := p.ValidateBasic()
		require.ErrorIs(t, err, types.ErrInvalidIBCData)
	})

	t.Run("no height or time offset set", func(t *testing.T) {
		p := validProposal()
		p.IcsToHeightOffset = 0
		p.IcsToTimeOffset = 0
		err := p.ValidateBasic()
		require.ErrorIs(t, err, types.ErrInvalidOffsetHeightTimeout)
	})

	t.Run("invalid hrp", func(t *testing.T) {
		p := validProposal()
		p.Hrp = "INVALID"
		require.Error(t, p.ValidateBasic())
	})
}

func TestNewDeleteHrpIbcChannelProposal(t *testing.T) {
	content := types.NewDeleteHrpIbcChannelProposal("title", "description", "osmo")

	proposal, ok := content.(*types.DeleteHrpIbcChannelProposal)
	require.True(t, ok)

	require.Equal(t, "title", proposal.GetTitle())
	require.Equal(t, "description", proposal.GetDescription())
	require.Equal(t, types.RouterKey, proposal.ProposalRoute())
	require.Equal(t, types.ProposalTypeDeleteHrpIbcChannel, proposal.ProposalType())
	require.Equal(t, "osmo", proposal.Hrp)

	str := proposal.String()
	require.Contains(t, str, "title")
	require.Contains(t, str, "description")
	require.Contains(t, str, "osmo")
}

func TestDeleteHrpIbcChannelProposal_ValidateBasic(t *testing.T) {
	validProposal := func() *types.DeleteHrpIbcChannelProposal {
		return &types.DeleteHrpIbcChannelProposal{
			Title:       "title",
			Description: "description",
			Hrp:         "osmo",
		}
	}

	t.Run("valid proposal", func(t *testing.T) {
		require.NoError(t, validProposal().ValidateBasic())
	})

	t.Run("empty title fails abstract validation", func(t *testing.T) {
		p := validProposal()
		p.Title = ""
		require.Error(t, p.ValidateBasic())
	})

	t.Run("invalid hrp", func(t *testing.T) {
		p := validProposal()
		p.Hrp = "INVALID"
		require.Error(t, p.ValidateBasic())
	})

	t.Run("empty hrp", func(t *testing.T) {
		p := validProposal()
		p.Hrp = ""
		require.Error(t, p.ValidateBasic())
	})
}

// ensure long strings/oddball input doesn't panic ValidateBasic (defense-in-depth check)
func TestUpdateHrpIbcChannelProposal_ValidateBasic_OversizedTitle(t *testing.T) {
	p := &types.UpdateHrpIbcChannelProposal{
		Title:             strings.Repeat("a", 1000),
		Description:       "description",
		Hrp:               "osmo",
		SourceChannel:     "channel-0",
		IcsToHeightOffset: 100,
		IcsToTimeOffset:   0,
	}
	require.Error(t, p.ValidateBasic())
}
