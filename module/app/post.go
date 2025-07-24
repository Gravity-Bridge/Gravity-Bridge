package app

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type LoggingPostDecorator struct {
	message string
}

func NewLoggingPostDecorator(message string) LoggingPostDecorator {
	return LoggingPostDecorator{
		message: message,
	}
}

func (ld LoggingPostDecorator) PostHandle(ctx sdk.Context, tx sdk.Tx, simulate, success bool, next sdk.PostHandler) (newCtx sdk.Context, err error) {
	fmt.Println(ld.message)
	return next(ctx, tx, simulate, success)
}
