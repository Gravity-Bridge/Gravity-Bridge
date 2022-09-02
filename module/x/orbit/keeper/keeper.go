package keeper

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	ibctransferkeeper "github.com/cosmos/ibc-go/v3/modules/apps/transfer/keeper"
	"github.com/tendermint/tendermint/libs/log"

	bech32ibckeeper "github.com/osmosis-labs/bech32-ibc/x/bech32ibc/keeper"

	gravitykeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/orbit/types"
)

// Check that our expected keeper types are implemented
var _ types.StakingKeeper = (*stakingkeeper.Keeper)(nil)
var _ types.SlashingKeeper = (*slashingkeeper.Keeper)(nil)
var _ types.DistributionKeeper = (*distrkeeper.Keeper)(nil)
var _ types.GravityKeeper = (*gravitykeeper.Keeper)(nil)

// Keeper maintains the link to storage and exposes getter/setter methods for the various parts of the state machine
type Keeper struct {
	// NOTE: If you add anything to this struct, add a nil check to ValidateMembers below!
	storeKey   sdk.StoreKey // Unexposed key to access store from sdk.Context
	paramSpace paramtypes.Subspace

	// NOTE: If you add anything to this struct, add a nil check to ValidateMembers below!
	cdc               codec.BinaryCodec // The wire codec for binary encoding/decoding.
	bankKeeper        *bankkeeper.BaseKeeper
	StakingKeeper     *stakingkeeper.Keeper
	SlashingKeeper    *slashingkeeper.Keeper
	DistKeeper        *distrkeeper.Keeper
	accountKeeper     *authkeeper.AccountKeeper
	ibcTransferKeeper *ibctransferkeeper.Keeper
	bech32IbcKeeper   *bech32ibckeeper.Keeper
	gravityKeeper     *gravitykeeper.Keeper
}

// Check for nil members
func (k Keeper) ValidateMembers() {
	if k.bankKeeper == nil {
		panic("Nil bankKeeper!")
	}
	if k.StakingKeeper == nil {
		panic("Nil StakingKeeper!")
	}
	if k.SlashingKeeper == nil {
		panic("Nil SlashingKeeper!")
	}
	if k.DistKeeper == nil {
		panic("Nil DistKeeper!")
	}
	if k.accountKeeper == nil {
		panic("Nil accountKeeper!")
	}
	if k.ibcTransferKeeper == nil {
		panic("Nil ibcTransferKeeper!")
	}
	if k.bech32IbcKeeper == nil {
		panic("Nil bech32IbcKeeper!")
	}
}

// NewKeeper returns a new instance of the gravity keeper
func NewKeeper(
	storeKey sdk.StoreKey,
	paramSpace paramtypes.Subspace,
	cdc codec.BinaryCodec,
	bankKeeper *bankkeeper.BaseKeeper,
	stakingKeeper *stakingkeeper.Keeper,
	slashingKeeper *slashingkeeper.Keeper,
	distKeeper *distrkeeper.Keeper,
	accKeeper *authkeeper.AccountKeeper,
	ibcTransferKeeper *ibctransferkeeper.Keeper,
	bech32IbcKeeper *bech32ibckeeper.Keeper,
	gravityKeeper *gravitykeeper.Keeper,
) Keeper {
	// set KeyTable if it has not already been set
	if !paramSpace.HasKeyTable() {
		paramSpace = paramSpace.WithKeyTable(types.ParamKeyTable())
	}

	k := Keeper{
		storeKey:   storeKey,
		paramSpace: paramSpace,

		cdc:               cdc,
		bankKeeper:        bankKeeper,
		StakingKeeper:     stakingKeeper,
		SlashingKeeper:    slashingKeeper,
		DistKeeper:        distKeeper,
		accountKeeper:     accKeeper,
		ibcTransferKeeper: ibcTransferKeeper,
		bech32IbcKeeper:   bech32IbcKeeper,
		gravityKeeper:     gravityKeeper,
	}

	k.ValidateMembers()

	return k
}

////////////////////////
/////// HELPERS ////////
////////////////////////

/////////////////////////////
//////// PARAMETERS /////////
/////////////////////////////

// GetParams returns the parameters from the store
func (k Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	k.paramSpace.GetParamSet(ctx, &params)
	return
}

// SetParams sets the parameters in the store
func (k Keeper) SetParams(ctx sdk.Context, ps types.Params) {
	k.paramSpace.SetParamSet(ctx, &ps)
}

// logger returns a module-specific logger.
func (k Keeper) logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}
