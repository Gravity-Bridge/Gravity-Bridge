package keeper

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/types"
)

func (k Keeper) HandleUpdateHrpIbcChannelProposal(ctx sdk.Context, p *types.UpdateHrpIbcChannelProposal) error {
	err := types.ValidateHrp(p.Hrp)
	if err != nil {
		return err
	}

	_, found := k.channelKeeper.GetChannel(ctx, k.tk.GetPort(ctx), p.SourceChannel)

	if !found {
		return errorsmod.Wrap(types.ErrInvalidIBCData, fmt.Sprintf("channel not found: %s", p.SourceChannel))
	}

	return k.setHrpIbcRecord(ctx, types.HrpIbcRecord{
		Hrp:               p.Hrp,
		SourceChannel:     p.SourceChannel,
		IcsToHeightOffset: p.IcsToHeightOffset,
		IcsToTimeOffset:   p.IcsToTimeOffset,
	})
}
