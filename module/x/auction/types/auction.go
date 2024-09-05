package types

import (
	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (a AuctionPeriod) ValidateBasic() error {
	if a.EndBlockHeight <= a.StartBlockHeight {
		return errorsmod.Wrapf(ErrInvalidAuctionPeriod, "end block height (%d) must be after start block height (%d)", a.EndBlockHeight, a.StartBlockHeight)
	}
	if a.StartBlockHeight == 0 {
		return errorsmod.Wrap(ErrInvalidAuctionPeriod, "start block height must be positive")
	}
	// Similarly the ID is valid based on the type
	return nil
}

func NewAuction(id uint64, amount sdk.Coin) Auction {
	return Auction{
		Id:         id,
		Amount:     amount,
		HighestBid: nil,
	}
}
func (a Auction) ValidateBasic() error {
	if err := a.Amount.Validate(); err != nil {
		return errorsmod.Wrapf(ErrInvalidAuction, "invalid amount: %v", err)
	}

	if a.HighestBid != nil {
		if err := a.HighestBid.ValidateBasic(); err != nil {
			return errorsmod.Wrapf(ErrInvalidAuction, "invalid bid: %v", err)
		}
	}
	// The ID is valid based on the type
	return nil
}

func (b Bid) ValidateBasic() error {
	if err := isPositive(b.BidAmount); err != nil {
		return errorsmod.Wrap(ErrInvalidBid, "bid amount must be positive")
	}
	if _, err := sdk.AccAddressFromBech32(b.BidderAddress); err != nil {
		return errorsmod.Wrapf(ErrInvalidBid, "invalid bidder: %v", err)
	}

	return nil
}
