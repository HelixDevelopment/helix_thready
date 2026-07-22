// Package ocr is the Helix Thready OCR adapter.
//
// It fills the VisionEngine OCR gap recorded in the MVP gap register as
// [GAP: 2.6] ("VisionEngine has NO OCR engine"). The design is a two-pass
// hybrid:
//
//  1. A fast text pass behind the OCRProvider seam. This module ships a REAL
//     implementation of that pass, TesseractProvider, which shells out to the
//     installed `tesseract` binary (no cgo, stdlib only).
//  2. An LLM-vision second pass — declared here as the LLMVisionProvider seam
//     and orchestrated by Hybrid — which is intentionally NOT implemented in
//     this module. It is marked [BUILD-NEW] and never calls a fake backend.
//
// Module path: digital.vasic.ocr
package ocr

import (
	"context"
	"errors"
)

// ErrTesseractUnavailable is returned when the `tesseract` binary cannot be
// located on PATH. Callers should treat this as "fast pass unavailable" and
// fall back to another OCRProvider (e.g. the LLM-vision pass) rather than
// treating it as a hard failure. It is never used to disguise a real OCR error.
var ErrTesseractUnavailable = errors.New("ocr: tesseract binary not found on PATH")

// ErrImageNotFound is returned when the image path handed to a provider does
// not exist or is not a regular file.
var ErrImageNotFound = errors.New("ocr: image file not found")

// BBox is an axis-aligned bounding box in image pixel coordinates. The origin
// (0,0) is the top-left of the image; X grows right, Y grows down. These map
// directly onto tesseract's TSV columns left/top/width/height.
type BBox struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// TextRegion is one recognised text fragment together with where it was found
// and how confident the engine is. For TesseractProvider a region corresponds
// to a single recognised word (TSV level 5).
type TextRegion struct {
	Text       string  `json:"text"`
	BBox       BBox    `json:"bbox"`
	Confidence float64 `json:"confidence"` // 0..100 as reported by the engine; -1 means "unknown".
}

// Result is the outcome of an OCR pass over one image.
type Result struct {
	// FullText is the plain-text reading order transcription of the image.
	FullText string `json:"full_text"`
	// Regions is the per-word breakdown with geometry and confidence. It may be
	// empty even when FullText is populated if the engine returned no structured
	// output.
	Regions []TextRegion `json:"regions"`
	// Engine names the provider that produced this result (e.g. "tesseract").
	Engine string `json:"engine"`
}

// OCRProvider is the seam every OCR pass implements. VisionEngine depends on
// this interface, never on a concrete engine, so the fast (tesseract) pass and
// the LLM-vision pass are interchangeable and composable (see Hybrid).
type OCRProvider interface {
	// Recognize performs OCR on the image at imagePath and returns the extracted
	// text. It must honour ctx for cancellation/timeout.
	Recognize(ctx context.Context, imagePath string) (Result, error)
}
