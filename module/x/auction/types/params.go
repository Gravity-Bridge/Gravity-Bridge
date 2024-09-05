package types

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/config"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

// Default Params values
var (
	DefaultAuctionLength        uint64 = 85600 // This default should be longer than the governance period to allow for disabling the auction module, determined with Proposal #204
	DefaultMinBidFee            uint64 = 3110  // This default was determined with Proposal #203
	DefaultNonAuctionableTokens        = []string{"ugraviton"}
	DefaultBurnWinningBids             = true
	DefaultEnabled                     = true
)

// Param store keys
var (
	ParamsStoreKeyAuctionLength        = []byte("AuctionLength")
	ParamsStoreKeyMinBidFee            = []byte("MinBidFee")
	ParamsStoreKeyNonAuctionableTokens = []byte("NonAuctionableTokens")
	ParamsStoreKeyBurnWinningBids      = []byte("BurnWinningBids")
	ParamsStoreKeyEnabled              = []byte("Enabled")
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
	minBidFee uint64,
	nonAuctionableTokens []string,
	burnWinningBids bool,
	enabled bool,
) Params {
	return Params{
		AuctionLength:        auctionLength,
		MinBidFee:            minBidFee,
		NonAuctionableTokens: nonAuctionableTokens,
		BurnWinningBids:      burnWinningBids,
		Enabled:              enabled,
	}
}

// DefaultParams defines the default parameters for the GravityBridge auction module
func DefaultParams() Params {
	return NewParams(
		DefaultAuctionLength,
		DefaultMinBidFee,
		DefaultNonAuctionableTokens,
		DefaultBurnWinningBids,
		DefaultEnabled,
	)
}

// ParamSetPairs get the params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(ParamsStoreKeyAuctionLength, &p.AuctionLength, isPositive),
		paramtypes.NewParamSetPair(ParamsStoreKeyMinBidFee, &p.MinBidFee, isNonNegative),
		paramtypes.NewParamSetPair(ParamsStoreKeyNonAuctionableTokens, &p.NonAuctionableTokens, validNonAuctionableDenoms),
		paramtypes.NewParamSetPair(ParamsStoreKeyBurnWinningBids, &p.BurnWinningBids, isBoolean),
		paramtypes.NewParamSetPair(ParamsStoreKeyEnabled, &p.Enabled, isBoolean),
	}
}

// Validate checks that the parameters have valid values.
func (p Params) ValidateBasic() error {
	// AuctionLength (nonzero)
	if err := isPositive(p.AuctionLength); err != nil {
		return errorsmod.Wrap(ErrInvalidParams, "auction length must be positive")
	}

	// MinBidFee (uint type check)
	if err := isNonNegative(p.MinBidFee); err != nil {
		return errorsmod.Wrap(ErrInvalidParams, "bid fee basis points must be non-negative")
	}

	// NonAuctionableTokens (valid denoms + contains native token)
	if err := validNonAuctionableDenoms(p.NonAuctionableTokens); err != nil {
		return err
	}

	// BurnWinningBids (boolean type check)
	if err := isBoolean(p.BurnWinningBids); err != nil {
		return errorsmod.Wrap(ErrInvalidParams, "burn winning bids must be a boolean")
	}

	// Enabled (boolean type check)
	if err := isBoolean(p.Enabled); err != nil {
		return errorsmod.Wrap(ErrInvalidParams, "enabled must be a boolean")
	}

	return nil
}

func isNonNegative(i interface{}) error {
	_, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("parameter not accepted: %T", i)
	}

	// If the value is a uint of any kind, it is non-negative

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

func validNonAuctionableDenoms(i interface{}) error {
	ival, ok := i.([]string)
	if !ok {
		return fmt.Errorf("parameter not accepted: %T", i)
	}

	found := false // Check for native token presence
	for j, s := range ival {
		if err := isValidDenom(s); err != nil {
			return errorsmod.Wrapf(ErrInvalidParams, "string %d is invalid: %s %v", j, s, err)
		}
		if s == config.NativeTokenDenom {
			found = true
		}
	}
	if !found {
		return errorsmod.Wrapf(ErrInvalidParams, "non auctionable tokens must contain the native token")
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
