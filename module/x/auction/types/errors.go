package types

import (
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// x/auction module sentinel errors
var (
	ErrNoPreviousAuctionPeriod = sdkerrors.Register(ModuleName, 1, "Previous auction period not found")
	ErrInvalidBidAmountGap     = sdkerrors.Register(ModuleName, 2, "Invalid bid amount gap")
	ErrAuctionPeriodNotFound   = sdkerrors.Register(ModuleName, 3, "Auction period not found")
	ErrAuctionNotFound         = sdkerrors.Register(ModuleName, 4, "Auction not found")
)
