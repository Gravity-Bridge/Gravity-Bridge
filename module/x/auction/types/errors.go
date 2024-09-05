package types

import (
	errorsmod "cosmossdk.io/errors"
)

// x/auction module sentinel errors
var (
	ErrInvalidAuctionPeriod = errorsmod.Register(ModuleName, 1, "Invalid Auction Period")
	ErrAuctionNotFound      = errorsmod.Register(ModuleName, 2, "Auction not found")
	ErrInvalidAuction       = errorsmod.Register(ModuleName, 3, "Invalid Auction")
	ErrInvalidBid           = errorsmod.Register(ModuleName, 4, "Invalid bid")
	ErrBidTooLow            = errorsmod.Register(ModuleName, 5, "Bid amount too low")
	ErrInvalidParams        = errorsmod.Register(ModuleName, 6, "Invalid Params")
	ErrDuplicateAuction     = errorsmod.Register(ModuleName, 7, "Duplicate Auction")
	ErrFundReturnFailure    = errorsmod.Register(ModuleName, 8, "Failed to return funds to bidder")
	ErrBidCollectionFailure = errorsmod.Register(ModuleName, 9, "Failed to collect bid")
	ErrAwardFailure         = errorsmod.Register(ModuleName, 10, "Failed to award auction to highest bidder")
	ErrDisabledModule       = errorsmod.Register(ModuleName, 11, "Auction Module is not currently enabled")
)
