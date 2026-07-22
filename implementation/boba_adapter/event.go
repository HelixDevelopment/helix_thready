// Package bobaadapter is the Helix Thready Boba callback-normalization adapter.
//
// It closes gap register [GAP: 6.4]: Boba-Base (milos85vasic/Boba-Base, a
// torrent meta-search / download engine) ALREADY exposes callbacks — an SSE
// "result_found" event stream on GET /api/v1/search and a hook-registration
// endpoint POST /api/v1/hooks that push search / download events — but its
// callback shape is BESPOKE (VERIFIED that the callbacks exist; the exact field
// names are bespoke and, where used here, inferred from Boba's meta-search
// domain and marked accordingly). This adapter does NOT add callbacks to Boba;
// it NORMALIZES Boba's existing events into the one shared Helix Thready
// callback envelope {job_id, state, progress, result_ref, error, ts} —
// byte-identical to implementation/metube_webhook (both lead with job_id and
// use the shared {job_id, state, progress, result_ref, error, ts} shape).
// implementation/callback_task carries the same six fields but names its first
// one task_id, a pre-existing sibling divergence that is out of scope here. It
// signs the envelope
// with "X-Thready-Signature: sha256=<hex>" (HMAC-SHA256 over the exact raw
// request body, event-bus contract §9) and fires it downstream. The result:
// Boba, MeTube and the Download Manager all speak ONE callback contract.
//
// Inference note: Boba's callbacks are VERIFIED to exist (SSE result_found +
// POST /api/v1/hooks). The concrete JSON field names consumed by the wire
// structs below (event, search_id, result{infohash,title,tracker,magnet,...},
// id, status, progress, path, error) are INFERRED from Boba's torrent
// meta-search domain and are lenient by design (multiple accepted spellings).
// Swapping them for Boba's exact keys is a one-line change per field and does
// not touch the normalization / signing / delivery logic.
//
// The module is self-contained and depends only on the Go standard library.
package bobaadapter

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// BobaEventType is the normalized kind of a Boba event.
type BobaEventType string

const (
	// EventResultFound is an SSE "result_found" search hit (a torrent/magnet was
	// found for a query). It is NOT terminal and does not fire a callback.
	EventResultFound BobaEventType = "result_found"
	// EventDownloadStarted: a download for a chosen result has begun.
	EventDownloadStarted BobaEventType = "download_started"
	// EventDownloadProgress: an in-flight download progress update.
	EventDownloadProgress BobaEventType = "download_progress"
	// EventDownloadComplete: terminal success — the download finished; Path holds
	// the result reference. This is the event that fires a standard callback.
	EventDownloadComplete BobaEventType = "download_complete"
	// EventDownloadError: terminal failure — the download failed; Error is set.
	EventDownloadError BobaEventType = "download_error"
)

// Terminal reports whether t is a terminal event type (fires a callback).
func (t BobaEventType) Terminal() bool {
	switch t {
	case EventDownloadComplete, EventDownloadError:
		return true
	default:
		return false
	}
}

// BobaEvent is the normalized, provider-neutral form of one Boba event, produced
// by ParseSSE / ParseHookPayload from Boba's bespoke JSON. The Bridge consumes
// these; nothing downstream of the parser sees Boba's wire shape.
type BobaEvent struct {
	// Type is the normalized event kind.
	Type BobaEventType
	// SearchID is Boba's search/session identifier, when present. [inferred]
	SearchID string
	// ResultID is the STABLE identifier of the torrent result / download used as
	// the callback job_id and the dedup key — Boba's explicit id, else the
	// torrent infohash. [inferred]
	ResultID string
	// Query is the originating search query. [inferred]
	Query string
	// Tracker is the originating tracker/indexer. [inferred]
	Tracker string
	// Title is the result title. [inferred]
	Title string
	// Magnet is the magnet URI of the result. [inferred]
	Magnet string
	// Torrent is a .torrent URL / path for the result. [inferred]
	Torrent string
	// Seeders is the reported seeder count. [inferred]
	Seeders int
	// State is Boba's raw status string, preserved verbatim for diagnostics. [inferred]
	State string
	// Progress is normalized to 0.0..1.0 (see normProgress). [inferred]
	Progress float64
	// Path is the completed download's result reference (path / asset ref). [inferred]
	Path string
	// Error is the failure message on a download-error event. [inferred]
	Error string
}

// wireBobaEvent is the lenient on-the-wire shape of one Boba event, spanning
// both the SSE result_found frame (nested "result") and download hook payloads
// (top-level id/status/progress/path). All field names are [inferred]; several
// accepted spellings are decoded so a small change in Boba's keys is absorbed.
type wireBobaEvent struct {
	Event    string          `json:"event"`
	Type     string          `json:"type"`
	SearchID string          `json:"search_id"`
	Query    string          `json:"query"`
	Result   *wireBobaResult `json:"result"`

	// Download-event top-level fields.
	ID       string   `json:"id"`
	InfoHash string   `json:"infohash"`
	Status   string   `json:"status"`
	Progress *float64 `json:"progress"`
	Path     string   `json:"path"`
	SavePath string   `json:"save_path"`
	File     string   `json:"file"`
	Error    string   `json:"error"`
	Message  string   `json:"message"`
}

// wireBobaResult is the nested result object of a result_found event. [inferred]
type wireBobaResult struct {
	ID       string `json:"id"`
	InfoHash string `json:"infohash"`
	Title    string `json:"title"`
	Tracker  string `json:"tracker"`
	Magnet   string `json:"magnet"`
	Torrent  string `json:"torrent"`
	Query    string `json:"query"`
	Seeders  int    `json:"seeders"`
}

// errNoData signals that an SSE frame carried no "data:" line (comment / keep-
// alive frame). Stream readers treat it as a skip, not a failure.
var errNoData = errors.New("bobaadapter: SSE frame has no data")

// ParseSSE parses ONE Boba SSE frame — the raw bytes of a single event block,
// e.g. "event: result_found\ndata: {...}\n" (multiple data: lines are joined
// with '\n' per the SSE spec) — into a normalized BobaEvent. It is pure and
// offline-testable. A frame with no data: line returns errNoData.
func ParseSSE(frame []byte) (BobaEvent, error) {
	var eventName string
	var data []string

	sc := bufio.NewScanner(bytes.NewReader(frame))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if line == "" || strings.HasPrefix(line, ":") {
			continue // blank line or SSE comment
		}
		field, value, found := strings.Cut(line, ":")
		if !found {
			field, value = line, ""
		}
		value = strings.TrimPrefix(value, " ") // strip one optional leading space
		switch field {
		case "event":
			eventName = value
		case "data":
			data = append(data, value)
		}
	}
	if err := sc.Err(); err != nil {
		return BobaEvent{}, fmt.Errorf("bobaadapter: scan SSE frame: %w", err)
	}
	if len(data) == 0 {
		return BobaEvent{}, errNoData
	}
	return decodeBoba([]byte(strings.Join(data, "\n")), eventName)
}

// ParseHookPayload maps a raw Boba hook POST body (the JSON Boba delivers to a
// URL registered via POST /api/v1/hooks) into a normalized BobaEvent. It is pure
// and offline-testable.
func ParseHookPayload(data []byte) (BobaEvent, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return BobaEvent{}, fmt.Errorf("bobaadapter: empty hook payload")
	}
	return decodeBoba(data, "")
}

// decodeBoba unmarshals a Boba JSON object into a normalized BobaEvent. sseEvent
// is the SSE "event:" name (empty for hook payloads) used as the event-kind
// fallback when the JSON body omits an explicit event/type field.
func decodeBoba(data []byte, sseEvent string) (BobaEvent, error) {
	var w wireBobaEvent
	if err := json.Unmarshal(bytes.TrimSpace(data), &w); err != nil {
		return BobaEvent{}, fmt.Errorf("bobaadapter: decode Boba event: %w", err)
	}
	return mapWire(w, sseEvent), nil
}

// mapWire normalizes a decoded wire event into a BobaEvent.
func mapWire(w wireBobaEvent, sseEvent string) BobaEvent {
	name := firstNonEmpty(w.Event, w.Type, sseEvent)
	et := normalizeType(name, w.Status)

	ev := BobaEvent{
		Type:     et,
		SearchID: w.SearchID,
		Query:    w.Query,
		State:    w.Status,
		Path:     firstNonEmpty(w.Path, w.SavePath, w.File),
		Error:    firstNonEmpty(w.Error, w.Message),
	}

	if w.Result != nil {
		ev.ResultID = firstNonEmpty(w.Result.ID, w.Result.InfoHash)
		ev.Title = w.Result.Title
		ev.Tracker = w.Result.Tracker
		ev.Magnet = w.Result.Magnet
		ev.Torrent = w.Result.Torrent
		ev.Seeders = w.Result.Seeders
		if ev.Query == "" {
			ev.Query = w.Result.Query
		}
	}
	// Top-level id / infohash win for download events (no nested result block).
	ev.ResultID = firstNonEmpty(ev.ResultID, w.ID, w.InfoHash)

	ev.Progress = normProgress(w.Progress)
	if et == EventDownloadComplete {
		ev.Progress = 1.0 // a completed download is fully done regardless of report
	}
	return ev
}

// classify maps one status/event token to a normalized type.
func classify(tok string) (BobaEventType, bool) {
	switch strings.ToLower(strings.TrimSpace(tok)) {
	case "result_found", "result", "found":
		return EventResultFound, true
	case "download_started", "started", "start", "queued":
		return EventDownloadStarted, true
	case "download_progress", "progress", "downloading":
		return EventDownloadProgress, true
	case "download_complete", "complete", "completed", "finished", "done":
		return EventDownloadComplete, true
	case "download_error", "error", "failed", "failure":
		return EventDownloadError, true
	}
	return "", false
}

// normalizeType resolves the event kind, preferring the explicit event name and
// falling back to the raw status string. An unrecognized name passes through
// verbatim (non-terminal) so unknown events are ignored rather than misfired.
func normalizeType(name, status string) BobaEventType {
	if t, ok := classify(name); ok {
		return t
	}
	if t, ok := classify(status); ok {
		return t
	}
	return BobaEventType(strings.TrimSpace(name))
}

// normProgress normalizes a reported progress value to 0.0..1.0. Boba's exact
// unit is [inferred]: a value > 1.0 is treated as a percent (0..100) and divided
// by 100; a value already in 0..1 is kept. The result is clamped.
func normProgress(p *float64) float64 {
	if p == nil {
		return 0
	}
	v := *p
	if v > 1.0 {
		v = v / 100.0
	}
	return clamp01(v)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
