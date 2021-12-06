package keeper

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

func (k Keeper) GetCosmosOriginatedDenom(ctx sdk.Context, tokenContract types.EthAddress) (string, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get([]byte(types.GetERC20ToDenomKey(tokenContract)))

	if bz != nil {
		return string(bz), true
	}
	return "", false
}

func (k Keeper) GetCosmosOriginatedERC20(ctx sdk.Context, denom string) (*types.EthAddress, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get([]byte(types.GetDenomToERC20Key(denom)))
	if bz != nil {
		ethAddr, err := types.NewEthAddress(string(bz))
		if err != nil {
			panic(fmt.Errorf("discovered invalid ERC20 address under key %v", string(bz)))
		}

		return ethAddr, true
	}
	return nil, false
}

func (k Keeper) setCosmosOriginatedDenomToERC20(ctx sdk.Context, denom string, tokenContract types.EthAddress) {
	store := ctx.KVStore(k.storeKey)
	store.Set([]byte(types.GetDenomToERC20Key(denom)), []byte(tokenContract.GetAddress()))
	store.Set([]byte(types.GetERC20ToDenomKey(tokenContract)), []byte(denom))
}

// DenomToERC20 returns (bool isCosmosOriginated, EthAddress ERC20, err)
// Using this information, you can see if an asset is native to Cosmos or Ethereum,
// and get its corresponding ERC20 address.
// This will return an error if it cant parse the denom as a gravity denom, and then also can't find the denom
// in an index of ERC20 contracts deployed on Ethereum to serve as synthetic Cosmos assets.
func (k Keeper) DenomToERC20Lookup(ctx sdk.Context, denom string) (bool, *types.EthAddress, error) {
	// First try parsing the ERC20 out of the denom
	tc1, err := types.GravityDenomToERC20(denom)

	if err != nil {
		// Look up ERC20 contract in index and error if it's not in there.
		tc2, exists := k.GetCosmosOriginatedERC20(ctx, denom)
		if !exists {
			return false, nil,
				sdkerrors.Wrap(types.ErrInvalid, fmt.Sprintf("denom not a gravity voucher coin: %s, and also not in cosmos-originated ERC20 index", err))
		}
		// This is a cosmos-originated asset
		return true, tc2, nil
	}

	// This is an ethereum-originated asset
	return false, tc1, nil
}

// RewardToERC20Lookup is a specialized function wrapping DenomToERC20Lookup designed to validate
// the validator set reward any time we generate a validator set
func (k Keeper) RewardToERC20Lookup(ctx sdk.Context, coin sdk.Coin) (*types.EthAddress, sdk.Int) {
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
		if err != nil {
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

	// If it is not in there, it is not a cosmos originated token, turn the ERC20 into a gravity denom
	return false, types.GravityDenom(tokenContract)
}

// IterateERC20ToDenom iterates over erc20 to denom relations
func (k Keeper) IterateERC20ToDenom(ctx sdk.Context, cb func([]byte, *types.ERC20ToDenom) bool) {
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.ERC20ToDenomKey))
	iter := prefixStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		erc20ToDenom := types.ERC20ToDenom{
			Erc20: string(iter.Key()),
			Denom: string(iter.Value()),
		}
		// cb returns true to stop early
		if cb(iter.Key(), &erc20ToDenom) {
			break
		}
	}
}
