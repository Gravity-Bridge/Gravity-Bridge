package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkante "github.com/cosmos/cosmos-sdk/x/auth/ante"

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
	params := m.GetParams(ctx)
	bidToken := m.MintKeeper.GetParams(ctx).MintDenom
	minBidFee := sdk.NewIntFromUint64(params.MinBidFee)

	// Check the bidderAddress's address is valid
	bidderAddress, err := sdk.AccAddressFromBech32(msg.Bidder)
	if err != nil {
		return nil, sdkerrors.Wrap(err, "invalid bidder")
	}
	bidder := m.AccountKeeper.GetAccount(ctx, bidderAddress)

	// Check the bid meets min bid amount
	bidAmount := sdk.NewIntFromUint64(msg.Amount)

	// Check the supplied fee meets the minimum
	feeInt := sdk.NewIntFromUint64(msg.BidFee)
	if feeInt.LT(minBidFee) {
		return nil, sdkerrors.Wrapf(types.ErrInvalidBid, "bid fee (%v) must be at least %v", feeInt, minBidFee)
	}

	// Check the bidder is the new highest bidder
	currentAuction := m.Keeper.GetAuctionById(ctx, msg.AuctionId)
	if currentAuction == nil {
		return nil, sdkerrors.Wrapf(types.ErrAuctionNotFound, "no active auction with id %d", msg.AuctionId)
	}
	oldBidder := ""
	highestBid := currentAuction.HighestBid
	if highestBid != nil {
		if bidAmount.LT(sdk.NewIntFromUint64(highestBid.BidAmount)) {
			return nil, sdkerrors.Wrapf(types.ErrBidTooLow, "bid must surpass current highest %v", highestBid)
		}
		oldBidder = highestBid.BidderAddress

		// Disallow re-bidding
		if msg.Bidder == oldBidder {
			return nil, sdkerrors.Wrapf(types.ErrInvalidBid, "bidding again as the current highest bidder is not allowed")
		}
	}

	// Check the bidder's balance is sufficient
	totalValue := bidAmount.Add(feeInt)
	bidderBalance := m.BankKeeper.GetBalance(ctx, bidderAddress, bidToken) // Nonexistant accounts are treated as having 0 balance
	if bidderBalance.Amount.LT(totalValue) {
		return nil,
			sdkerrors.Wrapf(
				sdkerrors.ErrInsufficientFunds,
				"insufficient balance, bid=[%v] fee=[%v] balance=[%v]",
				bidAmount, feeInt, bidderBalance,
			)
	}

	transferToStakers := sdk.NewCoins(sdk.NewCoin(bidToken, feeInt))
	transferToModule := sdk.NewCoin(bidToken, bidAmount)

	// Deduct the stakers' fee
	err = sdkante.DeductFees(m.BankKeeper, ctx, bidder, transferToStakers)
	if err != nil {
		ctx.Logger().Error("Could not deduct bid fee!", "error", err, "account", bidderAddress, "fee", transferToStakers)
		return nil, err
	}

	// Release the old highest bid
	if highestBid != nil {
		oldBid := sdk.NewCoin(bidToken, sdk.NewIntFromUint64(highestBid.BidAmount))
		oldBidder := sdk.MustAccAddressFromBech32(highestBid.BidderAddress)
		err := m.Keeper.ReturnPreviousBidAmount(ctx, oldBidder, oldBid)
		if err != nil {
			ctx.Logger().Error("Could not return previous highest bid!", "error", err, "oldBidder", oldBidder, "oldBid", oldBid)
			return nil, err
		}
	}

	// Transfer the bid amount to the module
	if err := m.Keeper.LockBidAmount(ctx, bidderAddress, transferToModule); err != nil {
		return nil, sdkerrors.Wrap(err, "unable to lock bid amount")
	}

	// Store the msg sender as the current highest bidder
	updatedBid := types.Bid{
		BidAmount:     msg.Amount,
		BidderAddress: msg.Bidder,
	}
	if err := m.Keeper.UpdateHighestBidder(ctx, currentAuction.Id, updatedBid); err != nil {
		return nil, sdkerrors.Wrap(err, "unable to update highest bidder")
	}

	// Emit an event to mark a new highest bidder
	ctx.EventManager().EmitEvent(types.NewEventNewHighestBidder(msg.AuctionId, sdk.NewIntFromUint64(msg.Amount), oldBidder))

	return &types.MsgBidResponse{}, nil
}
