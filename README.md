# goety

[goety](https://www.merriam-webster.com/dictionary/goety) is a small cli to help with common actions when working with dynamodb.

Purge tables, dump data into a json file, seed tables from json file.

## Install

## Using golang

```bash
go install github.com/code-gorilla-au/goety@latest
```

## Usage

```bash

goety 

Power tools to interact with dynamodb tables

Usage:
  goety [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  dump        dump the contents of a dynamodb to a file
  help        Help about any command
  purge       purge a dynamodb table of all items
  seed        seed a dynamodb table from file

Flags:
  -r, --aws-region string   aws region the table is located (default "ap-southeast-2")
  -d, --dry-run             dry run does not perform actions, only logs them
  -h, --help                help for goety
  -v, --verbose             add verbose logging

Use "goety [command] --help" for more information about a command.

```

## Purge

```bash
goety purge -h

Usage:
  goety purge -t [TABLE_NAME] -p [PARTITION_KEY] -s [SORT_KEY] [flags]

Flags:
  -e, --endpoint string        DynamoDB endpoint to connect to, if none is provide it will use the default aws endpoint
  -h, --help                   help for purge
  -p, --partition-key string   The name of the partition key (default "pk")
  -s, --sort-key string        The name of the sort key (default "sk")
  -t, --table string           table name

Global Flags:
  -r, --aws-region string   aws region the table is located (default "ap-southeast-2")
  -d, --dry-run             dry run does not perform actions, only logs them
  -v, --verbose             add verbose logging
```

## Dump

```bash
dump will scan all items within a dynamodb table and write the contents to a file

Usage:
  goety dump -t [TABLE_NAME] -p [FILE_PATH] [flags]

Flags:
  -N, --attribute-name string    Filter expression attribute names
  -V, --attribute-value string   Filter expression attribute values
  -a, --attributes strings       Optionally specify a list of attributes to extract from the table
  -e, --endpoint string          DynamoDB endpoint to connect to, if none is provide it will use the default aws endpoint
  -f, --filter string            Filter expression to apply to the scan operation
  -h, --help                     help for dump
  -l, --limit int32              Limit the number of items returned per scan iteration
  -p, --path string              file path to save the json output
  -R, --raw-output               Optional flag to output the dynamodb scan without transformation
  -t, --table string             table name

Global Flags:
  -r, --aws-region string   aws region the table is located (default "ap-southeast-2")
  -d, --dry-run             dry run does not perform actions, only logs them
  -v, --verbose             add verbose logging
```

## Seed

```bash
seed will read a json file and write the contents to a dynamodb table

Usage:
  goety seed -t [TABLE_NAME] -f [FILE_PATH] [flags]

Flags:
  -e, --endpoint string   DynamoDB endpoint to connect to, if none is provide it will use the default aws endpoint
  -f, --file string       File path
  -h, --help              help for seed
  -t, --table string      Table name

Global Flags:
  -r, --aws-region string   aws region the table is located (default "ap-southeast-2")
  -d, --dry-run             dry run does not perform actions, only logs them
  -v, --verbose             add verbose logging

```

### Basic usage

getting started.

```bash

# short flags
goety purge -t <table-name> -p <partition-key> -s <sort-key>
# with long flags
goety purge --table <table-name> --partition-key <partition-key> --sort-key <sort-key>

```

### Dry run

The dry run flag does not perform purge; it logs what items will be deleted to standard out.

```bash
# short flags
goety purge -t <table-name> -p <partition-key> -s <sort-key> -d
# with long flags
goety purge --table <table-name> --partition-key <partition-key> --sort-key <sort-key> --dry-run
```

### Verbose

Add additional logs to the output.

```bash

# short flags
goety purge -t <table-name> -p <partition-key> -s <sort-key> -v
# with long flags
goety purge --table <table-name> --partition-key <partition-key> --sort-key <sort-key> --verbose

```

## Development

Developing against the CLI needs a local DynamoDB. `docker-compose.yaml` runs the `amazon/dynamodb-local` image on port 8000, and `cmd/local` is the e2e table setup harness: it creates the test table and seeds it with 1,000 items.

Prerequisites: Go, Docker (with `docker-compose`), and `jq` (used by the e2e assertions). A `.env.local` is loaded by the Makefile for the endpoint, table name, keys, and credentials — run `make setup` to copy `.env.example` if you don't have one.

Key targets:

```bash
make build        # build the goety binary
make db-seed      # start local DynamoDB and create/seed the table via cmd/local
make db-kill      # stop the local DynamoDB container
make test-dump      # integration: dump the seeded table
make test-seed      # integration: seed the table from a dump
make test-purge     # integration: purge the table
make e2e          # full end-to-end harness: dump, purge, and seed, 1000 items verified
make ci           # unit tests + lint + vuln scan (the local CI equivalent)
```
