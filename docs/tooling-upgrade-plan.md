# Tooling upgrade plan

Agents must drive work through the `Makefile` so CI and local workflows stay identical. When a task needs a command that has no target, record it here with the proposed target **before** using a workaround.

## How to use

1. Run `rtk make help` and use the closest existing target.
2. If no target fits, add a row below with the command, why it is needed, and the proposed target.
3. Use a direct command only as a temporary, local fallback. Do not add it to CI, scripts, or other automation until the target exists.
4. Remove or mark a row `done` once the target is added to the `Makefile`.

## Proposed targets

| Status | Command needed | Why | Proposed target |
| --- | --- | --- | --- |
| proposed | `go test ./internal/<pkg> -run <TestName> -v` | Red/green iteration on a single odize group without waiting for the full suite. | `test-target` with `PKG` and `RUN` variables, e.g. `rtk make test-target PKG=./internal/goety RUN=TestService_Purge` |
| proposed | `go test -race ./...` | `docs/plan.md` requires the race detector after spinner/progress changes. | `test-race` |
| proposed | `go test -tags=integration ./...` | Run only tagged integration tests (`ODIZE_TAGS=integration`) against local DynamoDB. | `test-integration` |
| proposed | `go mod tidy` | Keep `go.mod`/`go.sum` clean after dependency changes. CI does not check this today. | `tidy` |
| proposed | `gofmt -l .` / `goimports -l .` | Verify formatting before review; `make lint` only runs `go vet` and `golangci-lint`. | `fmt-check` |
| proposed | `rm -f goety coverage.out coverage.html test.json` | Remove the local binary and generated coverage/test artifacts. | `clean` |

## Notes

- `rtk make` and `rtk go test` are already available through `rtk`; the missing piece is a stable Make target, not the underlying command.
- `make ci` requires `golangci-lint` and `govulncheck`; `rtk make install-ci` installs them.
