// Package metering is the reusable usage-metering and billing core for Helix
// Thready. It supports the operator's chosen "subscription + metered" billing
// model (MVP decision matrix, Q11) for a large-scale multi-tenant account model
// where usage metering, quotas, and billing must exist from day one.
//
// The package is stdlib-only and split into four concerns:
//
//   - UsageEvent + Recorder: record per-account usage events and aggregate the
//     summed quantity of a metric over a time window (Period).
//   - QuotaPolicy: per-account, per-metric limits with an atomic
//     check-and-reserve Allow so concurrent callers can never jointly overshoot
//     a limit.
//   - Plan + Biller: a subscription tier (base fee, per-metric included
//     allowances and overage rates) that produces an Invoice with LineItems.
//   - Money: all money is integer minor units (cents, the Cents type). No
//     float is used for money anywhere, so billing is deterministic.
//
// The billing formula per metric is:
//
//	overageUnits = max(0, used - includedUnits)
//	blocks       = ceil(overageUnits / blockUnits)   // blockUnits defaults to 1
//	lineCents    = blocks * centsPerBlock
//
// With the default blockUnits == 1 this reduces exactly to the canonical
// "max(0, used - included) * rate" per-unit overage. blockUnits > 1 models
// realistic block pricing (e.g. "$0.05 per 1000 searches") while keeping the
// arithmetic in exact integers.
package metering
