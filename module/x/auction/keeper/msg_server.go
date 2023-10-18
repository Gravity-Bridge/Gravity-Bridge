package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

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

// Bid msg add a bid entry to the queue to be processed by the end of each block
func (k msgServer) Bid(ctx context.Context, msg *types.MsgBid) (res *types.MsgBidResponse, err error) {
	err = msg.ValidateBasic()

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(sdkCtx)
	if err != nil {
		return nil, sdkerrors.Wrap(err, "Key not valid")
	}

	// Only accept native token denom only
	if msg.Amount.Denom != k.StakingKeeper.BondDenom(sdkCtx) {
		return nil, fmt.Errorf("Invalid denom %s should be %s", msg.Amount.Denom, k.StakingKeeper.BondDenom(sdkCtx))
	}

	// Check if bidder has enough balance to submit a bid
	bidderBalance := k.BankKeeper.GetBalance(sdkCtx, sdk.MustAccAddressFromBech32(msg.Bidder), msg.Amount.Denom)
	if bidderBalance.IsLT(msg.Amount) {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInsufficientFunds, "Insurfficient balance, expect to have: %v instead have: %v", msg.Amount.Amount, bidderBalance.Amount)
	}

	// Check if bid amount is greater than min bid amount allowed
	if msg.Amount.IsLT(sdk.NewCoin(msg.Amount.Denom, sdk.NewIntFromUint64(params.MinBidAmount))) {
		return nil, types.ErrInvalidBidAmount
	}

	// Fetch current auction period
	latestAuctionPeriod, found := k.GetLatestAuctionPeriod(sdkCtx)
	if !found {
		return nil, types.ErrNoPreviousAuctionPeriod
	}

	// check if an auction periods is occuring
	if latestAuctionPeriod.EndBlockHeight < uint64(sdkCtx.BlockHeight()) {
		return nil, fmt.Errorf("Cannot submit bid for Auction Periods that is had passed")
	}

	currentAuction, found := k.GetAuctionById(sdkCtx, msg.AuctionId)
	if !found {
		return nil, types.ErrAuctionNotFound
	}
	highestBid := currentAuction.HighestBid

	// If highest bid exist need to check the bid gap and bid amount is higher than previous highest bid amount
	if highestBid != nil {
		if msg.Amount.IsGTE(highestBid.BidAmount) &&
			(msg.Amount.Sub(highestBid.BidAmount)).IsLT(sdk.NewCoin(msg.Amount.Denom, sdk.NewIntFromUint64(params.BidGap))) {
			return nil, types.ErrInvalidBidAmountGap
		}
		if msg.Amount.IsLT(highestBid.BidAmount) {
			return nil, types.ErrInvalidBidAmount
		}
	}

	var bid *types.Bid

	switch {
	case highestBid == nil:
		err = k.LockBidAmount(sdkCtx, msg.Bidder, msg.Amount)
		if err != nil {
			return nil, fmt.Errorf("Unable to send fund to the auction module account: %s", err.Error())
		}
		bid = &types.Bid{
			AuctionId:     msg.AuctionId,
			BidAmount:     msg.Amount,
			BidderAddress: msg.Bidder,
		}

	case highestBid.BidderAddress == msg.Bidder:
		bidAmountGap := msg.Amount.Sub(highestBid.BidAmount)
		// Send the added amount to auction module
		err := k.LockBidAmount(sdkCtx, msg.Bidder, bidAmountGap)
		if err != nil {
			return nil, fmt.Errorf("Unable to send fund to the auction module account: %s", err.Error())
		}
		bid = &types.Bid{
			AuctionId:     highestBid.AuctionId,
			BidAmount:     msg.Amount,
			BidderAddress: highestBid.BidderAddress,
		}

	default:
		// Return fund to the pervious highest bidder
		err := k.ReturnPrevioudBidAmount(sdkCtx, highestBid.BidderAddress, highestBid.BidAmount)
		if err != nil {
			return nil, fmt.Errorf("Unable to return fund to the previous highest bidder: %s", err.Error())
		}

		err = k.LockBidAmount(sdkCtx, msg.Bidder, msg.Amount)
		if err != nil {
			return nil, fmt.Errorf("Unable to send fund to the auction module account: %s", err.Error())
		}
		bid = &types.Bid{
			AuctionId:     highestBid.AuctionId,
			BidAmount:     msg.Amount,
			BidderAddress: msg.Bidder,
		}
	}

	// Update the new bid entry
	k.UpdateAuctionNewBid(sdkCtx, msg.AuctionId, *bid)

	// nolint: exhaustruct
	return &types.MsgBidResponse{}, nil
}
