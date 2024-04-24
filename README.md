# goety

[goety](https://www.merriam-webster.com/dictionary/goety) is a small cli to help with common actions when working with dynamodb.

## Install

## Using golang

```bash
go install https://github.com/code-gorilla-au/goety@latest
```

## Usage

```bash

goety 

dynamodb purge tool

Usage:
  goety [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  dump        dump the contents of a dynamodb to a file
  help        Help about any command
  purge       purge a dynamodb table of all items

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
  goety dump -t [TABLE_NAME] [flags]

Flags:
  -e, --endpoint string   DynamoDB endpoint to connect to, if none is provide it will use the default aws endpoint
  -h, --help              help for dump
  -p, --path string       file path to save the json output
  -t, --table string      table name

Global Flags:
  -r, --aws-region string   aws region the table is located (default "ap-southeast-2")
  -d, --dry-run             dry run does not perform actions, only logs them
  -v, --verbose             add verbose logging
```

### Basic usage

gets started

```bash

# short flags
goety purge -t <table-name> -p <partition-key> -s <sort-key>
# with long flags
goety purge --table <table-name> --partition-key <partition-key> --sort-key <sort-key>

```

### Dry run

The dry run flag does not perform purge, it logs what items will be deleted to standard out.

```bash
# short flags
goety purge -t <table-name> -p <partition-key> -s <sort-key> -d
# with long flags
goety purge --table <table-name> --partition-key <partition-key> --sort-key <sort-key> --dry-run
```

### Verbose

Add additional logs to the output

```bash

# short flags
goety purge -t <table-name> -p <partition-key> -s <sort-key> -v
# with long flags
goety purge --table <table-name> --partition-key <partition-key> --sort-key <sort-key> --verbose

```