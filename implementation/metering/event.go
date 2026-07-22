package metering

import "time"

// Common metric names used across Helix Thready. Callers may use any string;
// these constants exist for consistency and to avoid typos.
const (
	MetricPostsProcessed  = "posts_processed"
	MetricBytesDownloaded = "bytes_downloaded"
	MetricStorageBytes    = "storage_bytes"
	MetricAssetsStored    = "assets_stored"
	MetricSearches        = "searches"
)

// UsageEvent is a single recorded unit of account activity. Quantity is an
// integer count in the event's Unit (e.g. 1 post, 4096 bytes). TimestampUnix is
// Unix seconds (UTC) at which the usage occurred; it drives period windowing.
type UsageEvent struct {
	AccountID     string
	Metric        string
	Quantity      int64
	TimestampUnix int64
	Unit          string
}

// Period is a half-open time window [Start, End) in Unix seconds (UTC). An
// event at exactly Start is inside the window; an event at exactly End is not.
type Period struct {
	Start int64 // inclusive, Unix seconds
	End   int64 // exclusive, Unix seconds
}

// Contains reports whether the given Unix-second timestamp falls in [Start, End).
func (p Period) Contains(tsUnix int64) bool {
	return tsUnix >= p.Start && tsUnix < p.End
}

// MonthUTC returns the billing Period covering the given calendar month in UTC,
// from the first instant of the month (inclusive) to the first instant of the
// next month (exclusive).
func MonthUTC(year int, month time.Month) Period {
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	return Period{Start: start.Unix(), End: end.Unix()}
}
