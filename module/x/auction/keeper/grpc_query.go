package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

// nolint: exhaustruct
var _ types.QueryServer = Keeper{}

// Params returns the current module Params
func (k Keeper) Params(c context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	var params types.Params
	k.paramSpace.GetParamSet(sdk.UnwrapSDKContext(c), &params)
	return &types.QueryParamsResponse{Params: params}, nil
}

// AllAuctionsByBidder returns any auctions whose Highest Bidder is the requested bidder
func (k Keeper) AllAuctionsByBidder(c context.Context, req *types.QueryAllAuctionsByBidderRequest) (*types.QueryAllAuctionsByBidderResponse, error) {
	auctions := k.GetAllAuctionsByBidder(sdk.UnwrapSDKContext(c), req.Address)

	return &types.QueryAllAuctionsByBidderResponse{Auctions: auctions}, nil
}

// AuctionByDenom returns the auction with the given denom, if any exists
func (k Keeper) AuctionByDenom(c context.Context, req *types.QueryAuctionByDenomRequest) (*types.QueryAuctionByDenomResponse, error) {
	auction := k.GetAuctionByDenom(sdk.UnwrapSDKContext(c), req.AuctionDenom)

	return &types.QueryAuctionByDenomResponse{Auction: auction}, nil
}

// AuctionById returns the auction with the given id, if any exists
func (k Keeper) AuctionById(c context.Context, req *types.QueryAuctionByIdRequest) (*types.QueryAuctionByIdResponse, error) {
	auction := k.GetAuctionById(sdk.UnwrapSDKContext(c), req.AuctionId)

	return &types.QueryAuctionByIdResponse{Auction: auction}, nil
}

// AuctionPeriod returns information about the current auction period
func (k Keeper) AuctionPeriod(c context.Context, req *types.QueryAuctionPeriodRequest) (*types.QueryAuctionPeriodResponse, error) {
	period := k.GetAuctionPeriod(sdk.UnwrapSDKContext(c))

	return &types.QueryAuctionPeriodResponse{AuctionPeriod: period}, nil
}

// Auctions returns all the active auctions
func (k Keeper) Auctions(c context.Context, req *types.QueryAuctionsRequest) (*types.QueryAuctionsResponse, error) {
	auctions := k.GetAllAuctions(sdk.UnwrapSDKContext(c))

	return &types.QueryAuctionsResponse{Auctions: auctions}, nil
}

// AuctionPool returns the balances waiting to be included in the next auction period. Note that this does not include
// the balances from active auctions which have no highest bidder, but those will be recycled into new auctions as well
func (k Keeper) AuctionPool(c context.Context, req *types.QueryAuctionPoolRequest) (*types.QueryAuctionPoolResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	account := k.GetAuctionPoolAccount(ctx).String()
	balances := k.GetAuctionPoolBalances(ctx)

	return &types.QueryAuctionPoolResponse{Account: account, Balances: balances}, nil
}
