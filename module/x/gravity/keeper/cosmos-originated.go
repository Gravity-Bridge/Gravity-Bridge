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

// validateCosmosOriginatedMapping is the single source of truth for the structural rules that
// every cosmos-originated denom<->ERC20 mapping must satisfy, independent of context. It is shared
// by the write path (setCosmosOriginatedMapping), the attestation handler (handleErc20Deployed),
// and the store invariant (ValidateCrossListIntegrity) so those sites can never disagree about
// what constitutes a legal mapping. It enforces:
//   - denom must not contain an embedded Ethereum address
//   - denom must not collide with an eth-originated gravity or gravity2 denom
//   - the ERC20 must not be in the remapped eth-originated set
//
// It deliberately does NOT perform duplicate detection or bidirectional-consistency checks, which
// are specific to the calling context (write-time rejection vs. read-time invariant verification).
func (k Keeper) validateCosmosOriginatedMapping(ctx sdk.Context, denom string, tokenContract types.EthAddress) error {
	// reject denoms that would parse as a gravity or gravity2 eth-originated denom. These are
	// checked before the broader embedded-address check so a gravity/gravity2 voucher (which also
	// embeds an address) reports the more specific collision.
	if _, err := types.GravityDenomToERC20(denom); err == nil {
		return errorsmod.Wrapf(types.ErrInvalid,
			"cosmos-originated denom %q collides with an eth-originated gravity denom", denom)
	}
	if _, err := types.Gravity2DenomToERC20(denom); err == nil {
		return errorsmod.Wrapf(types.ErrInvalid,
			"cosmos-originated denom %q collides with an eth-originated gravity2 denom", denom)
	}
	// reject denoms containing an embedded Ethereum address
	if types.ContainsEthAddress(denom) {
		return errorsmod.Wrapf(types.ErrInvalid,
			"cosmos-originated denom %q contains an embedded Ethereum address and would collide with an eth-originated asset", denom)
	}
	// reject ERC20s in the remapped (eth-originated) set
	if k.IsRemappedERC20(ctx, tokenContract) {
		return errorsmod.Wrapf(types.ErrInvalid,
			"cosmos-originated ERC20 %s is also in the remapped eth-originated set and cannot be registered as cosmos-originated",
			tokenContract.GetAddress().Hex())
	}
	return nil
}

// setCosmosOriginatedMapping registers a bidirectional denom<->ERC20 mapping in the
// cosmos-originated index after enforcing the shared structural constraints in
// validateCosmosOriginatedMapping plus write-time duplicate prevention: neither denom nor ERC20
// may already be registered.
func (k Keeper) setCosmosOriginatedMapping(ctx sdk.Context, denom string, tokenContract types.EthAddress) error {
	if err := k.validateCosmosOriginatedMapping(ctx, denom, tokenContract); err != nil {
		return err
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

// classifyCosmosOriginated is the single source of truth for validating and building the
// AssetOrigin of a cosmos-originated (denom <-> tokenContract) pair. Both ClassifyERC20 and
// ClassifyDenom delegate their cosmos-originated branch here so the rules can never diverge.
//
// The caller argument ("ClassifyERC20" / "ClassifyDenom") is used only to prefix error messages
// so failures point back to the entry point. It enforces:
//   - the denom passes ValidateStrictDenom()
//   - the denom<->ERC20 mapping is bidirectionally consistent in both index directions
//   - the ERC20 is not simultaneously in the remapped eth-originated set
func (k Keeper) classifyCosmosOriginated(ctx sdk.Context, caller string, denom string, tokenContract types.EthAddress) (*types.AssetOrigin, error) {
	if err := types.ValidateStrictDenom(denom); err != nil {
		return nil, errorsmod.Wrapf(types.ErrInvalidDenom, "%s: the denom %s for cosmos-originated ERC20 %s is invalid: %v",
			caller, denom, tokenContract.GetAddress().Hex(), err)
	}
	// Check both directions of the double index agree. Whichever direction the caller looked up
	// first, both must round-trip back to the same pair.
	if reverseERC20, ok := k.getCosmosOriginatedERC20ForDenom(ctx, denom); !ok || reverseERC20.GetAddress() != tokenContract.GetAddress() {
		return nil, errorsmod.Wrapf(types.ErrInvalid,
			"%s: cosmos-originated mapping for denom %q / ERC20 %s is not bidirectionally consistent",
			caller, denom, tokenContract.GetAddress().Hex())
	}
	if reverseDenom, ok := k.getCosmosOriginatedDenomForERC20(ctx, tokenContract); !ok || reverseDenom != denom {
		return nil, errorsmod.Wrapf(types.ErrInvalid,
			"%s: cosmos-originated mapping for denom %q / ERC20 %s is not bidirectionally consistent (reverse lookup returned %q)",
			caller, denom, tokenContract.GetAddress().Hex(), reverseDenom)
	}
	if k.IsRemappedERC20(ctx, tokenContract) {
		return nil, errorsmod.Wrapf(types.ErrInvalid,
			"%s: cosmos-originated ERC20 %s is also in the remapped eth-originated set, which is not allowed",
			caller, tokenContract.GetAddress().Hex())
	}
	origin := types.AssetOrigin{
		Origin:     types.AssetOriginCosmos,
		IsRemapped: false,
		Denom:      denom,
		ERC20:      &tokenContract,
	}
	origin.AssertValid()
	return &origin, nil
}

// classifyEthOriginated is the single source of truth for validating and building the AssetOrigin
// of an Ethereum-originated asset. Both ClassifyERC20 and ClassifyDenom delegate their
// eth-originated branch here so the rules do not diverge.
//
// isRemapped selects the gravity2 (remapped) vs gravity (default) namespace. denom is the
// caller-authoritative denom string: ClassifyDenom passes the exact input denom so its casing is
// preserved, while ClassifyERC20 passes the derived denom "gravity0x". The caller argument is used
// only to prefix error messages. This function enforces:
//   - x/gravity never sets bank metadata for eth-originated vouchers, so any metadata is rejected
//   - the denom round-trips back to tokenContract under the selected namespace
//   - the denom passes ValidateStrictDenom()
func (k Keeper) classifyEthOriginated(ctx sdk.Context, caller string, denom string, tokenContract types.EthAddress, isRemapped bool) (*types.AssetOrigin, error) {
	namespace := "gravity"
	if isRemapped {
		namespace = "gravity2"
	}

	// This check merely ensures that Eth-originated assets do not have bank metadata because
	// x/gravity does not set it; this should be unnecessary but will make metadata changes obvious.
	if meta, exists := k.bankKeeper.GetDenomMetaData(ctx, denom); exists {
		return nil, errorsmod.Wrapf(types.ErrInvalid,
			"%s: Eth-originated %s denom %s has bank metadata %s, which is not allowed", caller, namespace, denom, meta.Name)
	}

	// Round-trip: the denom must parse back to the same ERC20 address under the selected namespace.
	var reparsed *types.EthAddress
	var parseErr error
	if isRemapped {
		reparsed, parseErr = types.Gravity2DenomToERC20(denom)
	} else {
		reparsed, parseErr = types.GravityDenomToERC20(denom)
	}
	if parseErr != nil || reparsed.GetAddress() != tokenContract.GetAddress() {
		return nil, errorsmod.Wrapf(types.ErrInvalid,
			"%s: eth-originated denom %q failed round-trip validation for ERC20 %s",
			caller, denom, tokenContract.GetAddress().Hex())
	}

	if err := types.ValidateStrictDenom(denom); err != nil {
		return nil, errorsmod.Wrapf(types.ErrInvalidDenom, "%s: the denom %s for eth-originated ERC20 %s is invalid: %v",
			caller, denom, tokenContract.GetAddress().Hex(), err)
	}

	origin := types.AssetOrigin{
		Origin:     types.AssetOriginEthereum,
		IsRemapped: isRemapped,
		Denom:      denom,
		ERC20:      &tokenContract,
	}
	origin.AssertValid()
	return &origin, nil
}

// ClassifyERC20 determines the origin of an ERC20 address (Cosmos or Ethereum) and returns a fully populated AssetOrigin.
//
// It shares its per-origin validation with ClassifyDenom via classifyCosmosOriginated and
// classifyEthOriginated so the two entry points do not diverge.
//
// Returns an error if any of these checks fail:
//   - Cosmos-originated: the denom fails ValidateStrictDenom(), the mapping is inconsistent, or the ERC20 is also remapped
//   - Eth-originated: the derived denom must round-trip back to the same ERC20 address
//   - Eth-originated: the denom fails ValidateStrictDenom()
//   - Eth-originated: bank metadata unexpectedly exists for the derived denom
func (k Keeper) ClassifyERC20(ctx sdk.Context, tokenContract types.EthAddress) (*types.AssetOrigin, error) {
	// Check cosmos-originated index first
	if denom, exists := k.getCosmosOriginatedDenomForERC20(ctx, tokenContract); exists {
		return k.classifyCosmosOriginated(ctx, "ClassifyERC20", denom, tokenContract)
	}

	// Eth-originated path: derive the canonical denom for the appropriate namespace
	isRemapped := k.IsRemappedERC20(ctx, tokenContract)
	var denom string
	if isRemapped {
		denom = types.Gravity2Denom(tokenContract)
	} else {
		denom = types.GravityDenom(tokenContract)
	}
	return k.classifyEthOriginated(ctx, "ClassifyERC20", denom, tokenContract, isRemapped)
}

// ClassifyDenom determines the origin of a denom (Cosmos or Ethereum) and returns a fully populated AssetOrigin.
//
// It shares its per-origin validation with ClassifyERC20 via classifyCosmosOriginated and
// classifyEthOriginated so the two entry points can never disagree.
//
// Returns an error if any of these checks fail:
//   - Unrecognized denom
//   - A gravity/gravity2-prefixed denom whose remapped state doesn't match IsRemappedERC20
//   - The denom fails ValidateStrictDenom()
//   - Eth-originated: bank metadata unexpectedly exists for the denom
//   - Cosmos-originated: the mapping is inconsistent
func (k Keeper) ClassifyDenom(ctx sdk.Context, denom string) (*types.AssetOrigin, error) {
	// gravity2 prefix -> eth-originated remapped erc20
	if tc, err := types.Gravity2DenomToERC20(denom); err == nil {
		if !k.IsRemappedERC20(ctx, *tc) {
			return nil, errorsmod.Wrapf(types.ErrInvalid, "ClassifyDenom: gravity2 denom %s does not correspond to a remapped ERC20", denom)
		}
		return k.classifyEthOriginated(ctx, "ClassifyDenom", denom, *tc, true)
	}

	// gravity prefix -> eth-originated erc20
	if tc, err := types.GravityDenomToERC20(denom); err == nil {
		if k.IsRemappedERC20(ctx, *tc) {
			return nil, errorsmod.Wrapf(types.ErrInvalid, "ERC20 %s was remapped; new deposits use %s",
				tc.GetAddress().Hex(), types.Gravity2Denom(*tc))
		}
		return k.classifyEthOriginated(ctx, "ClassifyDenom", denom, *tc, false)
	}

	// cosmos-originated token
	if tc, exists := k.getCosmosOriginatedERC20ForDenom(ctx, denom); exists {
		return k.classifyCosmosOriginated(ctx, "ClassifyDenom", denom, *tc)
	}

	// Unknown asset, return an error
	//nolint: exhaustruct
	return nil, types.ErrInvalidDenom.Wrapf("denom %q is not registered as a known bridged asset", denom)
}

// RewardToERC20Lookup validates a valset reward coin and returns its ERC20 address and amount.
// Panics if the coin is invalid or the denom is not a known bridged asset (operator error).
func (k Keeper) RewardToERC20Lookup(ctx sdk.Context, coin sdk.Coin) (*types.EthAddress, math.Int) {
	if !coin.IsValid() || coin.IsZero() {
		panic("Bad validator set relaying reward!")
	}
	origin, err := k.ClassifyDenom(ctx, coin.Denom)
	if err != nil {
		panic(fmt.Sprintf("Invalid Valset reward! Denom %q is not a known bridged asset. Correct or remove the parameter value", coin.Denom))
	}
	if origin.Origin != types.AssetOriginCosmos && origin.Origin != types.AssetOriginEthereum {
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
