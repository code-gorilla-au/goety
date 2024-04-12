package commands

import (
	"context"
	"errors"
	"os"

	"github.com/code-gorilla-au/goety/internal/dynamodb"
	"github.com/code-gorilla-au/goety/internal/goety"
	"github.com/code-gorilla-au/goety/internal/logging"
	"github.com/spf13/cobra"
)

var (
	flagDumpTableName    string
	flagDumpEndpoint     string
	flagDumpPartitionKey string
	flagDumpSortKey      string
)

var dumpCmd = &cobra.Command{
	Use:   "dump -t [TABLE_NAME] -p [PARTITION_KEY] -s [SORT_KEY]",
	Short: "dump the contents of a dynamodb to a file",
	Long:  "dump will scan all items within a dynamodb table and write the contents to a file",
	Run:   dumpFunc,
}

func init() {
	dumpCmd.Flags().StringVarP(&flagDumpTableName, "table", "t", "", "table name")
	dumpCmd.Flags().StringVarP(&flagDumpEndpoint, "endpoint", "e", "", "DynamoDB endpoint to connect to, if none is provide it will use the default aws endpoint")
	dumpCmd.Flags().StringVarP(&flagDumpPartitionKey, "partition-key", "p", "pk", "The name of the partition key")
	dumpCmd.Flags().StringVarP(&flagDumpSortKey, "sort-key", "s", "sk", "The name of the sort key")
}

func dumpFunc(cmd *cobra.Command, args []string) {
	log := logging.New(flagRootVerbose)
	ctx := context.Background()

	if err := parseDumpFlag(); err != nil {
		log.Error("error parsing flags", "error", err)
		os.Exit(1)
	}

	log.Debug("loading dynamodb client")
	dbClient, err := dynamodb.NewClient(ctx, flagRootAwsRegion, flagPurgeEndpoint)
	if err != nil {
		log.Error("could not load client")
		os.Exit(1)
	}

	g := goety.New(dbClient, log, flagRootDryRun)
	g.Dump(ctx, flagDumpTableName)

}

// parsePurgeFlag will validate the flags passed to the purge command
func parseDumpFlag() error {
	if flagDumpTableName == "" {
		return errors.New("table name is required")
	}
	if flagDumpPartitionKey == "" {
		return errors.New("partition key is required")
	}
	return nil
}
