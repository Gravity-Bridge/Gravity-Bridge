package keeper

import (
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

func NewGravityProposalHandler(k Keeper) govtypes.Handler {
	return func(ctx sdk.Context, content govtypes.Content) error {
		switch c := content.(type) {
		case *types.UpdateAllowListProposal:
			return k.HandleUpdateAllowListProposal(ctx, c)
		default:
			return sdkerrors.Wrapf(sdkerrors.ErrUnknownRequest, "unrecognized Gravity proposal content type: %T", c)
		}
	}
}

func (k Keeper) HandleUpdateAllowListProposal(ctx sdk.Context, p *types.UpdateAllowListProposal) error {
	params := k.GetParams(ctx)

	if p.ActionType == types.ActionType_ACTION_TYPE_ADD_TOKEN {
		params.AllowTokens[p.Denom] = true
	} else {
		delete(params.AllowTokens, p.Denom)
	}

	k.SetParams(ctx, params)

	return nil
}
