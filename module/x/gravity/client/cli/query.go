package cli

import (
	"encoding/hex"
	"strconv"

	errorsmod "cosmossdk.io/errors"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	typesv2 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types/v2"
)

const (
	FlagOrder     = "order"
	FlagClaimType = "claim-type"
	FlagNonce     = "nonce"
	FlagEthHeight = "eth-height"
	FlagUseV1Key  = "use-v1-key"
)

// GetQueryCmd bundles all the query subcmds together so they appear under `gravity query` or `gravity q`
func GetQueryCmd() *cobra.Command {
	// nolint: exhaustruct
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
		CmdGetValsetConfirmsByNonce(),
		CmdGetLastValsetRequests(),
		CmdGetLastPendingValsetRequestByAddr(),
		CmdGetBatchFees(),
		CmdGetLastPendingBatchRequestByAddr(),
		CmdGetLastPendingLogicCallByAddr(),
		CmdGetOutgoingTxBatches(),
		CmdGetOutgoingLogicCalls(),
		CmdGetBatchRequestByNonce(),
		CmdGetBatchConfirms(),
		CmdGetLogicConfirms(),
		CmdGetLastEventNonceByAddr(),
		CmdDenomToERC20(),
		CmdERC20ToDenom(),
		CmdGetLastObservedEthBlock(),
		CmdGetLastObservedEthNonce(),
		CmdGetAttestations(),
		CmdGetDelegateKeysByValidator(),
		CmdGetDelegateKeysByOrchestrator(),
		CmdGetDelegateKeysByEthAddr(),
		CmdGetPendingSendToEth(),
		CmdGetPendingIbcAutoForwards(),
		CmdGetQueryParams(),
		// V2 queries:
		CmdGetPendingSendToEthV2(),
		CmdGetOutgoingTxBatchesByAddr(),
	}...)

	return gravityQueryCmd
}

// CmdGetCurrentValset fetches the current validator set
func CmdGetCurrentValset() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "current-valset",
		Short: "Query current valset",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
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

// CmdGetValsetRequest fetches a historical valset with the given valset nonce, for use in Ethereum relaying
func CmdGetValsetRequest() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "valset-request [nonce]",
		Short: "Get requested valset with a particular nonce",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
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

// CmdGetValsetConfirm fetches a confirm for the valset with the given nonce made by the given validator
func CmdGetValsetConfirm() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "valset-confirm [nonce] [bech32 validator address]",
		Short: "Get valset confirmation with a particular nonce from a particular validator",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
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

// CmdGetValsetConfirmsByNonce fetches all valset confirms for the valset with the given nonce
func CmdGetValsetConfirmsByNonce() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "valset-confirms-by-nonce [nonce]",
		Short: "Get valset confirms with a particular nonce",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			nonce, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}

			req := &types.QueryValsetConfirmsByNonceRequest{
				Nonce: nonce,
			}

			res, err := queryClient.ValsetConfirmsByNonce(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdGetLastValsetRequests fetches the most recent valset requests
func CmdGetLastValsetRequests() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "last-valset-requests",
		Short: "Get the most recent valset requests",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			req := &types.QueryLastValsetRequestsRequest{}

			res, err := queryClient.LastValsetRequests(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdGetLastPendingValsetRequestByAddr fetches the valset to be confirmed next by the given validator, if any exists
func CmdGetLastPendingValsetRequestByAddr() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "pending-valset-request [bech32 orchestrator address]",
		Short: "Get the latest valset request which has not been signed by a particular orchestrator",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
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

func CmdGetBatchFees() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "batch-fees",
		Short: "Get the potential fees for new batches",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			req := &types.QueryBatchFeeRequest{}

			res, err := queryClient.BatchFees(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdGetLastPendingBatchRequestByAddr fetches the batch to be confirmed next by the given validator, if any exists
func CmdGetLastPendingBatchRequestByAddr() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "pending-batch-request [bech32 orchestrator address]",
		Short: "Get the latest outgoing TX batch request which has not been signed by a particular orchestrator",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
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

// CmdGetLastPendingLogicCallByAddr fetches the logic call to be confirmed next by the given validator, if any exists
func CmdGetLastPendingLogicCallByAddr() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "pending-logic-call [bech32 orchestrator address]",
		Short: "Get the latest outgoing logic call which has not been signed by a particular orchestrator",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			req := &types.QueryLastPendingLogicCallByAddrRequest{
				Address: args[0],
			}

			res, err := queryClient.LastPendingLogicCallByAddr(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdGetOutgoingTxBatches() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "outgoing-tx-batches",
		Short: "Get the outgoing TX batches",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			req := &types.QueryOutgoingTxBatchesRequest{}

			res, err := queryClient.OutgoingTxBatches(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdGetOutgoingLogicCalls() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "outgoing-logic-call",
		Short: "Get the outgoing logic calls",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			req := &types.QueryOutgoingLogicCallsRequest{}

			res, err := queryClient.OutgoingLogicCalls(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdGetBatchRequestByNonce() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "batch-request-by-nonce [nonce] [contract address]",
		Short: "Get batch requests with a particular nonce",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			nonce, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}

			req := &types.QueryBatchRequestByNonceRequest{
				Nonce:           nonce,
				ContractAddress: args[1],
			}

			res, err := queryClient.BatchRequestByNonce(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdGetBatchConfirms() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "batch-confirms [nonce] [contract address]",
		Short: "Get batch confirms with a particular nonce and contract address",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			nonce, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}

			req := &types.QueryBatchConfirmsRequest{
				Nonce:           nonce,
				ContractAddress: args[1],
			}

			res, err := queryClient.BatchConfirms(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdGetLogicConfirms() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "logic-confirms [hex invalidation id] [invalidation nonce]",
		Short: "Get logic call confirms with a particular invalidation id (hexadecimal) and invalidation nonce",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			invalidationId, err := hex.DecodeString(args[0])
			if err != nil {
				return err
			}

			nonce, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}

			req := &types.QueryLogicConfirmsRequest{
				InvalidationId:    invalidationId,
				InvalidationNonce: nonce,
			}

			res, err := queryClient.LogicConfirms(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdGetLastEventNonceByAddr() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "last-event-nonce-by-addr [bech32 orchestrator address]",
		Short: "Get the latest event nonce handled by a particular orchestrator",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			req := &types.QueryLastEventNonceByAddrRequest{
				Address: args[0],
			}

			res, err := queryClient.LastEventNonceByAddr(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdDenomToERC20 fetches the ERC20 contract address for a given Cosmos denom
func CmdDenomToERC20() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "denom-to-erc20 [denom]",
		Short: "Query the ERC20 contract address for a given Cosmos denom",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			req := &types.QueryDenomToERC20Request{
				Denom: args[0],
			}

			res, err := queryClient.DenomToERC20(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdERC20ToDenom fetches the Cosmos denom for a given ERC20 contract address
func CmdERC20ToDenom() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "erc20-to-denom [erc20]",
		Short: "Query the cosmos denom for a given ERC20 contract address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			req := &types.QueryERC20ToDenomRequest{
				Erc20: args[0],
			}

			res, err := queryClient.ERC20ToDenom(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdGetLastObservedEthBlock fetches the Ethereum block height for the most recent "observed" Attestation, indicating
// the state of Cosmos consensus on the submitted Ethereum events
// nolint: dupl
func CmdGetLastObservedEthBlock() *cobra.Command {
	short := "Query the last observed Ethereum block height"
	long := short + "\n\n" +
		"This value is expected to lag the actual Ethereum block height significantly due to 1. Ethereum Finality and 2. Consensus mirroring the state on Ethereum" + "\n" +
		"Note that when querying with --height less than 1282013 '--use-v1-key' must be provided to locate the value"

	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "last-observed-eth-block",
		Short: short,
		Long:  long,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			useV1Key, err := cmd.Flags().GetBool(FlagUseV1Key)
			if err != nil {
				return err
			}

			req := &types.QueryLastObservedEthBlockRequest{
				UseV1Key: useV1Key,
			}
			res, err := queryClient.GetLastObservedEthBlock(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	cmd.Flags().Bool(FlagUseV1Key, false, "if querying with --height less than 1282013 this flag must be provided to locate the Last Observed Ethereum Height")
	return cmd
}

// CmdGetLastObservedEthNonce fetches the Ethereum event nonce for the most recent "observed" Attestation, indicating
// // the state of Cosmos consensus on the submitted Ethereum events
// nolint: dupl
func CmdGetLastObservedEthNonce() *cobra.Command {
	short := "Query the last observed Ethereum event nonce"
	long := short + "\n\n" +
		"This this is likely to lag the last executed event a little due to 1. Ethereum Finality and 2. Consensus mirroring the Ethereum state" + "\n" +
		"Note that when querying with --height less than 1282013 '--use-v1-key' must be provided to locate the value"

	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "last-observed-eth-nonce",
		Short: short,
		Long:  long,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			useV1Key, err := cmd.Flags().GetBool(FlagUseV1Key)
			if err != nil {
				return err
			}

			req := &types.QueryLastObservedEthNonceRequest{
				UseV1Key: useV1Key,
			}
			res, err := queryClient.GetLastObservedEthNonce(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	cmd.Flags().Bool(FlagUseV1Key, false, "if querying with --height less than 1282013 this must be set to true to locate the Last Observed Ethereum Event Nonce")
	return cmd
}

// CmdGetAttestations fetches the most recently created Attestations in the store (only the most recent 1000 are available)
// up to an optional limit
func CmdGetAttestations() *cobra.Command {
	short := "Query gravity current and historical attestations (only the most recent 1000 are stored)"
	long := short + "\n\n" + "Optionally provide a limit to reduce the number of attestations returned" + "\n" +
		"Note that when querying with --height less than 1282013 '--use-v1-key' must be provided to locate the attestations"

	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "attestations [optional limit]",
		Args:  cobra.MaximumNArgs(1),
		Short: short,
		Long:  long,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			var limit uint64
			// Limit is 0 or whatever the user put in
			if len(args) == 0 || args[0] == "" {
				limit = 0
			} else {
				limit, err = strconv.ParseUint(args[0], 10, 64)
				if err != nil {
					return err
				}
			}
			orderBy, err := cmd.Flags().GetString(FlagOrder)
			if err != nil {
				return err
			}
			claimType, err := cmd.Flags().GetString(FlagClaimType)
			if err != nil {
				return err
			}
			nonce, err := cmd.Flags().GetUint64(FlagNonce)
			if err != nil {
				return err
			}
			height, err := cmd.Flags().GetUint64(FlagEthHeight)
			if err != nil {
				return err
			}
			useV1Key, err := cmd.Flags().GetBool(FlagUseV1Key)
			if err != nil {
				return err
			}

			req := &types.QueryAttestationsRequest{
				Limit:     limit,
				OrderBy:   orderBy,
				ClaimType: claimType,
				Nonce:     nonce,
				Height:    height,
				UseV1Key:  useV1Key,
			}
			res, err := queryClient.GetAttestations(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	// Global flags
	flags.AddQueryFlagsToCmd(cmd)
	// Local flags
	cmd.Flags().String(FlagOrder, "asc", "order attestations by eth block height: set to 'desc' for reverse ordering")
	cmd.Flags().String(FlagClaimType, "", "which types of claims to filter, empty for all or one of: CLAIM_TYPE_SEND_TO_COSMOS, CLAIM_TYPE_BATCH_SEND_TO_ETH, CLAIM_TYPE_ERC20_DEPLOYED, CLAIM_TYPE_LOGIC_CALL_EXECUTED, CLAIM_TYPE_VALSET_UPDATED")
	cmd.Flags().Uint64(FlagNonce, 0, "the exact nonce to find, 0 for any")
	cmd.Flags().Uint64(FlagEthHeight, 0, "the exact ethereum block height an event happened at, 0 for any")
	cmd.Flags().Bool(FlagUseV1Key, false, "if querying with --height less than 1282013 this flag must be provided to locate the attestations")

	return cmd
}

func CmdGetDelegateKeysByValidator() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "delegate-keys-by-val-addr [address]",
		Short: "Query the delegate keys for a validator by the validator address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			req := &types.QueryDelegateKeysByValidatorAddress{
				ValidatorAddress: args[0],
			}

			res, err := queryClient.GetDelegateKeyByValidator(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdGetDelegateKeysByOrchestrator() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "delegate-keys-by-orch-addr [address]",
		Short: "Query the delegate keys for a validator by the orchestrator address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			req := &types.QueryDelegateKeysByOrchestratorAddress{
				OrchestratorAddress: args[0],
			}

			res, err := queryClient.GetDelegateKeyByOrchestrator(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdGetDelegateKeysByEthAddr() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "delegate-keys-by-eth-addr [address]",
		Short: "Query the delegate keys for a validator by the eth address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			req := &types.QueryDelegateKeysByEthAddress{
				EthAddress: args[0],
			}

			res, err := queryClient.GetDelegateKeyByEth(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdGetPendingSendToEth fetches all pending Sends to Ethereum made by the given address
func CmdGetPendingSendToEth() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "pending-send-to-eth [address]",
		Short: "Query transactions waiting to go to Ethereum",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
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

// CmdGetPendingIbcAutoForwards fetches the next IBC auto forwards to be executed, up to an optional limit
func CmdGetPendingIbcAutoForwards() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "pending-ibc-auto-forwards [optional limit]",
		Short: "Query SendToCosmos transactions waiting to be forwarded over IBC",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			var limit uint64 = 0
			if args[0] != "" {
				var err error
				limit, err = strconv.ParseUint(args[0], 10, 0)
				if err != nil {
					return errorsmod.Wrapf(err, "Unable to parse limit from %v", args[0])
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

// CmdGetQueryParams fetches the current Gravity module params
func CmdGetQueryParams() *cobra.Command {
	// nolint: exhaustruct
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

////////////////////////////// V2 QUERIES /////////////////////////////////////////////////////////////////////////////////////////////////////////

// CmdGetPendingSendToEthV2 fetches all pending Sends to Ethereum (optionally made by the given address) in an updated way
// There are actually 2 GRPC queries this will hit depending on if the address is provided: GetPendingSendToEthV2 and GetPendingSendToEthV2BySender
func CmdGetPendingSendToEthV2() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "pending-send-to-eth-v2 [optional address]",
		Short: "Query transactions waiting to go to Ethereum",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := typesv2.NewQueryClient(clientCtx)

			if len(args) == 0 {
				req := &typesv2.QueryPendingSendToEthV2{}

				res, err := queryClient.GetPendingSendToEthV2(cmd.Context(), req)
				if err != nil {
					return err
				}

				return clientCtx.PrintProto(res)

			} else {
				req := &typesv2.QueryPendingSendToEthV2BySender{
					Sender: args[0],
				}

				res, err := queryClient.GetPendingSendToEthV2BySender(cmd.Context(), req)
				if err != nil {
					return err
				}

				return clientCtx.PrintProto(res)
			}
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdGetOutgoingTxBatchesByAddr fetches all
func CmdGetOutgoingTxBatchesByAddr() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "outgoing-tx-batches-by-addr [address]",
		Short: "Query outgoing TX batches by their token contract address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := typesv2.NewQueryClient(clientCtx)

			req := &typesv2.QueryOutgoingTxBatchesByAddrRequest{
				Address: args[0],
			}

			res, err := queryClient.GetOutgoingTxBatchesByAddr(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
