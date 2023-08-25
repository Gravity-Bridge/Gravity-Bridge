package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

// Default Params values
var (
	DefaultAuctionLength        uint64 = 100 // TODO: Determine default length of auctions
	DefaultMinBidAmount         uint64 = 1   // TODO: Determine default min bid amount ugraviton
	DefaultMinBidFee            uint64 = 1   // TODO: Determine default min bid fee ugraviton
	DefaultNonAuctionableTokens        = []string{"ugraviton"}
	DefaultBurnWinningBids             = false
)

// Param store keys
var (
	ParamsStoreKeyAuctionLength        = []byte("AuctionLength")
	ParamsStoreKeyMinBidAmount         = []byte("MinBidAmount")
	ParamsStoreKeyMinBidFee            = []byte("MinBidFee")
	ParamsStoreKeyNonAuctionableTokens = []byte("NonAuctionableTokens")
	ParamsStoreKeyBurnWinningBids      = []byte("BurnWinningBids")
)

var _ paramtypes.ParamSet = (*Params)(nil)

// ParamKeyTable for auction module
func ParamKeyTable() paramtypes.KeyTable {
	// nolint: exhaustruct
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// NewParams creates a new Params object
func NewParams(
	auctionLength uint64,
	minBidAmount uint64,
	minBidFee uint64,
	nonAuctionableTokens []string,
	burnWinningBids bool,
) Params {
	return Params{
		AuctionLength:        auctionLength,
		MinBidAmount:         minBidAmount,
		MinBidFee:            minBidFee,
		NonAuctionableTokens: nonAuctionableTokens,
		BurnWinningBids:      burnWinningBids,
	}
}

// DefaultParams defines the default parameters for the GravityBridge auction module
func DefaultParams() Params {
	return NewParams(
		DefaultAuctionLength,
		DefaultMinBidAmount,
		DefaultMinBidFee,
		DefaultNonAuctionableTokens,
		DefaultBurnWinningBids,
	)
}

// ParamSetPairs get the params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(ParamsStoreKeyAuctionLength, &p.AuctionLength, isPositive),
		paramtypes.NewParamSetPair(ParamsStoreKeyMinBidAmount, &p.MinBidAmount, isNonNegative),
		paramtypes.NewParamSetPair(ParamsStoreKeyMinBidFee, &p.MinBidFee, isNonNegative),
		paramtypes.NewParamSetPair(ParamsStoreKeyNonAuctionableTokens, &p.NonAuctionableTokens, allValidDenoms),
		paramtypes.NewParamSetPair(ParamsStoreKeyBurnWinningBids, &p.BurnWinningBids, isBoolean),
	}
}

// Validate checks that the parameters have valid values.
func (p Params) ValidateBasic() error {
	if err := isPositive(p.AuctionLength); err != nil {
		return sdkerrors.Wrap(ErrInvalidParams, "auction length must be positive")
	}

	if err := isNonNegative(p.MinBidAmount); err != nil {
		return sdkerrors.Wrap(ErrInvalidParams, "min bid amount must be non-negative")
	}

	if err := isNonNegative(p.MinBidFee); err != nil {
		return sdkerrors.Wrap(ErrInvalidParams, "bid fee basis points must be non-negative")
	}

	if err := allValidDenoms(p.NonAuctionableTokens); err != nil {
		return sdkerrors.Wrap(ErrInvalidParams, "all non auctionable tokens must be valid denoms")
	}

	if err := isBoolean(p.BurnWinningBids); err != nil {
		return sdkerrors.Wrap(ErrInvalidParams, "burn winning bids must be a boolean")
	}

	return nil
}

func allValidDenoms(i interface{}) error {
	ival, ok := i.([]string)
	if !ok {
		return fmt.Errorf("parameter not accepted: %T", i)
	}
	for j, s := range ival {
		if err := isValidDenom(s); err != nil {
			return fmt.Errorf("string %d is invalid: %s %v", j, s, err)
		}
	}
	return nil
}

func isNonNegative(i interface{}) error {
	ival, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("parameter not accepted: %T", i)
	}

	if ival < 0 {
		return fmt.Errorf("parameter must be non-negative: %d", ival)
	}
	return nil
}

func isPositive(i interface{}) error {
	ival, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("parameter not accepted: %T", i)
	}

	if ival <= 0 {
		return fmt.Errorf("parameter must be positive: %d", ival)
	}
	return nil
}

// Determines that the Dec is within (0, 1] (0 exclusive, 1 inclusive)
func isDecZeroToOne(i interface{}) error {
	ival, ok := i.(sdk.Dec)
	if !ok {
		return fmt.Errorf("parameter not accepted: %T", i)
	}

	if ival.IsNegative() {
		return fmt.Errorf("parameter must be non-negative: %d", ival)
	}
	if ival.GT(sdk.OneDec()) {
		return fmt.Errorf("parameter must not be larger than 1: %d", ival)
	}
	return nil
}

func isValidDenom(i interface{}) error {
	ival, ok := i.(string)
	if !ok {
		return fmt.Errorf("parameter not accepted: %T", i)
	}

	if err := sdk.ValidateDenom(ival); err != nil {
		return fmt.Errorf("parameter must be a valid denom: %s, %v", ival, err)
	}
	return nil
}

func isBoolean(i interface{}) error {
	_, ok := i.(bool)
	if !ok {
		return fmt.Errorf("parameter not accepted: %T", i)
	}
	return nil
}
