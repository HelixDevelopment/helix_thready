package ocr

import (
	"context"
	"errors"
	"testing"
)

// fakeProvider is a genuine in-test OCRProvider double. Its Recognize returns a
// caller-supplied canned Result/error and records how many times it ran, so the
// Hybrid fallback orchestration can be exercised deterministically without any
// dependency on the real tesseract binary. This is a real interface
// implementation, not a mock framework — it satisfies OCRProvider (and thus the
// LLMVisionProvider seam, which embeds it) at compile time.
type fakeProvider struct {
	result Result
	err    error
	calls  int
}

func (f *fakeProvider) Recognize(_ context.Context, _ string) (Result, error) {
	f.calls++
	return f.result, f.err
}

// Compile-time proof the double satisfies both seams it is used against.
var (
	_ OCRProvider       = (*fakeProvider)(nil)
	_ LLMVisionProvider = (*fakeProvider)(nil)
)

// TestHybrid_ConfidentPrimary_SecondaryNotInvoked proves the happy path: when
// the primary returns non-empty text whose mean region confidence clears
// MinConfidence, Hybrid returns the primary result verbatim and NEVER touches
// the secondary (asserted via the call counter).
func TestHybrid_ConfidentPrimary_SecondaryNotInvoked(t *testing.T) {
	primary := &fakeProvider{result: Result{
		FullText: "PRIMARY CONFIDENT",
		Regions: []TextRegion{
			{Text: "PRIMARY", Confidence: 95},
			{Text: "CONFIDENT", Confidence: 97}, // mean 96 >= MinConfidence
		},
		Engine: "fake-primary",
	}}
	secondary := &fakeProvider{result: Result{FullText: "SECONDARY FALLBACK", Engine: "fake-secondary"}}

	h := &Hybrid{Primary: primary, Secondary: secondary, MinConfidence: 80}
	res, err := h.Recognize(context.Background(), "img.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secondary.calls != 0 {
		t.Fatalf("secondary must NOT be invoked on a confident primary; calls=%d", secondary.calls)
	}
	if primary.calls != 1 {
		t.Fatalf("primary should run exactly once; calls=%d", primary.calls)
	}
	if res.Engine != "fake-primary" {
		t.Errorf("expected primary result to be returned; Engine=%q, want %q", res.Engine, "fake-primary")
	}
	if res.FullText != "PRIMARY CONFIDENT" {
		t.Errorf("FullText = %q, want primary text %q", res.FullText, "PRIMARY CONFIDENT")
	}
}

// TestHybrid_ConfidentPrimary_NoRegions_SecondaryNotInvoked locks in the
// documented contract that an ABSENT confidence breakdown (no regions) must not
// by itself trigger the low-confidence fallback: meanConfidence returns 100, so
// the primary result stands and the secondary stays untouched.
func TestHybrid_ConfidentPrimary_NoRegions_SecondaryNotInvoked(t *testing.T) {
	primary := &fakeProvider{result: Result{
		FullText: "NO REGION BREAKDOWN",
		Regions:  nil, // no per-word confidences reported
		Engine:   "fake-primary",
	}}
	secondary := &fakeProvider{result: Result{FullText: "SECONDARY FALLBACK", Engine: "fake-secondary"}}

	h := &Hybrid{Primary: primary, Secondary: secondary, MinConfidence: 80}
	res, err := h.Recognize(context.Background(), "img.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secondary.calls != 0 {
		t.Fatalf("secondary must NOT fire when there is no confidence breakdown; calls=%d", secondary.calls)
	}
	if res.Engine != "fake-primary" || res.FullText != "NO REGION BREAKDOWN" {
		t.Errorf("expected primary passthrough, got Engine=%q FullText=%q", res.Engine, res.FullText)
	}
}

// TestHybrid_LowConfidencePrimary_FallsBackToSecondary proves the confidence
// fallback: when the primary's mean region confidence is below MinConfidence,
// Hybrid invokes the secondary and returns ITS result.
func TestHybrid_LowConfidencePrimary_FallsBackToSecondary(t *testing.T) {
	primary := &fakeProvider{result: Result{
		FullText: "prlmary blurry guess",
		Regions: []TextRegion{
			{Text: "prlmary", Confidence: 40},
			{Text: "blurry", Confidence: 30}, // mean 35 < MinConfidence
		},
		Engine: "fake-primary",
	}}
	secondary := &fakeProvider{result: Result{FullText: "SECONDARY CLEAN READ", Engine: "fake-secondary"}}

	h := &Hybrid{Primary: primary, Secondary: secondary, MinConfidence: 80}
	res, err := h.Recognize(context.Background(), "img.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secondary.calls != 1 {
		t.Fatalf("secondary must be invoked exactly once on low-confidence primary; calls=%d", secondary.calls)
	}
	if res.Engine != "fake-secondary" {
		t.Errorf("expected secondary result; Engine=%q, want %q", res.Engine, "fake-secondary")
	}
	if res.FullText != "SECONDARY CLEAN READ" {
		t.Errorf("FullText = %q, want secondary text %q", res.FullText, "SECONDARY CLEAN READ")
	}
}

// TestHybrid_EmptyPrimary_FallsBackToSecondary proves that empty/whitespace-only
// primary text triggers the fallback even when MinConfidence is unset (zero).
func TestHybrid_EmptyPrimary_FallsBackToSecondary(t *testing.T) {
	primary := &fakeProvider{result: Result{FullText: "   \n\t ", Engine: "fake-primary"}} // whitespace only
	secondary := &fakeProvider{result: Result{FullText: "SECONDARY FROM EMPTY", Engine: "fake-secondary"}}

	h := &Hybrid{Primary: primary, Secondary: secondary} // MinConfidence 0
	res, err := h.Recognize(context.Background(), "img.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secondary.calls != 1 {
		t.Fatalf("secondary must be invoked on empty primary; calls=%d", secondary.calls)
	}
	if res.Engine != "fake-secondary" || res.FullText != "SECONDARY FROM EMPTY" {
		t.Errorf("expected secondary result, got Engine=%q FullText=%q", res.Engine, res.FullText)
	}
}

// TestHybrid_PrimaryError_FallsBackToSecondary proves that a primary error
// routes to the secondary, and the secondary's clean result (nil error)
// replaces the primary failure.
func TestHybrid_PrimaryError_FallsBackToSecondary(t *testing.T) {
	primaryErr := errors.New("primary engine exploded")
	primary := &fakeProvider{err: primaryErr}
	secondary := &fakeProvider{result: Result{FullText: "SECONDARY AFTER ERROR", Engine: "fake-secondary"}}

	h := &Hybrid{Primary: primary, Secondary: secondary}
	res, err := h.Recognize(context.Background(), "img.png")
	if err != nil {
		t.Fatalf("expected secondary to recover with nil error, got: %v", err)
	}
	if secondary.calls != 1 {
		t.Fatalf("secondary must be invoked when primary errors; calls=%d", secondary.calls)
	}
	if res.Engine != "fake-secondary" || res.FullText != "SECONDARY AFTER ERROR" {
		t.Errorf("expected secondary result, got Engine=%q FullText=%q", res.Engine, res.FullText)
	}
}

// TestHybrid_WeakPrimary_NilSecondary_Passthrough proves that when the primary
// result warrants a fallback but no Secondary is wired, Hybrid transparently
// returns the (weak) primary result and error unchanged — no panic, no
// fabricated output.
func TestHybrid_WeakPrimary_NilSecondary_Passthrough(t *testing.T) {
	primaryErr := errors.New("primary engine unavailable")
	primary := &fakeProvider{
		result: Result{FullText: "", Engine: "fake-primary"},
		err:    primaryErr,
	}

	h := &Hybrid{Primary: primary} // Secondary intentionally nil
	res, err := h.Recognize(context.Background(), "img.png")
	if !errors.Is(err, primaryErr) {
		t.Fatalf("expected primary error passthrough, got: %v", err)
	}
	if res.Engine != "fake-primary" {
		t.Errorf("expected primary result passthrough; Engine=%q", res.Engine)
	}
	if primary.calls != 1 {
		t.Fatalf("primary should run exactly once; calls=%d", primary.calls)
	}
}
