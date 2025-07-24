package cmd

import (
	"context"
	"fmt"
	"testing"

	"cosmossdk.io/log"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/app"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/server"
	genutiltest "github.com/cosmos/cosmos-sdk/x/genutil/client/testutil"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestAddGenesisAccountCmd(t *testing.T) {
	home := t.TempDir()
	logger := log.NewNopLogger()
	cfg, err := genutiltest.CreateDefaultCometConfig(home)
	require.NoError(t, err)
	encodingConfig := app.NewEncodingConfig()
	require.NoError(t, err)
	serverCtx := server.NewContext(viper.New(), cfg, logger)
	clientCtx := client.Context{}.WithCodec(encodingConfig.Codec).WithHomeDir(home)
	ctx := context.Background()
	ctx = context.WithValue(ctx, client.ClientContextKey, &clientCtx)
	ctx = context.WithValue(ctx, server.ServerContextKey, serverCtx)

	addCmd := keys.AddKeyCommand()
	addCmd.Flags().AddFlagSet(keys.Commands().PersistentFlags())
	addCmd.Flags().Set("keyring-backend", "test")
	addCmd.Flags().Set("home", "/home/cbo/go/src/gravity-bridge-dev/module/validator1")
	addCmd.SetArgs([]string{"validator1"})

	err = addCmd.ExecuteContext(ctx)
	require.NoError(t, err)

	showCmd := keys.ShowKeysCmd()
	showCmd.Flags().AddFlagSet(keys.Commands().PersistentFlags())
	showCmd.Flags().Set("keyring-backend", "test")
	showCmd.Flags().Set("address", "true")
	showCmd.Flags().Set("home", "/home/cbo/go/src/gravity-bridge-dev/module/validator1")
	showCmd.SetArgs([]string{"validator1"})
	fmt.Println("Ladies and gentleslugs, now for the show")
	err = showCmd.ExecuteContext(ctx)
	require.NoError(t, err)
}
