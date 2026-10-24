# Plan: Simplify the goety codebase

Use this list to track what has been completed. Mark items `[x]` only after the change is implemented and `make test && make lint` pass.

## Current state

`goety` is a small DynamoDB CLI (purge, dump, seed). The business logic fits in a few files, but the code is more complex than its surface area requires.

## Goal

Make the codebase easier to read, test, and maintain by removing unnecessary indirection and duplication while preserving the existing CLI behaviour and test coverage.

## AWS SDK retries (context)

AWS SDK for Go v2 provides a default `Standard` retryer. It automatically retries throttling (`ProvisionedThroughputExceededException`), 5xx service errors, and transient network errors with backoff and jitter. **No third-party retry library is needed.**

The SDK does **not** retry `BatchWriteItem.UnprocessedItems` — that response is a partial success, not an error. The manual loop in `internal/dynamodb/dynamodb.go` is required and should be kept. Consider adding a small backoff between retries to avoid hammering DynamoDB under throttle.

## Work checklist

### Phase 1: CLI bootstrap and state cleanup

- [x] Delete `internal/logging/interfaces.go`; use `*slog.Logger` directly.
- [x] Fix `internal/logging/logging.go`:
  - [x] Remove package-level `baseLogger` mutation.
  - [x] Make `New(verbose)` return a fresh `*slog.Logger` each call.
  - [x] Use `slog.Default()` only as a fallback, not as mutable state.
- [ ] Create `internal/commands/runner.go` with a thin shared runner that owns:
  - [ ] Building the logger from `flagRootVerbose`.
  - [ ] Building the DynamoDB client from `flagRootAwsRegion` + command-specific endpoint.
  - [ ] Building `goety.Service`.
  - [ ] Attaching a non-blocking progress callback; wiring it to a spinner when not verbose.
  - [ ] Executing the command-specific action and returning its error.
- [ ] Refactor each command (`cmd_purge.go`, `cmd_dump.go`, `cmd_seed.go`) so it only declares flags and exposes a `RunE` function returning `error`.
- [ ] Remove `os.Exit(1)` duplication from commands.
- [ ] Set `rootCmd.SilenceErrors = true` (or equivalent) so the runner/logger owns error output and Cobra does not duplicate it.
- [ ] Decide on `cmd/local` and `cmd/spinner`:
  - [ ] If still useful as dev/test utilities, move them to `tools/` and document them in `README.md`.
  - [ ] If no longer used, delete them. (`cmd/spinner` depends on `internal/emitter`, which will be removed.)
- **Files affected:** `internal/commands/*`, `internal/logging/*`, `cmd/local/*`, `cmd/spinner/*`, `main.go`

### Phase 2: Progress reporting and spinner

- [ ] Delete `internal/emitter` package and its interfaces.
- [ ] Remove the unused `Emitter` interface from `internal/goety/interfaces.go`.
- [ ] Change `goety.Service` to accept a `Progress func(msg string)` callback.
  - [ ] Contract: `Progress` must return quickly and must not block the service.
- [ ] Update the spinner to implement the progress callback:
  - [ ] Store the latest message under a `sync.Mutex`.
  - [ ] Use a buffered `chan struct{}` (size 1) only as a wake signal for the draw goroutine.
  - [ ] Draw goroutine reads the latest message each frame; intermediate updates coalesce naturally.
- [ ] Remove goroutine coordination, blocking sends, and message-loss risk.
- **Files affected:** `internal/emitter/*`, `internal/spinner/*`, `internal/goety/*`

### Phase 3: DynamoDB internals and dump output

- [ ] Replace `ScanIterator` triple-return `(*ScanOutput, error, bool)` with `ScanAll(ctx, client, input, fn) error`, where `fn` returns an error to stop pagination.
- [ ] Remove `ErrNoItems` from `internal/dynamodb/types.go` and the dead check in `internal/goety/goety.go`.
- [ ] Delete `internal/dynamodb/types.go` and the `AVer`/`AV*` structs.
- [ ] Delete `ConvertAVValue` / `ConvertAVValues` in `internal/dynamodb/utils.go`.
- [ ] For `--raw-output`, marshal AWS SDK `types.AttributeValue` map directly (AWS types already marshal to DynamoDB JSON).
- [ ] For normal output, keep `FlattenAttrValue` / `FlattenAttrList`.
- [ ] Streamline dump JSON output:
  - [ ] Replace manual `[`, `,`, `]` management with `json.Encoder` and a small writer helper.
  - [ ] Write opening `[` and closing `]` once; encode each item and write comma separators between items.
  - [ ] Remove the `first` pointer and `transformDumpOutput` marshal/unmarshal round-trip.
  - [ ] For dry-run, accumulate into a slice and pretty-print once at the end.
- [ ] Add or update golden-file tests for dump output (raw and flattened) before or while changing serialization.
- **Files affected:** `internal/dynamodb/iterator.go`, `internal/dynamodb/iterator_test.go`, `internal/dynamodb/types.go`, `internal/dynamodb/utils.go`, `internal/goety/goety.go`, `internal/goety/goety_test.go`

### Phase 4: API simplification (optional but recommended)

- [ ] Replace `internal/goety/options.go` functional options with a plain `DumpOptions` struct populated by the command.
- [ ] Remove six small option functions and the `QueryFuncOpts` alias.
- [ ] Move `--attribute-name` / `--attribute-value` string parsing into the command layer.
- **Files affected:** `internal/goety/options.go`, `internal/goety/goety.go`, `internal/commands/cmd_dump.go`

## Alternative approach: surface-only cleanup

If you need to ship faster or want lower risk, do **Phase 1 and Phase 2 only**. This removes the most visible duplication, the logging global state, and the emitter deadlock, but leaves the custom attribute-value types, iterator, and dump streaming as-is. Return to Phase 3 and Phase 4 afterwards.

## Verification

After each phase, run:

```bash
make test
make lint
```

After Phase 3, also run the integration targets against a local DynamoDB:

```bash
make build
./goety dump ...
./goety seed ...
./goety purge ...
```

If possible, capture golden output from `dump` before changing serialization so regressions are obvious.

## Notes and reminders

- Keep the shared runner thin; command-specific flag parsing and option building belong in the commands.
- Keep the progress callback non-blocking; the service must not wait on the UI.
- The AWS SDK default retryer is sufficient; only `UnprocessedItems` need the manual loop in `batchWriteItem`.
- Consider adding a small backoff in the `UnprocessedItems` retry loop to avoid hammering DynamoDB under throttle.
