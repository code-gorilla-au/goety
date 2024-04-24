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
	flagPurgeTableName    string
	flagPurgeEndpoint     string
	flagPurgePartitionKey string
	flagPurgeSortKey      string
)

var purgeCmd = &cobra.Command{
	Use:   "purge -t [TABLE_NAME] -p [PARTITION_KEY] -s [SORT_KEY]",
	Short: "purge a dynamodb table of all items",
	Long:  "purge will scan all items within a dynamodb table and use a batch delete to remove all records",
	Run:   purgeFunc,
}

func init() {
	purgeCmd.Flags().StringVarP(&flagPurgeTableName, "table", "t", "", "table name")
	purgeCmd.Flags().StringVarP(&flagPurgeEndpoint, "endpoint", "e", "", "DynamoDB endpoint to connect to, if none is provide it will use the default aws endpoint")
	purgeCmd.Flags().StringVarP(&flagPurgePartitionKey, "partition-key", "p", "pk", "The name of the partition key")
	purgeCmd.Flags().StringVarP(&flagPurgeSortKey, "sort-key", "s", "sk", "The name of the sort key")
}

// purgeFunc is the entry point for the purge command. It will purge a dynamodb table of all items
func purgeFunc(cmd *cobra.Command, args []string) {
	log := logging.New(flagRootVerbose)
	ctx := context.Background()

	if err := parsePurgeFlag(); err != nil {
		log.Error("error parsing flags", "error", err)
		os.Exit(1)
	}

	log.Debug("loading dynamodb client")
	dbClient, err := dynamodb.NewClient(ctx, flagRootAwsRegion, flagPurgeEndpoint)
	if err != nil {
		log.Error("could not load client")
		os.Exit(1)
	}

	msgEmitter := emitter.New()

	goetyService := goety.New(dbClient, log, msgEmitter, flagRootDryRun)

	if !flagRootVerbose {
		spin := spinner.New(msgEmitter)
		spin.Start("starting purge")
		defer spin.Stop("")
	}

	if err = goetyService.Purge(ctx, flagPurgeTableName, goety.TableKeys{
		PartitionKey: flagPurgePartitionKey,
		SortKey:      flagPurgeSortKey,
	}); err != nil {
		log.Error("error purging table", "error", err)
		os.Exit(1)
	}

}

// parsePurgeFlag will validate the flags passed to the purge command
func parsePurgeFlag() error {
	if flagPurgeTableName == "" {
		return errors.New("table name is required")
	}
	if flagPurgePartitionKey == "" {
		return errors.New("partition key is required")
	}
	return nil
}
