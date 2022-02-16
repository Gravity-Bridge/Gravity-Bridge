package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	channelkeeper "github.com/cosmos/ibc-go/v2/modules/core/04-channel/keeper"
	ibcante "github.com/cosmos/ibc-go/v2/modules/core/ante"
)

// Constructs a new sdk.AnteHandler for the Gravity app.
// AnteHandlers are functions which validate Txs before their contained Msgs are executed, AnteHandlers are constructed
// from a chain of AnteDecorators, the final AnteDecorator in the chain being ante.Terminator.
// This custom AnteHandler constructor loosely chains together the pre-Terminator-ed default sdk AnteHandler
// with additional AnteDecorators. This complicated process is desirable because:
//   1. the default sdk AnteHandler can change on any upgrade (so we do not want to have a stale list of AnteDecorators),
//   2. it is not possible to modify an AnteHandler once it is constructed
func newAnteHandler(options ante.HandlerOptions, ibcChannelKeeper channelkeeper.Keeper) (*sdk.AnteHandler, error) {
	// Call the default sdk antehandler constructor to avoid auditing our changes in the future
	baseAnteHandler, err := ante.NewAnteHandler(options)
	if err != nil {
		return nil, sdkerrors.Wrap(err, "Unable to create baseAnteHanlder")
	}

	// Create additional AnteDecorators to chain together
	ibcAnteDecorator := ibcante.NewAnteDecorator(ibcChannelKeeper)
	addlDecorators := []sdk.AnteDecorator{ibcAnteDecorator}
	// Chain together and terminate the input decorators array
	customHandler := sdk.ChainAnteDecorators(addlDecorators...)

	// Create and return a function which ties the two handlers together
	fullHandler := addDecoratorsToHandler(baseAnteHandler, customHandler)
	return &fullHandler, nil
}

// Loosely chain together two AnteHandlers by first calling handler, then calling secondHandler with the result
// This is necessary because AnteHandlers are immutable once constructed, and the sdk does not expose its curated list
// of default AnteDecorators.
func addDecoratorsToHandler(handler sdk.AnteHandler, secondHandler sdk.AnteHandler) sdk.AnteHandler {
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
