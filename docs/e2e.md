# e2e

End-to-end checks against the local DynamoDB instance, orchestrated by
`scripts/e2e.mk`.

## Run

```bash
make e2e
```

This runs, in order:

| Target | What it does |
| --- | --- |
| `e2e-clean` | Removes previous e2e artifacts |
| `db-seed` | Starts Docker and creates/seeds the table via `cmd/local` |
| `e2e-dump` | Dumps the table to `e2e-dump.json` and asserts 1000 items |
| `e2e-purge` | Purges the table, dumps to `e2e-purge.json`, asserts 0 items |
| `e2e-seed` | Seeds from `e2e-dump.json`, dumps to `e2e-seed.json`, asserts the item count and compares the sorted JSON to the original dump |

A successful run ends with:

```text
e2e complete: dump, purge and seed verified
```

## Requirements

- Go
- Docker and `docker-compose`
- `jq`
- `.env.local` (run `make setup` if you don't have one). The Makefile loads it,
  so the endpoint, table name, keys and AWS credentials come from there.

## Targets

Each step can also be run on its own while debugging:

```bash
make e2e-clean   # remove generated JSON
make db-seed     # start and seed the local table
make e2e-dump    # dump and count
make e2e-purge   # purge and verify empty
make e2e-seed    # seed from e2e-dump.json and verify the round trip
```

`e2e-seed` depends on `e2e-dump`, so it always has a dump file to read.

## Artifacts

Written to the repository root and ignored by git (`e2e*.json`):

- `e2e-dump.json` — dump of the seeded table
- `e2e-purge.json` — dump after purge, should be `[]`
- `e2e-seed.json` — dump after re-seeding
- `e2e-expected.json` / `e2e-actual.json` — sorted copies used for comparison

## Cleanup

```bash
make e2e-clean   # remove JSON artifacts
make db-kill     # stop the local DynamoDB container
```
