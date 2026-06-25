package v2

import (
	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	// nolint: exhaustruct
	_ sdk.Msg = &MsgAirdropProposal{}
	// nolint: exhaustruct
	_ sdk.Msg = &MsgUnhaltBridgeProposal{}
	// nolint: exhaustruct
	_ sdk.Msg = &MsgUpdateParamsProposal{}
	// nolint: exhaustruct
	_ sdk.Msg = &MsgCosmosBridgeableTokensProposal{}
)

// MsgUpdateParamsProposal defines a message for updating gravity params via x/gov v1
// ======================================================

// ValidateBasic performs stateless checks
func (msg *MsgUpdateParamsProposal) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return errorsmod.Wrap(err, "invalid authority")
	}
	seen := make(map[string]struct{}, len(msg.ParamUpdates))
	for _, p := range msg.ParamUpdates {
		if _, dup := seen[p.Key]; dup {
			return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest,
				"duplicate param key in MsgUpdateParamsProposal: %q; "+
					"each param key may appear at most once per proposal",
				p.Key)
		}
		seen[p.Key] = struct{}{}
	}
	return nil
}

// GetSigners defines whose signature is required
func (msg MsgUpdateParamsProposal) GetSigners() []sdk.AccAddress {
	acc, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		panic("Invalid authority for MsgUpdateParamsProposal: " + err.Error())
	}
	return []sdk.AccAddress{acc}
}

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

// MsgCosmosBridgeableTokensProposal defines a message for submitting a cosmos bridgeable tokens proposal
// ======================================================

// ValidateBasic performs stateless checks
func (msg *MsgCosmosBridgeableTokensProposal) ValidateBasic() (err error) {
	_, err = sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		return err
	}
	return msg.Proposal.ValidateBasic()
}

// GetSigners defines whose signature is required
func (msg MsgCosmosBridgeableTokensProposal) GetSigners() []sdk.AccAddress {
	acc, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		panic("Invalid authority for MsgCosmosBridgeableTokensProposal: " + err.Error())
	}
	return []sdk.AccAddress{acc}
}
