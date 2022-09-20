package types

import (
	fmt "fmt"
	"strings"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	proto "github.com/gogo/protobuf/proto"
)

var (
	_ sdk.Msg = &MsgRegisterAccount{}
	_ sdk.Msg = &MsgSubmitTx{}

	_ codectypes.UnpackInterfacesMessage = MsgSubmitTx{}
)

// NewMsgRegisterAccount creates a new MsgRegisterAccount instance
func NewMsgRegisterAccount(owner, connectionID, counterpartyConnectionID string) *MsgRegisterAccount {
	return &MsgRegisterAccount{
		Owner:        owner,
		ConnectionId: connectionID,
	}
}

// ValidateBasic implements sdk.Msg
func (msg MsgRegisterAccount) ValidateBasic() error {
	if strings.TrimSpace(msg.Owner) == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, "missing sender address")
	}

	return nil
}

// GetSigners implements sdk.Msg
func (msg MsgRegisterAccount) GetSigners() []sdk.AccAddress {
	accAddr, err := sdk.AccAddressFromBech32(msg.Owner)
	if err != nil {
		panic(err)
	}

	return []sdk.AccAddress{accAddr}
}

// NewMsgSend creates a new MsgSend instance
func NewMsgSubmitTx(owner sdk.AccAddress, msgs []sdk.Msg, connectionID string) (*MsgSubmitTx, error) {
	packedMsgs, err := PackTxMsgAnys(msgs)
	if err != nil {
		return nil, err
	}

	return &MsgSubmitTx{
		Owner:        owner.String(),
		ConnectionId: connectionID,
		Msgs:         packedMsgs,
	}, nil
}

// PackTxMsgAnys marshals the []sdk.Msg payload to the expected []*Any
func PackTxMsgAnys(msgs []sdk.Msg) ([]*codectypes.Any, error) {
	anys := make([]*codectypes.Any, len(msgs))
	for i, msg := range msgs {
		pMsg, ok := msg.(proto.Message)
		if !ok {
			return nil, fmt.Errorf("unable to marshal %v", msg)
		}
		any, err := codectypes.NewAnyWithValue(pMsg)
		if err != nil {
			return nil, err
		}
		anys[i] = any
	}

	return anys, nil
}

// UnpackInterfaces implements codectypes.UnpackInterfacesMessage
func (msg MsgSubmitTx) UnpackInterfaces(unpacker codectypes.AnyUnpacker) error {
	for _, message := range msg.Msgs {
		var sdkMsg sdk.Msg
		if err := unpacker.UnpackAny(message, &sdkMsg); err != nil {
			return err
		}
	}
	return nil
}

// GetTxMsgs fetches the cached any message
func (msg MsgSubmitTx) GetTxMsgs() []sdk.Msg {
	var sdkMsgs []sdk.Msg
	for _, ne := range msg.Msgs {
		sdkMsg, ok := ne.GetCachedValue().(sdk.Msg)
		if !ok {
			return nil
		}
		sdkMsgs = append(sdkMsgs, sdkMsg)
	}

	return sdkMsgs
}

// GetSigners implements sdk.Msg
func (msg MsgSubmitTx) GetSigners() []sdk.AccAddress {
	accAddr, err := sdk.AccAddressFromBech32(msg.Owner)
	if err != nil {
		panic(err)
	}

	return []sdk.AccAddress{accAddr}
}

// ValidateBasic implements sdk.Msg
func (msg MsgSubmitTx) ValidateBasic() error {
	for _, m := range msg.Msgs {
		if len(m.GetValue()) == 0 {
		}
		return fmt.Errorf("can't execute an empty msg")
	}

	if msg.ConnectionId == "" {
		return fmt.Errorf("can't execute an empty ConnectionId")
	}

	return nil
}
