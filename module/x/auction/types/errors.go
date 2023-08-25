package types

import (
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// x/auction module sentinel errors
var (
	ErrInvalidAuctionPeriod = sdkerrors.Register(ModuleName, 1, "Invalid Auction Period")
	ErrAuctionNotFound      = sdkerrors.Register(ModuleName, 2, "Auction not found")
	ErrInvalidAuction       = sdkerrors.Register(ModuleName, 3, "Invalid Auction")
	ErrInvalidBid           = sdkerrors.Register(ModuleName, 4, "Invalid bid")
	ErrBidTooLow            = sdkerrors.Register(ModuleName, 5, "Bid amount too low")
	ErrInvalidParams        = sdkerrors.Register(ModuleName, 6, "Invalid Params")
	ErrDuplicateAuction     = sdkerrors.Register(ModuleName, 7, "Duplicate Auction")
	ErrFundReturnFailure    = sdkerrors.Register(ModuleName, 8, "Failed to return funds to bidder")
	ErrBidCollectionFailure = sdkerrors.Register(ModuleName, 9, "Failed to collect bid")
	ErrAwardFailure         = sdkerrors.Register(ModuleName, 10, "Failed to award auction to highest bidder")
)
