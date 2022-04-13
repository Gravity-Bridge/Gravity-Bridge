package cli

import (
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

func GetQueryCmd() *cobra.Command {
	//nolint: exhaustivestruct
	gravityQueryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the gravity module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	gravityQueryCmd.AddCommand([]*cobra.Command{
		CmdGetCurrentValset(),
		CmdGetValsetRequest(),
		CmdGetValsetConfirm(),
		CmdGetPendingValsetRequest(),
		CmdGetPendingOutgoingTXBatchRequest(),
		CmdGetPendingSendToEth(),
		GetCmdPendingIbcAutoForwards(),
		GetCmdQueryParams(),
	}...)

	return gravityQueryCmd
}

func CmdGetCurrentValset() *cobra.Command {
	//nolint: exhaustivestruct
	cmd := &cobra.Command{
		Use:   "current-valset",
		Short: "Query current valset",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)

			req := &types.QueryCurrentValsetRequest{}

			res, err := queryClient.CurrentValset(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdGetValsetRequest() *cobra.Command {
	//nolint: exhaustivestruct
	cmd := &cobra.Command{
		Use:   "valset-request [nonce]",
		Short: "Get requested valset with a particular nonce",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)

			nonce, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}

			req := &types.QueryValsetRequestRequest{
				Nonce: nonce,
			}

			res, err := queryClient.ValsetRequest(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdGetValsetConfirm() *cobra.Command {
	//nolint: exhaustivestruct
	cmd := &cobra.Command{
		Use:   "valset-confirm [nonce] [bech32 validator address]",
		Short: "Get valset confirmation with a particular nonce from a particular validator",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)

			nonce, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}

			req := &types.QueryValsetConfirmRequest{
				Nonce:   nonce,
				Address: args[1],
			}

			res, err := queryClient.ValsetConfirm(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdGetPendingValsetRequest() *cobra.Command {
	//nolint: exhaustivestruct
	cmd := &cobra.Command{
		Use:   "pending-valset-request [bech32 orchestrator address]",
		Short: "Get the latest valset request which has not been signed by a particular orchestrator",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)

			req := &types.QueryLastPendingValsetRequestByAddrRequest{
				Address: args[0],
			}

			res, err := queryClient.LastPendingValsetRequestByAddr(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdGetPendingOutgoingTXBatchRequest() *cobra.Command {
	//nolint: exhaustivestruct
	cmd := &cobra.Command{
		Use:   "pending-batch-request [bech32 orchestrator address]",
		Short: "Get the latest outgoing TX batch request which has not been signed by a particular orchestrator",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)

			req := &types.QueryLastPendingBatchRequestByAddrRequest{
				Address: args[0],
			}

			res, err := queryClient.LastPendingBatchRequestByAddr(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdGetPendingSendToEth() *cobra.Command {
	//nolint: exhaustivestruct
	cmd := &cobra.Command{
		Use:   "pending-send-to-eth [address]",
		Short: "Query transactions waiting to go to Ethereum",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)

			req := &types.QueryPendingSendToEth{
				SenderAddress: args[0],
			}

			res, err := queryClient.GetPendingSendToEth(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func GetCmdPendingIbcAutoForwards() *cobra.Command {
	//nolint: exhaustivestruct
	cmd := &cobra.Command{
		Use:   "pending-ibc-auto-forwards [optional limit]",
		Short: "Query SendToCosmos transactions waiting to be forwarded over IBC",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)

			var limit uint64 = 0
			if args[0] != "" {
				var err error
				limit, err = strconv.ParseUint(args[0], 10, 0)
				if err != nil {
					return sdkerrors.Wrapf(err, "Unable to parse limit from %v", args[0])
				}
			}

			req := &types.QueryPendingIbcAutoForwards{Limit: limit}
			res, err := queryClient.GetPendingIbcAutoForwards(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func GetCmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Args:  cobra.NoArgs,
		Short: "Query gravity params",
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
