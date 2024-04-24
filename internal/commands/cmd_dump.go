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
	flagDumpTableName string
	flagDumpEndpoint  string
	flagDumpFilePath  string
)

var dumpCmd = &cobra.Command{
	Use:   "dump -t [TABLE_NAME]",
	Short: "dump the contents of a dynamodb to a file",
	Long:  "dump will scan all items within a dynamodb table and write the contents to a file",
	Run:   dumpFunc,
}

func init() {
	dumpCmd.Flags().StringVarP(&flagDumpTableName, "table", "t", "", "table name")
	dumpCmd.Flags().StringVarP(&flagDumpEndpoint, "endpoint", "e", "", "DynamoDB endpoint to connect to, if none is provide it will use the default aws endpoint")
	dumpCmd.Flags().StringVarP(&flagDumpFilePath, "path", "p", "", "file path to save the json output")
}

func dumpFunc(cmd *cobra.Command, args []string) {
	log := logging.New(flagRootVerbose)
	ctx := context.Background()

	if err := parseDumpFlag(); err != nil {
		log.Error("error parsing flags", "error", err)
		os.Exit(1)
	}

	log.Debug("loading dynamodb client")
	dbClient, err := dynamodb.NewClient(ctx, flagRootAwsRegion, flagDumpEndpoint)
	if err != nil {
		log.Error("could not load client")
		os.Exit(1)
	}

	msgEmitter := emitter.New()

	g := goety.New(dbClient, log, msgEmitter, flagRootDryRun)

	if !flagRootVerbose {
		spin := spinner.New(msgEmitter)
		spin.Start("starting dump")
		defer spin.Stop("dump complete")
	}
	_ = g.Dump(ctx, flagDumpTableName, flagDumpFilePath)

}

// parsePurgeFlag will validate the flags passed to the purge command
func parseDumpFlag() error {
	if flagDumpTableName == "" {
		return errors.New("table name is required")
	}
	if flagDumpFilePath == "" {
		return errors.New("file path is required")
	}
	return nil
}
