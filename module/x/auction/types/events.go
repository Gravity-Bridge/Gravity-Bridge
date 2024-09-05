package types

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// When a new auction period starts
	EventTypePeriodStart         = "auction_period_start"
	AttributeKeyPeriodStartStart = "start_height"
	AttributeKeyPeriodStartEnd   = "end_height"

	// When a new auction is created
	EventTypeAuction          = "auction"
	AttributeKeyAuctionId     = "id"
	AttributeKeyAuctionDenom  = "denom"
	AttributeKeyAuctionAmount = "amount"

	// When the highest bidder changes on an auction
	EventTypeNewHighestBidder = "new_highest_bidder"
	AttributeKeyHBAuctionId   = "auction_id"
	AttributeKeyHBBidAmount   = "bid_amount"
	AttributeKeyHBOldBidder   = "old_bidder"

	// When an auction is awarded to the highest bidder
	EventTypeAuctionAward      = "auction_award"
	AttributeKeyAwardAuctionId = "auction_id"
	AttributeKeyAwardBidAmount = "bid_amount"
	AttributeKeyAwardBidder    = "bidder"
	AttributeKeyAwardAmount    = "award_amount"
	AttributeKeyAwardDenom     = "award_denom"

	// When no sufficient bids are made for an auction
	EventTypeAuctionFailure      = "auction_failure"
	AttributeKeyFailureAuctionId = "auction_id"
	AttributeKeyFailureAmount    = "amount"
	AttributeKeyFailureDenom     = "denom"

	// When an auction period ends
	EventTypePeriodEnd         = "auction_period_end"
	AttributeKeyPeriodEndStart = "start_height"
	AttributeKeyPeriodEndEnd   = "end_height"
)

// NewEventPeriodStart creates an event to mark a new auction period
func NewEventPeriodStart(startHeight uint64, endHeight uint64) sdk.Event {
	return sdk.NewEvent(
		EventTypePeriodStart,
		sdk.NewAttribute(AttributeKeyPeriodStartStart, fmt.Sprint(startHeight)),
		sdk.NewAttribute(AttributeKeyPeriodStartEnd, fmt.Sprint(endHeight)),
	)
}

// NewEventAuction creates an event to mark the creation of an auction
func NewEventAuction(id uint64, auctionDenom string, auctionAmount math.Int) sdk.Event {
	return sdk.NewEvent(
		EventTypeAuction,
		sdk.NewAttribute(AttributeKeyAuctionId, fmt.Sprint(id)),
		sdk.NewAttribute(AttributeKeyAuctionDenom, auctionDenom),
		sdk.NewAttribute(AttributeKeyAuctionAmount, auctionAmount.String()),
	)
}

// NewEventNewHighestBidder creates an event to mark the designation of a highest bidder
func NewEventNewHighestBidder(auctionId uint64, bidAmount math.Int, oldBidder string) sdk.Event {
	return sdk.NewEvent(
		EventTypeNewHighestBidder,
		sdk.NewAttribute(AttributeKeyHBAuctionId, fmt.Sprint(auctionId)),
		sdk.NewAttribute(AttributeKeyHBBidAmount, bidAmount.String()),
		sdk.NewAttribute(AttributeKeyHBOldBidder, oldBidder),
	)
}

// NewEventAuctionAward creates an event to mark the award of an auction to its highest bidder
func NewEventAuctionAward(auctionId uint64, bidAmount math.Int, bidder sdk.AccAddress, awardDenom string, awardAmount math.Int) sdk.Event {
	return sdk.NewEvent(
		EventTypeAuctionAward,
		sdk.NewAttribute(AttributeKeyAwardAuctionId, fmt.Sprint(auctionId)),
		sdk.NewAttribute(AttributeKeyAwardBidAmount, bidAmount.String()),
		sdk.NewAttribute(AttributeKeyAwardBidder, bidder.String()),
		sdk.NewAttribute(AttributeKeyAwardAmount, awardDenom),
		sdk.NewAttribute(AttributeKeyAwardDenom, awardAmount.String()),
	)
}

// NewEventAuctionFailure creates an event to mark an auction which has not received a minimum bid
func NewEventAuctionFailure(auctionId uint64, auctionDenom string, auctionAmount math.Int) sdk.Event {
	return sdk.NewEvent(
		EventTypeAuctionFailure,
		sdk.NewAttribute(AttributeKeyAuctionId, fmt.Sprint(auctionId)),
		sdk.NewAttribute(AttributeKeyAuctionDenom, auctionDenom),
		sdk.NewAttribute(AttributeKeyAuctionAmount, auctionAmount.String()),
	)
}

// NewEventPeriodEnd creates an event to mark the end of an auction period
func NewEventPeriodEnd(startHeight uint64, endHeight uint64) sdk.Event {
	return sdk.NewEvent(
		EventTypePeriodEnd,
		sdk.NewAttribute(AttributeKeyPeriodEndStart, fmt.Sprint(startHeight)),
		sdk.NewAttribute(AttributeKeyPeriodEndEnd, fmt.Sprint(endHeight)),
	)
}
