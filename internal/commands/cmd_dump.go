package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/code-gorilla-au/goety/internal/goety"
	"github.com/spf13/cobra"
)

var (
	flagDumpTableName       string
	flagDumpEndpoint        string
	flagDumpFilePath        string
	flagDumpExtractAttrs    []string
	flagDumpLimit           int32
	flagDumpFilterExp       string
	flagDumpFilterAttrName  string
	flagDumpFilterAttrValue string
	flagDumpRawOutput       bool
)

var dumpCmd = &cobra.Command{
	Use:   "dump -t [TABLE_NAME] -p [FILE_PATH]",
	Short: "dump the contents of a dynamodb to a file",
	Long:  "dump will scan all items within a dynamodb table and write the contents to a file",
	RunE:  dumpFunc,
}

func init() {
	dumpCmd.Flags().StringVarP(&flagDumpTableName, "table", "t", "", "table name")
	dumpCmd.Flags().StringVarP(&flagDumpEndpoint, "endpoint", "e", "", "DynamoDB endpoint to connect to, if none is provide it will use the default aws endpoint")
	dumpCmd.Flags().StringVarP(&flagDumpFilePath, "path", "p", "", "file path to save the json output")
	dumpCmd.Flags().StringSliceVarP(&flagDumpExtractAttrs, "attributes", "a", []string{}, "Optionally specify a list of attributes to extract from the table")
	dumpCmd.Flags().Int32VarP(&flagDumpLimit, "limit", "l", 0, "Limit the number of items returned per scan iteration")
	dumpCmd.Flags().StringVarP(&flagDumpFilterExp, "filter", "f", "", "Filter expression to apply to the scan operation")
	dumpCmd.Flags().StringVarP(&flagDumpFilterAttrName, "attribute-name", "N", "", "Filter expression attribute names")
	dumpCmd.Flags().StringVarP(&flagDumpFilterAttrValue, "attribute-value", "V", "", "Filter expression attribute values")
	dumpCmd.Flags().BoolVarP(&flagDumpRawOutput, "raw-output", "R", false, "Optional flag to output the dynamodb scan without transformation")
}

func dumpFunc(cmd *cobra.Command, args []string) error {
	if err := parseDumpFlag(); err != nil {
		return err
	}

	filterNames, err := parseAttributeNames(flagDumpFilterAttrName)
	if err != nil {
		return err
	}

	filterValues, err := parseAttributeValues(flagDumpFilterAttrValue)
	if err != nil {
		return err
	}

	return bootstrap(cmd, flagDumpEndpoint, func(ctx context.Context, service goety.Service) error {
		return service.Dump(
			ctx,
			flagDumpTableName,
			flagDumpFilePath,
			goety.WithAttrs(flagDumpExtractAttrs),
			goety.WithLimit(flagDumpLimit),
			goety.WithFilterExpression(flagDumpFilterExp),
			goety.WithFilterNameAttrs(filterNames),
			goety.WithFilterNameValues(filterValues),
			goety.WithRawOutput(flagDumpRawOutput),
		)
	})
}

func parseAttributeNames(input string) (map[string]string, error) {
	if input == "" {
		return nil, nil
	}

	names := make(map[string]string)

	for _, entry := range strings.Split(input, ",") {
		rawKey, value, found := strings.Cut(entry, "=")
		if !found {
			return nil, fmt.Errorf("invalid attribute name %q: expected key=value", entry)
		}

		key := strings.TrimSpace(rawKey)
		if key == "" {
			return nil, fmt.Errorf("invalid attribute name %q: key is empty", entry)
		}

		names[key] = strings.TrimSpace(value)
	}

	return names, nil
}

func parseAttributeValues(input string) (map[string]types.AttributeValue, error) {
	if input == "" {
		return nil, nil
	}

	values := make(map[string]types.AttributeValue)

	for _, entry := range strings.Split(input, ",") {
		rawKey, value, found := strings.Cut(entry, "=")
		if !found {
			return nil, fmt.Errorf("invalid attribute value %q: expected key=value", entry)
		}

		key := strings.TrimSpace(rawKey)
		if key == "" {
			return nil, fmt.Errorf("invalid attribute value %q: key is empty", entry)
		}

		values[key] = &types.AttributeValueMemberS{Value: strings.TrimSpace(value)}
	}

	return values, nil
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
