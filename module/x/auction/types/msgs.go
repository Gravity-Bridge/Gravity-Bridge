package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// nolint: exhaustruct
var (
	_ sdk.Msg = &MsgBid{}
)

// NewMsgBid returns a new msgSetOrchestratorAddress
func NewMsgBid(auctionId uint64, bidder string, amount sdk.Coin) *MsgBid {
	return &MsgBid{
		AuctionId: auctionId,
		Bidder:    bidder,
		Amount:    amount,
	}
}

// Route should return the name of the module
func (msg *MsgBid) Route() string { return RouterKey }

// Type should return the action
func (msg *MsgBid) Type() string { return "bid" }

// ValidateBasic performs stateless checks
func (msg *MsgBid) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Bidder); err != nil {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, msg.Bidder)
	}
	if !msg.Amount.IsValid() {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidCoins, msg.Amount.String())
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (msg *MsgBid) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}

// GetSigners defines whose signature is required
func (msg *MsgBid) GetSigners() []sdk.AccAddress {
	acc, err := sdk.AccAddressFromBech32(msg.Bidder)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{sdk.AccAddress(acc)}
}
