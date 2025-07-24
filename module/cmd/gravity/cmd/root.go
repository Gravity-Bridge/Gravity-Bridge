package cmd

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"

	"github.com/spf13/cast"
	"github.com/spf13/cobra"

	"cosmossdk.io/log"
	simappparams "cosmossdk.io/simapp/params"
	confixcmd "cosmossdk.io/tools/confix/cmd"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/debug"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/client/pruning"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/client/snapshot"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/server"
	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"

	cfg "github.com/cometbft/cometbft/config"
	tmcli "github.com/cometbft/cometbft/libs/cli"

	ethermint "github.com/evmos/ethermint/crypto/hd"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/app"
	_ "github.com/Gravity-Bridge/Gravity-Bridge/module/config" // import config to register the Bech32 codec
)

// InvCheckPeriodPrimes A collection of all primes in (15, 200), for use with the crisis module's Invariant Check Period
// feature but enabling fast block times by randomly selecting one of these primes with added security by halting the chain when something goes wrong.
// These primes were selected for several reasons: a) Gravity has a validator limit of 200 thus too many collisions in this set are unlikely (fast blocks),
// b) in the worst case the chain will halt ~ 20 minutes after invariant failure with more likely halt at ~13 minutes after,
// c) a recent improvement to the sdk brings faster invariant checks
var InvCheckPeriodPrimes = []uint{17, 19, 23, 29, 31, 37, 41, 43, 47, 53, 59, 61, 67, 71, 73, 79, 83, 89, 97, 101, 103, 107, 109, 113, 127, 131, 137, 139, 149, 151, 157, 163, 167, 173, 179, 181, 191, 193, 197, 199}

// NewRootCmd creates a new root command for simd. It is called once in the
// main function.
func NewRootCmd() (*cobra.Command, simappparams.EncodingConfig) {
	encodingConfig := app.NewEncodingConfig()
	// nolint: exhaustruct
	initClientCtx := client.Context{}.
		WithCodec(encodingConfig.Codec).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithInput(os.Stdin).
		WithAccountRetriever(types.AccountRetriever{}).
		WithBroadcastMode(flags.BroadcastSync).
		WithHomeDir(app.DefaultNodeHome).
		WithKeyringOptions(ethermint.EthSecp256k1Option()).
		WithViper("gravity")

	// nolint: exhaustruct
	rootCmd := &cobra.Command{
		Use:   "gravity",
		Short: "Stargate Gravity App",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// set the default command outputs
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())

			initClientCtx, err := client.ReadPersistentCommandFlags(initClientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			initClientCtx, err = config.ReadFromClientConfig(initClientCtx)
			if err != nil {
				return err
			}

			if err := client.SetCmdClientContextHandler(initClientCtx, cmd); err != nil {
				return err
			}

			gravityAppTemplate, gravityAppConfig := initAppConfig()
			gravityTMConfig := initTendermintConfig()

			return server.InterceptConfigsPreRunHandler(cmd, gravityAppTemplate, gravityAppConfig, gravityTMConfig)
		},
	}

	initRootCmd(rootCmd, encodingConfig, initClientCtx)

	return rootCmd, encodingConfig
}

// initTendermintConfig helps to override default Tendermint Config values.
// return tmcfg.DefaultConfig if no custom configuration is required for the application.
func initTendermintConfig() *cfg.Config {
	cfg := cfg.DefaultConfig()

	// these values put a higher strain on node memory
	// cfg.P2P.MaxNumInboundPeers = 100
	// cfg.P2P.MaxNumOutboundPeers = 40

	return cfg
}

// initAppConfig defines the default configuration for a gravity instance. These defaults can be overridden via an
// app.toml file or with flags provided on the command line
func initAppConfig() (string, interface{}) {
	type GravityAppConfig struct {
		serverconfig.Config
	}

	// DEFAULT SERVER CONFIGURATIONS
	srvConfig := serverconfig.DefaultConfig()

	// CUSTOM APP CONFIG - add members to this struct to add gravity-specific configuration options
	// NOTE: Make sure config options are explained with their default values in gravityAppTemplate
	gravityAppConfig := GravityAppConfig{
		Config: *srvConfig,
	}

	// CUSTOM CONFIG TEMPLATE - add to this string when adding gravity-specific configurations have been added to
	// GravityAppConfig above, an example can be seen at https://github.com/cosmos/cosmos-sdk/blob/master/simapp/simd/cmd/root.go
	gravityAppTemplate := serverconfig.DefaultConfigTemplate

	return gravityAppTemplate, gravityAppConfig
}

// Execute executes the root command.
func Execute(rootCmd *cobra.Command) error {
	// Create and set a client.Context on the command's Context. During the pre-run
	// of the root command, a default initialized client.Context is provided to
	// seed child command execution with values such as AccountRetriver, Keyring,
	// and a Tendermint RPC. This requires the use of a pointer reference when
	// getting and setting the client.Context. Ideally, we utilize
	// https://github.com/spf13/cobra/pull/1118.
	srvCtx := server.NewDefaultContext()
	ctx := context.Background()
	// nolint: exhaustruct
	ctx = context.WithValue(ctx, client.ClientContextKey, &client.Context{})
	ctx = context.WithValue(ctx, server.ServerContextKey, srvCtx)

	rootCmd.PersistentFlags().String("log_level", "info", "The logging level in the format of <module>:<level>,...")

	executor := tmcli.PrepareBaseCmd(rootCmd, "", app.DefaultNodeHome)
	return executor.ExecuteContext(ctx)
}

func initRootCmd(
	rootCmd *cobra.Command,
	encodingConfig simappparams.EncodingConfig,
	initClientCtx client.Context,
) {
	var tempApp = app.TemporaryApp()
	rootCmd.AddCommand(
		InitCmd(*tempApp.ModuleBasicManager, app.DefaultNodeHome),
		CollectGenTxsCmd(banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome, GravityMessageValidator, encodingConfig.TxConfig.SigningContext().ValidatorAddressCodec()),
		genutilcli.MigrateGenesisCmd(genutilcli.MigrationMap),
		GenTxCmd(*tempApp.ModuleBasicManager, encodingConfig.TxConfig, banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome),
		genutilcli.ValidateGenesisCmd(*tempApp.ModuleBasicManager),
		AddGenesisAccountCmd(app.DefaultNodeHome),
		tmcli.NewCompletionCmd(rootCmd, true),
		testnetCmd(*tempApp.ModuleBasicManager, banktypes.GenesisBalancesIterator{}),
		debug.Cmd(),
		MigrateGravityGenesisCmd(),
		confixcmd.ConfigCommand(),
		pruning.Cmd(newApp, app.DefaultNodeHome),
		snapshot.Cmd(newApp),
	)

	server.AddCommands(rootCmd, app.DefaultNodeHome, newApp, createSimappAndExport, addModuleInitFlags)

	// add keybase, auxiliary RPC, query, and tx child commands
	rootCmd.AddCommand(
		server.StatusCommand(),
		queryCommand(),
		txCommand(),
		keys.Commands(),
		Commands(app.DefaultNodeHome),
	)

	autoCliOpts := tempApp.AutoCliOpts()
	initClientCtx, err := config.ReadDefaultValuesFromDefaultClientConfig(initClientCtx)
	if err != nil {
		panic(err)
	}
	autoCliOpts.Keyring, err = keyring.NewAutoCLIKeyring(initClientCtx.Keyring)
	if err != nil {
		panic(err)
	}
	autoCliOpts.ClientCtx = initClientCtx
	if err := autoCliOpts.EnhanceRootCommand(rootCmd); err != nil {
		panic(err)
	}

}

func addModuleInitFlags(startCmd *cobra.Command) {
	crisis.AddModuleInitFlags(startCmd)
}

func queryCommand() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		rpc.ValidatorCommand(),
		authcmd.QueryTxsByEventsCmd(),
		authcmd.QueryTxCmd(),
		rpc.QueryEventForTxCmd(),
		server.QueryBlocksCmd(),
		server.QueryBlockResultsCmd(),
	)

	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

	return cmd
}

func txCommand() *cobra.Command {
	// nolint: exhaustruct
	cmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		authcmd.GetSignCommand(),
		authcmd.GetSignBatchCommand(),
		authcmd.GetMultiSignCommand(),
		authcmd.GetMultiSignBatchCmd(),
		authcmd.GetValidateSignaturesCommand(),
		flags.LineBreak,
		authcmd.GetBroadcastCommand(),
		authcmd.GetEncodeCommand(),
		authcmd.GetDecodeCommand(),

		flags.LineBreak,
	)

	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

	return cmd
}

func newApp(logger log.Logger, db dbm.DB, traceStore io.Writer, appOpts servertypes.AppOptions) servertypes.Application {
	baseAppOptions := server.DefaultBaseappOptions(appOpts)

	skipUpgradeHeights := make(map[int64]bool)
	for _, h := range cast.ToIntSlice(appOpts.Get(server.FlagUnsafeSkipUpgrades)) {
		skipUpgradeHeights[int64(h)] = true
	}

	invCheckPer := cast.ToUint(appOpts.Get(server.FlagInvCheckPeriod))
	if invCheckPer == uint(0) {
		logger.Info("--inv-check-period NOT PROVIDED, A RANDOM PRIME PERIOD WILL BE GENERATED")
		invCheckPer = generatePrimeInvCheckPeriod()
		logger.Info(fmt.Sprintf("This node will check invariants every %d blocks", invCheckPer))
	}

	return app.NewGravityApp(
		logger, db, traceStore, true, skipUpgradeHeights,
		cast.ToString(appOpts.Get(flags.FlagHome)),
		invCheckPer,
		appOpts,
		baseAppOptions...,
	)
}

// generatePrimeInvCheckPeriod generates a random index into the InvCheckPeriodPrimes for use in newApp
func generatePrimeInvCheckPeriod() uint {
	idx := rand.Intn(len(InvCheckPeriodPrimes))
	return InvCheckPeriodPrimes[idx]
}

func createSimappAndExport(
	logger log.Logger, db dbm.DB, traceStore io.Writer, height int64, forZeroHeight bool, jailAllowedAddrs []string,
	appOpts servertypes.AppOptions, modulesToExport []string) (servertypes.ExportedApp, error) {

	var gravity *app.Gravity
	if height != -1 {
		gravity = app.NewGravityApp(logger, db, traceStore, false, map[int64]bool{}, "", uint(1), appOpts)

		if err := gravity.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	} else {
		gravity = app.NewGravityApp(logger, db, traceStore, true, map[int64]bool{}, "", uint(1), appOpts)
	}

	return gravity.ExportAppStateAndValidators(forZeroHeight, jailAllowedAddrs, modulesToExport)
}
