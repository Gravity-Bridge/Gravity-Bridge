package keeper

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"
	math "cosmossdk.io/math"

	"cosmossdk.io/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

func (k Keeper) GetCosmosOriginatedDenom(ctx sdk.Context, tokenContract types.EthAddress) (string, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.GetERC20ToDenomKey(tokenContract))

	if bz != nil {
		return string(bz), true
	}
	return "", false
}

func (k Keeper) GetCosmosOriginatedERC20(ctx sdk.Context, denom string) (*types.EthAddress, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.GetDenomToERC20Key(denom))
	if bz != nil {
		ethAddr, err := types.NewEthAddressFromBytes(bz)
		if err != nil {
			panic(fmt.Errorf("discovered invalid ERC20 address under key %v", string(bz)))
		}

		return ethAddr, true
	}
	return nil, false
}

// IterateCosmosOriginatedERC20s iterates through every erc20 under DenomToERC20Key, passing it to the given callback.
// cb should return true to stop iteration, false to continue
func (k Keeper) IterateCosmosOriginatedERC20s(ctx sdk.Context, cb func(key []byte, erc20 *types.EthAddress) (stop bool)) {
	store := ctx.KVStore(k.storeKey)
	prefixStore := prefix.NewStore(store, types.DenomToERC20Key)
	iter := prefixStore.Iterator(nil, nil)

	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		erc20, err := types.NewEthAddressFromBytes(iter.Value())
		if err != nil {
			panic(fmt.Sprintf("Discovered invalid eth address under key %v in IterateCosmosOriginatedERC20s: %v", iter.Key(), err))
		}
		// cb returns true to stop early
		if cb(iter.Key(), erc20) {
			break
		}
	}
}

func (k Keeper) setCosmosOriginatedDenomToERC20(ctx sdk.Context, denom string, tokenContract types.EthAddress) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.GetDenomToERC20Key(denom), tokenContract.GetAddress().Bytes())
	store.Set(types.GetERC20ToDenomKey(tokenContract), []byte(denom))
}

// DenomToERC20Lookup returns (bool isCosmosOriginated, EthAddress ERC20, err)
// Using this information, you can see if an asset is native to Cosmos or Ethereum,
// and get its corresponding ERC20 address.
// This will return an error if it cant parse the denom as a gravity denom, and then also can't find the denom
// in an index of ERC20 contracts deployed on Ethereum to serve as synthetic Cosmos assets.
func (k Keeper) DenomToERC20Lookup(ctx sdk.Context, denom string) (bool, *types.EthAddress, error) {
	// first, prefer gravity2 prefixes for Ethereum-originated assets
	if tc1, err := types.Gravity2DenomToERC20(denom); err == nil {
		if k.IsRemappedERC20(ctx, *tc1) {
			return false, tc1, nil
		}
		return false, nil, errorsmod.Wrapf(types.ErrInvalid,
			"gravity2 denom %s does not correspond to a remapped ERC20", denom)
	}

	// next, try to look up the regular gravity prefix
	tc2, err := types.GravityDenomToERC20(denom)
	if err != nil {
		// finally, check if this is a cosmos originated denom
		tc3, exists := k.GetCosmosOriginatedERC20(ctx, denom)
		if !exists {
			return false, nil,
				errorsmod.Wrap(types.ErrInvalid, fmt.Sprintf("denom not a gravity voucher coin: %s, and also not in cosmos-originated ERC20 index", err))
		}
		return true, tc3, nil
	}

	if k.IsRemappedERC20(ctx, *tc2) {
		return false, nil, errorsmod.Wrapf(types.ErrInvalid,
			"ERC20 %s was remapped, new deposits use %s.",
			tc2.GetAddress().Hex(), types.Gravity2Denom(*tc2))
	}

	// This is an ethereum-originated asset
	return false, tc2, nil
}

// RewardToERC20Lookup is a specialized function wrapping DenomToERC20Lookup designed to validate
// the validator set reward any time we generate a validator set
func (k Keeper) RewardToERC20Lookup(ctx sdk.Context, coin sdk.Coin) (*types.EthAddress, math.Int) {
	if !coin.IsValid() || coin.IsZero() {
		panic("Bad validator set relaying reward!")
	} else {
		// reward case, pass to DenomToERC20Lookup
		_, address, err := k.DenomToERC20Lookup(ctx, coin.Denom)
		if err != nil {
			// This can only ever happen if governance sets a value for the reward
			// which is not a valid ERC20 that as been bridged before (either from or to Cosmos)
			// We'll classify that as operator error and just panic
			panic("Invalid Valset reward! Correct or remove the paramater value")
		}
		return address, coin.Amount
	}
}

// ERC20ToDenom returns (bool isCosmosOriginated, string denom, err)
// Using this information, you can see if an ERC20 address representing an asset is native to Cosmos or Ethereum,
// and get its corresponding denom
func (k Keeper) ERC20ToDenomLookup(ctx sdk.Context, tokenContract types.EthAddress) (bool, string) {
	// First try looking up tokenContract in index
	dn1, exists := k.GetCosmosOriginatedDenom(ctx, tokenContract)
	if exists {
		// It is a cosmos originated asset
		return true, dn1
	}

	// if this is a remapped token, make sure to return the gravity2 denom
	if k.IsRemappedERC20(ctx, tokenContract) {
		return false, types.Gravity2Denom(tokenContract)
	}

	// If it is not a cosmos originated token and not a remapped token, turn the ERC20 into a gravity denom
	return false, types.GravityDenom(tokenContract)
}

// SetRemappedERC20 marks an Ethereum ERC20 address as having been remapped.
// Once set, ERC20ToDenomLookup returns the gravity2 denom
// for new deposits and DenomToERC20Lookup rejects the old gravity-prefixed denom.
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

// EnsureCosmosBridgeable checks CosmosBridgeableTokens allowlist in one place
// making the logic more testable.
func (k Keeper) EnsureCosmosBridgeable(ctx sdk.Context, denom string) error {
	_, isCosmosOriginated := k.GetCosmosOriginatedERC20(ctx, denom)
	if !isCosmosOriginated {
		return nil
	}
	params, err := k.GetParams(ctx)
	if err != nil {
		return errorsmod.Wrap(err, "could not get params")
	}
	for _, bridgeable := range params.CosmosBridgeableTokens {
		if bridgeable == denom {
			return nil
		}
	}
	return errorsmod.Wrapf(
		types.ErrInvalid,
		"cosmos-originated token %s is not on the CosmosBridgeableTokens allowlist",
		denom,
	)
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
