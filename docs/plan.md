# Plan: Simplify the goety codebase

Use this list to track what has been completed. Mark items `[x]` only after the change is implemented and `make test && make lint` pass.

## Current state

`goety` is a small DynamoDB CLI (purge, dump, seed). The shared command bootstrap, stderr spinner, and CLI error handling are now in place. Further simplification remains in the DynamoDB scan adapter, output conversion, and service options; the checklist below tracks those later phases.

## Goal

Make the codebase easier to read, test, and maintain by removing unnecessary indirection and duplication while preserving the existing CLI behaviour and test coverage.

## Design principles

- **One owner per concern.** Commands parse flags; a thin runner wires dependencies; `goety.Service` implements behaviour; the DynamoDB client is a thin adapter.
- **CLI syntax stops at the command layer.** Comma-separated lists and `key=value` pairs exist only because cobra asks the user for input. The service receives typed values (`[]string`, `map[string]string`, `map[string]types.AttributeValue`). DynamoDB expression grammar — joining a projection with `", "`, building expression maps — stays in the service.
- **Explicit dependencies.** Pass `*slog.Logger` and clients as arguments. Do not look dependencies up through `context.Context`.
- **Return errors, do not log-and-exit.** Wrap with context (`fmt.Errorf("scan table: %w", err)`) and let the CLI boundary print once.
- **UI on stderr, data on stdout.** The spinner and progress messages must not corrupt `dump --dry-run` output.
- **Delete abstractions until a second implementation exists.** No interface, option function, or type alias that has exactly one caller and no test seam.
- **Use standard constructs.** Prefer `iter.Seq2` for pagination over custom iterator types, channels, or callback-plus-sentinel APIs.

## Review findings (verified against the current tree)

These correct assumptions made in the earlier draft and shape the checklist below.

- `types.AttributeValue` does **not** marshal to DynamoDB JSON. `json.Marshal` of `*types.AttributeValueMemberS{Value: "pk"}` produces `{"Value":"pk"}`, not `{"S":"pk"}`. Removing the `AV*` types therefore requires a small replacement converter, otherwise `--raw-output` changes shape and the existing test breaks.
- `dynamodb.Client.dryRun` is never set by `NewClient`; production dry-run never reaches the client. The branch is dead in production and only exercised by tests. The service already owns dry-run.
- `--limit` is now wired: `Dump` copies `QueryOpts.Limit` into `ScanInput.Limit`, and the Phase 4 item is done. The earlier finding that the flag had no effect is stale; Phase 5 must not drop the wiring while replacing the option type.
- `--attribute-name` / `--attribute-value` are parsed inside the service (`internal/goety/options.go`), and malformed entries are dropped with `continue`. That is the boundary violation Phase 5 fixes: a bad flag produces an empty map and only surfaces later as a DynamoDB `ValidationException`. Replacing the options with a struct does **not** fix it — the parse has to move to `internal/commands/cmd_dump.go`.
- Functional options are safe as implemented: `WithQueryOptions` allocates `&QueryOpts{}` locally, applies each option sequentially in one goroutine, and does not leak the value before it is complete. There is no aliasing hazard, and because all six options write distinct fields, order is irrelevant. Phase 5 deletes them because they have exactly one caller in the same internal package, not because the pattern is unsafe.
- The runner passes the configured logger explicitly to `dynamodb.NewClient`; the prior context-based logger lookup has been removed.
- `internal/dynamodb/types.go` also defines `Client`; the struct must move before that file is deleted.
- `Put`/`PutItem` are unused in production. They exist only in the interface, generated mocks, and the separate SDK client in `cmd/local`.
- `json.Encoder.Encode` appends a newline per value, so it cannot produce the compact `[{"..."},{"..."}]` output the current tests assert without extra handling.
- `NewWith` has been folded into `NewClient`, which now loads default configuration and applies region/endpoint options while receiving the logger explicitly.
- `cmd/local` is the local/e2e DynamoDB setup harness: it creates the table and seeds 1,000 items. `make db-seed` invokes it, and the `test-purge` / `test-seed` / `test-dump` integration targets depend on it. It is not a candidate for deletion.
- `cmd/spinner` was only a demo of the emitter/spinner pair and was deleted in Phase 1.
- Phase order in the earlier draft was backwards: the runner cannot wire a progress callback before the service and spinner expose that API.

## AWS SDK retries (context)

AWS SDK for Go v2 provides a default `Standard` retryer. It automatically retries throttling (`ProvisionedThroughputExceededException`), 5xx service errors, and transient network errors with backoff and jitter. **No third-party retry library is needed.**

The SDK does **not** retry `BatchWriteItem.UnprocessedItems` — that response is a partial success, not an error. The manual loop in `internal/dynamodb/dynamodb.go` is required and should be kept. Add a small backoff between retries so a throttled table is not hammered.

## Work checklist

### Phase 3: DynamoDB adapter and scan iteration

- [x] Move the `Client` struct from `internal/dynamodb/types.go` to `internal/dynamodb/dynamodb.go` (or `client.go`).
- [x] Remove the dead `ErrNoItems` check from `Dump`, then delete `ErrNoItems` from `internal/dynamodb/types.go`. Leave `AVer` / `AV*` in place until Phase 4 replaces them; removing them here would silently break `--raw-output` for an intermediate commit.
- [x] Remove `dryRun` from the DynamoDB client and delete the `batchWriteItem` dry-run branch and its two "should not make db call on dry run" tests. The service is the single owner of dry-run.
- [x] Remove `Put` from `goety.DynamoClient`, `Client.Put`, and `PutItem` from `ddbClient`; run `go generate ./...` to regenerate mocks.
- [x] Replace `ScanIterator` with a standard-library range iterator (the module targets Go 1.27):

  ```go
  func ScanAll(ctx context.Context, scanner Scanner, input *dynamodb.ScanInput) iter.Seq2[*dynamodb.ScanOutput, error]
  ```

  - [x] Clone the input (`in := *input`) inside the iterator so pagination does not mutate the caller's `ScanInput`.
  - [x] Yield `(nil, err)` once on a scan error and stop.
  - [x] Callers use `for out, err := range ddb.ScanAll(...)`; `return`/`break` stops pagination without a sentinel error.
  - [x] The current triple return `(*ScanOutput, error, bool)` is the real defect: `done` is easy to misread, `nil` means both "finished" and "no result", and the caller still owns the loop. Use `iter.Seq2`; do not add a custom iterator type, `Next() (T, error)` wrapper, channel, or callback-plus-sentinel.
  - [x] Add a test that covers multiple pages, a scan error, and early `break`.
- [x] Keep `BatchWriteItem.UnprocessedItems` looping, but add a small backoff before retrying.
- **Files affected:** `internal/dynamodb/*`, `internal/goety/goety.go`, `internal/goety/interfaces.go`, generated mocks

### Phase 4: Dump and seed output

- [ ] Delete `AVer` / `AV*` (and the now-empty `internal/dynamodb/types.go`) and replace `ConvertAVValue` / `ConvertAVValues` with one recursive converter, for example `toDynamoJSON(types.AttributeValue) any`, that emits the DynamoDB JSON shape (`{"S": ...}`, `{"N": ...}`, `{"M": {...}}`, `{"L": [...]}`, and so on). This is required because the AWS types do not do it themselves.
- [ ] Keep `FlattenAttrValue` / `FlattenAttrList` for normal output.
- [x] Optional: return `json.Number` for `N` instead of `float64`. This preserves large integers and makes flattening infallible, removing error plumbing. If taken, update the numeric tests.
  - Done: large integers are preserved and the numeric tests are updated. The `error` return of `FlattenAttrValue` is still there (nested `M`/`L` plumbing remains), so the "infallible" simplification is not taken.
- [ ] Replace the `first *bool` writer with a small array helper:

  ```go
  type jsonArrayWriter struct { w io.Writer; wrote bool }
  func (w *jsonArrayWriter) Write(v any) error
  func (w *jsonArrayWriter) Close() error
  ```

  - [ ] Write `[` when the file opens and `]` when it closes.
  - [ ] Use `json.Marshal` per item; do **not** use `json.Encoder` unless newline-delimited output is acceptable.
  - [x] Ensure the closing `]` is written on error as well, or delete the partial file. A truncated JSON file is worse than no file.
- [ ] Remove `transformDumpOutput`; raw dry-run can collect `map[string]any` directly and marshal once.
- [x] For `--dry-run`, accumulate items for `Dump` and `Seed`, then pretty-print once. Do not print one JSON document per seeded item.
- [ ] Make `prettyPrint` return an error instead of swallowing it, and write to an explicit `io.Writer`.
- [x] Wire `--limit` into `ScanInput.Limit` or remove the flag and its README entry. Recommendation: wire it; it is one line and the help text already promises it.
  - Done: `Dump` sets `ScanInput.Limit` from `QueryOpts.Limit`. `DumpOptions.Limit` must keep its pointer type so "unset" stays distinguishable from `0`.
- [ ] Tests: extend the existing exact-output assertions rather than adding golden files.
  - [ ] Multiple items and multiple scan pages.
  - [ ] Raw output with nested `M`/`L` values.
  - [ ] Empty result still produces `[]`.
  - [ ] Flattened and raw output are byte-identical to the pre-refactor output.
- **Files affected:** `internal/dynamodb/utils.go`, `internal/dynamodb/iterator.go`, `internal/goety/goety.go`, `internal/goety/goety_test.go`, `internal/dynamodb/*_test.go`

### Phase 5: Options and API surface

The option machinery is a surface-area and boundary problem, not a defect in functional options. Delete it because `Dump` has exactly one production caller (`dumpFunc`; `goety.New` is only constructed in `bootstrap.go`) that always passes all six options, and because the option closures implement CLI syntax inside the domain package. The struct swap and the parse move address different axes: the first removes the mechanism, the second moves the responsibility. Doing only the first leaves the CLI grammar advertised on an exported struct.

- [ ] Replace `internal/goety/options.go` functional options with a plain `DumpOptions` struct populated by the command. Remove `QueryFuncOpts`, `WithQueryOptions`, and the six `With*` functions.
- [ ] Move `--attribute-name` / `--attribute-value` parsing into `internal/commands/cmd_dump.go`, beside `parseDumpFlag`; the service receives `map[string]string` and `map[string]types.AttributeValue`, never comma-separated strings.
  - [ ] Use `strings.Cut(entry, "=")` instead of `strings.Split`, so filter values containing `=` (padded base64, for example) survive.
  - [ ] Return an error for a malformed entry (no `=`, or an empty key) instead of silently `continue`-ing. Today a bad `--attribute-name` yields an empty map and fails later as a DynamoDB `ValidationException`. This is a CLI behaviour change and needs a regression test that fails against the current code.
  - [ ] Pin the duplicate-key behaviour (last entry wins) with a test.
  - [ ] Return the parse error before `service.Dump`, so a flag typo fails without constructing a DynamoDB client. Follows the `cmd_purge.go` / `TableKeys` precedent: build a typed value from flags, then call the service.
- [ ] Keep projection formatting in the service: `strings.Join(attrs, ", ")` is DynamoDB expression grammar, not CLI grammar. `DumpOptions.ProjectedExpressions` stays `[]string`.
- [ ] Preserve the existing `--limit` wiring when converting to the struct (see Phase 4). Keep `Limit *int32`.
- [ ] Rename `internal/goety/types.go` to `service.go`, and rename `QueryOpts` to `DumpOptions`; the service performs a `Scan`, so the `Query` naming is wrong.
- [ ] Tests: update the `WithLimit(10)` / `WithAttrs(...)` / `WithRawOutput(true)` call sites in `goety_test.go` to struct literals, and add a table-driven test for the command-layer parse helper covering empty input, multiple pairs, whitespace, `=` inside a value, a malformed entry, and duplicate keys.
- **Files affected:** `internal/goety/options.go` (deleted), `internal/goety/types.go` (renamed), `internal/goety/goety.go`, `internal/commands/cmd_dump.go`, `internal/goety/goety_test.go`, new `internal/commands/*_test.go`

### Phase 6: Documentation and dev tooling

- [ ] Update `README.md` for the current CLI:
  - [ ] `dump` requires both `--table` and `--path`; the usage line currently omits `-p`.
  - [ ] Document `--raw-output`.
  - [ ] Document that progress goes to stderr and data goes to stdout.
  - [ ] Document `--limit` semantics (items evaluated per scan page), or remove it.
  - [ ] Add a "Development" section covering `make db-seed` and `cmd/local` as the e2e table setup harness.
- [ ] Confirm `make ci` still covers the changed packages.

## Alternative approach: surface-only cleanup

If you need to ship faster or want lower risk, do **Phase 1 and Phase 2 only**. That removes the emitter deadlock, the logging/context indirection, and the three copies of CLI bootstrap, while leaving the attribute-value conversion, iterator, and dump streaming as-is. Return to Phases 3–6 afterwards.

## Verification

After each phase:

```bash
go generate ./...   # only when interfaces or //go:generate directives change
make test
make lint
```

Run the race detector after Phase 1, because the spinner and progress callback share state:

```bash
go test -race ./...
```

Before starting Phase 4, capture current output from a real or local table so the serializer changes are provably byte-identical:

```bash
make build
./goety dump -e "$DYNAMODB_LOCAL_ENDPOINT" -t "$TEST_TABLE_NAME" -p /tmp/dump-flat.json
./goety dump -e "$DYNAMODB_LOCAL_ENDPOINT" -t "$TEST_TABLE_NAME" -p /tmp/dump-raw.json --raw-output
```

After Phase 3, run the integration targets:

```bash
make test-seed
make test-dump
make test-purge
```

After Phase 5, confirm filter flags fail locally instead of at the DynamoDB boundary, and that a well-formed filter still round-trips:

```bash
make build
# must error with a local parse message, not a DynamoDB ValidationException
./goety dump -e "$DYNAMODB_LOCAL_ENDPOINT" -t "$TEST_TABLE_NAME" -p /tmp/dump-filter.json \
  --filter "#pk = :pk" --attribute-name "#pk"
# must succeed and produce the same file as before the refactor
./goety dump -e "$DYNAMODB_LOCAL_ENDPOINT" -t "$TEST_TABLE_NAME" -p /tmp/dump-filter.json \
  --filter "#pk = :pk" --attribute-name "#pk=pk" --attribute-value ":pk=1"
```

## Notes and reminders

- Keep the runner thin. Command-specific flag parsing and option building belong in the commands. CLI syntax (`a=1,b=2`, comma lists) never crosses into `internal/goety`; DynamoDB expression syntax stays inside it.
- Keep the progress callback non-blocking. It should only store a string under a mutex.
- The service is the only dry-run owner. Do not propagate `dryRun` into the DynamoDB client.
- The AWS SDK default retryer is sufficient; only `UnprocessedItems` need the manual loop.
- Use the standard library `iter` package (`iter.Seq2`) for scan pagination; no custom iterator types, channels, or callback sentinels.
- Do not use `context.Context` as a service locator for the logger or clients.
- Out of scope: changing output formats, adding new commands, or reworking DynamoDB key handling beyond what is listed.
