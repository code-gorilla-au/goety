---
name: tdd
description: Enforce red-green-refactor in the goety repo using Go's testing package plus odize groups and moq mocks. Use for every behavior change, feature, refactor, or bug fix before writing production code.
---

# TDD for goety

This is the repository's iron rule: **no production code without a failing test first.**

## When to use

Use this skill for any change that alters behavior: new flags, commands, service logic, adapters, error handling, or a bug fix. If you are only editing docs or comments, TDD does not apply.

## Cycle

Work in small loops. Never batch multiple behaviors into one red/green cycle.

### 1. Red

1. Find the package that owns the behavior (`internal/goety`, `internal/dynamodb`, `internal/commands`, etc.).
2. Add or extend a `_test.go` file in that package. Do not create a new test framework or helper package.
3. Express exactly one behavior with an odize group test.
4. Run it and confirm it fails for the intended reason:

   ```bash
   rtk go test ./internal/goety -run TestService_Purge -v
   ```

   For the whole suite use `rtk make test-unit`. A targeted `rtk go test` is a temporary fallback; record a missing `test-target` Make target in `docs/tooling-upgrade-plan.md` if you rely on it.

5. If the test passes immediately, either the behavior already exists or the test is not asserting anything useful. Fix the test before continuing.

### 2. Green

Write the smallest production change that makes the failing test pass. Do not add options, abstractions, or extra behavior the test does not demand.

### 3. Refactor

Improve names, extract helpers, or remove duplication while the test stays green. Re-run the focused test after each edit.

### 4. Gate

```bash
rtk make ci
```

`make ci` runs `log`, `test`, `lint` (`go vet` + `golangci-lint`), and `scan` (`govulncheck`). If a required tool is missing, install it with `rtk make install-ci` instead of skipping the gate.

Before opening a PR: keep the change to one small vertical slice, and title the PR with the master Conventional Commit for the batch (`feat:`, `fix:`, `refactor:`, ...). Squash merge uses the PR title for releases, not the individual commits.

## Odize patterns

Every test is an odize group. Create fresh state in `BeforeEach`, reset shared counters in `AfterEach`, and assert the group result at the end.

```go
func TestClient_BatchWriteItems(t *testing.T) {
	logger := logging.New(true)
	ctx := context.Background()
	items := []map[string]types.AttributeValue{
		{"pk": &types.AttributeValueMemberS{Value: "pk"}},
	}

	var client Client
	var db ddbClientMock

	group := odize.NewGroup(t, nil)

	group.BeforeEach(func() {
		db = ddbClientMock{
			// Only the third-party AWS SDK call is faked.
			BatchWriteItemFunc: func(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
				return &dynamodb.BatchWriteItemOutput{}, nil
			},
		}
		client = Client{db: &db, logger: logger}
	})

	err := group.
		Test("should write items in a batch", func(t *testing.T) {
			_, err := client.BatchWriteItems(ctx, "table", items)
			odize.AssertNoError(t, err)
		}).
		Test("should return the error when the batch write fails", func(t *testing.T) {
			db.BatchWriteItemFunc = func(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
				return nil, errors.New("boom")
			}

			_, err := client.BatchWriteItems(ctx, "table", items)
			odize.AssertError(t, err)
		}).
		Run()

	odize.AssertNoError(t, err)
}
```

Rules:

- Group constructor: `odize.NewGroup(t, nil)`. Use `&[]string{"integration"}` only for tests that need real infrastructure.
- Hooks: `BeforeAll`, `BeforeEach`, `AfterEach`, `AfterAll`.
- Assertions: `odize.AssertEqual`, `AssertTrue`, `AssertFalse`, `AssertNoError`, `AssertError`, `AssertNil`.
- Options: `odize.Skip()`, `odize.Only()`. Remove `Skip`/`Only` before finishing.
- Sub-test names must read as sentences starting with `should ...`.
- Assert the exact error with `errors.Is` when the error is wrapped.
- Keep the final `odize.AssertNoError(t, err)` after `Run()`.

## What may be mocked

Mock third-party dependencies that would contact an external service. In this repo that is the AWS SDK/DynamoDB boundary (`ddbClient` in `internal/dynamodb`). Fake `Scan`, `BatchWriteItem`, `PutItem`, and friends with a generated moq mock or an inline fake.

**File I/O is the one sanctioned exception.** `FileReader` / `FileWriter` / `FileReadWriter` (`internal/goety/interfaces.go`) wrap `io` specifically so tests can simulate files, including failures a real filesystem cannot produce deterministically (create error, mid-stream write error, close error). Service tests that touch files must stub this boundary with an in-memory implementation; it is intentionally stubbed and must not be replaced with real files. `OSFile` is the production implementation, and `t.TempDir()` is only for tests of `OSFile` itself.

Do not mock code you own otherwise:

- `goety.Service`, `dynamodb.Client`, or internal wiring interfaces such as `DynamoClient`.
- Loggers, emitters, spinners — use the real implementations; inject a `bytes.Buffer` through an `io.Writer` when you need to assert output.

If a test cannot work without any other internal mock, move the test to the external boundary or refactor production code so the real dependency can be used. Do not add an interface just to mock it. A service test should wire the real `dynamodb.Client` around the SDK fake rather than mocking `DynamoClient`; add a constructor seam if the current wiring does not allow it.

- Do not hit a real AWS account or table in unit tests.
- Do not add new mocks of internal types. Legacy internal mocks may stay until the file is next changed; do not widen a PR just to delete them.
- Never hand-edit `mocks_test.go`. Change the interface in `internal/*/interfaces.go` and regenerate:

  ```bash
  rtk make generate
  ```

- Stub the `FileReadWriter` for service tests that read or write files; reserve `t.TempDir()` for tests of `OSFile` itself.
- Use a `bytes.Buffer` behind the file stub or an injected `io.Writer` when you need to assert output.
- Cover at least one error path per new branch: scan error, create/open error, decode error, batch write error, empty input.

## Bug fixes

1. Write a regression test that reproduces the bug and fails on the current code.
2. Confirm the failure message matches the bug.
3. Apply the minimal fix.
4. Keep the regression test permanently.

## Never

- Never write implementation before the failing test.
- Never mock code you own, except the `FileReadWriter` wrapper, which exists to simulate files.
- Never weaken, delete, or skip an assertion to make the suite pass.
- Never mark tests `t.Skip` permanently to hide a failure.
- Never refactor unrelated code in the same change.
- Never bundle unrelated work into one PR.
- Never finish without running `rtk make ci`.
