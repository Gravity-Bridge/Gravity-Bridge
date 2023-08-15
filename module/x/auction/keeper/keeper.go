package keeper

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

type Keeper struct {
	storeKey   sdk.StoreKey // Unexposed key to access store from sdk.Context
	paramSpace paramtypes.Subspace

	cdc           codec.BinaryCodec // The wire codec for binary encoding/decoding.
	BankKeeper    *bankkeeper.BaseKeeper
	AccountKeeper *authkeeper.AccountKeeper
	DistKeeper    *distrkeeper.Keeper
	StakingKeeper *stakingkeeper.Keeper
}

func NewKeeper(
	storeKey sdk.StoreKey,
	paramSpace paramtypes.Subspace,
	cdc codec.BinaryCodec,
	bankKeeper *bankkeeper.BaseKeeper,
	accKeeper *authkeeper.AccountKeeper,
	distKeeper *distrkeeper.Keeper,
	stakingKeeper *stakingkeeper.Keeper,
) Keeper {
	// set KeyTable if it has not already been set
	if !paramSpace.HasKeyTable() {
		paramSpace = paramSpace.WithKeyTable(types.ParamKeyTable())
	}

	k := Keeper{
		storeKey:      storeKey,
		paramSpace:    paramSpace,
		cdc:           cdc,
		BankKeeper:    bankKeeper,
		AccountKeeper: accKeeper,
		DistKeeper:    distKeeper,
		StakingKeeper: stakingKeeper,
	}
	return k
}

// GetParams get params
// GetParams returns the parameters from the store
func (k Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	k.paramSpace.GetParamSet(ctx, &params)
	return
}

// SetParams sets the parameters in the store
func (k Keeper) SetParams(ctx sdk.Context, ps types.Params) {
	k.paramSpace.SetParamSet(ctx, &ps)
}

// SendToCommunityPool send the remain tokens
// from auction module account back to community pool
func (k Keeper) SendToCommunityPool(ctx sdk.Context, coins sdk.Coins) error {
	if err := k.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, distrtypes.ModuleName, coins); err != nil {
		return sdkerrors.Wrap(err, "Fail to transfer token to community pool")
	}
	feePool := k.DistKeeper.GetFeePool(ctx)
	feePool.CommunityPool = feePool.CommunityPool.Add(sdk.NewDecCoinsFromCoins(coins...)...)
	k.DistKeeper.SetFeePool(ctx, feePool)
	return nil
}

// SendToCommunityPool send the auctioned tokens
// from community pool to auction module account
func (k Keeper) SendFromCommunityPool(ctx sdk.Context, coins sdk.Coins) error {
	err := k.DistKeeper.DistributeFromFeePool(ctx, coins, k.AccountKeeper.GetModuleAddress(types.ModuleName))
	if err != nil {
		return sdkerrors.Wrap(err, "Fail to transfer token to auction module")
	}
	return nil
}

func (k Keeper) ReturnPrevioudBidAmount(ctx sdk.Context, recipient string, amount sdk.Coin) error {
	sdkAcc, err := sdk.AccAddressFromBech32(recipient)
	if err != nil {
		return fmt.Errorf("Unable to get account from Bech32 address: %s", err.Error())
	}
	err = k.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sdkAcc, sdk.NewCoins(amount))
	return err
}

func (k Keeper) LockBidAmount(ctx sdk.Context, sender string, amount sdk.Coin) error {
	sdkAcc, err := sdk.AccAddressFromBech32(sender)
	if err != nil {
		return fmt.Errorf("Unable to get account from Bech32 address: %s", err.Error())
	}
	err = k.BankKeeper.SendCoinsFromAccountToModule(ctx, sdkAcc, types.ModuleName, sdk.NewCoins(amount))
	return err
}
