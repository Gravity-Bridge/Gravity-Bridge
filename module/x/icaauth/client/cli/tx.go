package cli

import (
	"fmt"
	"os"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pkg/errors"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/icaauth/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// GetTxCmd creates and returns the icaauth tx command
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		getRegisterAccountCmd(),
		getSubmitTxCmd(),
	)

	return cmd
}

// getRegisterAccountCmd contains the CLI command for creating a new interchain account on a foreign chain
func getRegisterAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "register",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := types.NewMsgRegisterAccount(
				clientCtx.GetFromAddress().String(),
				viper.GetString(FlagConnectionID),
				viper.GetString(FlagCounterpartyConnectionID),
			)

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().AddFlagSet(fsConnectionPair)
	_ = cmd.MarkFlagRequired(FlagConnectionID)
	_ = cmd.MarkFlagRequired(FlagCounterpartyConnectionID)

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// getSubmitTxCmd contains the CLI command for executing one or multiple json-formatted Msgs as an interchain account
// on a foreign chain
func getSubmitTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "submit-tx [JSON msgs or path/to/sdk_msgs.json]",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			cdc := codec.NewProtoCodec(clientCtx.InterfaceRegistry)

			var txMsgs []sdk.Msg

			// Attempt single msg on command line
			var txMsg sdk.Msg
			if err := cdc.UnmarshalInterfaceJSON([]byte(args[0]), &txMsg); err == nil {
				txMsgs = []sdk.Msg{txMsg}
			} else {
				// Attempt multiple msgs on command line
				if err := cdc.UnmarshalInterfaceJSON([]byte(args[0]), &txMsgs); err != nil {
					// Finally, attempt to read a file
					contents, err := os.ReadFile(args[0])
					if err != nil {
						return errors.Wrap(err, "neither JSON input nor path to .json file for sdk msg were provided")
					}

					// The file contains a single Msg
					if err := cdc.UnmarshalInterfaceJSON(contents, &txMsg); err == nil {
						txMsgs = []sdk.Msg{txMsg}
					} else if err := cdc.UnmarshalInterfaceJSON(contents, &txMsgs); err != nil { // Try multiple Msgs
						return errors.Wrap(err, "error unmarshalling sdk msgs file - make sure you have provided an array of messages")
					}
				}
			}

			// Format the discovered Msgs for ICA submission
			msg, err := types.NewMsgSubmitTx(clientCtx.GetFromAddress(), txMsgs, viper.GetString(FlagConnectionID))
			if err != nil {
				return err
			}

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().AddFlagSet(fsConnectionPair)

	_ = cmd.MarkFlagRequired(FlagConnectionID)
	_ = cmd.MarkFlagRequired(FlagCounterpartyConnectionID)

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
