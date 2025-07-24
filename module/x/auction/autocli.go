package auction

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

func (am AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service:              types.Query_serviceDesc.ServiceName,
			EnhanceCustomCommand: true, // We provide custom Storage and Code in client/cli/tx.go
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "Params",
					Use:       "params",
					Short:     "Get the auction params",
					Long:      "Get the auction parameter values.",
				},
				{
					RpcMethod: "AuctionPeriod",
					Use:       "auction-period",
					Short:     "Get auction period by id",
					Long:      "Get all auction periods for active auctions by id.",
				},
				{
					RpcMethod: "Auctions",
					Use:       "auctions",
					Short:     "Get all active auctions",
					Long:      "Get all active auctions in the store.",
				},
				{
					RpcMethod: "AuctionByDenom",
					Use:       "auction-by-denom [denom]",
					Short:     "Get auction by denom",
					Long:      "Get the auction for a specific token denom.",
				},
				{
					RpcMethod: "AuctionById",
					Use:       "auctions-by-id [id]",
					Short:     "Get auction by id",
					Long:      "Get the auction with a particular id.",
				},
				{
					RpcMethod: "AllAuctionsByBidder",
					Use:       "auctions-by-bidder [bidder]",
					Short:     "Get auctions by bidder",
					Long:      "Get all auctions where a specific bidder has placed the current highest bid.",
				},
				{
					RpcMethod: "AuctionPool",
					Use:       "auction-pool [denom]",
					Short:     "Get auction pool account",
					Long:      "Gets the auction pool account and its current balances of tokens",
				},
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service: types.Msg_serviceDesc.ServiceName,
		},
	}
}
