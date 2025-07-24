package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math/unsafe"
	gravitytypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	cfg "github.com/cometbft/cometbft/config"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/input"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/go-bip39"
	"github.com/spf13/cobra"
)

const (
	// FlagOverwrite defines a flag to overwrite an existing genesis JSON file.
	FlagOverwrite = "overwrite"

	// FlagSeed defines a flag to initialize the private validator key from a specific seed.
	FlagRecover = "recover"

	// FlagDefaultBondDenom defines the default denom to use in the genesis file.
	FlagDefaultBondDenom = "default-denom"
)

// InitCmd returns a command that initializes all files needed for Tendermint
// and the respective application.
// Note that this is a fork of the upstream sdk init command, with the change that we use
// a custom genesis message validator so that we can support our custom validator init procedures
func InitCmd(mbm module.BasicManager, defaultNodeHome string) *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:   "init [moniker]",
		Short: "Initialize private validator, p2p, genesis, and application configuration files",
		Long:  `Initialize validators's and node's configuration files.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			cdc := clientCtx.Codec

			serverCtx := server.GetServerContextFromCmd(cmd)
			config := serverCtx.Config
			config.SetRoot(clientCtx.HomeDir)

			chainID, err := cmd.Flags().GetString(flags.FlagChainID)
			if err != nil {
				return err
			}
			switch {
			case chainID != "":
			case clientCtx.ChainID != "":
				chainID = clientCtx.ChainID
			default:
				chainID = fmt.Sprintf("test-chain-%v", unsafe.Str(6))
			}

			// Get bip39 mnemonic
			var mnemonic string
			recover, err := cmd.Flags().GetBool(FlagRecover)
			if err != nil {
				return err
			}
			if recover {
				inBuf := bufio.NewReader(cmd.InOrStdin())
				value, err := input.GetString("Enter your bip39 mnemonic", inBuf)
				if err != nil {
					return err
				}

				mnemonic = value
				if !bip39.IsMnemonicValid(mnemonic) {
					return errors.New("invalid mnemonic")
				}
			}

			// Get initial height
			initHeight, err := cmd.Flags().GetInt64(flags.FlagInitHeight)
			if err != nil {
				return err
			}
			if initHeight < 1 {
				initHeight = 1
			}

			nodeID, _, err := genutil.InitializeNodeValidatorFilesFromMnemonic(config, mnemonic)
			if err != nil {
				return err
			}

			config.Moniker = args[0]

			genFile := config.GenesisFile()
			overwrite, err := cmd.Flags().GetBool(FlagOverwrite)
			if err != nil {
				return err
			}
			defaultDenom, err := cmd.Flags().GetString(FlagDefaultBondDenom)
			if err != nil {
				return err
			}

			// use os.Stat to check if the file exists
			_, err = os.Stat(genFile)
			if !overwrite && !os.IsNotExist(err) {
				return fmt.Errorf("genesis.json file already exists: %v", genFile)
			}

			// Overwrites the SDK default denom for side-effects
			if defaultDenom != "" {
				sdk.DefaultBondDenom = defaultDenom
			}
			appGenState := mbm.DefaultGenesis(cdc)

			appState, err := json.MarshalIndent(appGenState, "", " ")
			if err != nil {
				return errorsmod.Wrap(err, "Failed to marshal default genesis state")
			}

			// nolint: exhaustruct
			appGenesis := &genutiltypes.AppGenesis{}
			if _, err := os.Stat(genFile); err != nil {
				if !os.IsNotExist(err) {
					return err
				}
			} else {
				appGenesis, err = genutiltypes.AppGenesisFromFile(genFile)
				if err != nil {
					return errorsmod.Wrap(err, "Failed to read genesis doc from file")
				}
			}

			appGenesis.AppName = version.AppName
			appGenesis.AppVersion = version.Version
			appGenesis.ChainID = chainID
			appGenesis.AppState = appState
			appGenesis.InitialHeight = initHeight
			// nolint: exhaustruct
			appGenesis.Consensus = &genutiltypes.ConsensusGenesis{
				Validators: nil,
			}

			bytes, err := json.MarshalIndent(appGenesis, "", "  ")
			if err != nil {
				return errorsmod.Wrap(err, "Failed to marshal genesis doc")
			}
			println("Genesis doc:", string(bytes))

			if err = genutil.ExportGenesisFile(appGenesis, genFile); err != nil {
				return errorsmod.Wrap(err, "Failed to export genesis file")
			}

			toPrint := newPrintInfo(config.Moniker, chainID, nodeID, "", appState)

			cfg.WriteConfigFile(filepath.Join(config.RootDir, "config", "config.toml"), config)
			return displayInfo(toPrint)
		},
	}

	cmd.Flags().String(flags.FlagHome, defaultNodeHome, "node's home directory")
	cmd.Flags().BoolP(FlagOverwrite, "o", false, "overwrite the genesis.json file")
	cmd.Flags().Bool(FlagRecover, false, "provide seed phrase to recover existing key instead of creating")
	cmd.Flags().String(flags.FlagChainID, "", "genesis file chain-id, if left blank will be randomly created")
	cmd.Flags().String(FlagDefaultBondDenom, "", "genesis file default denomination, if left blank default value is 'stake'")
	cmd.Flags().Int64(flags.FlagInitHeight, 1, "specify the initial block height at genesis")

	return cmd
}

func GravityMessageValidator(msgs []sdk.Msg) error {
	if _, ok := msgs[0].(*stakingtypes.MsgCreateValidator); !ok {
		return fmt.Errorf("unexpected GenTx message type; expected: MsgCreateValidator, got: %T", msgs[0])
	}
	if _, ok := msgs[1].(*gravitytypes.MsgSetOrchestratorAddress); !ok {
		return fmt.Errorf("unexpected GenTx message type; expected: MsgSetOrchestratorAddress, got: %T", msgs[1])
	}

	for _, msg := range msgs {
		if m, ok := msg.(sdk.HasValidateBasic); ok {
			if err := m.ValidateBasic(); err != nil {
				return fmt.Errorf("invalid GenTx '%s': %w", msgs[0], err)
			}
		}
	}

	return nil
}
