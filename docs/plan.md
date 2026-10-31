# goety — Refactor Roadmap

Principal-engineer review: simplify toward idiomatic Go, critique the design, tighten test coverage, and improve how the tool communicates progress vs. debug detail.

The baseline is strong: the `iter.Seq2` paginator (`ScanAll`), the functional-options surface for `Dump`, the clean `slog` setup, and strict mock discipline (only the SDK boundary + `FileReadWriter`) are all idiomatic. The weak spots concentrate in one function — `Service.Dump` — plus a small set of communication/design inconsistencies that read as "accidentally complex" rather than "deliberately hard."

Legend for priorities: **P0** = correctness (do first), **P1** = high-leverage readability/design, **P2** = cheap hygiene, **Comm** = UX/communication, **P3** = coverage. Items are numbered **§1–§27** in **execution order** by tier (P0 → P1 → P2 → Comm → P3); work them top to bottom. The “Recommended build order” checklist at the bottom is the same items collapsed to one line each, with the same §numbers.


---

## P1 — Design & simplification (highest-leverage)

### 5. Drop the `AVer` marker interface and the `Marshal → Unmarshal` round-trip
`internal/dynamodb/types.go`; `internal/goety/goety.go:332–341`.

- `IsAV() bool` is never called; it exists only to make the 10 structs an interface. The field names (`S`/`N`/`BOOL`/`M`/`L`/…) are what make `json.Marshal` emit the raw wire shape (`{"pk":{"S":"pk"}}`), so keep the structs but stop faking polymorphism. `ConvertAVValues` can build JSON-ready output directly.
- `transformDumpOutput` does `ConvertAVValues` → `json.Marshal` → `json.Unmarshal` back into `[]map[string]any`. The round-trip exists **only** to erase `AVer → any` to match the non-raw path's typing. But `dryRunResult` is only ever printed via `prettyPrint` — it crosses no boundary requiring a fixed type. Print the AVer slice directly (json handles concrete types) and drop both the Unmarshal and the type-erasure rationale. Fewer allocations, clearer intent.

### 6. Reconcile `dry run` semantics across commands
`slogger`— the service. One flag, three meanings:

- **Purge** (`goety.go:70–74`): previews the **first page only**, then `return nil` inside the loop. A 1000-item table → `purge -d` shows 25, not 1000.
- **Dump** (`goety.go:142–147, 158–162`): accumulates **all pages**, prints. Full preview.
- **Seed** (`goety.go:217–220, 241–243`): accumulates all items, prints, skips writes. Full preview.

CLI help promises *"dry run does not perform actions, only logs them"* (README:126). Purge's first-page-only preview arguably breaks that promise. Fix: make them consistent (Purge loops to the end in dry-run like Dump), or document the per-command difference deliberately. `Service` is the single owner of dry-run behavior (per AGENTS.md), so centralizing the checks would expose the inconsistency at the seams.

### 7. Co-locate the `Service` type with its methods
`internal/goety/service.go:9–31` defines `Service`/`TableKeys`/`DumpOptions`/`DumpOption`; **all methods** (`Purge`, `Dump`, `Seed`) live in `goety.go`. "Where is the service?" requires hopping files. Convention favors type + methods together. Either move the defs into `goety.go` or consolidate so the file names the type. Low priority, but it signals the layout follows an older model than the code now.

### 8. Name `defaultBatchSize` for what it means; expose the coupling
`internal/goety/goety.go:18`. It serves two roles: (a) the **scan page size** in `Purge` (line 58), and (b) the **batch-write/delete chunk size** in `Seed` (line 231). (b) is a hard DynamoDB constraint (≤25 `WriteRequest`s per `BatchWriteItem`); (a) is arbitrary but aligned to (b) for convenience.

The hidden invariant: `Purge` calls `BatchDeleteItems` **once per page**, so the delete chunk size *is* the scan page size. Raise the scan `Limit` above 25 in a future optimization and `BatchDeleteItems` gets >25 items → SDK `ValidationException`. Today the constraint is enforced only by the two values coinciding.

Fix: name it for the meaning (`maxBatchWrite = 25`) and, if a separate scan page size is wanted, introduce a distinct named constant with a one-line comment tying them. Make the invariant visible.

---

## P2 — Cheap hygiene (quick, self-contained PRs)

### 9. Fix three copy-paste doc comments
- `internal/commands/cmd_seed.go:30` — `seedFunc`'s doc says **`purgeFunc is the entry point for the purge command. It will purge a dynamodb table of all items`**. Wrong function name, wrong command.
- `internal/commands/cmd_seed.go:45` — `parseSeedFlag`'s doc says **`parsePurgeFlag will validate the flags passed to the purge command`**.
- `internal/commands/cmd_dump.go:123` — `parseDumpFlag`'s doc says **`parsePurgeFlag will validate the flags passed to the purge command`** but the function is dump.

These are "comments that restate the code but get the name wrong" — exactly the class AGENTS.md warns about ("no useless comments"). Correct all three to name the actual function/command.

### 10. Rename `internal/goety/writer.go` → `file.go`; rename the `OSFile` receiver
`internal/goety/writer.go` holds `OSFile`, the production `FileReadWriter` — but the filename `writer.go` undersells it (`Open` reads). Rename to `file.go`. Also the receiver is named `w`:

```go
func (w *OSFile) Create(...) ...
func (w *OSFile) Open(...) ...
```

`w` on a file opener reads as a leftover from a writer-only version; `f` is clearer. Combine with the rename.

### 11. Add the `OSFile` compile-time assertion
`internal/goety/interfaces.go` has `var _ DynamoClient = (*ddb.Client)(nil)` (line 20) but **not** `var _ FileReadWriter = (*OSFile)(nil)`. Nothing enforces `OSFile` keeps satisfying `FileReadWriter` at the type site. Add the assertion beside the `DynamoClient` one. Catches interface drift.

### 12. Remove the stray test debug print
`internal/goety/goety_test.go:223`:

```go
createFunc: func(filename string, perm fs.FileMode) (io.WriteCloser, error) {
    fmt.Println("creating file", filename)   // leftover debug → pollutes stdout
    callWriteFile++
    return &mockWriteCloser{}, nil
},
```

No assertion depends on it. Delete it (or turn into `t.Log`).

### 13. Clean up `cmd/local/main.go`
- Line 39: `return` after `os.Exit(1)` is unreachable. Drop it, or restructure to propagate the error up instead of exiting in a helper.
- Line 49: `"error seeing table"` → `"seeding"`.
- Line 53: it logs `"seed complete"` even when `seedTable` returned an error (it only logs the error at line 49 and falls through). Minor for a local harness; either early-return on error or gate the success line.

### 14. `JSONStringify`: fall back to `fmt.Sprint(v)` on error
`internal/dynamodb/utils.go:10–17`. On marshal failure it returns `""`, so per-item debug logs print `item=` (empty), hiding why. A `fmt.Sprint(v)` fallback shows the raw value. Small, but it's in the same debug-log family as the communication work in P1/§comm.

---

## Communication — progress vs. debug (the "better communication" ask)

The mechanics are sound (`slog` → stderr, `TextHandler`, level-gating); the **design** inverts feedback.

### 15. The verbose flag inverts user feedback (biggest comm issue)
`internal/commands/bootstrap.go:26–32`; `internal/logging/logging.go`.

Normal mode (`!flagRootVerbose`):
- Spinner created → `progress = spin.Update` → `reportProgress` renders as the spinner line ("scanning table …", "deleted N items", "dump complete, saved N items to …") → **rich live progress**.
- Logger at `Info` → minimal slog output.

Verbose mode:
- `if !flagRootVerbose` is false → **no spinner**, `progress = nil` → every `reportProgress` call silently drops → **no live progress at all**.
- Logger drops to `Debug` with `AddSource: true` (logging.go:20) → every line now carries `file.go:NN`.
- Adapter per-item debug floods (next item) + `slogger.Debug("dry run enabled")` (goety.go:71, :159).

**Net: verbose = lose the spinner (no visible progress) + gain a wall of developer-grade logs.** Backwards from intuition — "verbose" should add *helpful* feedback, not remove the progress you were watching and replace it with per-item noise.

Fix: keep the spinner/progress visible in **both** modes (don't gate `progress` on `!flagRootVerbose`; or at least log the progress messages at Info/Debug so they appear when verbose). Treat the spinner (UI feedback) and `slog` (operational detail) as **complementary**, not competing. Secondary: consider decoupling `AddSource` from "verbose" into its own toggle so source attribution is opt-in independent of the noise.

### 16. Deroute per-item adapter debug to per-batch summaries
`internal/dynamodb/dynamodb.go:61, 76`. Both `BatchWriteItems` and `BatchDeleteItems` log **every** item/key at Debug:

```go
for _, item := range items {
    c.logger.Debug("adding item to batch write", "item", JSONStringify(item))
    ...
}
```

A 25-item batch = 25 lines per call. Silent at `Info`, a flood at `Debug`. Per-line information is low. Replace with one summary per operation:

```go
c.logger.Debug("batch write", "op", op, "table", tableName, "count", len(requests))
```

(and likewise for delete). Keeps the useful detail (how many, where) and drops the noise.

### 17. De-duplicate the `Dump` completion signal
`internal/goety/goety.go:169–170`. On non-dry success, **both** fire:

```go
s.reportProgress(fmt.Sprintf("dump complete, saved %d items to %s", itemsScanned, path))
s.logger.Info("dump complete", "items", itemsScanned)
```

Same event via two channels. Pick one (progress for the user / spinner; or the log for ops) — not both. `Purge` and `Seed` only use `reportProgress`, so `Dump` is the odd one out; drop the redundant `logger.Info` (or demote it) so completion is said once.

### 18. Hoist `Debug("dry run enabled")` out of the loop; reword for effect
`internal/goety/goety.go:71` (Purge), `:159` (Dump). It's invariant info, yet in Purge it sits inside the `ScanAll` iteration, guarded by `dryRun`, followed immediately by `prettyPrint` + `return nil` (first-page-only). In Dump it's outside the scan loop (fine) but still per-call.

Fix: hoist to **once, at the top** of each method (right after the opening), not inside per-iteration flow. In Purge specifically, (loop-internal) + (dry-run-guarded) + (immediately followed by early return) buries what is effectively a "preview: first page only" marker. Moving it up makes the behavior legible at a glance.

On wording: the current string states the *mode*, not the *effect*. For a user-facing dry-run message, prefer one that says what is happening, e.g. `"dry run: previewing items (nothing written/deleted)"` — the "better communication" part is telling the user the **consequence**, not just the flag value.

---

## P3 — Test coverage (specific, high-value additions)

Every finding below was traced against the actual fakes in `internal/goety/goety_test.go:137–191`; each is simulatable (or trivially extensible) and pins behavior that is currently only implicit.

### 19. Dump: multi-item AND multi-page output (the comma logic is never actually hit)
Every current dump test uses **one item per page, one page total**. `writeDumpItems` (`goety.go:302–307`) has:

```go
if !*first {
    if _, err = w.Write([]byte(",")); err != nil { ... }
}
*first = false
```

With one item + one page: `first` starts `true` → no comma written → then `first = false`. So the **`,` write** and the `first`-persists-across-pages invariant are never exercised. Add:

1. **Multi-item single page**: page with 2+ items → assert buffer is `[{...},{...}]` (comma *between* items within a page).
2. **Multi-page (each non-empty)**: two consecutive non-empty pages → assert `[{...},{...}]` again. This exercises **cross-page comma continuity** — page 2's `writeDumpItems` sees `first == false` (persisted from page 1) and correctly emits a `,` before its first item. The `first` flag's persistence across pages (pointer-shared into each call, carried across iterations) is a subtle invariant with **zero test coverage** today. High value: it's the actual "streaming JSON array across pages" behavior the whole `dumpWriter` concept rests on.

### 20. Dump: isolate a mid-dump write failure (today it's the *open* failure)
Current coverage:
- ✅ scan failure mid-dump (line 300) — `]` correctly appended via the deferred close.
- ✅ create-file failure (line 332).
- ⚠️ "writing items fails" (line 342) — sets `writeErr` on the `mockWriteCloser`, so the **first** write (the `[` in `openDumpFile`) fails → it actually tests the **open-phase** write failure, under a misleading name.

Not covered: write failure **after** a successful page (page 1 writes cleanly, page 2's `writeDumpItems` errors). The existing `mockWriteCloser.Write` (`goety_test.go:155–160`) has a blanket `writeErr` that fires on *every* write. To isolate, extend the fake (see 21) and add a test where only the post-first-page write fails → assert the buffer holds page-1 content + the closing `]` (written by the deferred `closeDumpFile`, since `closed` is still false when the mid-dump write error fires) and the surfaced error is the page-2 write error.

### 21. Dump: the close-file error is untested *and untestable as the fake stands*
`internal/goety/goety.go:110–113` has a real close-error path:

```go
if cerr := w.Close(); cerr != nil && err == nil {
    err = fmt.Errorf("close dump file: %w", cerr)
}
```

But the write-side fake hardcodes:

```go
func (m *mockWriteCloser) Close() error { return nil }   // goety_test.go:162
```

So on the write side `cerr` is always `nil` and the `"close dump file: %w"` branch can **never fire** in any test today. Fix: extend `mockWriteCloser` with a `closeErr` field (mirroring `mockReadCloser`'s `closeErr` at line 188), then add a test that sets it and asserts the `"close dump file:"` prefix. Turns a dead-from-the-tests branch into a pinned one.

### 22. Seed: empty array `[]` → no-op, no error (AGENTS.md "empty input")
No seed test feeds an empty array. Per `goety.go` Seed: `dec.Token()` reads `[`; the `delim != '['` check passes; `for dec.More()` is immediately false (no elements); the loop body never runs; `len(batch) == 0` so the trailing-flush at line 245 is skipped; `itemsLoaded == 0`; `return nil`. So an empty file → no error, 0 items, no `BatchWriteItems` call — currently **untested**. AGENTS.md explicitly lists "empty input" as a case to cover. Add: feed `[]` → assert no error, `callOpen == 1`, `callBatchWrite == 0`. Cheap, pins the empty-input no-op.

### 23. Seed: valid JSON that is *not* an array → "expected JSON array"
`internal/goety/goety.go:197–205`. Any valid-JSON-non-array input (object `{"a":1}`, scalar `"hello"`/`42`/`true`, `null`) reaches the `!ok || delim != '['` branch → `"expected JSON array, got ..."`. The existing "decode fails" test (line 510) feeds `not json` (malformed), hitting the `Token`/`Decode` error — **not** this type-check branch. So the shape-validation is untested. Add: feed a valid JSON object → assert the `"expected JSON array"` prefix.

### 24. Seed: close-file error — simulatable but untested
`internal/goety/goety.go:187–193`. Seed's deferred close:

```go
defer func() {
    if cerr := r.Close(); cerr != nil {
        if err == nil {
            err = fmt.Errorf("close seed file: %w", cerr)
        }
    }
}()
```

`mockReadCloser` (`goety_test.go:186–191`) **has** a `closeErr` field and `Close()` returns it. So the close-failure is **simulatable** with the existing fake — but no seed test sets it. Add: one test where `openFunc` returns a `mockReadCloser` whose buffer decodes fine but with `closeErr: errors.New("close error")` → assert the returned error has the `"close seed file:"` prefix and wraps the seeded error. Pins the deferred-close error capture (named-return + defer interplay — the same shape as `Dump`'s close).

### 25. Purge: assert the *actual key set* passed to `BatchDeleteItems`
`internal/goety/goety_test.go` purge tests. Every purge test asserts `callBatchDelete == 1` (a count) but none inspects **what** was passed. `Purge` projects only `pk`+`sk` (`AttributesToGet`, line 57) and deletes those — a keys-only delete (correct, but implicit). Modify the `DynamoClientMock`'s `BatchDeleteItemsFunc` to capture `keys`, and assert it is exactly the projected pk/sk pair. Pins "we delete by the projected keys only, not the full item."

### 26. Purge: multi-page non-dry purge (paginate past the first page)
All current purge tests use one page. A non-dry purge with ≥2 non-empty pages (the mock's `ScanFunc` returns a `LastEvaluatedKey` on the first page, then empties) → assert `callBatchDelete == 2` and that the **combined** key set matches the union. Exercises pagination at the service level (the iterator itself is covered in `dynamodb_test.go`, but the *service walking past the first page in non-dry mode* is not).

Side benefit: with a multi-page dry-run purge, you'd observe the first-page-only quirk from §6 — a single extra assertion ("dry-run preview count == 1 page worth, not the total") makes the current asymmetry explicit and gives a future fix a regression anchor.

### 27. Seed: mid-decode failure still closes the file (deferred-close on error)
`goety.go` Seed wraps close in a `defer`. No test today drives a **decode** error (line 510's `not json` actually trips the *array* check before the loop). Craft a buffer where the array opens, one item decodes, then the second hits invalid JSON — `dec.Decode` errors mid-loop → return → deferred close runs → assert no spurious close-error is introduced (close succeeds) and the error is the decode one. This also implicitly exercises the `err == nil` guard in the close path (since `err` is already non-nil from the decode, a hypothetical close failure wouldn't overwrite it).

---

## What to keep (the parts that are genuinely good)

Balance the critique — these are doing the right thing and shouldn't be touched in a simplification pass:

1. **`ScanAll` (`iter.Seq2`)** — the single most idiomatic piece of code in the repo. Copy input, paginate via `ExclusiveStartKey`, yield `(out, err)`, stop on `LastEvaluatedKey == nil` or a caller `break`. Exactly how a modern Go library exposes lazy paginated iteration. Keep.
2. **Functional options for `Dump`** — `WithAttrs`, `WithLimit`, `WithFilter*`, `WithRawOutput`, folded via `WithDumpOptions`. Clean, discoverable, extensible. All 6 `DumpOptions` fields have a `With*`. (User confirmed: keep — it's a fundamental design principle here.)
3. **Strict mock discipline** — only the SDK boundary (`DynamoClient`) and the file I/O seam (`FileReadWriter`) are mocked; `Service`, the logger, the spinner, and the DynamoDB `Client` use real implementations. Matches the "iron rules" exactly. `mocks_test.go` files are properly moq-generated from `//go:generate` directives.
4. **`slog` setup** — `TextHandler` to stderr, `LevelInfo` by default, `LevelDebug` + `AddSource` in verbose, `.With(app=goety)` context. Tidy, low-ceremony, correct.
5. **Error wrapping** — consistent `fmt.Errorf("...: %w", err)` throughout the service and adapter (`"scan items for purge: %w"`, `"write opening bracket to dump file: %w"`, …). Grep-friendly, preserves the chain.
6. **The spinner** — a self-contained `io.Writer`-based animation: mutex-protected latest-message, ticker-driven draw loop, `stopOnce` idempotent stop, `finished.WaitGroup` for clean shutdown. Well-tested, including the "stop when never started" edge.
7. **The odize test style** — `NewGroup` + `BeforeEach`/`AfterEach` + chained `.Test(...)` + `.Run()`, counter-reset in `AfterEach`, `odize.Assert*` consistently. Clean and declarative.
8. **`ScanAll`'s doc comment** — documents the non-mutation contract, the yield-once error, the break-to-stop behavior, and shows usage. The documentation standard the rest should meet.

---

## Recommended build order (one-glance checklist)

> **How to read this.** Every item has exactly one number — a body section, §1–§27 — assigned in execution order by tier. Work down the list top to bottom: **P0 → P1 → P2 → Comm → P3**, and within a tier, in listed order. So "what is one?" = §1, the first item in the list, a P0 correctness fix (`batchWriteItem`). "In what order?" = the tier order above, then items top to bottom within each tier. Each line below is a body item collapsed to one sentence; follow the § to the full analysis and code evidence.

**P0 — correctness** *(do first)*
- §1. `batchWriteItem` returns pre-retry `output` on `ctx.Done()` → stale `UnprocessedItems`. (`dynamodb.go:106–107`) *Latent today — all service callers discard the returned output (`if _, err := ...`); demote to P2 if you want strict observable-impact tiers, but still add a regression test.*
- §2. `openDumpFile` drops the write error when close also fails. (`goety.go:272–279`) *Error-attribution, not data-loss — the dump fails either way; this is which error the user sees.*

**P1 — design & top simplifications**
- §3. Extract a `dumpWriter` type from `Dump` — collapses `first`, `closed`, the deferred-close closure (writes to the named `err`), the manual `]`, and the duplicate bracket write. *Highest-leverage change.*
- §4. DRY `extractAttrValue` / `convertAVValue` — duplicated M/L recursion (10 cases each); factor a shared M/L helper; add a `default` for unknown types. *Recommended route; the table-driven alternative is riskier for marginal gain.*
- §5. Drop the `AVer` marker interface (nothing calls `IsAV`) and the `transformDumpOutput` `Marshal → Unmarshal` round-trip. *Verify `--raw-output` consumers before touching — the wire shape `{"S":...}` may be a downstream contract.*
- §6. Reconcile `dry run` semantics — Purge previews first page only; Dump/Seed preview all. Make consistent or document deliberately. *At minimum: document (cheap, non-breaking). Full reconciliation = behavior change → product sign-off.*
- §7. Co-locate the `Service` type with its methods (split across `service.go` and `goety.go`). *Low priority — layout, not behavior. Bundle with the `dumpWriter` extraction (touches the same file) or skip.*
- §8. Name `defaultBatchSize` for what it means and expose the scan-page == batch-max coupling. *The one-line comment capturing the invariant is 90% of the value; the rename is optional polish.*

**P2 — cheap hygiene**
- §9. Fix three copy-paste doc comments naming the wrong function/command.
- §10. Rename `internal/goety/writer.go` → `file.go`; rename the `OSFile` receiver `w` → `f`.
- §11. Add `var _ FileReadWriter = (*OSFile)(nil)` next to the `DynamoClient` assertion.
- §12. Remove the stray `fmt.Println("creating file", filename)` in the dump test.
- §13. `cmd/local/main.go`: unreachable `return` after `os.Exit`, "seeing"→"seeding" typo, "seed complete" logged on error.
- §14. `JSONStringify`: fall back to `fmt.Sprint(v)` on error instead of `""`.

**Comm — progress vs. debug (UX)**
- §15. The verbose flag inverts user feedback — verbose removes the spinner (no progress) and adds per-item debug + `file:line`. Keep progress visible in both modes.
- §16. Deroute per-item adapter debug to per-batch summaries.
- §17. De-duplicate the `Dump` completion signal (progress + log fire the same event).
- §18. Hoist `Debug("dry run enabled")` out of the loops; reword for effect.

**P3 — test coverage**
- §19. Dump: multi-item (≥2) **and** multi-page (≥2 non-empty) test — pins the comma between items and across pages (`first`'s cross-page persistence is never exercised today).
- §20. Dump: isolate a **mid-dump write** failure (page 1 ok, page 2 errors) — distinct from the existing open-phase and scan failures; assert the `]` is appended by the deferred close.
- §21. Dump: extend `mockWriteCloser` with `closeErr` (mirror `mockReadCloser`), then assert the `"close dump file:"` path (currently dead-from-tests because `Close()` is hardcoded `nil`).
- §22. Seed: empty `[]` → no-op, no error (pins the AGENTS.md "empty input" case).
- §23. Seed: valid JSON that is *not* an array → "expected JSON array".
- §24. Seed: close-file error — simulatable but untested; assert "close seed file:"
- §25. Purge: assert the *actual key set* passed to `BatchDeleteItems` (pins keys-only projection → delete).
- §26. Purge: multi-page non-dry purge (walk past the first page) → assert combined key set + call count.
- §27. Seed: mid-decode failure still closes the file (deferred close); surfaced error is the decode one.

---

## Execution notes

- Each P1 item is a self-contained vertical slice (test + change + doc note). Prefer landing §3 (the `dumpWriter`) first — it unblocks the cleanest home for the new dump coverage items (§19–§21) because the tests then assert on the type's behavior rather than on the tangled named-return/closure.
- The P0 items are independent of the P1 refactor; either order is fine, but land P0 first if you want a quick confidence win before the larger structural change.
- When touching `dynamodb.go` for the `batchWriteItem` fix, also consider adding a test in `dynamodb_test.go` that drives an unprocessed-retry (the mock `ddbClient` can return non-empty `UnprocessedItems` on the first call, empty on the second) and assert the returned `UnprocessedItems` is empty — that's the direct regression test for P0 §1.
- The `AVer` removal (§5) is the riskiest of the "keep-it-idiomatic" items because the wire shape (`{"pk":{"S":"pk"}}`) is user-visible via `--raw-output`. If you drop the interface, keep the struct field names exactly (`S`, `N`, `B`, `BOOL`, `NULL`, `M`, `L`, `SS`, `NS`, `BS`) so `json.Marshal`'s output is unchanged. A `dump --raw-output` e2e assertion (there's already a `test-dump` target) would pin this.
- No new dependencies introduced by any of the above. The `errors.Join` use in §2/§14 requires Go 1.20+, and `go.mod` declares 1.27.3, so it's fine.
- If you land the `dumpWriter` extraction, the `writeDumpItems` and `transformDumpOutput` helpers become private methods or standalone functions on the new type — the `first *bool` parameter disappears entirely (it's a field now), which is the single clearest "before/after" readability win in this whole review.

## Follow-up questions (open, not blocking)

- Is the `--raw-output` wire shape part of any downstream contract (i.e., do consumers parse the dumped JSON and rely on the `{"S": ...}` shape)? If yes, treat it as semi-public and be conservative in §5.
- Should the `dry run` inconsistency (§6) be resolved by making **all** commands preview-all (more intuitive) or by documenting **Purge's** first-page-only as a deliberate "peek" mode (less change, more docs)? I lean toward "all preview-all" for symmetry, but it's a product call.
- The `AVer` structs duplicate the AWS SDK's own `types.AttributeValue` hierarchy. If the SDK ever adds a member type, you'd need to mirror it here. Is there appetite for a small codegen or a build-time assertion (e.g., a test that the set of handled `AttributeValueMember*` types matches the SDK's known set) to catch drift? Low priority, but worth a line in the tooling plan if it comes up.

## What I did *not* change (and why)

To be explicit about scope: this review proposes changes but does **not** itself rewrite the code. The following are deliberately left alone even where I have opinions, because the "cost of touching" outweighs the "gain":

- The `Service` **value receiver** choice (all methods are `(s Service)`): it's correct given there's no mutation during the method bodies. Pointer receivers would be a style preference, not a bug. Don't churn.
- The `DumpOption = func(*DumpOptions) *DumpOptions` **returning-self** idiom: some teams prefer `func(*DumpOptions)` (void) since the mutation is in-place. But all 6 `With*` functions follow the same pattern, and changing them is pure churn with no behavior delta. If you do a sweeping style pass someday, sure — but not as part of the simplification work here.
- The `go.mod` `go 1.27.3` directive: AGENTS.md explicitly says "do not downgrade." Whether 1.27.3 is the *ideal* is a separate discussion. Leave it.
- The Makefile layout (delegated to `scripts/*.mk`): it's a reasonable split. The "missing commands" list in `docs/tooling-upgrade-plan.md` already captures the gaps. Don't re-organize the Makefile as part of this review.
- The `docker-compose.yaml` / local DynamoDB setup: it's an e2e concern, orthogonal to the code-quality work. Only touch it if a test change forces it.

## Out of scope (nice-to-have, not now)

- Migrating from `spf13/cobra` to `urfave/cli` (or vice versa): the cobra setup is fine; switching is a project-wide preference call, not a simplification.
- Adding a `--json` global flag to make *all* output machine-parseable (not just `dump --raw-output`): a feature request, not a cleanup.
- Introducing a `context`-cancelation-aware progress spinner (so `Ctrl-C` cleanly stops the animation mid-operation): the current spinner *does* stop cleanly on `Stop()`, but there's no automatic `ctx.Done()` hook. Nice polish, not a bug.
- Refactoring the `cmd/local` harness into a proper library (so the e2e seed logic is reusable/testable beyond the Makefile target): it's a 100-line throwaway harness; over-investing in it is its own anti-pattern.

---

## Appendix — file map (what I reviewed, for the reader)

| Package | Files reviewed | Role |
| --- | --- | --- |
| `goety` (root) | `main.go`, `go.mod` | CLI entry, module def |
| `internal/commands` | `commands.go`, `bootstrap.go`, `cmd_purge.go`, `cmd_dump.go`, `cmd_seed.go`, `cmd_dump_test.go` | Cobra wiring, dep bootstrap, flag parsing |
| `internal/goety` | `goety.go`, `service.go`, `options.go`, `writer.go`, `interfaces.go`, `mocks_test.go`, `goety_test.go` | Service domain (Purge/Dump/Seed), functional options, file I/O seam, generated mocks, service tests |
| `internal/dynamodb` | `dynamodb.go`, `iterator.go`, `utils.go`, `types.go`, `interfaces.go` | SDK adapter, `ScanAll` paginator, attribute-value transforms, `AVer` types |
| `internal/logging` | `logging.go`, `constants.go` | `slog` factory, level gating, `AddSource` policy |
| `internal/spinner` | `spinner.go`, `sprites.go`, `spinner_test.go` | `io.Writer`-based animated progress, braille sprite |
| `cmd/local` | `main.go` | e2e table-create + seed harness (1000 items) |
| `docs` | `tooling-upgrade-plan.md` (existing), `plan.md` (this doc) | Missing-commands log, this refactor roadmap |
| root config | `Makefile`, `README.md`, `AGENTS.md`, `.golangci.yaml` | Build/test targets, user docs, agent rules, lint config |

That's the whole codebase; no material file was left unread.
