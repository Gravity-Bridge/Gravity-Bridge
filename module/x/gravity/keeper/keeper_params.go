package keeper

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

func (k Keeper) GetParamsIfSet(ctx sdk.Context) (params types.Params, err error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has(types.ParamsKey) {
		return types.Params{}, errorsmod.Wrapf(types.ErrParamsNotFound, "params not found in store")
	}
	return k.GetParams(ctx)
}

func (k Keeper) GetParams(ctx sdk.Context) (params types.Params, err error) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.ParamsKey)
	if bz == nil {
		return params, types.ErrParamsNotFound
	}
	err = k.cdc.Unmarshal(bz, &params)
	if err != nil {
		return params, err
	}
	return params, nil
}

func (k Keeper) SetParams(ctx sdk.Context, params types.Params) error {
	if err := params.ValidateBasic(); err != nil {
		return err
	}
	store := ctx.KVStore(k.storeKey)
	bz, err := k.cdc.Marshal(&params)
	if err != nil {
		return err
	}
	store.Set(types.ParamsKey, bz)
	return nil
}

// GetBridgeContractAddress returns the bridge contract address on ETH
func (k Keeper) GetBridgeContractAddress(ctx sdk.Context) *types.EthAddress {
	params, err := k.GetParams(ctx)

	if err != nil {
		panic(errorsmod.Wrap(err, "failed to get params"))
	}
	ea, err := types.NewEthAddress(params.BridgeEthereumAddress)
	if err != nil {
		panic(errorsmod.Wrap(err, "failed to parse bridge ethereum address"))
	}
	return ea
}

// GetBridgeChainID returns the chain id of the ETH chain we are running against
func (k Keeper) GetBridgeChainID(ctx sdk.Context) uint64 {
	params, err := k.GetParams(ctx)
	if err != nil {
		panic(errorsmod.Wrap(err, "failed to get params"))
	}
	if params.BridgeChainId == 0 {
		panic("BridgeChainId is not set in params")
	}
	return params.BridgeChainId
}

// GetGravityID returns the GravityID the GravityID is essentially a salt value
// for bridge signatures, provided each chain running Gravity has a unique ID
// it won't be possible to play back signatures from one bridge onto another
// even if they share a validator set.
//
// The lifecycle of the GravityID is that it is set in the Genesis file
// read from the live chain for the contract deployment, once a Gravity contract
// is deployed the GravityID CAN NOT BE CHANGED. Meaning that it can't just be the
// same as the chain id since the chain id may be changed many times with each
// successive chain in charge of the same bridge
func (k Keeper) GetGravityID(ctx sdk.Context) string {
	params, err := k.GetParams(ctx)
	if err != nil {
		panic(errorsmod.Wrap(err, "failed to get params"))
	}
	return params.GravityId
}

// Set GravityID sets the GravityID the GravityID is essentially a salt value
// for bridge signatures, provided each chain running Gravity has a unique ID
// it won't be possible to play back signatures from one bridge onto another
// even if they share a validator set.
//
// The lifecycle of the GravityID is that it is set in the Genesis file
// read from the live chain for the contract deployment, once a Gravity contract
// is deployed the GravityID CAN NOT BE CHANGED. Meaning that it can't just be the
// same as the chain id since the chain id may be changed many times with each
// successive chain in charge of the same bridge
func (k Keeper) SetGravityID(ctx sdk.Context, v string) {
	params, err := k.GetParams(ctx)
	if err != nil {
		panic(errorsmod.Wrap(err, "failed to get params"))
	}
	params.GravityId = v

	if err := k.SetParams(ctx, params); err != nil {
		panic(errorsmod.Wrap(err, "failed to set params"))
	}
}
