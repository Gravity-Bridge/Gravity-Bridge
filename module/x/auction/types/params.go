package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

// Default values
const (
	DefaultAuctionEpoch  uint64 = 10
	DefaultAuctionPeriod uint64 = 10
	DefaultMinBidAmount  uint64 = 1000
	DefaultBidGap        uint64 = 100
)

var _ paramtypes.ParamSet = (*Params)(nil)

// ParamKeyTable for auction module
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// Param store keys
var (
	KeyAuctionEpoch  = []byte("AuctionEpoch")
	KeyAuctionPeriod = []byte("AuctionPeriod")
	KeyMinBidAmount  = []byte("MinBidAmount")
	KeyBidGap        = []byte("BidGap")
	KeyAuctionRate   = []byte("AuctionRate")
	KeyAllowTokens   = []byte("AllowTokens")
)

// NewParams creates a new Params object
func NewParams(auctionEpoch uint64, auctionPeriod uint64, minBidAmount uint64, bidGap uint64, auctionRate sdk.Dec, allowTokens map[string]bool) Params {
	return Params{
		AuctionEpoch:  auctionEpoch,
		AuctionPeriod: auctionPeriod,
		MinBidAmount:  minBidAmount,
		BidGap:        bidGap,
		AuctionRate:   auctionRate,
		AllowTokens:   allowTokens,
	}
}

// DefaultParams defines the default parameters for the GravityBridge auction module
func DefaultParams() Params {
	return NewParams(DefaultAuctionEpoch, DefaultAuctionPeriod, DefaultMinBidAmount, DefaultBidGap, sdk.NewDec(1), make(map[string]bool))
}

// ParamSetPairs get the params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(KeyAuctionEpoch, &p.AuctionEpoch, isPositive),
		paramtypes.NewParamSetPair(KeyAuctionPeriod, &p.AuctionPeriod, isPositive),
		paramtypes.NewParamSetPair(KeyMinBidAmount, &p.MinBidAmount, isPositive),
		paramtypes.NewParamSetPair(KeyBidGap, &p.BidGap, isPositive),
		paramtypes.NewParamSetPair(KeyAuctionRate, &p.AuctionRate, isDecPositive),
		paramtypes.NewParamSetPair(KeyAllowTokens, &p.AllowTokens, nil),
	}
}

// Validate checks that the parameters have valid values.
func (p Params) Validate() error {
	if p.AuctionEpoch <= 0 {
		return fmt.Errorf("auction epoch should be positive")
	}
	if p.AuctionPeriod <= 0 {
		return fmt.Errorf("auction period should be positive")
	}
	if p.MinBidAmount <= 0 {
		return fmt.Errorf("minimum bid amount should be positive")
	}
	if p.BidGap <= 0 {
		return fmt.Errorf("bid gap should be positive")
	}
	if p.AuctionRate.IsNegative() || p.AuctionRate.GT(sdk.NewDec(1)) {
		return fmt.Errorf("auction rate should be between 0 and 1")
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

func isDecPositive(i interface{}) error {
	ival, ok := i.(sdk.Dec)
	if !ok {
		return fmt.Errorf("parameter not accepted: %T", i)
	}

	if ival.IsNegative() {
		return fmt.Errorf("parameter must be positive: %d", ival)
	}
	return nil
}
