package metering

import "sync"

// accountMetric identifies a per-account, per-metric usage bucket.
type accountMetric struct {
	account string
	metric  string
}

// Recorder stores usage events and aggregates them over time windows. It is
// safe for concurrent use by multiple goroutines. Events are held in per-account
// per-metric buckets so aggregation only scans the relevant slice.
type Recorder struct {
	mu      sync.RWMutex
	buckets map[accountMetric][]UsageEvent
}

// NewRecorder returns an empty, ready-to-use Recorder.
func NewRecorder() *Recorder {
	return &Recorder{buckets: make(map[accountMetric][]UsageEvent)}
}

// Record appends a usage event. It is safe to call concurrently.
func (r *Recorder) Record(e UsageEvent) {
	k := accountMetric{account: e.AccountID, metric: e.Metric}
	r.mu.Lock()
	r.buckets[k] = append(r.buckets[k], e)
	r.mu.Unlock()
}

// RecordUsage is a convenience wrapper around Record that constructs the event.
func (r *Recorder) RecordUsage(accountID, metric string, quantity, tsUnix int64, unit string) {
	r.Record(UsageEvent{
		AccountID:     accountID,
		Metric:        metric,
		Quantity:      quantity,
		TimestampUnix: tsUnix,
		Unit:          unit,
	})
}

// Aggregate returns the summed Quantity of all events for the given account and
// metric whose timestamp falls within period. Events outside the window are
// excluded. It is safe to call concurrently with Record.
func (r *Recorder) Aggregate(accountID, metric string, period Period) int64 {
	k := accountMetric{account: accountID, metric: metric}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var sum int64
	for _, e := range r.buckets[k] {
		if period.Contains(e.TimestampUnix) {
			sum += e.Quantity
		}
	}
	return sum
}

// PeriodUsage returns, for one account, the summed quantity of every metric that
// has events within period. Metrics with no in-window usage are omitted. This is
// the per-account "period bucket" consumed by billing.
func (r *Recorder) PeriodUsage(accountID string, period Period) map[string]int64 {
	out := make(map[string]int64)
	r.mu.RLock()
	defer r.mu.RUnlock()
	for k, events := range r.buckets {
		if k.account != accountID {
			continue
		}
		var sum int64
		for _, e := range events {
			if period.Contains(e.TimestampUnix) {
				sum += e.Quantity
			}
		}
		if sum != 0 {
			out[k.metric] = sum
		}
	}
	return out
}
