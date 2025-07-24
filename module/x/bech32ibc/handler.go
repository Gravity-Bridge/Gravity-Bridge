package bech32ibc

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/types"
)

func NewBech32IBCProposalHandler(k keeper.Keeper) govtypes.Handler {
	return func(ctx sdk.Context, content govtypes.Content) error {
		switch c := content.(type) {
		case *types.UpdateHrpIbcChannelProposal:
			return handleUpdateHrpIbcChannelProposal(ctx, k, c)

		default:
			return errorsmod.Wrapf(sdkerrors.ErrUnknownRequest, "unrecognized bech32 ibc proposal content type: %T", c)
		}
	}
}

func handleUpdateHrpIbcChannelProposal(ctx sdk.Context, k keeper.Keeper, p *types.UpdateHrpIbcChannelProposal) error {
	return k.HandleUpdateHrpIbcChannelProposal(ctx, p)
}
