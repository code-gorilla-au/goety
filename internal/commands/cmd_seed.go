package commands

import (
	"context"
	"errors"

	"github.com/code-gorilla-au/goety/internal/goety"
	"github.com/spf13/cobra"
)

var (
	flagSeedTableName string
	flagSeedEndpoint  string
	flagSeedFile      string
)

var seedCmd = &cobra.Command{
	Use:   "seed -t [TABLE_NAME] -f [FILE_PATH]",
	Short: "seed a dynamodb table from file",
	Long:  "seed will read a json file and write the contents to a dynamodb table",
	RunE:  seedFunc,
}

func init() {
	seedCmd.Flags().StringVarP(&flagSeedTableName, "table", "t", "", "Table name")
	seedCmd.Flags().StringVarP(&flagSeedEndpoint, "endpoint", "e", "", "DynamoDB endpoint to connect to, if none is provide it will use the default aws endpoint")
	seedCmd.Flags().StringVarP(&flagSeedFile, "file", "f", "", "File path")
}

// purgeFunc is the entry point for the purge command. It will purge a dynamodb table of all items
func seedFunc(cmd *cobra.Command, args []string) error {
	return bootstrap(cmd, flagSeedEndpoint, func(ctx context.Context, service goety.Service) error {
		if err := parseSeedFlag(); err != nil {
			return err
		}

		if err := service.Seed(ctx, flagSeedTableName, flagSeedFile); err != nil {
			return err
		}

		return nil
	})
}

// parsePurgeFlag will validate the flags passed to the purge command
func parseSeedFlag() error {
	if flagSeedTableName == "" {
		return errors.New("table name is required")
	}
	return nil
}
