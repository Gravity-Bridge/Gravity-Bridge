package types

import (
	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authlegacy "github.com/cosmos/cosmos-sdk/x/auth/migrations/legacytx"
)

// nolint: exhaustruct
var (
	_ sdk.Msg = &MsgBid{}
)

const (
	TypeMsgBid = "bid"
)

// nolint: exhaustruct
var (
	_ sdk.Msg              = &MsgBid{}
	_ authlegacy.LegacyMsg = &MsgBid{}
)

// NewMsgBid returns a new msgSetOrchestratorAddress
func NewMsgBid(auctionId uint64, bidder string, amount uint64, fee uint64) *MsgBid {
	return &MsgBid{
		AuctionId: auctionId,
		Bidder:    bidder,
		Amount:    amount,
		BidFee:    fee,
	}
}

// Route should return the name of the module
func (msg *MsgBid) Route() string { return RouterKey }

// Type should return the action
func (msg *MsgBid) Type() string { return TypeMsgBid }

// ValidateBasic performs stateless checks
func (msg *MsgBid) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Bidder); err != nil {
		return errorsmod.Wrapf(ErrInvalidBid, "invalid bidder: %v", err)
	}
	if msg.Amount == 0 {
		return errorsmod.Wrap(ErrInvalidBid, "bid amount must be positive")
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
