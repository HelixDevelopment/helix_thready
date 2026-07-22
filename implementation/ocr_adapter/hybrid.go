package ocr

import (
	"context"
	"errors"
	"strings"
)

// LLMVisionProvider is the SECOND-PASS seam of the hybrid OCR design: a
// vision-capable LLM that reads an image and returns text. It is deliberately
// declared here as an interface ONLY.
//
// [BUILD-NEW] No concrete LLMVisionProvider is implemented in this module. A
// real implementation would call a hosted or local vision model (e.g. an
// Anthropic vision request) and adapt its response into Result. That network/
// model integration is out of scope for the OCR adapter and MUST NOT be faked
// here — there is intentionally no stub that returns invented text.
//
// It shares the OCRProvider shape so a real implementation drops straight into
// Hybrid and anywhere else an OCRProvider is expected.
type LLMVisionProvider interface {
	OCRProvider
}

// Hybrid orchestrates the two-pass design from [GAP: 2.6]: a fast Primary pass
// (e.g. TesseractProvider) with an optional LLM-vision Secondary pass used as a
// fallback.
//
// The fallback fires only when the primary result is unusable — an error, empty
// text, or mean confidence below MinConfidence — AND a Secondary is actually
// wired. Because this module ships no concrete LLMVisionProvider, Secondary is
// nil in practice and Hybrid transparently returns the real primary result. No
// fabricated LLM call ever happens: the secondary is invoked only if the caller
// supplies a genuine implementation.
type Hybrid struct {
	// Primary is the fast OCR pass. Required.
	Primary OCRProvider
	// Secondary is the LLM-vision fallback. Optional; nil disables fallback.
	// [BUILD-NEW] wire a real LLMVisionProvider here once it exists.
	Secondary LLMVisionProvider
	// MinConfidence is the mean-region confidence (0..100) below which the
	// primary result is considered weak enough to warrant the LLM pass. Zero
	// means "only fall back on error or empty text".
	MinConfidence float64
}

// ErrNoPrimary is returned when Hybrid is used without a Primary provider.
var ErrNoPrimary = errors.New("ocr: hybrid has no primary provider configured")

// Recognize runs the primary pass and, only when warranted and available, the
// secondary LLM-vision pass.
func (h *Hybrid) Recognize(ctx context.Context, imagePath string) (Result, error) {
	if h.Primary == nil {
		return Result{}, ErrNoPrimary
	}

	res, err := h.Primary.Recognize(ctx, imagePath)

	if !h.needsFallback(res, err) {
		return res, err
	}

	// Fallback path. Only reached with a genuinely-wired Secondary.
	// [BUILD-NEW] Secondary is nil in this module, so we never fabricate output;
	// we return the primary's real result (and error, if any) unchanged.
	if h.Secondary == nil {
		return res, err
	}
	return h.Secondary.Recognize(ctx, imagePath)
}

// needsFallback decides whether the primary result is too weak to trust.
func (h *Hybrid) needsFallback(res Result, err error) bool {
	if err != nil {
		return true
	}
	if strings.TrimSpace(res.FullText) == "" {
		return true
	}
	if h.MinConfidence > 0 && meanConfidence(res.Regions) < h.MinConfidence {
		return true
	}
	return false
}

// meanConfidence returns the average confidence across regions, or 100 when
// there are no regions (so an absent breakdown does not by itself trigger a
// low-confidence fallback).
func meanConfidence(regions []TextRegion) float64 {
	if len(regions) == 0 {
		return 100
	}
	var sum float64
	for _, r := range regions {
		sum += r.Confidence
	}
	return sum / float64(len(regions))
}

// Ensure Hybrid satisfies the OCRProvider seam at compile time.
var _ OCRProvider = (*Hybrid)(nil)
