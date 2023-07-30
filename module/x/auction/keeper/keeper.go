package keeper

import (
	"github.com/cosmos/cosmos-sdk/codec"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

type Keeper struct {
	// NOTE: If you add anything to this struct, add a nil check to ValidateMembers below!
	storeKey   sdk.StoreKey // Unexposed key to access store from sdk.Context
	paramSpace paramtypes.Subspace

	// NOTE: If you add anything to this struct, add a nil check to ValidateMembers below!
	cdc           codec.BinaryCodec // The wire codec for binary encoding/decoding.
	BankKeeper    *bankkeeper.BaseKeeper
	AccountKeeper *authkeeper.AccountKeeper
	DistKeeper    *distrkeeper.Keeper
}

func NewKeeper(
	storeKey sdk.StoreKey,
	paramSpace paramtypes.Subspace,
	cdc codec.BinaryCodec,
	bankKeeper *bankkeeper.BaseKeeper,
	accKeeper *authkeeper.AccountKeeper,
	distKeeper *distrkeeper.Keeper,
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

// SendToCommunityPool handles incorrect SendToCosmos calls to the community pool, since the calls
// have already been made on Ethereum there's nothing we can do to reverse them, and we should at least
// make use of the tokens which would otherwise be lost
func (k Keeper) SendToCommunityPool(ctx sdk.Context, coins sdk.Coins) error {
	if err := k.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, distrtypes.ModuleName, coins); err != nil {
		return sdkerrors.Wrap(err, "transfer to community pool failed")
	}
	feePool := k.DistKeeper.GetFeePool(ctx)
	feePool.CommunityPool = feePool.CommunityPool.Add(sdk.NewDecCoinsFromCoins(coins...)...)
	k.DistKeeper.SetFeePool(ctx, feePool)
	return nil
}

func (k Keeper) SendFromCommunityPool(ctx sdk.Context, coins sdk.Coins) error {
	if err := k.BankKeeper.SendCoinsFromModuleToModule(ctx, distrtypes.ModuleName, types.ModuleName, coins); err != nil {
		return sdkerrors.Wrap(err, "transfer to auction module failed")
	}
	return nil
}
