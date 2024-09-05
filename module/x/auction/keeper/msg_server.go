package keeper

import (
	"context"
	"fmt"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkante "github.com/cosmos/cosmos-sdk/x/auth/ante"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/config"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the gov MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

// nolint: exhaustruct
var _ types.MsgServer = msgServer{}

const BASIS_POINTS_DIVISOR uint64 = 10000 // one basis point is one hundredth of one percent, so fee is amount * (points / 10000)

// Bid processes a MsgBid:
// Performs validation
// Collects fees
// Updates the auction in the store
func (m msgServer) Bid(goCtx context.Context, msg *types.MsgBid) (res *types.MsgBidResponse, err error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	moduleAddress := m.AccountKeeper.GetModuleAddress(types.ModuleName)

	// Store the module and bidder balances for later verification
	var oldBidderAcc, newBidderAcc sdk.AccAddress
	var oldBid, newBid, bidFee sdk.Coin
	var oldBidderStartGrav, oldBidderEndGrav, newBidderStartGrav, newBidderEndGrav sdk.Coin
	modStartGrav := m.BankKeeper.GetBalance(ctx, moduleAddress, config.NativeTokenDenom)
	var modEndGrav sdk.Coin
	var successfulBid bool

	// Schedule balance change assertions after Bid executes
	defer func() {
		// defer will execute this func() just before any `return` from Bid executes

		// Only assert balance changes if this Msg has a chance to be applied to state
		if err == nil {
			// Collect the ending balances
			newBidderEndGrav = m.BankKeeper.GetBalance(ctx, newBidderAcc, config.NativeTokenDenom)
			if !oldBidderAcc.Empty() {
				oldBidderEndGrav = m.BankKeeper.GetBalance(ctx, oldBidderAcc, config.NativeTokenDenom)
			}
			modEndGrav = m.BankKeeper.GetBalance(ctx, moduleAddress, config.NativeTokenDenom)

			// Assert that the correct balance changes have happened
			assertBalanceChanges(successfulBid, modStartGrav, modEndGrav, oldBidderAcc, oldBidderStartGrav, oldBidderEndGrav, newBidderStartGrav, newBidderEndGrav, oldBid, newBid, bidFee)
		}
	}()

	params := m.GetParams(ctx)
	if !params.Enabled {
		return nil, types.ErrDisabledModule
	}

	bidToken := m.MintKeeper.GetParams(ctx).MintDenom
	minBidFee := sdk.NewIntFromUint64(params.MinBidFee)

	// Check the newBidderAcc's address is valid
	newBidderAcc, err = sdk.AccAddressFromBech32(msg.Bidder)
	if err != nil {
		return nil, errorsmod.Wrap(err, "invalid bidder")
	}
	newBidderStartGrav = m.BankKeeper.GetBalance(ctx, newBidderAcc, config.NativeTokenDenom)

	bidder := m.AccountKeeper.GetAccount(ctx, newBidderAcc)

	// Check the bid meets min bid amount
	bidAmount := sdk.NewIntFromUint64(msg.Amount)

	// Check the supplied fee meets the minimum
	feeInt := sdk.NewIntFromUint64(msg.BidFee)
	if feeInt.LT(minBidFee) {
		return nil, errorsmod.Wrapf(types.ErrInvalidBid, "bid fee (%v) must be at least %v", feeInt, minBidFee)
	}

	// Check the bidder is the new highest bidder
	currentAuction := m.Keeper.GetAuctionById(ctx, msg.AuctionId)
	if currentAuction == nil {
		return nil, errorsmod.Wrapf(types.ErrAuctionNotFound, "no active auction with id %d", msg.AuctionId)
	}
	if currentAuction.Amount.Denom == bidToken {
		panic("Bid for auction of the native token")
	}

	oldBidder := ""
	highestBid := currentAuction.HighestBid
	if highestBid != nil {
		if bidAmount.LT(sdk.NewIntFromUint64(highestBid.BidAmount)) {
			return nil, errorsmod.Wrapf(types.ErrBidTooLow, "bid must surpass current highest %v", highestBid)
		}
		oldBidder = highestBid.BidderAddress

		// Disallow re-bidding
		if msg.Bidder == oldBidder {
			return nil, errorsmod.Wrapf(types.ErrInvalidBid, "bidding again as the current highest bidder is not allowed")
		}
		oldBidderAcc = sdk.MustAccAddressFromBech32(oldBidder)
		oldBidderStartGrav = m.BankKeeper.GetBalance(ctx, oldBidderAcc, config.NativeTokenDenom)
	}

	// Check the bidder's balance is sufficient
	totalValue := bidAmount.Add(feeInt)
	bidderBalance := m.BankKeeper.GetBalance(ctx, newBidderAcc, bidToken) // Nonexistant accounts are treated as having 0 balance
	if bidderBalance.Amount.LT(totalValue) {
		return nil,
			errorsmod.Wrapf(
				sdkerrors.ErrInsufficientFunds,
				"insufficient balance, bid=[%v] fee=[%v] balance=[%v]",
				bidAmount, feeInt, bidderBalance,
			)
	}

	bidFee = sdk.NewCoin(bidToken, feeInt)
	transferToStakers := sdk.NewCoins(bidFee)
	transferToModule := sdk.NewCoin(bidToken, bidAmount)

	// Deduct the stakers' fee
	err = sdkante.DeductFees(m.BankKeeper, ctx, bidder, transferToStakers)
	if err != nil {
		ctx.Logger().Error("Could not deduct bid fee!", "error", err, "account", newBidderAcc, "fee", transferToStakers)
		return nil, err
	}

	// Release the old highest bid
	if highestBid != nil {
		oldBid = sdk.NewCoin(bidToken, sdk.NewIntFromUint64(highestBid.BidAmount))
		oldBidder := sdk.MustAccAddressFromBech32(highestBid.BidderAddress)
		err := m.Keeper.ReturnPreviousBidAmount(ctx, oldBidder, oldBid)
		if err != nil {
			ctx.Logger().Error("Could not return previous highest bid!", "error", err, "oldBidder", oldBidder, "oldBid", oldBid)
			return nil, err
		}
	}

	// Transfer the bid amount to the module
	if err := m.Keeper.LockBidAmount(ctx, newBidderAcc, transferToModule); err != nil {
		return nil, errorsmod.Wrap(err, "unable to lock bid amount")
	}

	// Store the msg sender as the current highest bidder
	updatedBid := types.Bid{
		BidAmount:     msg.Amount,
		BidderAddress: msg.Bidder,
	}
	if err := m.Keeper.UpdateHighestBidder(ctx, currentAuction.Id, updatedBid); err != nil {
		return nil, errorsmod.Wrap(err, "unable to update highest bidder")
	}

	newBid = sdk.NewCoin(config.NativeTokenDenom, sdk.NewIntFromUint64(updatedBid.BidAmount))

	// Emit an event to mark a new highest bidder
	ctx.EventManager().EmitEvent(types.NewEventNewHighestBidder(msg.AuctionId, sdk.NewIntFromUint64(msg.Amount), oldBidder))

	successfulBid = true

	return &types.MsgBidResponse{}, nil
}

func assertBalanceChanges(
	successfulBid bool,
	modStartGrav, modEndGrav sdk.Coin,
	oldBidderAcc sdk.AccAddress,
	oldBidderStartGrav, oldBidderEndGrav sdk.Coin,
	newBidderStartGrav, newBidderEndGrav sdk.Coin,
	oldBid, newBid, bidFee sdk.Coin,
) {
	auctionHasOldBidder := !oldBidderAcc.Empty()
	// An unsuccessful bid should not change any balances (gas + Tx fees are out of scope before the Msg is executed)
	if !successfulBid {
		if !modStartGrav.Equal(modEndGrav) {
			panic(fmt.Sprintf("Unexpected module balance change with unsuccessful bid: %v -> %v", modStartGrav, modEndGrav))
		}
		if auctionHasOldBidder {
			if !oldBidderStartGrav.Equal(oldBidderEndGrav) {
				panic(fmt.Sprintf("Unexpected old bidder balance change with unsuccessful bid: %v -> %v", oldBidderStartGrav, oldBidderEndGrav))
			}
		}
		if !newBidderStartGrav.Equal(newBidderEndGrav) {
			panic(fmt.Sprintf("Unexpected new bidder balance change with unsuccessful bid: %v -> %v", newBidderStartGrav, newBidderEndGrav))
		}

		return
	}

	// Otherwise the bid was successful, the module account should now hold the new bidder's bid.
	// If there was an old bidder, they should have regained their bid.

	modDiff := modEndGrav.Sub(modStartGrav)
	if auctionHasOldBidder {
		// The old bidder's balance should increase
		oldBidderDiff := oldBidderEndGrav.Sub(oldBidderStartGrav)
		if !oldBidderDiff.Equal(oldBid) {
			panic(fmt.Sprintf("Expected old bidder to receive their locked Grav (%v), instead their balance increased by: %v", oldBid.Amount, oldBidderDiff))
		}

		// The module's balance should decrease by the released bid amount, but increase by the higher new bid amount
		outbidAmount := newBid.Sub(oldBid)
		if !modDiff.Equal(outbidAmount) {
			panic(fmt.Sprintf("Expected module Grav balance to increase by (new bid - old bid = %v), instead it increased by: %v", outbidAmount, modDiff))
		}
	} else if !modDiff.Equal(newBid) {
		// The module's balance should increase by exactly the bid amount
		panic(fmt.Sprintf("Expected module Grav balance to increase by new bid (%v), instead it increased by: %v", newBid, modDiff))
	}

	// The new bidder's balance should decrease by their bid amount + bid fee
	newBidderDiff := newBidderStartGrav.Sub(newBidderEndGrav)
	if !newBidderDiff.Equal(newBid.Add(bidFee)) {
		panic(fmt.Sprintf("Expected new bidder Grav balance to decrease by bid + fee amount, instead their balance decreased by: %v", newBidderDiff))
	}

}
