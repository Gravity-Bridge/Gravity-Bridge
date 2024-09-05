package keeper

import (
	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/config"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

type Keeper struct {
	storeKey   storetypes.StoreKey // Unexposed key to access store from sdk.Context
	paramSpace paramtypes.Subspace

	cdc           codec.BinaryCodec // The wire codec for binary encoding/decoding.
	BankKeeper    *bankkeeper.BaseKeeper
	AccountKeeper *authkeeper.AccountKeeper
	DistKeeper    *distrkeeper.Keeper
	MintKeeper    *mintkeeper.Keeper
}

func NewKeeper(
	storeKey storetypes.StoreKey,
	paramSpace paramtypes.Subspace,
	cdc codec.BinaryCodec,
	bankKeeper *bankkeeper.BaseKeeper,
	accKeeper *authkeeper.AccountKeeper,
	distKeeper *distrkeeper.Keeper,
	mintKeeper *mintkeeper.Keeper,
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
		MintKeeper:    mintKeeper,
	}
	return k
}

// GetParams returns the parameters from the store
func (k Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	k.paramSpace.GetParamSet(ctx, &params)
	return
}

// SetParams sets the parameters in the store
func (k Keeper) SetParams(ctx sdk.Context, ps types.Params) {
	k.paramSpace.SetParamSet(ctx, &ps)
}

// Fetches the auction pool account, which holds tokens to be auctioned in the next period
func (k Keeper) GetAuctionPoolAccount(ctx sdk.Context) sdk.AccAddress {
	return k.AccountKeeper.GetModuleAddress(types.AuctionPoolAccountName)
}

// Fetches the balances to use in the next auction period
func (k Keeper) GetAuctionPoolBalances(ctx sdk.Context) sdk.Coins {
	return k.BankKeeper.GetAllBalances(ctx, k.GetAuctionPoolAccount(ctx))
}

// SendToAuctionPool sends the `coins` from module account to the auction pool
// Returns an error if the module is disabled, or on failure to send tokens
func (k Keeper) SendToAuctionPool(ctx sdk.Context, coins sdk.Coins) error {
	enabled := k.GetParams(ctx).Enabled
	if !enabled {
		return types.ErrDisabledModule
	}

	if err := k.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, types.AuctionPoolAccountName, coins); err != nil {
		return errorsmod.Wrap(err, "Failure to transfer tokens from auction module to auction pool")
	}
	return nil
}

// RemoveFromAuctionPool removes the auction tokens from auction pool and locks them in the auction module account
// Returns an error if the module is disabled, or on failure to lock tokens
func (k Keeper) RemoveFromAuctionPool(ctx sdk.Context, coin sdk.Coin) error {
	// TODO: No longer check for native denom?
	native := config.NativeTokenDenom
	if coin.Denom == native {
		return errorsmod.Wrapf(types.ErrInvalidAuction, "not allowed to collect community pool native token balance")
	}
	enabled := k.GetParams(ctx).Enabled
	if !enabled {
		return types.ErrDisabledModule
	}

	if err := k.BankKeeper.SendCoinsFromModuleToModule(ctx, types.AuctionPoolAccountName, types.ModuleName, sdk.NewCoins(coin)); err != nil {
		return errorsmod.Wrap(err, "Failure to transfer tokens from auction pool to auction module")
	}

	return nil
}

// ReturnPreviousBidAmount sends the `amount` from the module account to the `recipient`
// Returns an error if the module is disabled, or on failure to return tokens
func (k Keeper) ReturnPreviousBidAmount(ctx sdk.Context, recipient sdk.AccAddress, amount sdk.Coin) error {
	enabled := k.GetParams(ctx).Enabled
	if !enabled {
		return types.ErrDisabledModule
	}

	err := k.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipient, sdk.NewCoins(amount))
	return errorsmod.Wrap(err, types.ErrFundReturnFailure.Error())
}

// LockBidAmount sends the `amount` from the `sender` to the module account
// Returns an error if the module is disabled, or on failure to lock tokens
func (k Keeper) LockBidAmount(ctx sdk.Context, sender sdk.AccAddress, amount sdk.Coin) error {
	enabled := k.GetParams(ctx).Enabled
	if !enabled {
		return types.ErrDisabledModule
	}

	err := k.BankKeeper.SendCoinsFromAccountToModule(ctx, sender, types.ModuleName, sdk.NewCoins(amount))
	return errorsmod.Wrap(err, types.ErrBidCollectionFailure.Error())
}

// AwardAuction pays out the locked balance of `amount` to `bidder`
// Returns an error if the module is disabled, or on failure to award tokens
func (k Keeper) AwardAuction(ctx sdk.Context, bidder sdk.AccAddress, amount sdk.Coin) error {
	enabled := k.GetParams(ctx).Enabled
	if !enabled {
		return types.ErrDisabledModule
	}

	err := k.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, bidder, sdk.NewCoins(amount))
	return errorsmod.Wrap(err, types.ErrAwardFailure.Error())
}

// IsDenomAuctionable Checks `denom“ against the NonAuctionableTokens list
// Returns true if not in the list and false otherwise
func (k Keeper) IsDenomAuctionable(ctx sdk.Context, denom string) bool {
	if denom == k.MintKeeper.GetParams(ctx).MintDenom {
		return false
	}

	nonAuctionableTokens := k.GetParams(ctx).NonAuctionableTokens
	for _, nonAuctionable := range nonAuctionableTokens {
		if denom == nonAuctionable {
			return false
		}
	}

	return true
}

// Sends tokens from the Auction module's account (for active auctions and bids) to the community pool
// particularly useful when not burning winning bids
func (k Keeper) SendFromAuctionAccountToCommunityPool(ctx sdk.Context, coin sdk.Coin) error {
	return k.sendFromModuleAccountToCommunityPool(ctx, coin, types.ModuleName)
}

// Sends tokens from the Auction Pool's account (for future auctions) to the community pool
// particularly useful when holding a non-auctionable token
func (k Keeper) SendFromAuctionPoolToCommunityPool(ctx sdk.Context, coin sdk.Coin) error {
	return k.sendFromModuleAccountToCommunityPool(ctx, coin, types.AuctionPoolAccountName)
}

// Handles transferring auction module account balances to the community pool
func (k Keeper) sendFromModuleAccountToCommunityPool(ctx sdk.Context, coin sdk.Coin, moduleName string) error {
	coins := sdk.NewCoins(coin)
	if err := k.BankKeeper.SendCoinsFromModuleToModule(ctx, moduleName, distrtypes.ModuleName, coins); err != nil {
		return errorsmod.Wrap(err, "Failure to transfer tokens from auction pool to community pool")
	}
	feePool := k.DistKeeper.GetFeePool(ctx)
	feePool.CommunityPool = feePool.CommunityPool.Add(sdk.NewDecCoinsFromCoins(coins...)...)
	k.DistKeeper.SetFeePool(ctx, feePool)

	return nil
}

// prefixRange turns a prefix into a (start, end) range. The start is the given prefix value and
// the end is calculated by adding 1 bit to the start value. Nil is not allowed as prefix.
// Example: []byte{1, 3, 4} becomes []byte{1, 3, 5}
// []byte{15, 42, 255, 255} becomes []byte{15, 43, 0, 0}
//
// This util function was taken from gravity's keeper package
func prefixRange(prefix []byte) ([]byte, []byte) {
	if prefix == nil {
		panic("nil key not allowed")
	}
	// special case: no prefix is whole range
	if len(prefix) == 0 {
		return nil, nil
	}

	// copy the prefix and update last byte
	end := make([]byte, len(prefix))
	copy(end, prefix)
	l := len(end) - 1
	end[l]++

	// wait, what if that overflowed?....
	for end[l] == 0 && l > 0 {
		l--
		end[l]++
	}

	// okay, funny guy, you gave us FFF, no end to this range...
	if l == 0 && end[0] == 0 {
		end = nil
	}
	return prefix, end
}
