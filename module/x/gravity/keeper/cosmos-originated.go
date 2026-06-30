package keeper

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"
	math "cosmossdk.io/math"

	"cosmossdk.io/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// getCosmosOriginatedDenomForERC20 returns the cosmos-originated denom registered for the
// given ERC20 address, and whether such a mapping exists.
func (k Keeper) getCosmosOriginatedDenomForERC20(ctx sdk.Context, tokenContract types.EthAddress) (string, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.GetERC20ToDenomKey(tokenContract))
	if bz != nil {
		return string(bz), true
	}
	return "", false
}

// getCosmosOriginatedERC20ForDenom returns the ERC20 address registered for the given
// cosmos-originated denom, and whether such a mapping exists.
func (k Keeper) getCosmosOriginatedERC20ForDenom(ctx sdk.Context, denom string) (*types.EthAddress, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.GetDenomToERC20Key(denom))
	if bz != nil {
		ethAddr, err := types.NewEthAddressFromBytes(bz)
		if err != nil {
			panic(fmt.Errorf("discovered invalid ERC20 address under cosmos-originated denom key %q: %v", denom, err))
		}
		return ethAddr, true
	}
	return nil, false
}

// setCosmosOriginatedMapping registers a bidirectional denom<->ERC20 mapping in the
// cosmos-originated index after enforcing the following security constraints:
//   - denom must not contain an embedded Ethereum address
//   - denom must not collide with an eth-originated gravity or gravity2 denom
//   - the ERC20 must not already be in the remapped eth-originated set
//   - duplicate prevention: neither denom nor ERC20 may already be registered
func (k Keeper) setCosmosOriginatedMapping(ctx sdk.Context, denom string, tokenContract types.EthAddress) error {
	// reject denoms containing an embedded Ethereum address
	if types.ContainsEthAddress(denom) {
		return errorsmod.Wrapf(types.ErrInvalid,
			"cosmos-originated denom %q contains an embedded Ethereum address and would collide with an eth-originated asset", denom)
	}
	// reject denoms that would parse as a gravity or gravity2 eth-originated denom
	if _, err := types.GravityDenomToERC20(denom); err == nil {
		return errorsmod.Wrapf(types.ErrInvalid,
			"cosmos-originated denom %q collides with an eth-originated gravity denom", denom)
	}
	if _, err := types.Gravity2DenomToERC20(denom); err == nil {
		return errorsmod.Wrapf(types.ErrInvalid,
			"cosmos-originated denom %q collides with an eth-originated gravity2 denom", denom)
	}
	// reject ERC20s already in the remapped (eth-originated) set
	if k.IsRemappedERC20(ctx, tokenContract) {
		return errorsmod.Wrapf(types.ErrInvalid,
			"ERC20 %s is already in the remapped eth-originated set and cannot be registered as cosmos-originated",
			tokenContract.GetAddress().Hex())
	}
	// Prevent silent overwrites in either direction
	if existing, exists := k.getCosmosOriginatedERC20ForDenom(ctx, denom); exists {
		return errorsmod.Wrapf(types.ErrDuplicate,
			"denom %q is already mapped to cosmos-originated ERC20 %s", denom, existing.GetAddress().Hex())
	}
	if existingDenom, exists := k.getCosmosOriginatedDenomForERC20(ctx, tokenContract); exists {
		return errorsmod.Wrapf(types.ErrDuplicate,
			"ERC20 %s is already mapped to cosmos-originated denom %q", tokenContract.GetAddress().Hex(), existingDenom)
	}

	// Write the entries to the store
	store := ctx.KVStore(k.storeKey)
	store.Set(types.GetDenomToERC20Key(denom), tokenContract.GetAddress().Bytes())
	store.Set(types.GetERC20ToDenomKey(tokenContract), []byte(denom))
	return nil
}

// DeleteCosmosOriginatedMapping removes both directions of the cosmos-originated denom<->ERC20
// mapping from the store.
func (k Keeper) DeleteCosmosOriginatedMapping(ctx sdk.Context, tokenContract types.EthAddress, denom string) {
	store := ctx.KVStore(k.storeKey)
	store.Delete(types.GetDenomToERC20Key(denom))
	store.Delete(types.GetERC20ToDenomKey(tokenContract))
}

// IterateCosmosOriginatedMappings calls cb for every entry in the cosmos-originated denom<->ERC20
// index, providing the denom string and the corresponding EthAddress.
// cb should return true to stop iteration early.
func (k Keeper) IterateCosmosOriginatedMappings(ctx sdk.Context, cb func(denom string, erc20 *types.EthAddress) bool) {
	store := ctx.KVStore(k.storeKey)
	prefixStore := prefix.NewStore(store, types.DenomToERC20Key)
	iter := prefixStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		denom := string(iter.Key())
		erc20, err := types.NewEthAddressFromBytes(iter.Value())
		if err != nil {
			panic(fmt.Sprintf("discovered invalid EthAddress under cosmos-originated mapping key %q: %v", denom, err))
		}
		if cb(denom, erc20) {
			break
		}
	}
}

// ClassifyERC20 determines the origin of an ERC20 address (Cosmos or Ethereum) and returns a fully populated AssetOrigin.
//
// This function will panic if any of these checks fail:
//   - Eth-originated: the derived denom must round-trip back to the same ERC20 address
//   - The denom fails ValidateStrictDenom()
func (k Keeper) ClassifyERC20(ctx sdk.Context, tokenContract types.EthAddress) types.AssetOrigin {
	// Check cosmos-originated index first
	if denom, exists := k.getCosmosOriginatedDenomForERC20(ctx, tokenContract); exists {
		// Note: denom validation (ValidateStrictDenom) is intentionally left to the caller.
		// Panicking here would prevent the attestation handler from returning a proper error
		// when a corrupted or pre-validation entry exists in the store.
		origin := types.AssetOrigin{
			IsCosmosOriginated: true,
			IsEthOriginated:    false,
			IsRemapped:         false,
			Denom:              denom,
			ERC20:              &tokenContract,
		}
		origin.AssertValid()
		return origin
	}

	// Eth-originated path
	isRemapped := k.IsRemappedERC20(ctx, tokenContract)
	var denom string
	if isRemapped {
		denom = types.Gravity2Denom(tokenContract)
	} else {
		denom = types.GravityDenom(tokenContract)
	}

	// Eth address round-trip check — the derived denom must lead back to the same ERC20 address
	var reparsed *types.EthAddress
	var parseErr error
	if isRemapped {
		reparsed, parseErr = types.Gravity2DenomToERC20(denom)
	} else {
		reparsed, parseErr = types.GravityDenomToERC20(denom)
	}
	if parseErr != nil || reparsed.GetAddress() != tokenContract.GetAddress() {
		panic(fmt.Sprintf(
			"ClassifyERC20: eth-originated denom %q failed round-trip validation for ERC20 %s",
			denom, tokenContract.GetAddress().Hex()))
	}

	if err := types.ValidateStrictDenom(denom); err != nil {
		panic(fmt.Sprintf("ClassifyERC20: the denom %s for eth-originated ERC20 %s is invalid: %v",
			denom, tokenContract.GetAddress().Hex(), err))
	}
	origin := types.AssetOrigin{
		IsCosmosOriginated: false,
		IsEthOriginated:    true,
		IsRemapped:         isRemapped,
		Denom:              denom,
		ERC20:              &tokenContract,
	}
	origin.AssertValid()
	return origin
}

// ClassifyDenom determines the origin of an ERC20 address (Cosmos or Ethereum) and returns a fully populated AssetOrigin.
//
// Returns an error if the denom is not recognised as any known asset.
// For cosmos-originated assets, also verifies bidirectional store consistency.
// This function will panic if any of these checks fail:
//   - Unrecognized denom
//   - Eth-originated: the derived denom must round-trip back to the same ERC20 address
//   - The denom fails ValidateStrictDenom()
func (k Keeper) ClassifyDenom(ctx sdk.Context, denom string) types.AssetOrigin {
	// gravity2 prefix -> eth-originated remapped erc20
	if tc, err := types.Gravity2DenomToERC20(denom); err == nil {
		if !k.IsRemappedERC20(ctx, *tc) {
			panic(fmt.Sprintf("ClassifyDenom: gravity2 denom %s does not correspond to a remapped ERC20", denom))
		}
		if err := types.ValidateStrictDenom(denom); err != nil {
			panic(fmt.Sprintf("ClassifyDenom: the denom %s for remapped ERC20 %s is invalid: %v",
				denom, tc.GetAddress().Hex(), err))
		}
		origin := types.AssetOrigin{
			IsCosmosOriginated: false,
			IsEthOriginated:    true,
			IsRemapped:         true,
			Denom:              denom,
			ERC20:              tc,
		}
		origin.AssertValid()
		return origin
	}

	// gravity prefix -> eth-originated erc20
	if tc, err := types.GravityDenomToERC20(denom); err == nil {
		if k.IsRemappedERC20(ctx, *tc) {
			panic(fmt.Sprintf("ERC20 %s was remapped; new deposits use %s", tc.GetAddress().Hex(), types.Gravity2Denom(*tc)))
		}

		if err := types.ValidateStrictDenom(denom); err != nil {
			panic(fmt.Sprintf("ClassifyDenom: the denom %s for eth-originated ERC20 %s is invalid: %v",
				denom, tc.GetAddress().Hex(), err))
		}
		origin := types.AssetOrigin{
			IsCosmosOriginated: false,
			IsEthOriginated:    true,
			IsRemapped:         false,
			Denom:              denom,
			ERC20:              tc,
		}
		origin.AssertValid()
		return origin
	}

	// cosmos-originated token
	if tc, exists := k.getCosmosOriginatedERC20ForDenom(ctx, denom); exists {
		if err := types.ValidateStrictDenom(denom); err != nil {
			panic(fmt.Sprintf("ClassifyDenom: the denom %s for cosmos-originated ERC20 %s is invalid: %v",
				denom, tc.GetAddress().Hex(), err))
		}
		// Check the reverse mapping is consistent
		if reverseDenom, ok := k.getCosmosOriginatedDenomForERC20(ctx, *tc); !ok || reverseDenom != denom {
			panic(fmt.Sprintf("ClassifyDenom: cosmos-originated mapping for denom %q is not bidirectionally consistent (reverse lookup returned %q)",
				denom, reverseDenom))
		}

		origin := types.AssetOrigin{
			IsCosmosOriginated: true,
			IsEthOriginated:    false,
			IsRemapped:         false,
			Denom:              denom,
			ERC20:              tc,
		}
		origin.AssertValid()
		return origin
	}

	// Denom is not registered as any known asset. Return a zero AssetOrigin (both flags false).
	// Callers that require a known asset (e.g. RewardToERC20Lookup) must check the returned flags.
	//nolint: exhaustruct
	return types.AssetOrigin{}
}

// RewardToERC20Lookup validates a valset reward coin and returns its ERC20 address and amount.
// Panics if the coin is invalid or the denom is not a known bridged asset (operator error).
func (k Keeper) RewardToERC20Lookup(ctx sdk.Context, coin sdk.Coin) (*types.EthAddress, math.Int) {
	if !coin.IsValid() || coin.IsZero() {
		panic("Bad validator set relaying reward!")
	}
	origin := k.ClassifyDenom(ctx, coin.Denom)
	if !origin.IsCosmosOriginated && !origin.IsEthOriginated {
		// This can only happen if governance sets a reward denom that has never been bridged.
		panic(fmt.Sprintf("Invalid Valset reward! Denom %q is not a known bridged asset. Correct or remove the parameter value", coin.Denom))
	}
	return origin.ERC20, coin.Amount
}

// SetRemappedERC20 marks an Ethereum ERC20 address as having been remapped.
// Once set, ClassifyERC20 returns the gravity2 denom for new deposits and
// ClassifyDenom rejects the old gravity-prefixed denom.
func (k Keeper) SetRemappedERC20(ctx sdk.Context, tokenContract types.EthAddress) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.GetRemappedERC20Key(tokenContract), []byte{0x01})
}

// IsRemappedERC20 returns true if the given ERC20 address was marked as remapped.
func (k Keeper) IsRemappedERC20(ctx sdk.Context, tokenContract types.EthAddress) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has(types.GetRemappedERC20Key(tokenContract))
}

// IterateRemappedERC20s calls cb for each ERC20 address recorded as remapped.
// cb should return true to stop early.
func (k Keeper) IterateRemappedERC20s(ctx sdk.Context, cb func(addr types.EthAddress) bool) {
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.RemappedERC20Key)
	iter := prefixStore.Iterator(nil, nil)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		addr, err := types.NewEthAddressFromBytes(iter.Key())
		if err != nil {
			panic(fmt.Sprintf("invalid EthAddress in RemappedERC20 store: %v", iter.Key()))
		}
		if cb(*addr) {
			break
		}
	}
}

// DeleteCosmosOriginatedDenomToERC20 removes both directions of the denom to ERC20 mapping
// for a cosmos-originated registration.
func (k Keeper) DeleteCosmosOriginatedDenomToERC20(ctx sdk.Context, tokenContract types.EthAddress, denom string) {
	store := ctx.KVStore(k.storeKey)
	store.Delete(types.GetDenomToERC20Key(denom))
	store.Delete(types.GetERC20ToDenomKey(tokenContract))
}

// IterateERC20ToDenom iterates over erc20 to denom relations
func (k Keeper) IterateERC20ToDenom(ctx sdk.Context, cb func([]byte, *types.ERC20ToDenom) bool) {
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.ERC20ToDenomKey)
	iter := prefixStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		erc20, err := types.NewEthAddressFromBytes(iter.Key())
		if err != nil {
			panic("Invalid ERC20 to Denom mapping in store!")
		}
		erc20ToDenom := types.ERC20ToDenom{
			Erc20: erc20.GetAddress().String(),
			Denom: string(iter.Value()),
		}
		// cb returns true to stop early
		if cb(iter.Key(), &erc20ToDenom) {
			break
		}
	}
}
