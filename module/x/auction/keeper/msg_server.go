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

func (k msgServer) Bid(ctx context.Context, msg *types.MsgBid) (res *types.MsgBidResponse, err error) {
	err = msg.ValidateBasic()
	if err != nil {
		return nil, sdkerrors.Wrap(err, "Key not valid")
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(sdkCtx)
	// Fetch current auction period
	latestAuctionPeriod, found := k.GetLatestAuctionPeriod(sdkCtx)
	if !found {
		return nil, types.ErrNoPreviousAuctionPeriod
	}

	currentAuction, found := k.GetAuctionByPeriodIDAndAuctionId(sdkCtx, latestAuctionPeriod.Id, msg.AuctionId)

	if !found {
		return nil, types.ErrAuctionNotFound
	}
	highestBid := currentAuction.HighestBid

	// Check bid amount gap
	if (msg.Amount.Sub(*highestBid.BidAmount)).Amount.Uint64() < params.BidGap {
		return nil, types.ErrInvalidBidAmountGap
	}

	var bid *types.Bid

	if highestBid.BidderAddress == msg.Bidder {
		bidAmountGap := msg.Amount.Sub(*highestBid.BidAmount)
		// Send the added amount to auction module
		err := k.lockBidAmount(sdkCtx, msg.Bidder, bidAmountGap)
		if err != nil {
			return nil, fmt.Errorf("Unable to send fund to the auction module account: %s", err.Error())
		}
		bid = &types.Bid{
			AuctionId:     highestBid.AuctionId,
			BidAmount:     msg.Amount,
			BidderAddress: highestBid.BidderAddress,
		}
	} else {
		// Return fund to the pervious highest bidder
		err := k.returnPrevioudBidAmount(sdkCtx, highestBid.BidderAddress, *highestBid.BidAmount)
		if err != nil {
			return nil, fmt.Errorf("Unable to return fund to the previous highest bidder: %s", err.Error())
		}

		err = k.lockBidAmount(sdkCtx, msg.Bidder, *msg.Amount)
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

func (k msgServer) returnPrevioudBidAmount(ctx sdk.Context, recipient string, amount sdk.Coin) error {
	sdkAcc, err := sdk.AccAddressFromBech32(recipient)
	if err != nil {
		return fmt.Errorf("Unable to get account from Bech32 address: %s", err.Error())
	}
	err = k.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sdkAcc, sdk.NewCoins(amount))
	return err
}

func (k msgServer) lockBidAmount(ctx sdk.Context, sender string, amount sdk.Coin) error {
	sdkAcc, err := sdk.AccAddressFromBech32(sender)
	if err != nil {
		return fmt.Errorf("Unable to get account from Bech32 address: %s", err.Error())
	}
	err = k.BankKeeper.SendCoinsFromAccountToModule(ctx, sdkAcc, types.ModuleName, sdk.NewCoins(amount))
	return err
}
