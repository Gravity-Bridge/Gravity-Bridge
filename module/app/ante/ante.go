package ante

import (
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	"github.com/cosmos/cosmos-sdk/x/authz"
	"github.com/cosmos/cosmos-sdk/x/bank/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	ibcante "github.com/cosmos/ibc-go/v3/modules/core/ante"
	ibckeeper "github.com/cosmos/ibc-go/v3/modules/core/keeper"
)

// NewAnteHandler Constructs a new sdk.AnteHandler for the Gravity app.
// AnteHandlers are functions which validate Txs before their contained Msgs are executed, AnteHandlers are constructed
// from a chain of AnteDecorators, the final AnteDecorator in the chain being sdkante.Terminator.
// This custom AnteHandler constructor loosely chains together the pre-Terminator-ed default sdk AnteHandler
// with additional AnteDecorators. This complicated process is desirable because:
// 1. the default sdk AnteHandler can change on any upgrade (so we do not want to have a stale list of AnteDecorators),
// 2. it is not possible to modify an AnteHandler once it is constructed
func NewAnteHandler(options sdkante.HandlerOptions, ibcKeeper *ibckeeper.Keeper, cdc codec.BinaryCodec) (*sdk.AnteHandler, error) {
	// Call the default sdk antehandler constructor to avoid auditing our changes in the future
	baseAnteHandler, err := sdkante.NewAnteHandler(options)
	if err != nil {
		return nil, sdkerrors.Wrap(err, "Unable to create baseAnteHanlder")
	}

	// Create additional AnteDecorators to chain together
	ibcAnteDecorator := ibcante.NewAnteDecorator(ibcKeeper)
	minCommissionDecorator := NewMinCommissionDecorator(cdc)

	addlDecorators := []sdk.AnteDecorator{ibcAnteDecorator, minCommissionDecorator}
	// Chain together and terminate the input decorators array
	customHandler := sdk.ChainAnteDecorators(addlDecorators...)

	// Create and return a function which ties the two handlers together
	fullHandler := chainHandlers(baseAnteHandler, customHandler)
	return &fullHandler, nil
}

// Loosely chain together two AnteHandlers by first calling handler, then calling secondHandler with the result
// NOTE: This order is important due to the way GasMeter works, see sdk.ChainAnteDecorators for more info
// This process is necessary because AnteHandlers are immutable once constructed, and the sdk does not expose its
// curated list of default AnteDecorators.
func chainHandlers(handler sdk.AnteHandler, secondHandler sdk.AnteHandler) sdk.AnteHandler {
	return func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		// First run the original AnteHandler (likely the default sdk chain of decorators)
		newCtx, err := handler(ctx, tx, simulate)
		if err != nil { // Return early from an error
			return newCtx, err
		}

		// Next run the second handler
		return secondHandler(newCtx, tx, simulate)
	}
}

// MinCommissionDecorator was originally included in the Juno project at github.com/CosmosContracts/juno/app/ante.go
// This version was added to Juno by github user the-frey https://github.com/the-frey and Giancarlos Salas https://github.com/giansalex
// The Juno project obtained their initial version of this decorator from github.com/public-awesome/stargaze with original
// author Jorge Hernandez https://github.com/jhernandezb
type MinCommissionDecorator struct {
	cdc codec.BinaryCodec
}

func NewMinCommissionDecorator(cdc codec.BinaryCodec) MinCommissionDecorator {
	return MinCommissionDecorator{cdc}
}

func (min MinCommissionDecorator) AnteHandle(
	ctx sdk.Context, tx sdk.Tx,
	simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	msgs := tx.GetMsgs()
	minCommissionRate := sdk.NewDecWithPrec(5, 2)

	validMsg := func(m sdk.Msg) error {
		switch msg := m.(type) {
		case *stakingtypes.MsgCreateValidator:
			// prevent new validators joining the set with
			// commission set below 5%
			c := msg.Commission
			if c.Rate.LT(minCommissionRate) {
				return sdkerrors.Wrap(sdkerrors.ErrUnauthorized, "commission can't be lower than 5%")
			}
		case *stakingtypes.MsgEditValidator:
			// if commission rate is nil, it means only
			// other fields are affected - skip
			if msg.CommissionRate == nil {
				break
			}
			if msg.CommissionRate.LT(minCommissionRate) {
				return sdkerrors.Wrap(sdkerrors.ErrUnauthorized, "commission can't be lower than 5%")
			}
		}

		return nil
	}

	validAuthz := func(execMsg *authz.MsgExec) error {
		for _, v := range execMsg.Msgs {
			var innerMsg sdk.Msg
			err := min.cdc.UnpackAny(v, &innerMsg)
			if err != nil {
				return sdkerrors.Wrapf(sdkerrors.ErrUnauthorized, "cannot unmarshal authz exec msgs")
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
