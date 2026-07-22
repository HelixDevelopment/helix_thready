package metering

import "sync"

// Unlimited is the "remaining" value returned by Allow for a metric that has no
// configured limit (usage is permitted without bound).
const Unlimited int64 = -1

// QuotaPolicy enforces per-account, per-metric usage limits. Allow performs an
// atomic check-and-reserve: it verifies the requested amount fits under the
// limit and, in the same critical section, reserves it. Two concurrent callers
// can therefore never both succeed past the limit. It is safe for concurrent
// use by multiple goroutines.
type QuotaPolicy struct {
	mu     sync.Mutex
	limits map[accountMetric]int64
	used   map[accountMetric]int64
}

// NewQuotaPolicy returns an empty QuotaPolicy with no configured limits (every
// metric is Unlimited until SetLimit is called).
func NewQuotaPolicy() *QuotaPolicy {
	return &QuotaPolicy{
		limits: make(map[accountMetric]int64),
		used:   make(map[accountMetric]int64),
	}
}

// SetLimit configures the maximum cumulative reserved quantity for an account
// and metric. A limit of 0 denies all non-zero requests. Setting a limit does
// not change already-reserved usage.
func (q *QuotaPolicy) SetLimit(accountID, metric string, limit int64) {
	k := accountMetric{account: accountID, metric: metric}
	q.mu.Lock()
	q.limits[k] = limit
	q.mu.Unlock()
}

// Allow atomically checks whether reserving want additional units for the given
// account and metric would stay within the configured limit and, if so, reserves
// them. It returns whether the request was allowed and the remaining headroom.
//
//   - If no limit is configured, the request is always allowed and remaining is
//     Unlimited (-1).
//   - If allowed, remaining is limit-used AFTER the reservation.
//   - If denied (used+want would exceed limit), nothing is reserved and
//     remaining is the current limit-used (the headroom that still exists).
//   - A negative want is always denied and reserves nothing.
func (q *QuotaPolicy) Allow(accountID, metric string, want int64) (allowed bool, remaining int64) {
	k := accountMetric{account: accountID, metric: metric}
	q.mu.Lock()
	defer q.mu.Unlock()

	limit, hasLimit := q.limits[k]
	if !hasLimit {
		if want < 0 {
			return false, Unlimited
		}
		q.used[k] += want
		return true, Unlimited
	}

	used := q.used[k]
	if want < 0 || used+want > limit {
		return false, limit - used
	}
	q.used[k] = used + want
	return true, limit - q.used[k]
}

// Used returns the amount currently reserved for an account and metric.
func (q *QuotaPolicy) Used(accountID, metric string) int64 {
	k := accountMetric{account: accountID, metric: metric}
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.used[k]
}

// Remaining returns the headroom (limit-used) for an account and metric, or
// Unlimited if no limit is configured.
func (q *QuotaPolicy) Remaining(accountID, metric string) int64 {
	k := accountMetric{account: accountID, metric: metric}
	q.mu.Lock()
	defer q.mu.Unlock()
	limit, hasLimit := q.limits[k]
	if !hasLimit {
		return Unlimited
	}
	return limit - q.used[k]
}

// Release returns previously reserved quantity to the pool (for example when a
// tentatively reserved operation is cancelled). Used never drops below zero.
func (q *QuotaPolicy) Release(accountID, metric string, amount int64) {
	if amount <= 0 {
		return
	}
	k := accountMetric{account: accountID, metric: metric}
	q.mu.Lock()
	if q.used[k] <= amount {
		q.used[k] = 0
	} else {
		q.used[k] -= amount
	}
	q.mu.Unlock()
}
