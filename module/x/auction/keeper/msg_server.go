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
	bidderBalance := k.BankKeeper.GetBalance(sdkCtx, sdk.AccAddress(msg.Bidder), msg.Amount.Denom)
	if bidderBalance.IsLT(*msg.Amount) {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInsufficientFunds, "Insurfficient balance, expect to have: %v instead have: %v", msg.Amount.Amount, bidderBalance.Amount)
	}

	// Check if bid amount is greater than min bid amount allowed
	if msg.Amount.IsLT(sdk.NewCoin(msg.Amount.Denom, sdk.NewIntFromUint64(params.MinBidAmount))) {
		return nil, types.ErrInvalidBidAmount
	}

	bidsQueue, found := k.GetBidsQueue(sdkCtx, msg.AuctionId)
	if !found {
		return nil, fmt.Errorf("Bids queue for auction with id %v", msg.AuctionId)
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

	currentAuction, found := k.GetAuctionByPeriodIDAndAuctionId(sdkCtx, latestAuctionPeriod.Id, msg.AuctionId)
	if !found {
		return nil, types.ErrAuctionNotFound
	}
	highestBid := currentAuction.HighestBid

	// If highest bid exist need to check the bid gap
	if highestBid != nil &&
		msg.Amount.IsGTE(*highestBid.BidAmount) &&
		(msg.Amount.Sub(*highestBid.BidAmount)).IsLT(sdk.NewCoin(msg.Amount.Denom, sdk.NewIntFromUint64(params.BidGap))) {
		return nil, types.ErrInvalidBidAmountGap
	}

	if len(bidsQueue.Queue) == 0 {
		// For empty queue just add the new entry
		newBid := &types.Bid{
			AuctionId:     msg.AuctionId,
			BidAmount:     msg.Amount,
			BidderAddress: msg.Bidder,
		}
		k.AddBidToQueue(sdkCtx, *newBid, &bidsQueue)
	} else {
		for i, bid := range bidsQueue.Queue {
			// Check if bid entry from exact bidder exist yet
			if bid.AuctionId == msg.AuctionId && bid.BidderAddress == msg.Bidder {
				// Update bid amount of old entry
				bid.BidAmount = msg.Amount

				bidsQueue.Queue[i] = bid

				k.SetBidsQueue(sdkCtx, bidsQueue, msg.AuctionId)
			} else {
				newBid := &types.Bid{
					AuctionId:     msg.AuctionId,
					BidAmount:     msg.Amount,
					BidderAddress: msg.Bidder,
				}
				k.AddBidToQueue(sdkCtx, *newBid, &bidsQueue)
			}
		}
	}

	// nolint: exhaustruct
	return &types.MsgBidResponse{}, nil
}
