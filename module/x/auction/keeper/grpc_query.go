package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

type Querier struct {
	Keeper
}

func (k Querier) Params(c context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	params := k.GetParams(ctx)

	return &types.QueryParamsResponse{Params: params}, nil
}

func (k Querier) LatestAuctionPeriod(c context.Context, req *types.QueryLatestAuctionPeriod) (*types.QueryLatestAuctionPeriodResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	auctionPeriod, found := k.GetLatestAuctionPeriod(ctx)
	if !found {
		return &types.QueryLatestAuctionPeriodResponse{AuctionPeriod: nil}, nil
	}
	return &types.QueryLatestAuctionPeriodResponse{AuctionPeriod: &auctionPeriod}, nil
}

func (k Querier) AuctionByAuctionId(c context.Context, req *types.QueryAuctionByAuctionId) (*types.QueryAuctionByAuctionIdResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	auction, found := k.GetAuctionById(ctx, req.AuctionId)
	if !found {
		return &types.QueryAuctionByAuctionIdResponse{Auction: nil}, nil
	}
	return &types.QueryAuctionByAuctionIdResponse{Auction: &auction}, nil
}

func (k Querier) AllAuctionsByBidder(c context.Context, req *types.QueryAllAuctionsByBidder) (*types.QueryAllAuctionsByBidderResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	auctions := k.GetAllAuctionByBidder(ctx, req.Address)

	return &types.QueryAllAuctionsByBidderResponse{Auctions: auctions}, nil
}

func (k Querier) HighestBidByAuctionId(c context.Context, req *types.QueryHighestBidByAuctionId) (*types.QueryHighestBidByAuctionIdResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	bid, found := k.GetHighestBidByAuctionId(ctx, req.AuctionId)
	if !found {
		return &types.QueryHighestBidByAuctionIdResponse{Bid: nil}, nil
	}
	return &types.QueryHighestBidByAuctionIdResponse{Bid: &bid}, nil
}
