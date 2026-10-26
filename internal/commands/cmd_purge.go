package commands

import (
	"context"
	"errors"

	"github.com/code-gorilla-au/goety/internal/goety"
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
	RunE:  purgeFunc,
}

func init() {
	purgeCmd.Flags().StringVarP(&flagPurgeTableName, "table", "t", "", "table name")
	purgeCmd.Flags().StringVarP(&flagPurgeEndpoint, "endpoint", "e", "", "DynamoDB endpoint to connect to, if none is provide it will use the default aws endpoint")
	purgeCmd.Flags().StringVarP(&flagPurgePartitionKey, "partition-key", "p", "pk", "The name of the partition key")
	purgeCmd.Flags().StringVarP(&flagPurgeSortKey, "sort-key", "s", "sk", "The name of the sort key")
}

// purgeFunc is the entry point for the purge command. It will purge a dynamodb table of all items
func purgeFunc(cmd *cobra.Command, args []string) error {
	return bootstrap(cmd, flagPurgeEndpoint, func(ctx context.Context, service goety.Service) error {

		if err := parsePurgeFlag(); err != nil {
			return err
		}

		return service.Purge(ctx, flagPurgeTableName, goety.TableKeys{
			PartitionKey: flagPurgePartitionKey,
			SortKey:      flagPurgeSortKey,
		})
	})

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
