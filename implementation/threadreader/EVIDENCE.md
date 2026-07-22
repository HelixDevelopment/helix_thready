# ThreadReader — Build Evidence (physical, no bluff)

This file records the **real** captured output of building, vetting and testing the
`digital.vasic.threadreader` module. Nothing here is hand-edited output.

## Environment

```
$ go version
go version go1.26.4-X:nodwarf5 linux/amd64
```

- Module path: `digital.vasic.threadreader`
- `go.mod`: `go 1.26`
- Dependencies: **stdlib only** (`context`, `errors`, `fmt`, `sort`, `unicode`; tests add `reflect`, `testing`). No third-party modules, no `require` block.

## Command

```
cd implementation/threadreader && go build ./... && go vet ./... && go test ./... -v -race -count=1
```

## Result — build & vet

```
$ go build ./...
BUILD_OK
$ go vet ./...
VET_OK
```

Both exited 0 with no diagnostics.

## Result — test (go test ./... -v -race -count=1)

```
=== RUN   TestAssemble_ExcludesSystemReplies
--- PASS: TestAssemble_ExcludesSystemReplies (0.00s)
=== RUN   TestAssemble_ChronologicalOrderFromShuffledInput
--- PASS: TestAssemble_ChronologicalOrderFromShuffledInput (0.00s)
=== RUN   TestAssemble_DeterministicAcrossPermutations
--- PASS: TestAssemble_DeterministicAcrossPermutations (0.00s)
=== RUN   TestThread_HashtagsUnionAcrossChain
--- PASS: TestThread_HashtagsUnionAcrossChain (0.00s)
=== RUN   TestAssemble_ForwardedMessagePreserved
--- PASS: TestAssemble_ForwardedMessagePreserved (0.00s)
=== RUN   TestAssemble_MissingRootIsDeterministicError
--- PASS: TestAssemble_MissingRootIsDeterministicError (0.00s)
=== RUN   TestAssemble_DuplicatesDeduped
--- PASS: TestAssemble_DuplicatesDeduped (0.00s)
=== RUN   TestAssemble_DuplicateRootDeduped
--- PASS: TestAssemble_DuplicateRootDeduped (0.00s)
=== RUN   TestExtractHashtags_EdgeCases
=== RUN   TestExtractHashtags_EdgeCases/empty
=== RUN   TestExtractHashtags_EdgeCases/no_tags
=== RUN   TestExtractHashtags_EdgeCases/single
=== RUN   TestExtractHashtags_EdgeCases/multiple
=== RUN   TestExtractHashtags_EdgeCases/case_preserved
=== RUN   TestExtractHashtags_EdgeCases/case-sensitive_dedup_keeps_distinct
=== RUN   TestExtractHashtags_EdgeCases/dedup_identical
=== RUN   TestExtractHashtags_EdgeCases/adjacent_trailing_punctuation
=== RUN   TestExtractHashtags_EdgeCases/leading_and_wrapped
=== RUN   TestExtractHashtags_EdgeCases/digits_and_underscores
=== RUN   TestExtractHashtags_EdgeCases/unicode_cyrillic
=== RUN   TestExtractHashtags_EdgeCases/unicode_cjk_and_accents
=== RUN   TestExtractHashtags_EdgeCases/lone_hash
=== RUN   TestExtractHashtags_EdgeCases/not-a-tag_mid-token
=== RUN   TestExtractHashtags_EdgeCases/hash_glued_after_word
=== RUN   TestExtractHashtags_EdgeCases/newlines_separate
--- PASS: TestExtractHashtags_EdgeCases (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/empty (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/no_tags (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/single (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/multiple (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/case_preserved (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/case-sensitive_dedup_keeps_distinct (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/dedup_identical (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/adjacent_trailing_punctuation (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/leading_and_wrapped (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/digits_and_underscores (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/unicode_cyrillic (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/unicode_cjk_and_accents (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/lone_hash (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/not-a-tag_mid-token (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/hash_glued_after_word (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/newlines_separate (0.00s)
=== RUN   TestRead_SourceErrorPropagates
--- PASS: TestRead_SourceErrorPropagates (0.00s)
PASS
ok  	digital.vasic.threadreader	1.014s
```

## Pass/fail summary

| Metric | Value |
|--------|-------|
| Top-level test functions | 10 |
| Sub-tests (`ExtractHashtags` table) | 16 |
| Total `=== RUN` | 26 |
| Total `--- PASS` | 26 |
| Failures | 0 |
| Race detector (`-race`) | enabled — no data races reported |
| Package result | `ok  digital.vasic.threadreader` |

### Coverage of the 7 mandated TDD cases

1. **Organic-only / system excluded** → `TestAssemble_ExcludesSystemReplies` (both `thready-bot` replies dropped; a *different* bot kept as organic).
2. **Chronological order from shuffled input** → `TestAssemble_ChronologicalOrderFromShuffledInput` + `TestAssemble_DeterministicAcrossPermutations`.
3. **Hashtags from root AND replies** → `TestThread_HashtagsUnionAcrossChain` (link-only root; all tags come from replies; `#Video` deduped).
4. **Forwarded content preserved** → `TestAssemble_ForwardedMessagePreserved` (flag, text, attachment sha256 intact).
5. **Missing root → deterministic error** → `TestAssemble_MissingRootIsDeterministicError` (`ErrMissingRoot`, plus `ErrNoPosts` for empty).
6. **Duplicates deduped** → `TestAssemble_DuplicatesDeduped` + `TestAssemble_DuplicateRootDeduped`.
7. **`ExtractHashtags` edge cases** → `TestExtractHashtags_EdgeCases` (16 subtests: none/multiple/adjacent punctuation/unicode/underscore-digit/word-boundary/lone `#`).

## Honest verdict

**READY.** `go build`, `go vet` and `go test -race` all pass on Go 1.26 with zero
failures and zero race reports, stdlib only. No test was deleted or weakened to go
green, and no output above was fabricated — it is the verbatim tool output.

### Honest scope boundary (anti-bluff)

This module is the **pure, messenger-agnostic ThreadReader core** ([GAP: 5.1.3]).
It does **not** include live messenger adapters — the Telegram gotd/td MTProto
reader ([GAP: 5.1.1]) and the Max OneMe port ([GAP: 5.1.2]) remain BUILD-NEW /
`[OPEN: ING-1]` per `messenger-ingestion.md`. Those bind a real `MessageSource` to
this core; the in-repo `MessageSource` here is an in-memory fake used to prove the
assembly/filter/hashtag logic. That logic is real, compiling and test-green.

---

## Fix pass — deterministic multi-root tie-break (`rootLess`) coverage

**Reviewer finding (Important):** the deterministic multi-root tie-break
`rootLess` (`assembler.go:127`) had **0% coverage** because no fixture ever
produced more than one root candidate — the single-root fixtures short-circuit on
the `rootIdx == -1` branch and never call `rootLess`.

**Change:** added one test, `TestAssemble_MultiRootTieBreak`, that seeds MULTIPLE
root candidates (posts with empty/self-referential `ParentID`) and asserts the
concrete winning `Thread.Root.ID`:

- **Case A — different timestamps:** the earlier-timestamp root wins *even though it
  carries the lexicographically larger ID* (`root-zzz`@1000 beats `root-aaa`@2000),
  proving timestamp dominates the ID tie-break.
- **Case B — equal timestamps:** the lexicographically smaller ID wins
  (`root-a` beats self-referential `root-b`, both @5000), exercising the documented
  tie-break and the `ParentID == ID` self-root path.
- **Determinism:** each case runs Assemble over every rotation + the reversal of the
  input and asserts (a) `Root.ID` equals the expected value in every permutation and
  (b) the full assembled `Thread` is `reflect.DeepEqual` across permutations — shuffle
  order never changes the result.

**Production code:** UNCHANGED. The test exposed **no defect** in `rootLess` or root
selection; the existing logic already yields the spec-mandated deterministic winner.
No test was deleted, skipped or weakened.

### Command (verbatim)

```
$ go version
go version go1.26.4-X:nodwarf5 linux/amd64
$ cd implementation/threadreader && go build ./... && go vet ./... && gofmt -l . && go test ./... -v -race -count=1
```

### Result — build, vet, gofmt

```
$ go build ./...        # exit 0, no output
$ go vet ./...          # exit 0, no output
$ gofmt -l .            # empty output = all files already formatted
```

### Result — test (new + regression, `-v -race -count=1`)

```
=== RUN   TestAssemble_ExcludesSystemReplies
--- PASS: TestAssemble_ExcludesSystemReplies (0.00s)
=== RUN   TestAssemble_ChronologicalOrderFromShuffledInput
--- PASS: TestAssemble_ChronologicalOrderFromShuffledInput (0.00s)
=== RUN   TestAssemble_DeterministicAcrossPermutations
--- PASS: TestAssemble_DeterministicAcrossPermutations (0.00s)
=== RUN   TestThread_HashtagsUnionAcrossChain
--- PASS: TestThread_HashtagsUnionAcrossChain (0.00s)
=== RUN   TestAssemble_ForwardedMessagePreserved
--- PASS: TestAssemble_ForwardedMessagePreserved (0.00s)
=== RUN   TestAssemble_MissingRootIsDeterministicError
--- PASS: TestAssemble_MissingRootIsDeterministicError (0.00s)
=== RUN   TestAssemble_DuplicatesDeduped
--- PASS: TestAssemble_DuplicatesDeduped (0.00s)
=== RUN   TestAssemble_DuplicateRootDeduped
--- PASS: TestAssemble_DuplicateRootDeduped (0.00s)
=== RUN   TestAssemble_MultiRootTieBreak
=== RUN   TestAssemble_MultiRootTieBreak/earliest_timestamp_wins_over_smaller_ID
=== RUN   TestAssemble_MultiRootTieBreak/equal_timestamp:_lexicographically_smaller_ID_wins
--- PASS: TestAssemble_MultiRootTieBreak (0.00s)
    --- PASS: TestAssemble_MultiRootTieBreak/earliest_timestamp_wins_over_smaller_ID (0.00s)
    --- PASS: TestAssemble_MultiRootTieBreak/equal_timestamp:_lexicographically_smaller_ID_wins (0.00s)
=== RUN   TestExtractHashtags_EdgeCases
=== RUN   TestExtractHashtags_EdgeCases/empty
=== RUN   TestExtractHashtags_EdgeCases/no_tags
=== RUN   TestExtractHashtags_EdgeCases/single
=== RUN   TestExtractHashtags_EdgeCases/multiple
=== RUN   TestExtractHashtags_EdgeCases/case_preserved
=== RUN   TestExtractHashtags_EdgeCases/case-sensitive_dedup_keeps_distinct
=== RUN   TestExtractHashtags_EdgeCases/dedup_identical
=== RUN   TestExtractHashtags_EdgeCases/adjacent_trailing_punctuation
=== RUN   TestExtractHashtags_EdgeCases/leading_and_wrapped
=== RUN   TestExtractHashtags_EdgeCases/digits_and_underscores
=== RUN   TestExtractHashtags_EdgeCases/unicode_cyrillic
=== RUN   TestExtractHashtags_EdgeCases/unicode_cjk_and_accents
=== RUN   TestExtractHashtags_EdgeCases/lone_hash
=== RUN   TestExtractHashtags_EdgeCases/not-a-tag_mid-token
=== RUN   TestExtractHashtags_EdgeCases/hash_glued_after_word
=== RUN   TestExtractHashtags_EdgeCases/newlines_separate
--- PASS: TestExtractHashtags_EdgeCases (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/empty (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/no_tags (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/single (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/multiple (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/case_preserved (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/case-sensitive_dedup_keeps_distinct (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/dedup_identical (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/adjacent_trailing_punctuation (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/leading_and_wrapped (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/digits_and_underscores (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/unicode_cyrillic (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/unicode_cjk_and_accents (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/lone_hash (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/not-a-tag_mid-token (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/hash_glued_after_word (0.00s)
    --- PASS: TestExtractHashtags_EdgeCases/newlines_separate (0.00s)
=== RUN   TestRead_SourceErrorPropagates
--- PASS: TestRead_SourceErrorPropagates (0.00s)
PASS
ok  	digital.vasic.threadreader	1.014s
```

### Pass/fail summary (fix pass)

| Metric | Before | After |
|--------|--------|-------|
| Top-level test functions | 10 | **11** |
| Sub-tests | 16 | **18** (16 hashtags + 2 tie-break) |
| Total `--- PASS` | 26 | **29** |
| Failures | 0 | **0** |
| Race detector (`-race`) | clean | **clean** |

### Per-function coverage delta (`go tool cover -func`)

```
digital.vasic.threadreader/assembler.go:121:  isRootPost   100.0%
digital.vasic.threadreader/assembler.go:127:  rootLess     100.0%
digital.vasic.threadreader/assembler.go:65:   Assemble      96.6%
total:                                        (statements)  95.1%
```

| Function | Coverage before | Coverage after |
|----------|-----------------|----------------|
| `rootLess` (`assembler.go:127`) | **0.0%** | **100.0%** |

### Verdict

**READY.** The reviewer's single Important finding is resolved: `rootLess` goes from
0% to 100% covered by a test with concrete `Root.ID` assertions across all input
permutations. `go build`, `go vet`, `gofmt -l` and `go test -race` are all clean.
No production logic changed (no bug found), and no test was deleted, skipped or
weakened to go green.
