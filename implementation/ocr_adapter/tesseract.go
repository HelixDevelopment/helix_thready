package ocr

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// TesseractProvider is the REAL fast-pass OCRProvider. It drives the installed
// `tesseract` command-line binary and does no cgo. It performs two invocations
// per Recognize call:
//
//	tesseract <img> stdout        -> plain FullText (reading order)
//	tesseract <img> stdout tsv    -> per-word Regions with bbox + confidence
//
// If the binary is absent it returns ErrTesseractUnavailable so the caller can
// route to another provider instead of receiving fabricated text.
type TesseractProvider struct {
	// Binary is the tesseract executable to run. Empty means "tesseract" on PATH.
	Binary string
	// Languages is passed to `-l` (e.g. "eng" or "eng+deu"). Empty uses the
	// tesseract default.
	Languages string
	// ExtraArgs are appended before the output/config arguments (advanced use).
	ExtraArgs []string
}

// NewTesseractProvider builds a provider using the default `tesseract` binary
// and the given languages ("" for tesseract's default).
func NewTesseractProvider(languages string) *TesseractProvider {
	return &TesseractProvider{Languages: languages}
}

func (p *TesseractProvider) binary() string {
	if p.Binary != "" {
		return p.Binary
	}
	return "tesseract"
}

// Available reports whether the tesseract binary can be located on PATH.
func (p *TesseractProvider) Available() bool {
	_, err := exec.LookPath(p.binary())
	return err == nil
}

// Recognize implements OCRProvider using the real tesseract binary.
func (p *TesseractProvider) Recognize(ctx context.Context, imagePath string) (Result, error) {
	if _, err := exec.LookPath(p.binary()); err != nil {
		return Result{}, ErrTesseractUnavailable
	}

	// Give a clean, provider-level error for missing files rather than leaking a
	// raw tesseract exit message.
	info, err := os.Stat(imagePath)
	if err != nil || info.IsDir() {
		return Result{}, fmt.Errorf("%w: %s", ErrImageNotFound, imagePath)
	}

	fullText, err := p.runFullText(ctx, imagePath)
	if err != nil {
		return Result{}, err
	}

	regions, err := p.runTSV(ctx, imagePath)
	if err != nil {
		return Result{}, err
	}

	return Result{
		FullText: fullText,
		Regions:  regions,
		Engine:   "tesseract",
	}, nil
}

// runFullText runs `tesseract <img> stdout` and returns the trimmed transcription.
func (p *TesseractProvider) runFullText(ctx context.Context, imagePath string) (string, error) {
	args := p.baseArgs(imagePath)
	// No config name => plain text to stdout.
	out, err := p.run(ctx, args)
	if err != nil {
		return "", fmt.Errorf("ocr: tesseract text pass failed: %w", err)
	}
	return strings.TrimRight(out, "\n\x0c \t"), nil
}

// runTSV runs `tesseract <img> stdout tsv` and parses word-level regions.
func (p *TesseractProvider) runTSV(ctx context.Context, imagePath string) ([]TextRegion, error) {
	args := append(p.baseArgs(imagePath), "tsv")
	out, err := p.run(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("ocr: tesseract tsv pass failed: %w", err)
	}
	return parseTSV(out), nil
}

// baseArgs builds the leading arguments common to every invocation:
//
//	<binary is separate> <imagePath> stdout [-l <langs>] [extra...]
func (p *TesseractProvider) baseArgs(imagePath string) []string {
	args := []string{imagePath, "stdout"}
	if p.Languages != "" {
		args = append(args, "-l", p.Languages)
	}
	args = append(args, p.ExtraArgs...)
	return args
}

// run executes the tesseract binary and returns stdout, wrapping stderr into the
// error on failure.
func (p *TesseractProvider) run(ctx context.Context, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, p.binary(), args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", fmt.Errorf("%w: %s", err, msg)
		}
		return "", err
	}
	return stdout.String(), nil
}

// tsvColumns is the fixed column count of tesseract's TSV output.
//
//	level page_num block_num par_num line_num word_num left top width height conf text
const tsvColumns = 12

// tsvWordLevel is the TSV `level` value that denotes a single recognised word.
const tsvWordLevel = 5

// parseTSV converts tesseract TSV output into per-word TextRegions. It keeps
// only word-level rows (level 5) that carry non-empty text and a non-negative
// confidence. Malformed rows are skipped defensively rather than erroring —
// tesseract occasionally emits ragged rows and we prefer partial truth over a
// hard failure.
func parseTSV(tsv string) []TextRegion {
	var regions []TextRegion
	sc := bufio.NewScanner(strings.NewReader(tsv))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			// Skip the header row (starts with "level").
			first = false
			if strings.HasPrefix(line, "level\t") || strings.HasPrefix(line, "level ") {
				continue
			}
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		// The text column may itself contain spaces but never tabs, so split
		// into exactly tsvColumns fields.
		cols := strings.SplitN(line, "\t", tsvColumns)
		if len(cols) < tsvColumns {
			continue
		}
		level, err := strconv.Atoi(strings.TrimSpace(cols[0]))
		if err != nil || level != tsvWordLevel {
			continue
		}
		text := cols[11]
		if strings.TrimSpace(text) == "" {
			continue
		}
		conf, err := strconv.ParseFloat(strings.TrimSpace(cols[10]), 64)
		if err != nil || conf < 0 {
			continue
		}
		left := atoiSafe(cols[6])
		top := atoiSafe(cols[7])
		width := atoiSafe(cols[8])
		height := atoiSafe(cols[9])
		regions = append(regions, TextRegion{
			Text:       text,
			BBox:       BBox{X: left, Y: top, Width: width, Height: height},
			Confidence: conf,
		})
	}
	return regions
}

func atoiSafe(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

// Ensure TesseractProvider satisfies the OCRProvider seam at compile time.
var _ OCRProvider = (*TesseractProvider)(nil)
