package metering

import "sort"

// Line-item kinds.
const (
	KindBase    = "base"
	KindOverage = "overage"
)

// MetricRate describes how one metric is billed within a Plan.
//
//	IncludedUnits  units bundled into the base fee (no extra charge up to here)
//	BlockUnits     size of a billable overage block (must be >= 1; 0 is treated
//	               as 1). BlockUnits == 1 bills strictly per unit.
//	CentsPerBlock  price, in integer cents, of each started overage block.
//
// Overage is charged per *started* block: a partial block rounds up. With
// BlockUnits == 1 this is exact per-unit pricing.
type MetricRate struct {
	Metric        string
	IncludedUnits int64
	BlockUnits    int64
	CentsPerBlock Cents
}

// blockUnits normalizes BlockUnits so 0 behaves as 1 (per-unit).
func (m MetricRate) blockUnits() int64 {
	if m.BlockUnits <= 0 {
		return 1
	}
	return m.BlockUnits
}

// Plan is a subscription tier: a flat base fee plus per-metric metered overage.
type Plan struct {
	Name         string
	BaseFeeCents Cents
	rates        map[string]MetricRate
}

// NewPlan builds a Plan from a base fee and a set of metric rates.
func NewPlan(name string, baseFee Cents, rates ...MetricRate) *Plan {
	p := &Plan{Name: name, BaseFeeCents: baseFee, rates: make(map[string]MetricRate, len(rates))}
	for _, r := range rates {
		p.rates[r.Metric] = r
	}
	return p
}

// LineItem is one row of an Invoice.
type LineItem struct {
	Metric        string // empty for the base-fee line
	Kind          string // KindBase or KindOverage
	UsedUnits     int64
	IncludedUnits int64
	OverageUnits  int64
	AmountCents   Cents
}

// Invoice is the billing result for one account over one Period.
type Invoice struct {
	AccountID  string
	Plan       string
	Period     Period
	LineItems  []LineItem
	TotalCents Cents
}

// Biller couples a Plan with a Recorder to produce invoices from recorded usage.
type Biller struct {
	Plan     *Plan
	Recorder *Recorder
}

// NewBiller returns a Biller for the given plan and recorder.
func NewBiller(plan *Plan, rec *Recorder) *Biller {
	return &Biller{Plan: plan, Recorder: rec}
}

// Bill produces the Invoice for an account over the given period by reading the
// account's recorded usage from the Recorder.
func (b *Biller) Bill(accountID string, period Period) Invoice {
	usage := b.Recorder.PeriodUsage(accountID, period)
	return b.Plan.BillUsage(accountID, usage, period)
}

// BillUsage produces an Invoice from an explicit usage map (metric -> used
// units), without needing a Recorder. It is the pure billing computation and is
// deterministic: line items appear in a stable, sorted order.
//
// The invoice always starts with a base-fee line. For every metric configured
// in the plan, if usage exceeds the included allowance an overage line is added:
//
//	overageUnits = max(0, used - includedUnits)
//	blocks       = ceil(overageUnits / blockUnits)
//	amountCents  = blocks * centsPerBlock
//
// Metrics present in usage but not in the plan are ignored. A metric within its
// allowance produces no line. The total is the exact integer sum of all lines.
func (p *Plan) BillUsage(accountID string, usage map[string]int64, period Period) Invoice {
	inv := Invoice{
		AccountID: accountID,
		Plan:      p.Name,
		Period:    period,
	}

	base := LineItem{Kind: KindBase, AmountCents: p.BaseFeeCents}
	inv.LineItems = append(inv.LineItems, base)
	inv.TotalCents = p.BaseFeeCents

	// Deterministic ordering over the plan's metrics.
	metrics := make([]string, 0, len(p.rates))
	for m := range p.rates {
		metrics = append(metrics, m)
	}
	sort.Strings(metrics)

	for _, metric := range metrics {
		rate := p.rates[metric]
		used := usage[metric]
		overage := used - rate.IncludedUnits
		if overage <= 0 {
			continue
		}
		blocks := ceilDiv(overage, rate.blockUnits())
		amount := Cents(blocks) * rate.CentsPerBlock
		inv.LineItems = append(inv.LineItems, LineItem{
			Metric:        metric,
			Kind:          KindOverage,
			UsedUnits:     used,
			IncludedUnits: rate.IncludedUnits,
			OverageUnits:  overage,
			AmountCents:   amount,
		})
		inv.TotalCents += amount
	}

	return inv
}
