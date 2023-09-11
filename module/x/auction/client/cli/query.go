package cli

import (
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

// GetQueryCmd returns the cli query commands for this module
func GetQueryCmd() *cobra.Command {
	// Group auction queries under a subcommand
	// nolint: exhaustruct
	auctionQueryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	auctionQueryCmd.AddCommand([]*cobra.Command{
		GetCmdQueryParams(),
		GetCmdAllAuction(),
		GetCmdAuction(),
		GetCmdAuctionPeriods(),
		GetCmdHighestBid(),
	}...)

	return auctionQueryCmd
}
func GetCmdQueryParams() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "params",
		Args:  cobra.NoArgs,
		Short: "Query auction params",
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.Params(cmd.Context(), &types.QueryParamsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(&res.Params)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdAuctionPeriods fetches auction periods by id
func GetCmdAuctionPeriods() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "auction-periods-by-id [id]",
		Args:  cobra.ExactArgs(1),
		Short: "Query auction periods by id",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			argId, err := strconv.ParseUint(args[0], 10, 64)

			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.AuctionPeriodByAuctionId(cmd.Context(), &types.QueryAuctionPeriodById{
				Id: argId,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdAuction fetches auction by auction id and period id
// nolint: dupl
func GetCmdAuction() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "auction-by-aid-pid [auction-id] [period-id]",
		Args:  cobra.ExactArgs(2),
		Short: "Query auction by auction id and period id",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			argAuctionId, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}

			argPeriodId, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.AuctionByAuctionIdAndPeriodId(cmd.Context(), &types.QueryAuctionByAuctionIdAndPeriodId{
				AuctionId: argAuctionId,
				PeriodId:  argPeriodId,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdAllAuctionByBidderAndPeriodId fetches all auctions by bidder address and auction period id
func GetCmdAllAuction() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "autions-by-bidder-pid [address] [period-id]",
		Args:  cobra.ExactArgs(2),
		Short: "Query all auctions by bidder address and auction period id",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			argPeriodId, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}

			res, err := queryClient.AllAuctionsByBidderAndPeriodId(cmd.Context(), &types.QueryAllAuctionsByBidderAndPeriodId{
				Address:  args[0],
				PeriodId: argPeriodId,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdHighestBidBy fetches the highest bid of the auction with auction id and period id
// nolint: dupl
func GetCmdHighestBid() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "highest-bid-by-id [auction-id] [period-id]",
		Args:  cobra.ExactArgs(2),
		Short: "Query the highest bid of the auction with auction id and period id",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			argAuctionId, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}

			argPeriodId, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.HighestBidByAuctionIdAndPeriodId(cmd.Context(), &types.QueryHighestBidByAuctionIdAndPeriodId{
				AuctionId: argAuctionId,
				PeriodId:  argPeriodId,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
