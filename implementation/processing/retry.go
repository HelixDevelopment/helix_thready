package processing

import "time"

// RetryPolicy controls per-step retry with exponential backoff. A step is
// attempted up to MaxAttempts times; between attempts the engine waits
// BaseDelay * 2^(attempt-1), capped at MaxDelay. It mirrors
// skill_dispatch.RetryPolicy so the two modules retry identically.
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts (including the first). Values
	// below 1 are treated as 1.
	MaxAttempts int
	// BaseDelay is the wait before the second attempt.
	BaseDelay time.Duration
	// MaxDelay caps the exponential backoff. Zero means no cap.
	MaxDelay time.Duration
}

// DefaultRetryPolicy is a conservative policy for real deployments: 3 attempts,
// 100ms base, capped at 30s. Tests inject a fast policy.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxAttempts: 3, BaseDelay: 100 * time.Millisecond, MaxDelay: 30 * time.Second}
}

// attempts returns the effective attempt ceiling (>= 1).
func (p RetryPolicy) attempts() int {
	if p.MaxAttempts < 1 {
		return 1
	}
	return p.MaxAttempts
}

// Delay returns the backoff duration to wait BEFORE the given 1-based attempt.
// Delay(1) is 0 (no wait before the first attempt). Delay(2) == BaseDelay,
// Delay(3) == 2*BaseDelay, and so on, capped at MaxDelay. It is overflow-safe.
func (p RetryPolicy) Delay(attempt int) time.Duration {
	if attempt <= 1 {
		return 0
	}
	if p.BaseDelay <= 0 {
		return 0
	}
	d := p.BaseDelay
	for i := 1; i < attempt-1; i++ {
		d *= 2
		if d <= 0 { // multiplication overflowed
			if p.MaxDelay > 0 {
				return p.MaxDelay
			}
			return time.Duration(1) << 62
		}
		if p.MaxDelay > 0 && d >= p.MaxDelay {
			return p.MaxDelay
		}
	}
	if p.MaxDelay > 0 && d > p.MaxDelay {
		return p.MaxDelay
	}
	return d
}
