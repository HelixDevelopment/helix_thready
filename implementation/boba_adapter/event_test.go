package bobaadapter

import (
	"errors"
	"testing"
)

func TestParseSSE_ResultFoundFrame(t *testing.T) {
	// A canned Boba "result_found" SSE frame: event: line + a single data: line
	// carrying the nested result object.
	frame := []byte("event: result_found\n" +
		`data: {"search_id":"s-1","query":"ubuntu","result":{"infohash":"HASH1","title":"Ubuntu 24.04","tracker":"linuxtracker","magnet":"magnet:?xt=urn:btih:HASH1","seeders":42}}` + "\n")

	ev, err := ParseSSE(frame)
	if err != nil {
		t.Fatalf("ParseSSE: %v", err)
	}
	if ev.Type != EventResultFound {
		t.Errorf("Type = %q, want result_found", ev.Type)
	}
	if ev.Type.Terminal() {
		t.Error("result_found must not be terminal")
	}
	if ev.SearchID != "s-1" {
		t.Errorf("SearchID = %q, want s-1", ev.SearchID)
	}
	if ev.ResultID != "HASH1" {
		t.Errorf("ResultID = %q, want HASH1 (from infohash)", ev.ResultID)
	}
	if ev.Query != "ubuntu" {
		t.Errorf("Query = %q, want ubuntu", ev.Query)
	}
	if ev.Title != "Ubuntu 24.04" || ev.Tracker != "linuxtracker" || ev.Seeders != 42 {
		t.Errorf("result fields = %+v", ev)
	}
	if ev.Magnet != "magnet:?xt=urn:btih:HASH1" {
		t.Errorf("Magnet = %q", ev.Magnet)
	}
}

func TestParseSSE_MultiDataLinesJoined(t *testing.T) {
	// Per the SSE spec, multiple data: lines are concatenated with '\n'. Here the
	// JSON body is split across two data: lines.
	frame := []byte("event: download_complete\n" +
		"data: {\"id\":\"HASH2\",\"status\":\"complete\",\n" +
		"data: \"path\":\"/downloads/ubuntu.iso\"}\n")

	ev, err := ParseSSE(frame)
	if err != nil {
		t.Fatalf("ParseSSE: %v", err)
	}
	if ev.Type != EventDownloadComplete {
		t.Errorf("Type = %q, want download_complete", ev.Type)
	}
	if !ev.Type.Terminal() {
		t.Error("download_complete must be terminal")
	}
	if ev.ResultID != "HASH2" {
		t.Errorf("ResultID = %q, want HASH2", ev.ResultID)
	}
	if ev.Path != "/downloads/ubuntu.iso" {
		t.Errorf("Path = %q, want /downloads/ubuntu.iso", ev.Path)
	}
	if ev.Progress != 1.0 {
		t.Errorf("Progress = %v, want 1.0 for a completed download", ev.Progress)
	}
}

func TestParseSSE_CommentFrameIsNoData(t *testing.T) {
	// A keep-alive / comment-only frame carries no data: line.
	frame := []byte(": keep-alive ping\n")
	_, err := ParseSSE(frame)
	if !errors.Is(err, errNoData) {
		t.Fatalf("err = %v, want errNoData", err)
	}
}

func TestParseSSE_EventNameFallbackWhenBodyOmitsType(t *testing.T) {
	// The JSON body has no event/type field; the SSE "event:" name is the
	// fallback source of the event kind.
	frame := []byte("event: download_progress\n" +
		`data: {"id":"HASH3","progress":55}` + "\n")

	ev, err := ParseSSE(frame)
	if err != nil {
		t.Fatalf("ParseSSE: %v", err)
	}
	if ev.Type != EventDownloadProgress {
		t.Errorf("Type = %q, want download_progress", ev.Type)
	}
	// 55 is treated as a percent and normalized to 0.55.
	if ev.Progress != 0.55 {
		t.Errorf("Progress = %v, want 0.55 (percent normalized)", ev.Progress)
	}
}

func TestParseHookPayload_DownloadComplete(t *testing.T) {
	// A Boba hook POST body for a completed download, using save_path as the
	// result-reference spelling.
	body := []byte(`{"event":"download_complete","id":"HASH4","status":"completed","progress":1.0,"save_path":"/data/movie.mkv"}`)

	ev, err := ParseHookPayload(body)
	if err != nil {
		t.Fatalf("ParseHookPayload: %v", err)
	}
	if ev.Type != EventDownloadComplete {
		t.Errorf("Type = %q, want download_complete", ev.Type)
	}
	if ev.ResultID != "HASH4" {
		t.Errorf("ResultID = %q, want HASH4", ev.ResultID)
	}
	if ev.Path != "/data/movie.mkv" {
		t.Errorf("Path = %q, want /data/movie.mkv (from save_path)", ev.Path)
	}
}

func TestParseHookPayload_DownloadErrorStatusFallback(t *testing.T) {
	// No explicit event field: the kind is derived from status ("failed"), and
	// the error text falls back to "message".
	body := []byte(`{"infohash":"HASH5","status":"failed","message":"no seeders"}`)

	ev, err := ParseHookPayload(body)
	if err != nil {
		t.Fatalf("ParseHookPayload: %v", err)
	}
	if ev.Type != EventDownloadError {
		t.Errorf("Type = %q, want download_error", ev.Type)
	}
	if !ev.Type.Terminal() {
		t.Error("download_error must be terminal")
	}
	if ev.ResultID != "HASH5" {
		t.Errorf("ResultID = %q, want HASH5 (from infohash)", ev.ResultID)
	}
	if ev.Error != "no seeders" {
		t.Errorf("Error = %q, want %q (from message)", ev.Error, "no seeders")
	}
}

func TestParseHookPayload_EmptyIsError(t *testing.T) {
	if _, err := ParseHookPayload([]byte("   ")); err == nil {
		t.Fatal("expected error on empty hook payload")
	}
}

func TestParseHookPayload_BadJSONIsError(t *testing.T) {
	if _, err := ParseHookPayload([]byte("{not json")); err == nil {
		t.Fatal("expected error on malformed hook payload")
	}
}

func TestNormProgress_ClampAndPercent(t *testing.T) {
	cases := []struct {
		in   *float64
		want float64
	}{
		{ptr(0), 0},
		{ptr(0.5), 0.5},
		{ptr(1), 1},
		{ptr(55), 0.55}, // percent
		{ptr(150), 1},   // percent clamped
		{ptr(-3), 0},    // negative clamped
		{nil, 0},        // absent
	}
	for _, c := range cases {
		if got := normProgress(c.in); got != c.want {
			t.Errorf("normProgress(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func ptr(f float64) *float64 { return &f }
