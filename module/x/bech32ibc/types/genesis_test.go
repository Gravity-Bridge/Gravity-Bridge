package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/types"
)

func TestDefaultGenesis(t *testing.T) {
	gs := types.DefaultGenesis()

	require.Equal(t, "gravity", gs.NativeHRP)
	require.Empty(t, gs.HrpIBCRecords)
}

func TestGenesisState_Validate(t *testing.T) {
	// nolint: exhaustruct
	require.NoError(t, types.DefaultGenesis().Validate())

	// nolint: exhaustruct
	gs := types.GenesisState{
		NativeHRP: "gravity",
		HrpIBCRecords: []types.HrpIbcRecord{
			// nolint: exhaustruct
			{Hrp: "osmo", SourceChannel: "channel-0"},
		},
	}
	require.NoError(t, gs.Validate())
}
