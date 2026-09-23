package commands

import (
	"context"
	"errors"
	"os"

	"github.com/code-gorilla-au/goety/internal/dynamodb"
	"github.com/code-gorilla-au/goety/internal/emitter"
	"github.com/code-gorilla-au/goety/internal/goety"
	"github.com/code-gorilla-au/goety/internal/logging"
	"github.com/code-gorilla-au/goety/internal/spinner"
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
	Run:   seedFunc,
}

func init() {
	seedCmd.Flags().StringVarP(&flagSeedTableName, "table", "t", "", "Table name")
	seedCmd.Flags().StringVarP(&flagSeedEndpoint, "endpoint", "e", "", "DynamoDB endpoint to connect to, if none is provide it will use the default aws endpoint")
	seedCmd.Flags().StringVarP(&flagSeedFile, "file", "f", "", "File path")
}

// purgeFunc is the entry point for the purge command. It will purge a dynamodb table of all items
func seedFunc(cmd *cobra.Command, args []string) {
	log := logging.New(flagRootVerbose)
	ctx := context.Background()

	if err := parseSeedFlag(); err != nil {
		log.Error("error parsing flags", "error", err)
		os.Exit(1)
	}

	log.Debug("loading dynamodb client")
	dbClient, err := dynamodb.NewClient(ctx, flagRootAwsRegion, flagSeedEndpoint)
	if err != nil {
		log.Error("could not load client")
		os.Exit(1)
	}

	msgEmitter := emitter.New()

	goetyService := goety.New(dbClient, log, msgEmitter, flagRootDryRun)

	if !flagRootVerbose {
		spin := spinner.New(msgEmitter)
		spin.Start("")
		defer spin.Stop("")
	}

	file, err := os.Open(flagSeedFile)
	if err != nil {
		log.Error("error opening file", "error", err)
		os.Exit(1)
	}
	defer file.Close()

	if err = goetyService.Seed(ctx, flagSeedTableName, file); err != nil {
		log.Error("error seeding table", "error", err)
		os.Exit(1)
	}

}

// parsePurgeFlag will validate the flags passed to the purge command
func parseSeedFlag() error {
	if flagSeedTableName == "" {
		return errors.New("table name is required")
	}
	return nil
}
