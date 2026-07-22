package metering

import "strconv"

// Cents is a monetary amount expressed in integer minor units (US cents, or the
// minor unit of whatever currency the caller uses). All money math in this
// package is done in Cents; floating point is never used for money, so results
// are exact and deterministic.
type Cents int64

// String renders the amount as a decimal with two minor digits and a leading
// "$", e.g. Cents(10055).String() == "$100.55". It is a debugging/reporting
// convenience and does not localize the currency symbol.
func (c Cents) String() string {
	neg := c < 0
	v := int64(c)
	if neg {
		v = -v
	}
	major := v / 100
	minor := v % 100
	s := "$" + strconv.FormatInt(major, 10) + "." + pad2(minor)
	if neg {
		return "-" + s
	}
	return s
}

func pad2(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}

// ceilDiv returns ceil(a / b) for non-negative integers using only integer
// arithmetic (no float, overflow-safe). b must be > 0; callers guarantee that.
// A non-positive a yields 0.
func ceilDiv(a, b int64) int64 {
	if a <= 0 {
		return 0
	}
	q := a / b
	if a%b != 0 {
		q++
	}
	return q
}
