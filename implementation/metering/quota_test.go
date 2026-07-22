package metering

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestQuotaAllowUnderAndOver(t *testing.T) {
	q := NewQuotaPolicy()
	q.SetLimit("acct-1", MetricPostsProcessed, 100)

	// Reserve 30 -> allowed, 70 remaining.
	if ok, rem := q.Allow("acct-1", MetricPostsProcessed, 30); !ok || rem != 70 {
		t.Fatalf("Allow(30) = (%v, %d), want (true, 70)", ok, rem)
	}
	// Reserve 80 -> would total 110 > 100 -> denied, headroom stays 70.
	if ok, rem := q.Allow("acct-1", MetricPostsProcessed, 80); ok || rem != 70 {
		t.Fatalf("Allow(80) = (%v, %d), want (false, 70)", ok, rem)
	}
	// Reserve the remaining 70 -> allowed, 0 remaining.
	if ok, rem := q.Allow("acct-1", MetricPostsProcessed, 70); !ok || rem != 0 {
		t.Fatalf("Allow(70) = (%v, %d), want (true, 0)", ok, rem)
	}
	// Any further reservation is denied with 0 remaining.
	if ok, rem := q.Allow("acct-1", MetricPostsProcessed, 1); ok || rem != 0 {
		t.Fatalf("Allow(1) = (%v, %d), want (false, 0)", ok, rem)
	}
	if used := q.Used("acct-1", MetricPostsProcessed); used != 100 {
		t.Fatalf("Used = %d, want 100", used)
	}
}

func TestQuotaExactBoundary(t *testing.T) {
	q := NewQuotaPolicy()
	q.SetLimit("acct-1", MetricSearches, 10)
	// Reserving exactly the limit is allowed.
	if ok, rem := q.Allow("acct-1", MetricSearches, 10); !ok || rem != 0 {
		t.Fatalf("Allow(10) at limit 10 = (%v, %d), want (true, 0)", ok, rem)
	}
	// One more is denied.
	if ok, rem := q.Allow("acct-1", MetricSearches, 1); ok || rem != 0 {
		t.Fatalf("Allow(1) = (%v, %d), want (false, 0)", ok, rem)
	}
}

func TestQuotaZeroLimitDenies(t *testing.T) {
	q := NewQuotaPolicy()
	q.SetLimit("acct-1", MetricAssetsStored, 0)
	if ok, rem := q.Allow("acct-1", MetricAssetsStored, 1); ok || rem != 0 {
		t.Fatalf("Allow(1) under limit 0 = (%v, %d), want (false, 0)", ok, rem)
	}
}

func TestQuotaUnlimitedWhenUnset(t *testing.T) {
	q := NewQuotaPolicy()
	// No SetLimit -> unlimited.
	if ok, rem := q.Allow("acct-1", MetricBytesDownloaded, 1_000_000); !ok || rem != Unlimited {
		t.Fatalf("Allow on unset metric = (%v, %d), want (true, %d)", ok, rem, Unlimited)
	}
	if rem := q.Remaining("acct-1", MetricBytesDownloaded); rem != Unlimited {
		t.Fatalf("Remaining unset = %d, want %d", rem, Unlimited)
	}
}

func TestQuotaNegativeWantDenied(t *testing.T) {
	q := NewQuotaPolicy()
	q.SetLimit("acct-1", MetricSearches, 100)
	if ok, _ := q.Allow("acct-1", MetricSearches, -5); ok {
		t.Fatalf("Allow(-5) = allowed, want denied")
	}
	if used := q.Used("acct-1", MetricSearches); used != 0 {
		t.Fatalf("Used after denied negative = %d, want 0", used)
	}
}

func TestQuotaRelease(t *testing.T) {
	q := NewQuotaPolicy()
	q.SetLimit("acct-1", MetricSearches, 100)
	q.Allow("acct-1", MetricSearches, 80)
	q.Release("acct-1", MetricSearches, 30)
	if used := q.Used("acct-1", MetricSearches); used != 50 {
		t.Fatalf("Used after release = %d, want 50", used)
	}
	if ok, rem := q.Allow("acct-1", MetricSearches, 50); !ok || rem != 0 {
		t.Fatalf("Allow(50) after release = (%v, %d), want (true, 0)", ok, rem)
	}
}

// TestQuotaConcurrentReserveNeverOvershoots is the core safety property: with a
// limit of 100 and 200 goroutines each trying to reserve 1, exactly 100 must
// succeed and the reserved total must be exactly 100 -- never more. Run under
// -race, this also proves the check-and-reserve is free of data races.
func TestQuotaConcurrentReserveNeverOvershoots(t *testing.T) {
	q := NewQuotaPolicy()
	const limit = 100
	const goroutines = 200
	q.SetLimit("acct-1", MetricPostsProcessed, limit)

	var granted int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start // maximize contention: release all at once
			if ok, _ := q.Allow("acct-1", MetricPostsProcessed, 1); ok {
				atomic.AddInt64(&granted, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if granted != limit {
		t.Fatalf("granted = %d, want exactly %d", granted, limit)
	}
	if used := q.Used("acct-1", MetricPostsProcessed); used != limit {
		t.Fatalf("reserved total = %d, want exactly %d (never overshoot)", used, limit)
	}
	if rem := q.Remaining("acct-1", MetricPostsProcessed); rem != 0 {
		t.Fatalf("remaining = %d, want 0", rem)
	}
}

// TestQuotaConcurrentLargeReservations reserves in varied chunk sizes from many
// goroutines and asserts the reserved total never exceeds the limit.
func TestQuotaConcurrentLargeReservations(t *testing.T) {
	q := NewQuotaPolicy()
	const limit = 1000
	q.SetLimit("acct-1", MetricBytesDownloaded, limit)

	var wg sync.WaitGroup
	start := make(chan struct{})
	const goroutines = 300
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		want := int64(1 + (i % 7)) // 1..7
		go func(want int64) {
			defer wg.Done()
			<-start
			q.Allow("acct-1", MetricBytesDownloaded, want)
		}(want)
	}
	close(start)
	wg.Wait()

	if used := q.Used("acct-1", MetricBytesDownloaded); used > limit {
		t.Fatalf("reserved total = %d, exceeds limit %d", used, limit)
	}
}
