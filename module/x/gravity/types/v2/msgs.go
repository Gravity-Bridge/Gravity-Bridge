package v2

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	// nolint: exhaustruct
	_ sdk.Msg = &MsgAirdropProposal{}
	// nolint: exhaustruct
	_ sdk.Msg = &MsgIBCMetadataProposal{}
	// nolint: exhaustruct
	_ sdk.Msg = &MsgUnhaltBridgeProposal{}
)

// MsgAirdropProposal defines a message for submitting an airdrop proposal
// ======================================================

// ValidateBasic performs stateless checks
func (msg *MsgAirdropProposal) ValidateBasic() (err error) {
	_, err = sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		return err
	}
	return msg.Proposal.ValidateBasic()
}

// GetSigners defines whose signature is required
func (msg MsgAirdropProposal) GetSigners() []sdk.AccAddress {
	acc, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		panic("Invalid authority for MsgAirdropProposal: " + err.Error())
	}
	return []sdk.AccAddress{acc}
}

// MsgIBCMetadataProposal defines a message for submitting an IBC metadata proposal
// ======================================================

// ValidateBasic performs stateless checks
func (msg *MsgIBCMetadataProposal) ValidateBasic() (err error) {
	_, err = sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		return err
	}
	return msg.Proposal.ValidateBasic()
}

// GetSigners defines whose signature is required
func (msg MsgIBCMetadataProposal) GetSigners() []sdk.AccAddress {
	acc, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		panic("Invalid authority for MsgAirdropProposal: " + err.Error())
	}
	return []sdk.AccAddress{acc}
}

// MsgUnhaltBridgeProposal defines a message for submitting an unhalt bridge proposal
// ======================================================

// ValidateBasic performs stateless checks
func (msg *MsgUnhaltBridgeProposal) ValidateBasic() (err error) {
	_, err = sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		return err
	}
	return msg.Proposal.ValidateBasic()
}

// GetSigners defines whose signature is required
func (msg MsgUnhaltBridgeProposal) GetSigners() []sdk.AccAddress {
	acc, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		panic("Invalid authority for MsgAirdropProposal: " + err.Error())
	}
	return []sdk.AccAddress{acc}
}
