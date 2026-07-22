package metering

import (
	"testing"
	"time"
)

// proPlan is the worked example used across the billing tests.
//
//	Base fee:        $49.00                       -> 4900 cents
//	posts_processed: included 10,000, then 2c/post (per-unit; BlockUnits 1)
//	searches:        included 10,000, then 5c per 1,000 searches (block=1000)
//	storage_bytes:   included 5,000,000, then 25c per 1,000,000 bytes (block=1e6)
func proPlan() *Plan {
	return NewPlan("Pro", 4900,
		MetricRate{Metric: MetricPostsProcessed, IncludedUnits: 10_000, BlockUnits: 1, CentsPerBlock: 2},
		MetricRate{Metric: MetricSearches, IncludedUnits: 10_000, BlockUnits: 1_000, CentsPerBlock: 5},
		MetricRate{Metric: MetricStorageBytes, IncludedUnits: 5_000_000, BlockUnits: 1_000_000, CentsPerBlock: 25},
	)
}

// TestBillWorkedExample asserts the exact invoice for a fully worked scenario.
//
//	posts_processed: used 12,500 -> overage 2,500 -> 2,500 blocks x 2c   = 5,000c
//	searches:        used 25,600 -> overage 15,600 -> ceil(15,600/1,000)=16 x 5c = 80c
//	storage_bytes:   used 7,500,000 -> overage 2,500,000 -> ceil(2.5M/1M)=3 x 25c = 75c
//	bytes_downloaded: used 999,999 -> NOT in plan -> ignored
//	base:                                                                 4,900c
//	TOTAL = 4900 + 5000 + 80 + 75                                       = 10,055c ($100.55)
func TestBillWorkedExample(t *testing.T) {
	usage := map[string]int64{
		MetricPostsProcessed:  12_500,
		MetricSearches:        25_600,
		MetricStorageBytes:    7_500_000,
		MetricBytesDownloaded: 999_999, // not in the plan; must be ignored
	}
	inv := proPlan().BillUsage("acct-1", usage, Period{Start: 0, End: 1})

	const wantTotal Cents = 10_055
	if inv.TotalCents != wantTotal {
		t.Fatalf("total = %d (%s), want %d (%s)", inv.TotalCents, inv.TotalCents, wantTotal, wantTotal)
	}
	if inv.TotalCents.String() != "$100.55" {
		t.Fatalf("total string = %q, want %q", inv.TotalCents.String(), "$100.55")
	}

	// Verify each line item precisely. Order: base, then metrics sorted
	// alphabetically: posts_processed, searches, storage_bytes.
	want := []LineItem{
		{Metric: "", Kind: KindBase, AmountCents: 4900},
		{Metric: MetricPostsProcessed, Kind: KindOverage, UsedUnits: 12_500, IncludedUnits: 10_000, OverageUnits: 2_500, AmountCents: 5_000},
		{Metric: MetricSearches, Kind: KindOverage, UsedUnits: 25_600, IncludedUnits: 10_000, OverageUnits: 15_600, AmountCents: 80},
		{Metric: MetricStorageBytes, Kind: KindOverage, UsedUnits: 7_500_000, IncludedUnits: 5_000_000, OverageUnits: 2_500_000, AmountCents: 75},
	}
	if len(inv.LineItems) != len(want) {
		t.Fatalf("line items = %d, want %d: %+v", len(inv.LineItems), len(want), inv.LineItems)
	}
	for i, w := range want {
		if inv.LineItems[i] != w {
			t.Fatalf("line[%d] = %+v, want %+v", i, inv.LineItems[i], w)
		}
	}

	// Independent recomputation of the total from the line items.
	var sum Cents
	for _, li := range inv.LineItems {
		sum += li.AmountCents
	}
	if sum != inv.TotalCents {
		t.Fatalf("line-item sum %d != invoice total %d", sum, inv.TotalCents)
	}
}

// TestBillZeroUsageIsBaseOnly: an account with no usage pays exactly the base
// fee and has a single base line item.
func TestBillZeroUsageIsBaseOnly(t *testing.T) {
	inv := proPlan().BillUsage("acct-1", map[string]int64{}, Period{Start: 0, End: 1})
	if inv.TotalCents != 4900 {
		t.Fatalf("zero-usage total = %d, want 4900", inv.TotalCents)
	}
	if len(inv.LineItems) != 1 || inv.LineItems[0].Kind != KindBase {
		t.Fatalf("zero-usage line items = %+v, want a single base line", inv.LineItems)
	}
}

// TestBillWithinAllowanceNoOverage: usage under every allowance still bills base
// only (no overage lines).
func TestBillWithinAllowanceNoOverage(t *testing.T) {
	usage := map[string]int64{
		MetricPostsProcessed: 10_000,    // exactly included -> no overage
		MetricSearches:       9_999,     // under -> no overage
		MetricStorageBytes:   4_000_000, // under -> no overage
	}
	inv := proPlan().BillUsage("acct-1", usage, Period{Start: 0, End: 1})
	if inv.TotalCents != 4900 {
		t.Fatalf("within-allowance total = %d, want 4900", inv.TotalCents)
	}
	if len(inv.LineItems) != 1 {
		t.Fatalf("within-allowance line items = %d, want 1 (base only): %+v", len(inv.LineItems), inv.LineItems)
	}
}

// TestBillSingleUnitOverBlockRoundsUp: 1 unit over an allowance still costs a
// full block (ceil), proving block rounding.
func TestBillSingleUnitOverBlockRoundsUp(t *testing.T) {
	plan := NewPlan("Blocky", 1000,
		MetricRate{Metric: MetricSearches, IncludedUnits: 1000, BlockUnits: 1000, CentsPerBlock: 5},
	)
	inv := plan.BillUsage("acct-1", map[string]int64{MetricSearches: 1001}, Period{Start: 0, End: 1})
	// 1 unit over -> 1 started block -> 5c overage.
	if inv.TotalCents != 1005 {
		t.Fatalf("total = %d, want 1005 (base 1000 + one 5c block)", inv.TotalCents)
	}
}

// TestBillerViaRecorder proves the end-to-end path: record events, then Bill
// pulls the period usage from the Recorder and produces the same worked total.
func TestBillerViaRecorder(t *testing.T) {
	rec := NewRecorder()
	period := MonthUTC(2026, time.July)
	mid := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC).Unix()
	outside := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC).Unix()

	// In-period usage matching the worked example.
	rec.RecordUsage("acct-1", MetricPostsProcessed, 12_000, mid, "post")
	rec.RecordUsage("acct-1", MetricPostsProcessed, 500, mid, "post") // -> 12,500 total
	rec.RecordUsage("acct-1", MetricSearches, 25_600, mid, "search")
	rec.RecordUsage("acct-1", MetricStorageBytes, 7_500_000, mid, "byte")
	rec.RecordUsage("acct-1", MetricBytesDownloaded, 999_999, mid, "byte") // not in plan
	// Out-of-period usage must not be billed.
	rec.RecordUsage("acct-1", MetricPostsProcessed, 1_000_000, outside, "post")

	biller := NewBiller(proPlan(), rec)
	inv := biller.Bill("acct-1", period)

	if inv.TotalCents != 10_055 {
		t.Fatalf("recorder-driven total = %d (%s), want 10055 ($100.55)", inv.TotalCents, inv.TotalCents)
	}
	if inv.Plan != "Pro" || inv.AccountID != "acct-1" {
		t.Fatalf("invoice metadata = plan %q acct %q, want Pro/acct-1", inv.Plan, inv.AccountID)
	}
}

func TestCentsString(t *testing.T) {
	cases := []struct {
		c    Cents
		want string
	}{
		{0, "$0.00"},
		{5, "$0.05"},
		{99, "$0.99"},
		{100, "$1.00"},
		{4900, "$49.00"},
		{10_055, "$100.55"},
		{-250, "-$2.50"},
	}
	for _, tc := range cases {
		if got := tc.c.String(); got != tc.want {
			t.Fatalf("Cents(%d).String() = %q, want %q", int64(tc.c), got, tc.want)
		}
	}
}
