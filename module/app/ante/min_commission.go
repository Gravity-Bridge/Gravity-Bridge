package ante

import (
	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/authz"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// MinCommissionDecorator was originally included in the Juno project at github.com/CosmosContracts/juno/app/sdkante.go
// This version was added to Juno by github user the-frey https://github.com/the-frey and Giancarlos Salas https://github.com/giansalex
// The Juno project obtained their initial version of this decorator from github.com/public-awesome/stargaze with original
// author Jorge Hernandez https://github.com/jhernandezb
type MinCommissionDecorator struct {
	cdc codec.Codec
}

func NewMinCommissionDecorator(cdc codec.Codec) MinCommissionDecorator {
	return MinCommissionDecorator{cdc}
}

func (min MinCommissionDecorator) AnteHandle(
	ctx sdk.Context, tx sdk.Tx,
	simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	msgs := tx.GetMsgs()

	validMsg := func(m sdk.Msg) error {
		minCommissionRate := sdk.NewDecWithPrec(10, 2)
		switch msg := m.(type) {
		case *stakingtypes.MsgCreateValidator:
			// prevent new validators joining the set with
			// commission set below 10%
			c := msg.Commission
			if c.Rate.LT(minCommissionRate) {
				return errorsmod.Wrap(sdkerrors.ErrUnauthorized, "commission can't be lower than 10%")
			}
			if c.MaxRate.LT(minCommissionRate) {
				return errorsmod.Wrap(sdkerrors.ErrUnauthorized, "commission max rate can't be lower than 10%")
			}
		case *stakingtypes.MsgEditValidator:
			// if commission rate is nil, it means only
			// other fields are affected - skip
			if msg.CommissionRate == nil {
				break
			}
			if msg.CommissionRate.LT(minCommissionRate) {
				return errorsmod.Wrap(sdkerrors.ErrUnauthorized, "commission can't be lower than 10%")
			}
		}

		return nil
	}

	validAuthz := func(execMsg *authz.MsgExec) error {
		for _, v := range execMsg.Msgs {
			var innerMsg sdk.Msg
			err := min.cdc.UnpackAny(v, &innerMsg)
			if err != nil {
				return errorsmod.Wrapf(sdkerrors.ErrUnauthorized, "cannot unmarshal authz exec msgs")
			}

			err = validMsg(innerMsg)
			if err != nil {
				return err
			}
		}

		return nil
	}

	for _, m := range msgs {
		if msg, ok := m.(*authz.MsgExec); ok {
			if err := validAuthz(msg); err != nil {
				return ctx, err
			}
			continue
		}

		// validate normal msgs
		err = validMsg(m)
		if err != nil {
			return ctx, err
		}
	}

	return next(ctx, tx, simulate)
}
