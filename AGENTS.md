# AGENTS.md

Instructions for AI coding agents working in this repository. Read this before changing code.

## Project

`goety` is a Go CLI for common Amazon DynamoDB operations: `purge`, `dump`, and `seed`.

- Module: `github.com/code-gorilla-au/goety`
- Go: see `go.mod` (do not downgrade)
- CLI entry point: `main.go` -> `internal/commands`
- Service/domain layer: `internal/goety`
- DynamoDB adapter: `internal/dynamodb`
- Cross-cutting packages: `internal/logging`, `internal/emitter`, `internal/spinner`
- Local/e2e harness: `cmd/local` (creates and seeds a local DynamoDB table; used by `make db-seed`)
- Refactor roadmap: `docs/plan.md`

## Iron rules

1. **TDD is mandatory.** No production code is written before a failing test exists. This is non-negotiable. Follow `.agents/skills/tdd/SKILL.md`.
2. **Mock only external-service dependencies and the file I/O wrapper.** Tests may fake third-party clients that would otherwise contact a real service (in this repo, the AWS SDK/DynamoDB boundary). They may also stub the `FileReadWriter` wrapper, which exists to simulate files. Do not mock anything else: services, adapters, loggers, emitters, spinners. See [What may be mocked](#what-may-be-mocked).
3. **Prefix every shell command with `rtk`.** Examples: `rtk make test`, `rtk go test ./...`, `rtk git status`, `rtk git diff`.
4. **Go through the Makefile.** Run `rtk make help` to find the right target. Do not invent ad-hoc invocations for anything CI or another agent will run.
5. **If a required command has no Make target, record it in `docs/tooling-upgrade-plan.md`** with the proposed target before working around it. See [Missing commands](#missing-commands).
6. **Keep PRs small and vertical.** One behavior delivered end to end (test, implementation, docs) per PR. Split large work into sequential PRs instead of bundling unrelated changes.
7. **The PR title is the release signal.** Squash merge uses the PR title, not individual commits. Title it with the master Conventional Commit for the batch. See [Pull requests and commits](#pull-requests-and-commits).

## Commands

| Task | Command |
| --- | --- |
| List all targets | `rtk make help` |
| Install dev + scan tooling | `rtk make install` |
| Unit tests (all packages) | `rtk make test` |
| Unit tests + coverage profile | `rtk make test-unit` |
| HTML coverage report | `rtk make test-cover` |
| Lint (`go vet` + `golangci-lint`) | `rtk make lint` |
| Vulnerability scan | `rtk make scan` |
| Full CI (`log test lint scan`) | `rtk make ci` |
| Build binary | `rtk make build` |
| Regenerate mocks | `rtk make generate` |
| Start local DynamoDB + seed table | `rtk make db-seed` |
| Stop local DynamoDB | `rtk make db-kill` |
| Integration: seed / dump / purge | `rtk make test-seed` / `test-dump` / `test-purge` |

Notes:

- `make ci` requires `golangci-lint` and `govulncheck`. Install them with `rtk make install-ci` (or `rtk make install`) if they are missing.
- Integration targets build the CLI and expect `.env.local` (gitignored) with `DYNAMODB_LOCAL_ENDPOINT`, `TEST_TABLE_NAME`, `TEST_PRIMARY_KEY`, and `TEST_SORT_KEY`.
- `make build` writes a binary named after the repo root (`goety`), which is gitignored.

### Running a single test

There is no Make target for a focused test run yet. For local red/green iteration only, use a targeted `rtk go test` and add a `test-target` entry to `docs/tooling-upgrade-plan.md` if you need it repeatedly:

```bash
rtk go test ./internal/goety -run TestService_Purge -v
```

## TDD workflow (the iron rule)

For every behavior change, including bug fixes:

1. **Red** — write or extend an `_test.go` file that expresses the desired behavior. Run it and confirm it fails for the right reason.
2. **Green** — write the smallest production change that makes the test pass. Nothing more.
3. **Refactor** — improve names/structure with the test staying green.
4. **Gate** — run `rtk make ci`.

Additional rules:

- A bug fix starts with a failing regression test that reproduces the bug.
- Never weaken, skip, or delete an existing assertion to make a suite pass.
- Never write production code "first, tests after".
- Cover both the happy path and error paths (scan error, file error, batch write error, empty input).
- Test names are sentences starting with `should ...`.
- Do not hit AWS in unit tests. Fake only the AWS SDK/DynamoDB boundary; see [What may be mocked](#what-may-be-mocked).

## Testing conventions (odize)

The test framework is [`odize`](https://github.com/code-gorilla-au/odize). Use the standard library `testing` package underneath it.

```go
func TestService_Purge(t *testing.T) {
	group := odize.NewGroup(t, nil)

	group.BeforeEach(func() {
		// fresh real dependencies per test
	})

	group.AfterEach(func() {
		// reset shared counters/state
	})

	err := group.
		Test("should purge items", func(t *testing.T) {
			err := service.Purge(ctx, "my-table", TableKeys{PartitionKey: "pk", SortKey: "sk"})
			odize.AssertNoError(t, err)
			odize.AssertEqual(t, 1, callBatchDelete)
		}).
		Test("should not delete items on dry run", func(t *testing.T) {
			service.dryRun = true
			// ...
		}).
		Run()

	odize.AssertNoError(t, err)
}
```

- Lifecycle hooks: `BeforeAll`, `BeforeEach`, `AfterEach`, `AfterAll`.
- Assertions: `odize.AssertEqual`, `AssertTrue`, `AssertFalse`, `AssertNoError`, `AssertError`, `AssertNil`.
- Test options: `odize.Skip()`, `odize.Only()`.
- Tagged groups (`odize.NewGroup(t, &[]string{"integration"})`) are filtered with `ODIZE_TAGS`.

### What may be mocked

Mock third-party dependencies that would contact an external service. In this repo that is the AWS SDK/DynamoDB boundary (`ddbClient` in `internal/dynamodb`); a generated moq mock or inline fake may stand in for `Scan`, `BatchWriteItem`, `PutItem`, and similar calls.

**File I/O is the one sanctioned exception.** `FileReader` / `FileWriter` / `FileReadWriter` (`internal/goety/interfaces.go`) wrap `io` specifically so tests can simulate files, including failures a real filesystem cannot produce deterministically (create error, mid-stream write error, close error). Service tests that touch files must stub this boundary with an in-memory implementation; it is intentionally stubbed and must not be replaced with real files. `OSFile` is the production implementation, and `t.TempDir()` is only for tests of `OSFile` itself.

Do not mock code you own otherwise:

- `goety.Service`, `dynamodb.Client`, or internal wiring interfaces such as `DynamoClient`.
- Loggers, emitters, spinners: use the real implementations and inject an `io.Writer` when you need to assert output.

If a test needs any other internal mock, move the test to the external boundary or refactor production code so the real dependency can be used. Do not add an interface solely to create a mock.

- Generated mocks come from `//go:generate moq` directives in `internal/*/interfaces.go`:

  ```bash
  rtk make generate
  ```

  Never hand-edit `mocks_test.go`; change the interface and regenerate.
- Do not add new mocks of internal types. Legacy internal mocks may remain until the file is next changed; do not widen a PR solely to remove them.
- Stub the `FileReadWriter` for service tests that read or write files; reserve `t.TempDir()` for tests of `OSFile` itself.
- Use a `bytes.Buffer` behind the file stub or an injected `io.Writer` when you need to assert output.

## Code conventions

- Prefer the standard library and existing dependencies. Do not add a dependency without a clear need.
- Return errors; do not `log-and-exit` inside library packages. Wrap with context: `fmt.Errorf("scan table: %w", err)`.
- The CLI boundary prints errors once. `main` only exits non-zero.
- Keep dependency wiring explicit (logger/client arguments). Do not use `context.Context` as a service locator.
- `goety.Service` is the single owner of dry-run behavior; the DynamoDB adapter stays a thin wrapper.
- Keep machine-readable output (`dump`, dry-run JSON) on stdout. Progress/UI belongs on stderr where the roadmap says so.
- Follow the package layout and naming already present. Keep changes minimal and focused.
- **No useless comments.** Write code that explains itself through clear names and small functions. A comment must say something the code cannot: a non-obvious *why*, a constraint, or a workaround. Never write comments that restate what the code does, narrate the change, or label obvious steps. If a comment can be removed by renaming something or extracting a function, do that instead. Doc comments are only for exported identifiers whose behavior is not clear from the name and signature.
- Do not "clean up" unrelated code while implementing a feature.

## Missing commands

If a command you need is not in the Makefile:

1. Add an entry to `docs/tooling-upgrade-plan.md` describing the command, why it is needed, and the proposed Make target.
2. Use the closest existing Make target meanwhile.
3. Do not add ad-hoc flags to CI, scripts, or other automation.

## Pull requests and commits

- Default branch is `main`; changes land via pull requests.
- **PRs are small vertical slices.** Each PR delivers one reviewable behavior end to end: test, implementation, and docs. Split large work into sequential PRs; do not bundle unrelated refactors, formatting, or dependency bumps.
- **The PR title is what releases.** GitHub squash-merges with the PR title as the commit subject, so semantic-release ignores individual commit messages. Use the format:

  ```text
  <type>(<scope>): <summary>
  ```

  Release types: `feat` (minor), `fix` / `perf` / `refactor` (patch), plus non-release types such as `chore`, `docs`, `test`, `build`, and `ci`.
- **Choose the master type from the whole batch.** Use the highest-impact change in the PR: a feature makes it `feat`, a bug-only PR is `fix`, and a behavior break needs a `BREAKING CHANGE:` footer in the PR body (major). Never label a feature as `chore` to avoid a minor release.
- **The PR description carries the detail** that squash merge discards: what changed, why, how it was tested, and any `BREAKING CHANGE:` footer.
- Individual commits may still be conventional for rebase and cherry-pick, but do not rely on them for releases.
- Do not commit, create branches, or push unless the user explicitly asks.

## Definition of done

- [ ] Failing test written first, then implementation.
- [ ] New/changed behavior covered by odize group tests, including error paths.
- [ ] Tests mock only external-service dependencies and the `FileReadWriter` stub; no other internal mocks.
- [ ] `rtk make ci` passes.
- [ ] `rtk make generate` run if interfaces or `//go:generate` directives changed.
- [ ] Docs updated when CLI behavior or flags change.
- [ ] Any missing tooling recorded in `docs/tooling-upgrade-plan.md`.
- [ ] PR is one small vertical slice.
- [ ] PR title is the master Conventional Commit for the batch.
