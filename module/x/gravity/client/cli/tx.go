package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// GetTxCmd bundles all the subcmds together so they appear under `gravity tx`
func GetTxCmd(storeKey string) *cobra.Command {
	// needed for governance proposal txs in cli case
	// internal check prevents double registration in node case
	keeper.RegisterProposalTypes()

	// nolint: exhaustruct
	gravityTxCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Gravity transaction subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	gravityTxCmd.AddCommand([]*cobra.Command{
		CmdSendToEth(),
		CmdCancelSendToEth(),
		CmdRequestBatch(),
		CmdSetOrchestratorAddress(),
		CmdGovIbcMetadataProposal(),
		CmdGovAirdropProposal(),
		CmdGovUnhaltBridgeProposal(),
		CmdExecutePendingIbcAutoForwards(),
		CmdAddEvmChainProposal(),
		CmdRemoveEvmChainProposal(),
	}...)

	return gravityTxCmd
}

// CmdGovIbcMetadataProposal enables users to easily submit json file proposals for IBC Metadata registration, needed to
// send Cosmos tokens over to Ethereum
func CmdGovIbcMetadataProposal() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "gov-ibc-metadata [path-to-proposal-json] [initial-deposit]",
		Short: "Creates a governance proposal to set the Metadata of the given IBC token. Once the metadata is set this token can be moved to Ethereum using Gravity Bridge",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			cosmosAddr := cliCtx.GetFromAddress()

			initialDeposit, err := sdk.ParseCoinsNormalized(args[1])
			if err != nil {
				return sdkerrors.Wrap(err, "bad initial deposit amount")
			}

			if len(initialDeposit) != 1 {
				return fmt.Errorf("unexpected coin amounts, expecting just 1 coin amount for initialDeposit")
			}

			proposalFile := args[0]

			contents, err := os.ReadFile(proposalFile)
			if err != nil {
				return sdkerrors.Wrap(err, "failed to read proposal json file")
			}

			proposal := &types.IBCMetadataProposal{}
			err = json.Unmarshal(contents, proposal)
			if err != nil {
				return sdkerrors.Wrap(err, "proposal json file is not valid json")
			}
			if proposal.IbcDenom == "" ||
				proposal.Title == "" ||
				proposal.Description == "" ||
				proposal.EvmChainPrefix == "" ||
				proposal.Metadata.Base == "" ||
				proposal.Metadata.Name == "" ||
				proposal.Metadata.Display == "" ||
				proposal.Metadata.Symbol == "" {
				return fmt.Errorf("proposal json file is not valid, please check example json in docs")
			}

			// checks if the provided token denom is a proper IBC token, not a native token.
			if !strings.HasPrefix(proposal.IbcDenom, "ibc/") && !strings.HasPrefix(proposal.IbcDenom, "IBC/") {
				return sdkerrors.Wrap(types.ErrInvalid, "Target denom is not an IBC token")
			}

			// check that our base unit is the IBC token name on this chain. This makes setting/loading denom
			// metadata work out, as SetDenomMetadata uses the base denom as an index
			if proposal.Metadata.Base != proposal.IbcDenom {
				return sdkerrors.Wrap(types.ErrInvalid, "Metadata base must be the same as the IBC denom!")
			}

			metadataErr := proposal.Metadata.Validate()
			if metadataErr != nil {
				return sdkerrors.Wrap(metadataErr, "invalid metadata or proposal details!")
			}

			queryClientBank := banktypes.NewQueryClient(cliCtx)
			_, err = queryClientBank.DenomMetadata(cmd.Context(), &banktypes.QueryDenomMetadataRequest{Denom: proposal.IbcDenom})
			if err == nil {
				return sdkerrors.Wrap(metadataErr, "Attempting to set the metadata for a token that already has metadata!")
			}

			supply, err := queryClientBank.SupplyOf(cmd.Context(), &banktypes.QuerySupplyOfRequest{Denom: proposal.IbcDenom})
			if err != nil {
				return sdkerrors.Wrap(types.ErrInternal, "Failed to get supply data?")
			}
			if supply.GetAmount().Amount.Equal(sdk.ZeroInt()) {
				return sdkerrors.Wrap(types.ErrInvalid, "This ibc hash does not seem to exist on Gravity, are you sure you have the right one?")
			}

			proposalAny, err := codectypes.NewAnyWithValue(proposal)
			if err != nil {
				return sdkerrors.Wrap(err, "invalid metadata or proposal details!")
			}

			// Make the message
			msg := govtypes.MsgSubmitProposal{
				Proposer:       cosmosAddr.String(),
				InitialDeposit: initialDeposit,
				Content:        proposalAny,
			}
			if err := msg.ValidateBasic(); err != nil {
				return sdkerrors.Wrap(err, "Your proposal.json is not valid, please correct it")
			}
			// Send it
			return tx.GenerateOrBroadcastTxCLI(cliCtx, cmd.Flags(), &msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// AirdropProposalPlain is a struct with plaintext recipients so that the proposal.json can be readable
// and not subject to the strange encoding of the airdrop proposal tx where the recipients are packed as 20
// byte sets
type AirdropProposalPlain struct {
	Title       string
	Description string
	Denom       string
	Recipients  []string
	Amounts     []uint64
}

// CmdGovAirdropProposal enables users to easily submit json file proposals for token airdrops, eliminating the need for
// users to claim their airdrops / a custom on-chain module
func CmdGovAirdropProposal() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "gov-airdrop [path-to-proposal-json] [initial-deposit]",
		Short: "Creates a governance proposal for an airdrop",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			cosmosAddr := cliCtx.GetFromAddress()

			initialDeposit, err := sdk.ParseCoinsNormalized(args[1])
			if err != nil {
				return sdkerrors.Wrap(err, "bad initial deposit amount")
			}

			if len(initialDeposit) != 1 {
				return fmt.Errorf("unexpected coin amounts, expecting just 1 coin amount for initialDeposit")
			}

			proposalFile := args[0]

			contents, err := os.ReadFile(proposalFile)
			if err != nil {
				return sdkerrors.Wrap(err, "failed to read proposal json file")
			}

			proposal := &AirdropProposalPlain{}
			err = json.Unmarshal(contents, proposal)
			if err != nil {
				return sdkerrors.Wrap(err, "proposal json file is not valid json")
			}

			// convert the plaintext proposal to the actual type
			parsedRecipients := make([]sdk.AccAddress, len(proposal.Recipients))
			for i, v := range proposal.Recipients {
				parsed, err := sdk.AccAddressFromBech32(v)
				if err != nil {
					return sdkerrors.Wrap(err, "Address not valid!")
				}
				parsedRecipients[i] = parsed
			}
			byteEncodedRecipients := []byte{}
			for _, v := range parsedRecipients {
				byteEncodedRecipients = append(byteEncodedRecipients, v.Bytes()...)
			}

			finalProposal := &types.AirdropProposal{
				Title:       proposal.Title,
				Description: proposal.Description,
				Denom:       proposal.Denom,
				Amounts:     proposal.Amounts,
				Recipients:  byteEncodedRecipients,
			}

			proposalAny, err := codectypes.NewAnyWithValue(finalProposal)
			if err != nil {
				return sdkerrors.Wrap(err, "invalid metadata or proposal details!")
			}

			// Make the message
			msg := govtypes.MsgSubmitProposal{
				Proposer:       cosmosAddr.String(),
				InitialDeposit: initialDeposit,
				Content:        proposalAny,
			}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			// Send it
			return tx.GenerateOrBroadcastTxCLI(cliCtx, cmd.Flags(), &msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdGovUnhaltBridgeProposal enables users to easily submit json file proposals to set the Gravity module parameters
// which account for Ethereum forks, "rewinding" state and letting the chain achieve consensus after the fork is settled
func CmdGovUnhaltBridgeProposal() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "gov-unhalt-bridge [path-to-proposal-json] [initial-deposit]",
		Short: "Creates a governance proposal to unhalt the Ethereum bridge after an oracle dispute",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			cosmosAddr := cliCtx.GetFromAddress()

			initialDeposit, err := sdk.ParseCoinsNormalized(args[1])
			if err != nil {
				return sdkerrors.Wrap(err, "bad initial deposit amount")
			}

			if len(initialDeposit) != 1 {
				return fmt.Errorf("unexpected coin amounts, expecting just 1 coin amount for initialDeposit")
			}

			proposalFile := args[0]

			contents, err := os.ReadFile(proposalFile)
			if err != nil {
				return sdkerrors.Wrap(err, "failed to read proposal json file")
			}

			proposal := &types.UnhaltBridgeProposal{}
			err = json.Unmarshal(contents, proposal)
			if err != nil {
				return sdkerrors.Wrap(err, "proposal json file is not valid json")
			}

			proposalAny, err := codectypes.NewAnyWithValue(proposal)
			if err != nil {
				return sdkerrors.Wrap(err, "invalid metadata or proposal details!")
			}

			// Make the message
			msg := govtypes.MsgSubmitProposal{
				Proposer:       cosmosAddr.String(),
				InitialDeposit: initialDeposit,
				Content:        proposalAny,
			}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			// Send it
			return tx.GenerateOrBroadcastTxCLI(cliCtx, cmd.Flags(), &msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdAddEvmChainProposal enables users to create a proposal to add new EVM chains
func CmdAddEvmChainProposal() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "add-evm-chain [evm-chain-name] [evm-chain-prefix] [evm-chain-net-version] [evm-chain-gravity-id] [evm-chain-bridge-eth-address] [title] [initial-deposit] [description]",
		Short: "Creates a governance proposal to support a new EVM chain on the network",
		Args:  cobra.ExactArgs(8),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			cosmosAddr := cliCtx.GetFromAddress()

			initialDeposit, err := sdk.ParseCoinsNormalized(args[6])
			if err != nil {
				return sdkerrors.Wrap(err, "bad initial deposit amount")
			}

			if len(initialDeposit) != 1 {
				return fmt.Errorf("unexpected coin amounts, expecting just 1 coin amount for initialDeposit")
			}

			evmChainName := args[0]
			evmChainPrefix := args[1]
			evmChainNetVersion, err := strconv.ParseUint(string(args[2]), 10, 64)
			if err != nil {
				return fmt.Errorf("EVM chain net version should be an unsigned integer")
			}
			gravityId := args[3]
			bridgeEthAddress := args[4]

			proposal := &types.AddEvmChainProposal{EvmChainName: evmChainName, EvmChainPrefix: evmChainPrefix, EvmChainNetVersion: evmChainNetVersion, GravityId: gravityId, BridgeEthereumAddress: bridgeEthAddress, Title: args[5], Description: args[7]}
			proposalAny, err := codectypes.NewAnyWithValue(proposal)
			if err != nil {
				return sdkerrors.Wrap(err, "invalid metadata or proposal details!")
			}

			// Make the message
			msg := govtypes.MsgSubmitProposal{
				Proposer:       cosmosAddr.String(),
				InitialDeposit: initialDeposit,
				Content:        proposalAny,
			}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			// Send it
			return tx.GenerateOrBroadcastTxCLI(cliCtx, cmd.Flags(), &msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdRemoveEvmChainProposal enables users to create a proposal to add new EVM chains
func CmdRemoveEvmChainProposal() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-evm-chain [evm-chain-prefix] [initial-deposit] [title] [description]",
		Short: "Creates a governance proposal to remove an EVM chain on the network",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			cosmosAddr := cliCtx.GetFromAddress()

			initialDeposit, err := sdk.ParseCoinsNormalized(args[1])
			if err != nil {
				return sdkerrors.Wrap(err, "bad initial deposit amount")
			}

			if len(initialDeposit) != 1 {
				return fmt.Errorf("unexpected coin amounts, expecting just 1 coin amount for initialDeposit")
			}

			evmChainPrefix := args[0]

			proposal := &types.RemoveEvmChainProposal{EvmChainPrefix: evmChainPrefix, Title: args[2], Description: args[3]}
			proposalAny, err := codectypes.NewAnyWithValue(proposal)
			if err != nil {
				return sdkerrors.Wrap(err, "invalid metadata or proposal details!")
			}

			// Make the message
			msg := govtypes.MsgSubmitProposal{
				Proposer:       cosmosAddr.String(),
				InitialDeposit: initialDeposit,
				Content:        proposalAny,
			}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			// Send it
			return tx.GenerateOrBroadcastTxCLI(cliCtx, cmd.Flags(), &msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdSendToEth sends tokens to Ethereum. Locks Cosmos-side tokens into the Transaction pool for batching.
func CmdSendToEth() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "send-to-eth [eth-dest] [amount] [bridge-fee] [chain-fee] [evm chain prefix]",
		Short: "Adds a new entry to the transaction pool to withdraw an amount from the Ethereum bridge contract. This will not execute until a batch is requested and then actually relayed. Chain fee must be at least min_chain_fee_basis_points in `query gravity params`. Your funds can be reclaimed using cancel-send-to-eth so long as they remain in the pool",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			cosmosAddr := cliCtx.GetFromAddress()

			amount, err := sdk.ParseCoinsNormalized(args[1])
			if err != nil {
				return sdkerrors.Wrap(err, "amount")
			}
			bridgeFee, err := sdk.ParseCoinsNormalized(args[2])
			if err != nil {
				return sdkerrors.Wrap(err, "bridge fee")
			}
			chainFee, err := sdk.ParseCoinsNormalized(args[3])
			if err != nil {
				return sdkerrors.Wrap(err, "chain fee")
			}

			ethAddr, err := types.NewEthAddress(args[0])
			if err != nil {
				return sdkerrors.Wrap(err, "invalid eth address")
			}

			if len(amount) != 1 || len(bridgeFee) != 1 || len(chainFee) != 1 {
				return fmt.Errorf("unexpected coin amounts, expecting just 1 coin amount for both amount and bridgeFee")
			}

			// Make the message
			msg := types.MsgSendToEth{
				Sender:         cosmosAddr.String(),
				EthDest:        ethAddr.GetAddress().Hex(),
				Amount:         amount[0],
				BridgeFee:      bridgeFee[0],
				ChainFee:       chainFee[0],
				EvmChainPrefix: args[4],
			}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			// Send it
			return tx.GenerateOrBroadcastTxCLI(cliCtx, cmd.Flags(), &msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdCancelSendToEth enables users to take their Transaction out of the pool. Note that this cannot be done if it is
// locked up in a pending batch or if it has already been executed on Ethereum
func CmdCancelSendToEth() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "cancel-send-to-eth [transaction id] [emv chain prefix]",
		Short: "Removes an entry from the transaction pool, preventing your tokens from going to Ethereum and refunding the send.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			cosmosAddr := cliCtx.GetFromAddress()

			txId, err := strconv.ParseUint(args[0], 0, 64)
			if err != nil {
				return sdkerrors.Wrap(err, "failed to parse transaction id")
			}

			// Make the message
			msg := types.MsgCancelSendToEth{
				Sender:         cosmosAddr.String(),
				TransactionId:  txId,
				EvmChainPrefix: args[1],
			}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			// Send it
			return tx.GenerateOrBroadcastTxCLI(cliCtx, cmd.Flags(), &msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdRequestBatch requests that the validators create and confirm a batch to be sent to Ethereum. This
// is a manual command which duplicates the efforts of the Ethereum Relayer, likely not to be used often
func CmdRequestBatch() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "request-batch [token_contract_address]",
		Short: "Request a new batch on the cosmos side for pooled withdrawal transactions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			cosmosAddr := cliCtx.GetFromAddress()

			// TODO: better denom searching
			msg := types.MsgRequestBatch{
				Sender: cosmosAddr.String(),
				Denom:  fmt.Sprintf("gravity%s", args[0]),
			}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			// Send it
			return tx.GenerateOrBroadcastTxCLI(cliCtx, cmd.Flags(), &msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdSetOrchestratorAddress registers delegate keys for a validator so that their Orchestrator has authority to perform
// its responsibility
func CmdSetOrchestratorAddress() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "set-orchestrator-address [validator-address] [orchestrator-address] [ethereum-address]",
		Short: "Allows validators to delegate their voting responsibilities to a given key.",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := types.MsgSetOrchestratorAddress{
				Validator:    args[0],
				Orchestrator: args[1],
				EthAddress:   args[2],
			}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			// Send it
			return tx.GenerateOrBroadcastTxCLI(cliCtx, cmd.Flags(), &msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdExecutePendingIbcAutoForwards Executes a number of queued IBC Auto Forwards. When users perform a Send to Cosmos
// with a registered foreign address prefix (e.g. canto1... cre1...), their funds will be locked in the Gravity module
// until their pending forward is executed. This will send the funds to the equivalent gravity-prefixed account and then
// immediately create an IBC transfer to the destination chain to the original foreign account. If there is an IBC
// failure, the funds will be deposited on the gravity-prefixed account.
func CmdExecutePendingIbcAutoForwards() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "execute-pending-ibc-auto-forwards [forwards-to-execute] [evm chain prefix]",
		Short: "Executes a given number of IBC Auto-Forwards",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			sender := cliCtx.GetFromAddress()
			if sender.String() == "" {
				return fmt.Errorf("from address must be specified")
			}
			forwardsToClear, err := strconv.ParseUint(args[0], 10, 0)
			if err != nil {
				return sdkerrors.Wrap(err, "Unable to parse forwards-to-execute as an non-negative integer")
			}
			msg := types.MsgExecuteIbcAutoForwards{
				ForwardsToClear: forwardsToClear,
				Executor:        cliCtx.GetFromAddress().String(),
				EvmChainPrefix:  args[1],
			}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			// Send it
			return tx.GenerateOrBroadcastTxCLI(cliCtx, cmd.Flags(), &msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
