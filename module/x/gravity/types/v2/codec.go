package v2

import (
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

// RegisterInterfaces registers the interfaces for the proto stuff
// nolint: exhaustruct
func RegisterInterfaces(registry types.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgAirdropProposal{},
		&MsgUnhaltBridgeProposal{},
		&MsgUpdateParamsProposal{},
		&MsgSetCosmosBridgeableTokensProposal{},
		&MsgDeleteCosmosBridgeableTokensProposal{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}
