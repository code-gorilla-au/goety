# Plan: Simplify the goety codebase

Use this list to track what has been completed. Mark items `[x]` only after the change is implemented and `make test && make lint` pass.

## Current state

`goety` is a small DynamoDB CLI (purge, dump, seed). The business logic fits in a few files, but the code is more complex than its surface area requires. In particular, the same bootstrap (parse flags → build logger → build client → build service → start spinner → run) is copy-pasted in three commands, and several concerns have more than one owner: dry-run is handled by both the service and the DynamoDB client, errors are logged and returned and then logged again, and progress messages pass through an emitter, a channel, and a spinner.

## Goal

Make the codebase easier to read, test, and maintain by removing unnecessary indirection and duplication while preserving the existing CLI behaviour and test coverage.

## Design principles

- **One owner per concern.** Commands parse flags; a thin runner wires dependencies; `goety.Service` implements behaviour; the DynamoDB client is a thin adapter.
- **Explicit dependencies.** Pass `*slog.Logger` and clients as arguments. Do not look dependencies up through `context.Context`.
- **Return errors, do not log-and-exit.** Wrap with context (`fmt.Errorf("scan table: %w", err)`) and let the CLI boundary print once.
- **UI on stderr, data on stdout.** The spinner and progress messages must not corrupt `dump --dry-run` output.
- **Delete abstractions until a second implementation exists.** No interface, option function, or type alias that has exactly one caller and no test seam.
- **Use standard constructs.** Prefer `iter.Seq2` for pagination over custom iterator types, channels, or callback-plus-sentinel APIs.

## Review findings (verified against the current tree)

These correct assumptions made in the earlier draft and shape the checklist below.

- `types.AttributeValue` does **not** marshal to DynamoDB JSON. `json.Marshal` of `*types.AttributeValueMemberS{Value: "pk"}` produces `{"Value":"pk"}`, not `{"S":"pk"}`. Removing the `AV*` types therefore requires a small replacement converter, otherwise `--raw-output` changes shape and the existing test breaks.
- `dynamodb.Client.dryRun` is never set by `NewClient`/`NewWith`; production dry-run never reaches the client. The branch is dead in production and only exercised by tests. The service already owns dry-run.
- `--limit` is parsed and stored in `QueryOpts.Limit`, but `Dump` never copies it into `ScanInput`. The flag currently has no effect.
- Commands create a logger but never attach it to the context, so `dynamodb.NewClient` always logs through `slog.Default()` instead of the configured logger.
- `internal/dynamodb/types.go` also defines `Client`; the struct must move before that file is deleted.
- `Put`/`PutItem` are unused in production. They exist only in the interface, generated mocks, and the separate SDK client in `cmd/local`.
- `json.Encoder.Encode` appends a newline per value, so it cannot produce the compact `[{"..."},{"..."}]` output the current tests assert without extra handling.
- `NewWith` is only called by `NewClient`.
- `cmd/local` is the local/e2e DynamoDB setup harness: it creates the table and seeds 1,000 items. `make db-seed` invokes it, and the `test-purge` / `test-seed` / `test-dump` integration targets depend on it. It is not a candidate for deletion.
- `cmd/spinner` is only a demo of the emitter/spinner pair.
- Phase order in the earlier draft was backwards: the runner cannot wire a progress callback before the service and spinner expose that API.

## AWS SDK retries (context)

AWS SDK for Go v2 provides a default `Standard` retryer. It automatically retries throttling (`ProvisionedThroughputExceededException`), 5xx service errors, and transient network errors with backoff and jitter. **No third-party retry library is needed.**

The SDK does **not** retry `BatchWriteItem.UnprocessedItems` — that response is a partial success, not an error. The manual loop in `internal/dynamodb/dynamodb.go` is required and should be kept. Add a small backoff between retries so a throttled table is not hammered.

## Work checklist

### Phase 1: Progress reporting and spinner

This must land first because the runner depends on it.

- [ ] Delete `internal/emitter` (package, interfaces, tests).
- [ ] Remove the unused `Emitter` interface from `internal/goety/interfaces.go`.
- [ ] Give `goety.Service` a `progress func(string)` field instead of `emitter.MessagePublisher`.
  - [ ] Add it to `New`; treat `nil` as a no-op so tests do not need a mock.
  - [ ] Contract: `progress` only records the latest message. It must not block, read files, or write to the terminal on the caller's goroutine.
- [ ] Rewrite `internal/spinner`:
  - [ ] `New(w io.Writer)` so the spinner writes to stderr.
  - [ ] Run a `time.Ticker` loop; read the latest message under a `sync.Mutex` each frame.
  - [ ] `Update(msg)` stores the message under the mutex. No channel and no wake signal are needed; a ticker already redraws.
  - [ ] `Start` launches the loop, `Stop` signals a `done` channel and clears the line.
  - [ ] Expose an unexported frame-duration knob so tests do not sleep for real.
- [ ] Delete `cmd/spinner`, which exists only to demo the emitter/spinner pair.
- [ ] Replace every `s.emitter.Publish(...)` in `internal/goety/goety.go` with `s.progress(...)`.
- **Files affected:** `internal/emitter/*`, `internal/spinner/*`, `internal/goety/*`, `cmd/spinner/*`

### Phase 2: CLI runner and error handling

- [ ] Create `internal/commands/runner.go` with one helper, for example:

  ```go
  func run(cmd *cobra.Command, endpoint string, action func(context.Context, goety.Service) error) error
  ```

  It owns:
  - [ ] Building the logger with `logging.New(flagRootVerbose)`.
  - [ ] Building the DynamoDB client from `flagRootAwsRegion` + `endpoint`.
  - [ ] Building `goety.Service`.
  - [ ] Starting a stderr spinner when `!flagRootVerbose` and passing `spin.Update` as the progress callback.
  - [ ] Running `action` and returning its error.

- [ ] Refactor `cmd_purge.go`, `cmd_dump.go`, `cmd_seed.go` to use `RunE`, return errors, and contain only flags, validation, and option building.
- [ ] Remove every `os.Exit(1)` and `logging.New(...)` from the command files.
- [ ] Use `cmd.Context()` instead of `context.Background()` so cancellation can be wired later without touching commands.
- [ ] Error policy:
  - [ ] Set `rootCmd.SilenceUsage = true`; otherwise every runtime error prints the full usage text.
  - [ ] Leave `SilenceErrors` at its default so Cobra prints the error once.
  - [ ] `main` only exits non-zero; it does not print the error again.
  - [ ] Remove `logger.Error(...)` immediately before `return err` in the service and DynamoDB client; wrap the error instead.
- [ ] Change `logging.New` to write to `os.Stderr` so logs and progress never mix with `--dry-run` data on stdout.
- [ ] Remove `logging.FromContext` / `logging.WithContext` (`internal/logging/context.go`) once nothing reads a logger from a context.
- [ ] Remove `dynamodb.NewWith`; fold its two options into `NewClient(ctx, region, endpoint)`.
- [ ] Drop the logger from `dynamodb.Client`. It is a thin adapter; error logs duplicate the service and per-item `JSONStringify` debug logs are noise. Delete `JSONStringify` if it becomes unused.
- [ ] Keep `cmd/local` as the local/e2e DynamoDB setup harness used by `make db-seed`; do not fold it into the CLI. Document its role in `README.md` and clarify the `db-seed` target comment.
- **Files affected:** `internal/commands/*`, `internal/logging/*`, `internal/dynamodb/dynamodb.go`, `cmd/local/*`, `main.go`, `README.md`

### Phase 3: DynamoDB adapter and scan iteration

- [ ] Move the `Client` struct from `internal/dynamodb/types.go` to `internal/dynamodb/dynamodb.go` (or `client.go`).
- [ ] Remove the dead `ErrNoItems` check from `Dump`, then delete `ErrNoItems` from `internal/dynamodb/types.go`. Leave `AVer` / `AV*` in place until Phase 4 replaces them; removing them here would silently break `--raw-output` for an intermediate commit.
- [ ] Remove `dryRun` from the DynamoDB client and delete the `batchWriteItem` dry-run branch and its two "should not make db call on dry run" tests. The service is the single owner of dry-run.
- [ ] Remove `Put` from `goety.DynamoClient`, `Client.Put`, and `PutItem` from `ddbClient`; run `go generate ./...` to regenerate mocks.
- [ ] Replace `ScanIterator` with a standard-library range iterator (the module targets Go 1.27):

  ```go
  func ScanAll(ctx context.Context, scanner Scanner, input *dynamodb.ScanInput) iter.Seq2[*dynamodb.ScanOutput, error]
  ```

  - [ ] Clone the input (`in := *input`) inside the iterator so pagination does not mutate the caller's `ScanInput`.
  - [ ] Yield `(nil, err)` once on a scan error and stop.
  - [ ] Callers use `for out, err := range ddb.ScanAll(...)`; `return`/`break` stops pagination without a sentinel error.
  - [ ] The current triple return `(*ScanOutput, error, bool)` is the real defect: `done` is easy to misread, `nil` means both "finished" and "no result", and the caller still owns the loop. Use `iter.Seq2`; do not add a custom iterator type, `Next() (T, error)` wrapper, channel, or callback-plus-sentinel.
  - [ ] Add a test that covers multiple pages, a scan error, and early `break`.
- [ ] Keep `BatchWriteItem.UnprocessedItems` looping, but add a small backoff before retrying.
- **Files affected:** `internal/dynamodb/*`, `internal/goety/goety.go`, `internal/goety/interfaces.go`, generated mocks

### Phase 4: Dump and seed output

- [ ] Delete `AVer` / `AV*` (and the now-empty `internal/dynamodb/types.go`) and replace `ConvertAVValue` / `ConvertAVValues` with one recursive converter, for example `toDynamoJSON(types.AttributeValue) any`, that emits the DynamoDB JSON shape (`{"S": ...}`, `{"N": ...}`, `{"M": {...}}`, `{"L": [...]}`, and so on). This is required because the AWS types do not do it themselves.
- [ ] Keep `FlattenAttrValue` / `FlattenAttrList` for normal output.
- [ ] Optional: return `json.Number` for `N` instead of `float64`. This preserves large integers and makes flattening infallible, removing error plumbing. If taken, update the numeric tests.
- [ ] Replace the `first *bool` writer with a small array helper:

  ```go
  type jsonArrayWriter struct { w io.Writer; wrote bool }
  func (w *jsonArrayWriter) Write(v any) error
  func (w *jsonArrayWriter) Close() error
  ```

  - [ ] Write `[` when the file opens and `]` when it closes.
  - [ ] Use `json.Marshal` per item; do **not** use `json.Encoder` unless newline-delimited output is acceptable.
  - [ ] Ensure the closing `]` is written on error as well, or delete the partial file. A truncated JSON file is worse than no file.
- [ ] Remove `transformDumpOutput`; raw dry-run can collect `map[string]any` directly and marshal once.
- [ ] For `--dry-run`, accumulate items for `Dump` and `Seed`, then pretty-print once. Do not print one JSON document per seeded item.
- [ ] Make `prettyPrint` return an error instead of swallowing it, and write to an explicit `io.Writer`.
- [ ] Wire `--limit` into `ScanInput.Limit` or remove the flag and its README entry. Recommendation: wire it; it is one line and the help text already promises it.
- [ ] Tests: extend the existing exact-output assertions rather than adding golden files.
  - [ ] Multiple items and multiple scan pages.
  - [ ] Raw output with nested `M`/`L` values.
  - [ ] Empty result still produces `[]`.
  - [ ] Flattened and raw output are byte-identical to the pre-refactor output.
- **Files affected:** `internal/dynamodb/utils.go`, `internal/dynamodb/iterator.go`, `internal/goety/goety.go`, `internal/goety/goety_test.go`, `internal/dynamodb/*_test.go`

### Phase 5: Options and API surface

- [ ] Replace `internal/goety/options.go` functional options with a plain `DumpOptions` struct populated by the command.
- [ ] Remove `QueryFuncOpts`, `WithQueryOptions`, and the six `With*` functions.
- [ ] Move `--attribute-name` / `--attribute-value` string parsing into `internal/commands/cmd_dump.go`; the service should receive typed maps, not comma-separated strings.
- [ ] Wire or delete `DumpOptions.Limit` (see Phase 4).
- [ ] Rename `internal/goety/types.go` to `service.go`; a file named `types.go` holding the service is a smell.
- **Files affected:** `internal/goety/options.go`, `internal/goety/types.go`, `internal/goety/goety.go`, `internal/commands/cmd_dump.go`

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

## Notes and reminders

- Keep the runner thin. Command-specific flag parsing and option building belong in the commands.
- Keep the progress callback non-blocking. It should only store a string under a mutex.
- The service is the only dry-run owner. Do not propagate `dryRun` into the DynamoDB client.
- The AWS SDK default retryer is sufficient; only `UnprocessedItems` need the manual loop.
- Use the standard library `iter` package (`iter.Seq2`) for scan pagination; no custom iterator types, channels, or callback sentinels.
- Do not use `context.Context` as a service locator for the logger or clients.
- Out of scope: changing output formats, adding new commands, or reworking DynamoDB key handling beyond what is listed.
