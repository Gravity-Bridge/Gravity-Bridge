package cli

import (
	"fmt"
	"strconv"

	errorsmod "cosmossdk.io/errors"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"

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
		GetCmdParams(),
		GetCmdAuctionPeriod(),
		GetCmdAuctions(),
		GetCmdAuctionById(),
		GetCmdAuctionByDenom(),
		GetCmdAllAuctionsByBidder(),
		GetCmdAuctionPool(),
	}...)

	return auctionQueryCmd
}
func GetCmdParams() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "params",
		Args:  cobra.NoArgs,
		Short: "Fetch the auction module params",
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
			if res == nil {
				return fmt.Errorf("could not get the auction module params")
			}

			return clientCtx.PrintProto(&res.Params)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdAuctionPeriod fetches the current auction period
func GetCmdAuctionPeriod() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "auction-period",
		Args:  cobra.NoArgs,
		Short: "Query auction periods by id",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.AuctionPeriod(cmd.Context(), &types.QueryAuctionPeriodRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdAuctions fetches all auctions in the store
func GetCmdAuctions() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "auctions",
		Args:  cobra.NoArgs,
		Short: "Fetch active auctions",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.Auctions(cmd.Context(), &types.QueryAuctionsRequest{})
			if err != nil {
				return err
			}
			if res == nil || len(res.Auctions) == 0 {
				return fmt.Errorf("could not find any auctions, query the auction-period to learn when new auctions will be created")
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdAuctionByDenom fetches an auction with the given denom, if any exists
func GetCmdAuctionByDenom() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "auction-by-denom [denom]",
		Args:  cobra.ExactArgs(1),
		Short: "Fetch an active auction with the given denom",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			denom := args[0]
			err = sdk.ValidateDenom(denom)
			if err != nil {
				return errorsmod.Wrap(err, "Invalid query denom")
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.AuctionByDenom(cmd.Context(), &types.QueryAuctionByDenomRequest{
				AuctionDenom: denom,
			})
			if err != nil {
				return err
			}
			if res == nil || res.Auction == nil {
				return fmt.Errorf("could not find an auction for the given denom")
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdAuctionById fetches for an auction with the given id, if any exists
func GetCmdAuctionById() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "auction-by-id [id]",
		Args:  cobra.ExactArgs(1),
		Short: "Fetch an active auction with the given id",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return errorsmod.Wrap(err, "Invalid query id")
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.AuctionById(cmd.Context(), &types.QueryAuctionByIdRequest{
				AuctionId: id,
			})
			if err != nil {
				return err
			}
			if res == nil || res.Auction == nil {
				return fmt.Errorf("could not find an auction with id %v", id)
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdAllAuctionsByBidder all the auctions for which bidderAddress is the current highest bidder
func GetCmdAllAuctionsByBidder() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "auctions-by-bidder [bidderAddress]",
		Args:  cobra.ExactArgs(1),
		Short: "Fetch all auctions where bidderAddress is the current highest bidder",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			address := args[0]
			accAddress, err := sdk.AccAddressFromBech32(address)
			if err != nil {
				return errorsmod.Wrap(err, "Invalid query address")
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.AllAuctionsByBidder(cmd.Context(), &types.QueryAllAuctionsByBidderRequest{
				Address: accAddress.String(),
			})
			if err != nil {
				return err
			}
			if res == nil || len(res.Auctions) == 0 {
				return fmt.Errorf("could not find any auctions with the given bidder")
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdAuctionPool fetches the auction pool account and its balances
func GetCmdAuctionPool() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "auction-pool",
		Args:  cobra.NoArgs,
		Short: "Query the auction pool and its balances",
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.AuctionPool(cmd.Context(), &types.QueryAuctionPoolRequest{})
			if err != nil {
				return err
			}
			if res == nil {
				return fmt.Errorf("could not fetch the auction pool, try again or specify a different node")
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
