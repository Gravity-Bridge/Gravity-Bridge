package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TODO: Add any future invariants here
// TODO: (see the sdk docs for more info https://docs.cosmos.network/master/building-modules/invariants.html)
func AllInvariants(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		res, stop := FailingInvariant(k)(ctx)
		if stop {
			return res, stop
		}
		return res, stop
	}
}

// FailingInvariant is a simple invariant which always fails
func FailingInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		return "Who's a failure now, huh?!", true
	}
}
