package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/code-gorilla-au/env"
	"github.com/code-gorilla-au/goety/internal/logging"
)

func main() {

	logger := logging.New(false)
	ctx := context.Background()
	config := loadConfig()
	itemsToSeed := 1000

	logger.Info("loading dynamodb client", "config", config)

	db, err := dbClient(config)
	if err != nil {
		logger.Error("could not load client")
		os.Exit(1)
	}

	logger.Info("creating table", "table", config.TableName)
	if err := createTable(ctx, db, config); err != nil {
		if !strings.Contains(err.Error(), "ResourceInUseException") {
			logger.Error("could not create table", "error", err)
			os.Exit(1)
			return
		}
		logger.Info("table already exists")
	}

	logger.Info("seeding table", "table", config.TableName, "count", itemsToSeed)
	now := time.Now()

	_ = seedTable(db, config, 1000)

	since := time.Since(now).Seconds()
	logger.Info("seed complete", "duration", since)

}

type Config struct {
	TablePrimaryKey string
	TableSortKey    string
	TableName       string
	Endpoint        string
}

func loadConfig() Config {
	env.LoadEnvFile(".env.local")
	return Config{
		TablePrimaryKey: env.GetAsString("TEST_PRIMARY_KEY"),
		TableSortKey:    env.GetAsString("TEST_SORT_KEY"),
		TableName:       env.GetAsString("TEST_TABLE_NAME"),
		Endpoint:        env.GetAsString("DYNAMODB_LOCAL_ENDPOINT"),
	}
}

func dbClient(c Config) (*dynamodb.Client, error) {

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}

	db := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = &c.Endpoint
		o.Region = "ap-southeast-2"
	})

	return db, nil
}

func createTable(ctx context.Context, db *dynamodb.Client, config Config) error {
	_, err := db.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: &config.TableName,
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: &config.TablePrimaryKey,
				KeyType:       types.KeyTypeHash,
			},
			{
				AttributeName: &config.TableSortKey,
				KeyType:       types.KeyTypeRange,
			},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("pk"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("sk"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	return err
}

func seedTable(db *dynamodb.Client, config Config, items int) error {
	var allErrs error
	for i := 0; i < items; i++ {
		_, err := db.PutItem(context.Background(), &dynamodb.PutItemInput{
			TableName: &config.TableName,
			Item: map[string]types.AttributeValue{
				config.TablePrimaryKey: &types.AttributeValueMemberS{Value: fmt.Sprintf("pk#%d", i)},
				config.TableSortKey:    &types.AttributeValueMemberS{Value: fmt.Sprintf("sk#%d", i)},
			},
		})
		if err != nil {
			allErrs = errors.Join(allErrs, err)
		}
	}
	return allErrs
}
