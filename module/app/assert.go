package app

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/config"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// There are two places where the bech32 "gravity" account prefix can be set: the default sdkConfig and also NativeHrp
// in bech32ibc, this method asserts that the Gravity app has been properly configured with matching bech32 prefix
// Note: These checks are not in Gravity.ValidateMembers() because GetNativeHrp() requires a ctx, call this func
// just once on startup since sdkConfig is immutable and NativeHrp is not set by users.
func (app *Gravity) assertBech32PrefixMatches(ctx sdk.Context) {
	config := sdk.GetConfig()
	if app == nil || config == nil || app.Bech32IbcKeeper == nil {
		panic("Invalid app/config/keeper state")
	}
	nativePrefix, err := app.Bech32IbcKeeper.GetNativeHrp(ctx)
	if err != nil {
		panic(errorsmod.Wrap(err, "Error obtaining bech32ibc NativeHrp"))
	}
	configPrefix := config.GetBech32AccountAddrPrefix()
	if nativePrefix != configPrefix {
		panic(fmt.Sprintf("Mismatched bech32ibc NativeHrp (%v) and config Bech32 Account Prefix (%v)",
			nativePrefix, configPrefix))
	}
}

// The community pool holds a significant balance of GRAV, so to make sure it cannot be auctioned off
// (which would have to be for MUCH less GRAV than it is worth), assert that the NonAuctionableTokens list
// contains GRAV (ugraviton)
func (app *Gravity) assertNativeTokenIsNonAuctionable(ctx sdk.Context) {
	nonAuctionableTokens := app.AuctionKeeper.GetParams(ctx).NonAuctionableTokens
	nativeToken := app.MintKeeper.GetParams(ctx).MintDenom // GRAV

	for _, t := range nonAuctionableTokens {
		if t == nativeToken {
			// Success!
			return
		}
	}

	// Failure!
	panic(fmt.Sprintf("Auction module's nonAuctionableTokens (%v) MUST contain GRAV (%s)\n", nonAuctionableTokens, nativeToken))
}

// In the config directory is a constant which should represent the native token, this check ensures that constant is correct
func (app *Gravity) assertNativeTokenMatchesConstant(ctx sdk.Context) {
	hardcoded := config.NativeTokenDenom
	nativeToken := app.MintKeeper.GetParams(ctx).MintDenom

	if hardcoded != nativeToken {
		panic(fmt.Sprintf("The hard-coded native token denom (%s) must equal the actual native token (%s)\n", hardcoded, nativeToken))
	}
}
