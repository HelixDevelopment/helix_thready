# Metering / Usage-Billing Core (`digital.vasic.metering`)

Reusable usage-metering, quota, and billing core for Helix Thready. It backs the
operator's chosen **subscription + metered** billing model (MVP decision matrix,
Q11) for a large-scale multi-tenant account model where usage metering, quotas,
and billing must exist from day one.

- **Language / toolchain:** Go 1.26, **standard library only** (no third-party
  dependencies).
- **Money:** integer minor units (`Cents`, `int64`). No floating point is ever
  used for money — billing is exact and deterministic.
- **Concurrency:** `Recorder` and `QuotaPolicy` are safe for concurrent use;
  quota reservation is an atomic check-and-reserve.

## Concepts

### UsageEvent
A single unit of recorded activity:

```go
type UsageEvent struct {
    AccountID     string // tenant / account
    Metric        string // e.g. posts_processed, bytes_downloaded, searches
    Quantity      int64  // count in Unit
    TimestampUnix int64  // Unix seconds (UTC); drives period windowing
    Unit          string // e.g. "post", "byte", "search"
}
```

Metric name constants are provided: `MetricPostsProcessed`,
`MetricBytesDownloaded`, `MetricStorageBytes`, `MetricAssetsStored`,
`MetricSearches`.

### Recorder — record & aggregate
Thread-safe. Stores events in per-account, per-metric buckets and aggregates
over a half-open time window `Period{Start, End}` (`[Start, End)` in Unix
seconds).

```go
rec := metering.NewRecorder()
rec.RecordUsage("acct-1", metering.MetricPostsProcessed, 42, ts, "post")

july := metering.MonthUTC(2026, time.July)
total := rec.Aggregate("acct-1", metering.MetricPostsProcessed, july) // summed quantity in window
usage := rec.PeriodUsage("acct-1", july)                              // map[metric]sum for the period
```

Events outside the window are excluded from `Aggregate` / `PeriodUsage`.

### QuotaPolicy — per-account, per-metric limits
`Allow` performs an **atomic check-and-reserve**: it verifies the requested
amount fits under the limit and reserves it in the same critical section, so two
concurrent callers can never both cross the limit.

```go
q := metering.NewQuotaPolicy()
q.SetLimit("acct-1", metering.MetricSearches, 100)

ok, remaining := q.Allow("acct-1", metering.MetricSearches, 30) // ok=true,  remaining=70
ok, remaining =  q.Allow("acct-1", metering.MetricSearches, 80) // ok=false, remaining=70 (unchanged)
ok, remaining =  q.Allow("acct-1", metering.MetricSearches, 70) // ok=true,  remaining=0
q.Release("acct-1", metering.MetricSearches, 20)                // return reserved headroom
```

- Denied requests reserve nothing; `remaining` reports the headroom that still
  exists.
- A metric with no configured limit is unlimited; `Allow` returns
  `(true, metering.Unlimited)` where `Unlimited == -1`.

### Plan / Biller — invoices
A `Plan` is a subscription tier: a flat base fee plus per-metric metered
overage. `Bill` produces an `Invoice` of `LineItem`s with an exact integer total.

```go
plan := metering.NewPlan("Pro", 4900, // $49.00 base fee, in cents
    metering.MetricRate{Metric: metering.MetricPostsProcessed, IncludedUnits: 10_000, BlockUnits: 1,         CentsPerBlock: 2},  // 2c/post
    metering.MetricRate{Metric: metering.MetricSearches,       IncludedUnits: 10_000, BlockUnits: 1_000,     CentsPerBlock: 5},  // 5c per 1,000
    metering.MetricRate{Metric: metering.MetricStorageBytes,   IncludedUnits: 5_000_000, BlockUnits: 1_000_000, CentsPerBlock: 25}, // 25c per 1,000,000
)

biller := metering.NewBiller(plan, rec)
inv := biller.Bill("acct-1", july) // reads usage from the Recorder for the period
// or, from an explicit usage map without a Recorder:
inv = plan.BillUsage("acct-1", map[string]int64{metering.MetricPostsProcessed: 12_500}, july)
```

## Billing formula

For each metric configured in the plan:

```
overageUnits = max(0, used - includedUnits)
blocks       = ceil(overageUnits / blockUnits)   // blockUnits defaults to 1
lineCents    = blocks * centsPerBlock
```

- `blockUnits == 1` bills **strictly per unit** — this is exactly the canonical
  `max(0, used - included) * rate`.
- `blockUnits > 1` models realistic block pricing (e.g. "5c per 1,000
  searches"); a partial block **rounds up** (you pay per started block).
- The invoice always begins with a base-fee line. Metrics within their allowance
  add no line. Metrics used but not in the plan are ignored. The total is the
  exact `int64` sum of all line amounts.

### Worked example

Base `$49.00`; usage `posts_processed=12,500`, `searches=25,600`,
`storage_bytes=7,500,000` (plus an unmetered `bytes_downloaded`):

| Line              | Overage    | Amount  |
|-------------------|------------|---------|
| base fee          | —          | 4,900c  |
| `posts_processed` | 2,500      | 5,000c  |
| `searches`        | 15,600 → 16 blocks | 80c |
| `storage_bytes`   | 2,500,000 → 3 blocks | 75c |
| **Total**         |            | **10,055c = $100.55** |

This exact total is asserted in `TestBillWorkedExample`.

## Run the tests

```
cd implementation/metering
go build ./... && go vet ./... && gofmt -l . && go test ./... -v -race -count=1
```

19 tests, all passing under the race detector. See `EVIDENCE.md` for the full
captured build/test transcript, toolchain, and verdict.

## Files

| File               | Contents                                             |
|--------------------|------------------------------------------------------|
| `doc.go`           | Package overview and billing formula                 |
| `money.go`         | `Cents` type, formatting, integer `ceilDiv`          |
| `event.go`         | `UsageEvent`, `Period`, metric-name constants        |
| `recorder.go`      | `Recorder`: record, `Aggregate`, `PeriodUsage`       |
| `quota.go`         | `QuotaPolicy`: atomic `Allow`/reserve, limits        |
| `billing.go`       | `MetricRate`, `Plan`, `LineItem`, `Invoice`, `Biller`|
| `*_test.go`        | TDD suites (recorder, quota, billing)                |
| `EVIDENCE.md`      | Captured build/test evidence                         |
```
