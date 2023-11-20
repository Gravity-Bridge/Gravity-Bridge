package cli

import (
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

func CmdMsgBid() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "bid [auction-id] [amount] [optional fee]",
		Short: "Submit a bid on an auction: bid MUST be the new highest bid or will be rejected. Automatically supplies the minimum fee according to the module parameters.",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			auctionId, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid auction ID provided: %v", err)
			}

			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			offline, err := cmd.Flags().GetBool(flags.FlagOffline)
			if err != nil {
				return fmt.Errorf("unable to read offline flag: %v", err)
			}

			var params *types.Params
			if !clientCtx.Offline && !offline {
				queryCtx, err := client.GetClientQueryContext(cmd)
				if err != nil {
					return err
				}
				queryClient := types.NewQueryClient(queryCtx)
				res, err := queryClient.Params(cmd.Context(), &types.QueryParamsRequest{})
				if err != nil {
					return err
				}
				params = &res.Params
			}

			// If the user provided a custom fee, use that. Otherwise fetch the minimum and provide that amount.
			var bidFee uint64
			if len(args) == 2 {
				if offline {
					return fmt.Errorf("unable to respect '--%s' when no fee value is provided", flags.FlagOffline)
				}
				if params == nil {
					return fmt.Errorf("failed to get auction module params, try again or provide a bid fee")
				}
				bidFee = params.MinBidFee
			} else {
				intFee, err := strconv.Atoi(args[2])
				if err != nil {
					return fmt.Errorf("invalid fee provided: %v", err)
				}
				bidFee = uint64(intFee)
			}

			argAmount, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid amount: %v", err)
			}

			// Helpful checks if the params have been fetched
			if params != nil {
				if !params.Enabled {
					return fmt.Errorf("the auction module is currently disabled, bidding will fail")
				}

				// Check the current auction highest bid and error if bid is too low
				queryCtx, err := client.GetClientQueryContext(cmd)
				if err != nil {
					return err
				}
				queryClient := types.NewQueryClient(queryCtx)
				res, err := queryClient.AuctionById(cmd.Context(), &types.QueryAuctionByIdRequest{AuctionId: uint64(auctionId)})
				if err != nil {
					return fmt.Errorf("failed to fetch auction with id %v: %v", auctionId, err)
				}
				if res == nil || res.Auction == nil {
					return fmt.Errorf("could not find any auction with id %v", auctionId)
				}
				if res.Auction.HighestBid != nil && res.Auction.HighestBid.BidAmount >= uint64(argAmount) {
					return fmt.Errorf("bid amount (%d) is lower than current highest bid %v", argAmount, res.Auction.HighestBid)
				}
			}

			msg := types.NewMsgBid(
				uint64(auctionId),
				clientCtx.GetFromAddress().String(),
				uint64(argAmount),
				bidFee,
			)

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
