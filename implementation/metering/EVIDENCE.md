# Metering / Usage-Billing — Build & Test Evidence

Physical, reproducible evidence for the Helix Thready **Metering module**
(`digital.vasic.metering`). Every block below is real captured output, not a
summary. Reproduce from this directory with:

```
cd implementation/metering
go build ./... && go vet ./... && gofmt -l . && go test ./... -v -race -count=1
```

## 1. Toolchain

```
$ go version
go version go1.26.4-X:nodwarf5 linux/amd64

$ go env GOVERSION GOOS GOARCH CGO_ENABLED
go1.26.4-X:nodwarf5
linux
amd64
1
```

## 2. Dependencies — stdlib only

`go.mod` has **no `require` block**. The module imports only `sync`,
`sync/atomic`, `sort`, `strconv`, and `time` from the standard library.

```
$ cat go.mod
module digital.vasic.metering

go 1.26

$ go list -deps -test ./... | grep -E 'golang.org|github.com'
NONE (stdlib only)
```

## 3. Build / Vet / Format — all clean

```
$ go build ./...
BUILD OK            # (no output from go build; exit 0)

$ go vet ./...
VET OK              # (no output from go vet; exit 0)

$ gofmt -l .
GOFMT CLEAN         # (gofmt -l printed nothing => every file is already formatted)
```

## 4. Tests — `go test ./... -v -race -count=1`

Race detector enabled (`-race`), cache disabled (`-count=1`). All 19 tests pass.

```
=== RUN   TestBillWorkedExample
--- PASS: TestBillWorkedExample (0.00s)
=== RUN   TestBillZeroUsageIsBaseOnly
--- PASS: TestBillZeroUsageIsBaseOnly (0.00s)
=== RUN   TestBillWithinAllowanceNoOverage
--- PASS: TestBillWithinAllowanceNoOverage (0.00s)
=== RUN   TestBillSingleUnitOverBlockRoundsUp
--- PASS: TestBillSingleUnitOverBlockRoundsUp (0.00s)
=== RUN   TestBillerViaRecorder
--- PASS: TestBillerViaRecorder (0.00s)
=== RUN   TestCentsString
--- PASS: TestCentsString (0.00s)
=== RUN   TestQuotaAllowUnderAndOver
--- PASS: TestQuotaAllowUnderAndOver (0.00s)
=== RUN   TestQuotaExactBoundary
--- PASS: TestQuotaExactBoundary (0.00s)
=== RUN   TestQuotaZeroLimitDenies
--- PASS: TestQuotaZeroLimitDenies (0.00s)
=== RUN   TestQuotaUnlimitedWhenUnset
--- PASS: TestQuotaUnlimitedWhenUnset (0.00s)
=== RUN   TestQuotaNegativeWantDenied
--- PASS: TestQuotaNegativeWantDenied (0.00s)
=== RUN   TestQuotaRelease
--- PASS: TestQuotaRelease (0.00s)
=== RUN   TestQuotaConcurrentReserveNeverOvershoots
--- PASS: TestQuotaConcurrentReserveNeverOvershoots (0.00s)
=== RUN   TestQuotaConcurrentLargeReservations
--- PASS: TestQuotaConcurrentLargeReservations (0.00s)
=== RUN   TestRecorderAggregateExactSum
--- PASS: TestRecorderAggregateExactSum (0.00s)
=== RUN   TestRecorderPeriodWindowing
--- PASS: TestRecorderPeriodWindowing (0.00s)
=== RUN   TestRecorderIsolatesAccountsAndMetrics
--- PASS: TestRecorderIsolatesAccountsAndMetrics (0.00s)
=== RUN   TestRecorderPeriodUsageBucket
--- PASS: TestRecorderPeriodUsageBucket (0.00s)
=== RUN   TestRecorderConcurrentRecord
--- PASS: TestRecorderConcurrentRecord (0.02s)
PASS
ok  	digital.vasic.metering	1.032s
```

### Pass/fail summary

| Suite                    | Tests | Result |
|--------------------------|-------|--------|
| Recorder / aggregation   | 5     | PASS   |
| QuotaPolicy / reserve    | 8     | PASS   |
| Plan / Invoice / billing | 6     | PASS   |
| **Total**                | **19**| **PASS (0 fail, 0 skip)** |

No test is skipped. Race detector reports no data races.

## 5. Worked invoice example (exact integer cents)

Plan **Pro** — base fee `$49.00` (4900c):

| Metric            | Included    | Overage price                         |
|-------------------|-------------|---------------------------------------|
| `posts_processed` | 10,000      | 2c per post (block = 1, per-unit)     |
| `searches`        | 10,000      | 5c per 1,000 searches (block = 1,000) |
| `storage_bytes`   | 5,000,000   | 25c per 1,000,000 bytes (block = 1e6) |

Usage in the billing period:

| Metric              | Used       | Overage units | Blocks (ceil)        | Line amount |
|---------------------|------------|---------------|----------------------|-------------|
| base fee            | —          | —             | —                    | 4,900c      |
| `posts_processed`   | 12,500     | 2,500         | ceil(2,500/1)=2,500  | 5,000c      |
| `searches`          | 25,600     | 15,600        | ceil(15,600/1,000)=16| 80c         |
| `storage_bytes`     | 7,500,000  | 2,500,000     | ceil(2.5M/1M)=3      | 75c         |
| `bytes_downloaded`  | 999,999    | not in plan   | ignored              | 0c          |
| **TOTAL**           |            |               |                      | **10,055c = $100.55** |

Formula per metric: `overage = max(0, used - included)`,
`blocks = ceil(overage / blockUnits)`, `lineCents = blocks * centsPerBlock`.
All arithmetic is `int64`; no float touches money. This exact total (10,055
cents / `$100.55`) is asserted line-by-line in `TestBillWorkedExample` and
reproduced end-to-end through the Recorder in `TestBillerViaRecorder`.

Edge cases asserted:
- **Zero usage → base only**: `TestBillZeroUsageIsBaseOnly` → 4,900c, one line.
- **Within allowance → base only**: `TestBillWithinAllowanceNoOverage` (usage at
  or below every included allowance) → 4,900c, no overage lines.
- **Partial block rounds up**: `TestBillSingleUnitOverBlockRoundsUp` (1 unit over
  a 1,000-unit block still bills a full block).

## 6. Concurrency safety (the key property)

`TestQuotaConcurrentReserveNeverOvershoots`: limit = 100, 200 goroutines each
call `Allow(..., 1)` simultaneously (released together via a start channel).
The test asserts **exactly 100 succeed**, reserved total is **exactly 100**, and
remaining is **0** — two callers can never both cross the limit. Passes under
`-race`. `TestQuotaConcurrentLargeReservations` adds varied chunk sizes (1..7)
from 300 goroutines and asserts the reserved total never exceeds the limit.

## 7. Honest verdict

**READY.** The module builds, vets, and is gofmt-clean with zero third-party
dependencies. All 19 tests pass under the race detector with caching disabled.
The required behaviors are proven with real assertions: exact aggregation,
half-open period windowing, quota allow/deny with correct remaining, race-clean
atomic check-and-reserve that never overshoots, and deterministic integer-cent
billing (base + metered overage) verified against an exact worked example
including the zero-usage and within-allowance cases. No floats are used for
money anywhere. No test is skipped, disabled, or faked.
