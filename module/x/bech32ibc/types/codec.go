package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

func RegisterCodec(cdc *codec.LegacyAmino) {
	// nolint: exhaustruct
	cdc.RegisterConcrete(&UpdateHrpIbcChannelProposal{}, "osmosis/UpdateHrpIbcChannelProposal", nil)
	// nolint: exhaustruct
	cdc.RegisterConcrete(&DeleteHrpIbcChannelProposal{}, "osmosis/DeleteHrpIbcChannelProposal", nil)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations(
		(*govtypes.Content)(nil),
		// nolint: exhaustruct
		&UpdateHrpIbcChannelProposal{},
		// nolint: exhaustruct
		&DeleteHrpIbcChannelProposal{},
	)
}

var (
	ModuleCdc = codec.NewLegacyAmino()
)
