# Helix Thready — OCR Adapter (`digital.vasic.ocr`)

Real, self-contained Go module that gives the Helix Thready **VisionEngine** an
OCR engine. It closes gap register item **[GAP: 2.6]** — *"VisionEngine has NO
OCR engine"* — by implementing the fast text pass of a two-pass hybrid design.

- **Stdlib only, no cgo.** OCR is performed by shelling out to the installed
  `tesseract` command-line binary.
- **No bluff.** Given a real image, it returns the text tesseract actually read.
  See [`EVIDENCE.md`](./EVIDENCE.md) for captured proof.

## Purpose & design (hybrid)

VisionEngine depends on the `OCRProvider` seam, never on a concrete engine, so
passes are interchangeable:

1. **Fast text pass — `TesseractProvider` (REAL, shipped).** Runs the local
   `tesseract` binary. Cheap, offline, good on clean/printed text.
2. **LLM-vision pass — `LLMVisionProvider` (interface only, `[BUILD-NEW]`).** A
   vision-capable model for hard cases (handwriting, layout, low quality). It is
   declared but **not implemented here** and is **never faked**.
3. **`Hybrid`** orchestrates the two: run the primary; fall back to the secondary
   only when the primary errors, returns empty text, or falls below a confidence
   threshold **and** a real secondary is wired. With no secondary supplied (the
   default in this module) it transparently returns the real primary result.

## API

```go
import "digital.vasic.ocr"

type OCRProvider interface {
    Recognize(ctx context.Context, imagePath string) (Result, error)
}

type Result struct {
    FullText string        // reading-order transcription
    Regions  []TextRegion  // per-word breakdown
    Engine   string        // e.g. "tesseract"
}

type TextRegion struct {
    Text       string
    BBox       BBox     // X,Y,Width,Height in image pixels (origin top-left)
    Confidence float64  // 0..100 as reported by the engine; -1 = unknown
}
```

### Usage

```go
p := ocr.NewTesseractProvider("eng") // language(s) passed to tesseract -l; "" = default
res, err := p.Recognize(ctx, "/path/to/image.png")
if errors.Is(err, ocr.ErrTesseractUnavailable) {
    // binary missing — route to another provider instead of failing hard
}
fmt.Println(res.FullText)
for _, r := range res.Regions {
    fmt.Printf("%q @ %+v (%.1f%%)\n", r.Text, r.BBox, r.Confidence)
}
```

### Hybrid wiring

```go
h := &ocr.Hybrid{
    Primary:       ocr.NewTesseractProvider("eng"),
    Secondary:     nil, // [BUILD-NEW] plug a real LLMVisionProvider here later
    MinConfidence: 60,  // below this mean confidence, prefer the LLM pass (if wired)
}
res, err := h.Recognize(ctx, imagePath)
```

## What `TesseractProvider` runs

Per `Recognize` call, two invocations of the real binary:

| Purpose  | Command                         | Parsed into      |
|----------|---------------------------------|------------------|
| FullText | `tesseract <img> stdout`        | `Result.FullText`|
| Regions  | `tesseract <img> stdout tsv`    | `Result.Regions` (word rows, level 5) |

Errors are honest, never disguised:

- `ErrTesseractUnavailable` — binary not on PATH (detected via `exec.LookPath`).
- `ErrImageNotFound` — image path missing / not a regular file.
- `ErrNoPrimary` — `Hybrid` used without a primary provider.

## Requirements

- Go 1.26+
- `tesseract` on PATH (tested with 5.3.0). Language data for the `-l` languages.
- `convert` (ImageMagick) — **for the test suite only**, to synthesise images.

## Running the tests

```
cd implementation/ocr_adapter
go build ./... && go vet ./... && go test ./... -v -count=1
```

The real-OCR tests generate a PNG containing `Helix Thready 42` with ImageMagick,
run `TesseractProvider`, and assert the extracted `FullText` contains the known
words. They `t.Skip` **only** if `tesseract`/`convert` are genuinely absent; on
this host both are present, so the real assertions execute. See `EVIDENCE.md`.

## Files

| File | Role |
|------|------|
| `go.mod` | Module `digital.vasic.ocr`, Go 1.26, no dependencies |
| `ocr.go` | `OCRProvider` seam, `Result`, `TextRegion`, `BBox`, error sentinels |
| `tesseract.go` | `TesseractProvider` — real tesseract CLI driver + TSV parser |
| `hybrid.go` | `LLMVisionProvider` seam `[BUILD-NEW]` + `Hybrid` orchestrator |
| `tesseract_test.go` | TDD real-image tests + TSV unit test |
| `EVIDENCE.md` | Captured toolchain, real OCR output, test results, verdict |
