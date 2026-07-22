package metering

import (
	"sync"
	"testing"
	"time"
)

func TestRecorderAggregateExactSum(t *testing.T) {
	rec := NewRecorder()
	p := Period{Start: 1000, End: 2000}

	// N events, all inside the window, with known quantities.
	quantities := []int64{5, 10, 1, 4, 20, 60}
	var want int64
	for i, q := range quantities {
		rec.RecordUsage("acct-1", MetricPostsProcessed, q, int64(1000+i), "post")
		want += q
	}

	got := rec.Aggregate("acct-1", MetricPostsProcessed, p)
	if got != want {
		t.Fatalf("Aggregate = %d, want %d", got, want)
	}
}

func TestRecorderPeriodWindowing(t *testing.T) {
	rec := NewRecorder()
	p := Period{Start: 1000, End: 2000} // [1000, 2000)

	// Inside window.
	rec.RecordUsage("acct-1", MetricSearches, 3, 1000, "search") // Start is inclusive
	rec.RecordUsage("acct-1", MetricSearches, 7, 1500, "search")
	rec.RecordUsage("acct-1", MetricSearches, 5, 1999, "search")
	// Outside window.
	rec.RecordUsage("acct-1", MetricSearches, 100, 999, "search")  // before start
	rec.RecordUsage("acct-1", MetricSearches, 200, 2000, "search") // End is exclusive
	rec.RecordUsage("acct-1", MetricSearches, 300, 5000, "search") // far after

	got := rec.Aggregate("acct-1", MetricSearches, p)
	const want = 3 + 7 + 5
	if got != want {
		t.Fatalf("windowed Aggregate = %d, want %d (out-of-window events must be excluded)", got, want)
	}
}

func TestRecorderIsolatesAccountsAndMetrics(t *testing.T) {
	rec := NewRecorder()
	p := Period{Start: 0, End: 10_000}

	rec.RecordUsage("acct-1", MetricPostsProcessed, 10, 100, "post")
	rec.RecordUsage("acct-1", MetricSearches, 99, 100, "search")
	rec.RecordUsage("acct-2", MetricPostsProcessed, 500, 100, "post")

	if got := rec.Aggregate("acct-1", MetricPostsProcessed, p); got != 10 {
		t.Fatalf("acct-1 posts = %d, want 10", got)
	}
	if got := rec.Aggregate("acct-2", MetricPostsProcessed, p); got != 500 {
		t.Fatalf("acct-2 posts = %d, want 500", got)
	}
	if got := rec.Aggregate("acct-1", MetricSearches, p); got != 99 {
		t.Fatalf("acct-1 searches = %d, want 99", got)
	}
	// A metric never recorded for this account is zero.
	if got := rec.Aggregate("acct-2", MetricSearches, p); got != 0 {
		t.Fatalf("acct-2 searches = %d, want 0", got)
	}
}

func TestRecorderPeriodUsageBucket(t *testing.T) {
	rec := NewRecorder()
	p := MonthUTC(2026, time.July)
	mid := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC).Unix()
	before := time.Date(2026, time.June, 30, 23, 59, 59, 0, time.UTC).Unix()

	rec.RecordUsage("acct-1", MetricPostsProcessed, 40, mid, "post")
	rec.RecordUsage("acct-1", MetricPostsProcessed, 2, mid, "post")
	rec.RecordUsage("acct-1", MetricSearches, 9, mid, "search")
	rec.RecordUsage("acct-1", MetricStorageBytes, 100, before, "byte") // out of month

	usage := rec.PeriodUsage("acct-1", p)
	if usage[MetricPostsProcessed] != 42 {
		t.Fatalf("posts bucket = %d, want 42", usage[MetricPostsProcessed])
	}
	if usage[MetricSearches] != 9 {
		t.Fatalf("searches bucket = %d, want 9", usage[MetricSearches])
	}
	if _, ok := usage[MetricStorageBytes]; ok {
		t.Fatalf("storage_bytes should be excluded (recorded outside the period)")
	}
}

// TestRecorderConcurrentRecord exercises the recorder under -race with many
// concurrent writers and one reader, asserting the exact total afterwards.
func TestRecorderConcurrentRecord(t *testing.T) {
	rec := NewRecorder()
	p := Period{Start: 0, End: 1_000_000}

	const writers = 100
	const perWriter = 100
	var wg sync.WaitGroup
	wg.Add(writers)
	for w := 0; w < writers; w++ {
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				rec.RecordUsage("acct-1", MetricBytesDownloaded, 1, int64(i), "byte")
			}
		}(w)
	}
	// Concurrent reader to shake out data races with RLock/Lock.
	go func() {
		for i := 0; i < 50; i++ {
			_ = rec.Aggregate("acct-1", MetricBytesDownloaded, p)
		}
	}()
	wg.Wait()

	got := rec.Aggregate("acct-1", MetricBytesDownloaded, p)
	const want = writers * perWriter
	if got != want {
		t.Fatalf("concurrent total = %d, want %d", got, want)
	}
}
