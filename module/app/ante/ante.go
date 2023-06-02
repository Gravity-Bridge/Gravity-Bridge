package ante

import (
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	feegrantkeeper "github.com/cosmos/cosmos-sdk/x/feegrant/keeper"
	ibcante "github.com/cosmos/ibc-go/v4/modules/core/ante"
	ibckeeper "github.com/cosmos/ibc-go/v4/modules/core/keeper"
)

// NewAnteHandler Constructs a new sdk.AnteHandler for the Gravity app.
// AnteHandlers are functions which validate Txs before their contained Msgs are executed, AnteHandlers are constructed
// from a chain of AnteDecorators, the final AnteDecorator in the chain being sdkante.Terminator.
// This custom AnteHandler constructor loosely chains together the pre-Terminator-ed default sdk AnteHandler
// with additional AnteDecorators. This complicated process is desirable because:
// 1. the default sdk AnteHandler can change on any upgrade (so we do not want to have a stale list of AnteDecorators),
// 2. it is not possible to modify an AnteHandler once it is constructed
func NewAnteHandler(
	options sdkante.HandlerOptions,
	gravityKeeper *keeper.Keeper,
	accountKeeper *authkeeper.AccountKeeper,
	bankKeeper *bankkeeper.BaseKeeper,
	feegrantKeeper *feegrantkeeper.Keeper,
	ibcKeeper *ibckeeper.Keeper,
	cdc codec.BinaryCodec,
) (*sdk.AnteHandler, error) {
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
