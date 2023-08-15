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

func (k Querier) AuctionPeriodByAuctionId(c context.Context, req *types.QueryAuctionPeriodById) (*types.QueryAuctionPeriodByIdResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	auctionPeriod, found := k.GetAuctionPeriodByID(ctx, req.Id)
	if !found {
		return &types.QueryAuctionPeriodByIdResponse{AuctionPeriod: nil}, nil
	}
	return &types.QueryAuctionPeriodByIdResponse{AuctionPeriod: &auctionPeriod}, nil
}

func (k Querier) AuctionByAuctionIdAndPeriodId(c context.Context, req *types.QueryAuctionByAuctionIdAndPeriodId) (*types.QueryAuctionByAuctionIdAndPeriodIdResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	auction, found := k.GetAuctionByPeriodIDAndAuctionId(ctx, req.PeriodId, req.AuctionId)
	if !found {
		return &types.QueryAuctionByAuctionIdAndPeriodIdResponse{Auction: nil}, nil
	}
	return &types.QueryAuctionByAuctionIdAndPeriodIdResponse{Auction: &auction}, nil
}

func (k Querier) AllAuctionsByBidderAndPeriodId(c context.Context, req *types.QueryAllAuctionsByBidderAndPeriodId) (*types.QueryAllAuctionsByBidderAndPeriodIdResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	auctions := k.GetAllAuctionByBidderAndPeriodId(ctx, req.Address, req.PeriodId)

	return &types.QueryAllAuctionsByBidderAndPeriodIdResponse{Auctions: auctions}, nil
}

func (k Querier) HighestBidByAuctionIdAndPeriodId(c context.Context, req *types.QueryHighestBidByAuctionIdAndPeriodId) (*types.QueryHighestBidByAuctionIdAndPeriodIdResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	bid, found := k.GetHighestBidByAuctionIdAndPeriodID(ctx, req.AuctionId, req.PeriodId)
	if !found {
		return &types.QueryHighestBidByAuctionIdAndPeriodIdResponse{Bid: nil}, nil
	}
	return &types.QueryHighestBidByAuctionIdAndPeriodIdResponse{Bid: &bid}, nil
}
