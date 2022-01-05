package cmd

import (
	"encoding/json"
	"fmt"
	v100 "github.com/cosmos/ibc-go/v2/modules/core/legacy/v100"
	tmtypes "github.com/tendermint/tendermint/types"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	tmjson "github.com/tendermint/tendermint/libs/json"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/version"
	gentypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
)

// Currently we are seeing 6-8 second block times on Gravity, a safe maxExpectedBlockDelay is 3x - 5x the expected block time
// Here we go for 5 x 8 = 40 seconds
const gravityMaxExpectedBlockDelay = 40000000000
const flagGenesisTime = "genesis-time"

// MigrateGenesisCmd returns a command to execute genesis state migration.
// This is a copy of the genutil migrate cmd, with minimal changes to call the ibc v1->v2 genesis migration code
func MigrateGravityGenesisCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ibc-migrate [genesis-file]",
		Short: "Migrate gravity genesis to ibc v2",
		Long: fmt.Sprintf(`Migrate the source genesis for ibc v2 and print to STDOUT.

Example:
$ %s ibc-migrate /path/to/genesis.json --chain-id=gravity-bridge-2 --genesis-time=2019-04-22T17:00:00Z
`, version.AppName),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			var err error

			importGenesis := args[0]

			genDoc, err := validateGenDoc(importGenesis)
			if err != nil {
				return err
			}

			// Since some default values are valid values, we just print to
			// make sure the user didn't forget to update these values.
			if genDoc.ConsensusParams.Evidence.MaxBytes == 0 {
				fmt.Printf("Warning: consensus_params.evidence.max_bytes is set to 0. If this is" +
					" deliberate, feel free to ignore this warning. If not, please have a look at the chain" +
					" upgrade guide.\n")
			}

			var initialState gentypes.AppMap
			if err := json.Unmarshal(genDoc.AppState, &initialState); err != nil {
				return errors.Wrap(err, "failed to JSON unmarshal initial genesis state")
			}

			newGenState, err := v100.MigrateGenesis(initialState, clientCtx, *genDoc, gravityMaxExpectedBlockDelay)
			if err != nil {
				return errors.Wrap(err, "failed to migrate ibc genesis state from v1 to v2")
			}

			genDoc.AppState, err = json.Marshal(newGenState)
			if err != nil {
				return errors.Wrap(err, "failed to JSON marshal migrated genesis state")
			}

			genesisTime, _ := cmd.Flags().GetString(flagGenesisTime)
			if genesisTime != "" {
				var t time.Time

				err := t.UnmarshalText([]byte(genesisTime))
				if err != nil {
					return errors.Wrap(err, "failed to unmarshal genesis time")
				}

				genDoc.GenesisTime = t
			}

			chainID, _ := cmd.Flags().GetString(flags.FlagChainID)
			if chainID != "" {
				genDoc.ChainID = chainID
			}

			bz, err := tmjson.Marshal(genDoc)
			if err != nil {
				return errors.Wrap(err, "failed to marshal genesis doc")
			}

			sortedBz, err := sdk.SortJSON(bz)
			if err != nil {
				return errors.Wrap(err, "failed to sort JSON genesis doc")
			}

			cmd.Println(string(sortedBz))
			return nil
		},
	}

	cmd.Flags().String(flagGenesisTime, "", "override genesis_time with this flag")
	cmd.Flags().String(flags.FlagChainID, "", "override chain_id with this flag")

	return cmd
}

// validateGenDoc reads a genesis file and validates that it is a correct
// Tendermint GenesisDoc. This function does not do any cosmos-related
// validation.
func validateGenDoc(importGenesisFile string) (*tmtypes.GenesisDoc, error) {
	genDoc, err := tmtypes.GenesisDocFromFile(importGenesisFile)
	if err != nil {
		return nil, fmt.Errorf("%s. Make sure that"+
			" you have correctly migrated all Tendermint consensus params, please see the"+
			" chain migration guide for more info",
			err.Error(),
		)
	}

	return genDoc, nil
}
