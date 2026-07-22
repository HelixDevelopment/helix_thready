package ocr

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// knownWords are the tokens rendered into the generated test image. The real
// tesseract output MUST contain them for the test to pass — nothing is mocked.
var knownWords = []string{"Helix", "Thready", "42"}

// tesseractAvailable reports whether the real tesseract binary is on PATH.
func tesseractAvailable() bool {
	_, err := exec.LookPath("tesseract")
	return err == nil
}

// convertAvailable reports whether the ImageMagick `convert` binary is on PATH.
func convertAvailable() bool {
	_, err := exec.LookPath("convert")
	return err == nil
}

// makeTestImage renders a PNG containing the known text using ImageMagick's
// `convert`. It returns the absolute path to the generated file. The image is a
// real raster PNG on disk that tesseract will OCR — no fixtures, no fakes.
func makeTestImage(t *testing.T, text string) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "helix_thready.png")
	// convert -size 600x120 xc:white -gravity center -pointsize 40 -annotate 0 '<text>' out.png
	cmd := exec.Command("convert",
		"-size", "600x120", "xc:white",
		"-gravity", "center",
		"-pointsize", "40",
		"-annotate", "0", text,
		out,
	)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("convert failed to generate test image: %v\n%s", err, b)
	}
	return out
}

// TestTesseractRecognize_RealImage is the load-bearing assertion: generate a
// real PNG, run the real tesseract binary through TesseractProvider, and prove
// the extracted FullText actually contains the known words.
func TestTesseractRecognize_RealImage(t *testing.T) {
	if !tesseractAvailable() || !convertAvailable() {
		t.Skip("tesseract and/or convert not installed; skipping real-OCR test")
	}

	img := makeTestImage(t, "Helix Thready 42")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	p := NewTesseractProvider("eng")
	res, err := p.Recognize(ctx, img)
	if err != nil {
		t.Fatalf("Recognize returned error: %v", err)
	}

	if res.Engine != "tesseract" {
		t.Errorf("Engine = %q, want %q", res.Engine, "tesseract")
	}
	if strings.TrimSpace(res.FullText) == "" {
		t.Fatal("FullText is empty; tesseract extracted nothing")
	}

	t.Logf("OCR FullText: %q", res.FullText)
	for _, w := range knownWords {
		if !strings.Contains(res.FullText, w) {
			t.Errorf("FullText %q does not contain expected word %q", res.FullText, w)
		}
	}
}

// TestTesseractRecognize_TSVRegions proves the TSV pass yields per-word regions
// with plausible bounding boxes and confidences, and that the words themselves
// come back.
func TestTesseractRecognize_TSVRegions(t *testing.T) {
	if !tesseractAvailable() || !convertAvailable() {
		t.Skip("tesseract and/or convert not installed; skipping real-OCR test")
	}

	img := makeTestImage(t, "Helix Thready 42")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := NewTesseractProvider("eng").Recognize(ctx, img)
	if err != nil {
		t.Fatalf("Recognize returned error: %v", err)
	}

	if len(res.Regions) == 0 {
		t.Fatal("expected > 0 TSV regions, got 0")
	}
	t.Logf("parsed %d word region(s)", len(res.Regions))

	var joined []string
	for i, r := range res.Regions {
		joined = append(joined, r.Text)
		// Plausible bbox: within a 600x120 canvas, positive extents.
		if r.BBox.Width <= 0 || r.BBox.Height <= 0 {
			t.Errorf("region %d %q has non-positive size: %+v", i, r.Text, r.BBox)
		}
		if r.BBox.X < 0 || r.BBox.Y < 0 {
			t.Errorf("region %d %q has negative origin: %+v", i, r.Text, r.BBox)
		}
		if r.BBox.X+r.BBox.Width > 600 || r.BBox.Y+r.BBox.Height > 120 {
			t.Errorf("region %d %q bbox exceeds canvas: %+v", i, r.Text, r.BBox)
		}
		if r.Confidence < 0 || r.Confidence > 100 {
			t.Errorf("region %d %q confidence out of range: %v", i, r.Text, r.Confidence)
		}
		t.Logf("region[%d] text=%q bbox=%+v conf=%.2f", i, r.Text, r.BBox, r.Confidence)
	}

	all := strings.Join(joined, " ")
	for _, w := range knownWords {
		if !strings.Contains(all, w) {
			t.Errorf("region words %q do not contain expected word %q", all, w)
		}
	}
}

// TestTesseractRecognize_MissingFile proves a non-existent image yields a clean
// error and no fabricated result.
func TestTesseractRecognize_MissingFile(t *testing.T) {
	if !tesseractAvailable() {
		t.Skip("tesseract not installed; skipping")
	}
	p := NewTesseractProvider("eng")
	_, err := p.Recognize(context.Background(), filepath.Join(t.TempDir(), "does_not_exist.png"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	t.Logf("missing-file error (expected): %v", err)
}

// TestTesseractUnavailable proves that when the binary cannot be found we return
// ErrTesseractUnavailable rather than pretending to OCR.
func TestTesseractUnavailable(t *testing.T) {
	p := &TesseractProvider{Binary: "tesseract-does-not-exist-xyz"}
	if p.Available() {
		t.Fatal("Available() reported true for a bogus binary")
	}
	_, err := p.Recognize(context.Background(), "whatever.png")
	if err != ErrTesseractUnavailable {
		t.Fatalf("expected ErrTesseractUnavailable, got %v", err)
	}
}

// TestHybrid_PrimaryPath proves Hybrid returns the real primary (tesseract)
// result when no secondary LLM provider is wired — no fabricated fallback.
func TestHybrid_PrimaryPath(t *testing.T) {
	if !tesseractAvailable() || !convertAvailable() {
		t.Skip("tesseract and/or convert not installed; skipping real-OCR test")
	}
	img := makeTestImage(t, "Helix Thready 42")

	h := &Hybrid{Primary: NewTesseractProvider("eng")} // Secondary intentionally nil
	res, err := h.Recognize(context.Background(), img)
	if err != nil {
		t.Fatalf("Hybrid.Recognize error: %v", err)
	}
	for _, w := range knownWords {
		if !strings.Contains(res.FullText, w) {
			t.Errorf("Hybrid FullText %q missing %q", res.FullText, w)
		}
	}
}

// TestHybrid_NoPrimary proves a misconfigured Hybrid fails loudly.
func TestHybrid_NoPrimary(t *testing.T) {
	h := &Hybrid{}
	if _, err := h.Recognize(context.Background(), "x.png"); err != ErrNoPrimary {
		t.Fatalf("expected ErrNoPrimary, got %v", err)
	}
}

// TestParseTSV_Unit exercises the pure TSV parser without invoking tesseract,
// locking in header-skip, level filtering, empty-text and negative-conf drops.
func TestParseTSV_Unit(t *testing.T) {
	tsv := strings.Join([]string{
		"level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext",
		"1\t1\t0\t0\t0\t0\t0\t0\t600\t120\t-1\t",            // page level, dropped
		"5\t1\t1\t1\t1\t1\t40\t45\t120\t40\t96.5\tHelix",    // kept
		"5\t1\t1\t1\t1\t2\t180\t45\t150\t40\t95.2\tThready", // kept
		"5\t1\t1\t1\t1\t3\t350\t45\t50\t40\t-1\t",           // empty + neg conf, dropped
		"5\t1\t1\t1\t1\t4\t360\t45\t45\t40\t88.0\t42",       // kept
	}, "\n")

	regions := parseTSV(tsv)
	if len(regions) != 3 {
		t.Fatalf("expected 3 regions, got %d: %+v", len(regions), regions)
	}
	want := []string{"Helix", "Thready", "42"}
	for i, w := range want {
		if regions[i].Text != w {
			t.Errorf("region[%d].Text = %q, want %q", i, regions[i].Text, w)
		}
	}
	if regions[0].BBox != (BBox{X: 40, Y: 45, Width: 120, Height: 40}) {
		t.Errorf("region[0].BBox = %+v", regions[0].BBox)
	}
	if regions[0].Confidence != 96.5 {
		t.Errorf("region[0].Confidence = %v, want 96.5", regions[0].Confidence)
	}
}

// Compile-time proof the concrete providers satisfy the seam.
var (
	_ OCRProvider = (*TesseractProvider)(nil)
	_ OCRProvider = (*Hybrid)(nil)
)
