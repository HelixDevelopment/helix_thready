package bobaadapter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSSEReader_ParsesMultiFrameStream(t *testing.T) {
	// A live SSE server emitting several frames: a result_found, a comment
	// keep-alive (no data:, must be skipped), a progress update, and a terminal
	// download_complete — frames separated by blank lines per the SSE spec.
	stream := "event: result_found\n" +
		`data: {"result":{"infohash":"H1","title":"A"}}` + "\n" +
		"\n" +
		": keep-alive ping\n" +
		"\n" +
		"event: download_progress\n" +
		`data: {"id":"H1","progress":30}` + "\n" +
		"\n" +
		"event: download_complete\n" +
		`data: {"id":"H1","status":"complete","path":"/dl/a"}` + "\n" +
		"\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Errorf("Accept header = %q, want text/event-stream", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(stream))
	}))
	defer srv.Close()

	src := &SSEReader{BaseURL: srv.URL}
	var got []BobaEvent
	if err := src.Stream(context.Background(), func(ev BobaEvent) error {
		got = append(got, ev)
		return nil
	}); err != nil {
		t.Fatalf("Stream: %v", err)
	}

	// The comment frame is skipped; three real events remain.
	if len(got) != 3 {
		t.Fatalf("got %d events, want 3: %+v", len(got), got)
	}
	if got[0].Type != EventResultFound || got[0].ResultID != "H1" {
		t.Errorf("event 0 = %+v", got[0])
	}
	if got[1].Type != EventDownloadProgress || got[1].Progress != 0.30 {
		t.Errorf("event 1 = %+v", got[1])
	}
	if got[2].Type != EventDownloadComplete || got[2].Path != "/dl/a" {
		t.Errorf("event 2 = %+v", got[2])
	}
}

func TestSSEReader_HandlerErrorStopsStream(t *testing.T) {
	stream := "event: result_found\ndata: {\"result\":{\"infohash\":\"H1\"}}\n\n" +
		"event: result_found\ndata: {\"result\":{\"infohash\":\"H2\"}}\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(stream))
	}))
	defer srv.Close()

	src := &SSEReader{BaseURL: srv.URL}
	count := 0
	stop := io.EOF
	err := src.Stream(context.Background(), func(ev BobaEvent) error {
		count++
		return stop // abort after the first event
	})
	if err != stop {
		t.Fatalf("Stream err = %v, want the handler's error", err)
	}
	if count != 1 {
		t.Fatalf("handler called %d times, want 1 (stream must stop on error)", count)
	}
}

func TestSSEReader_Non2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	src := &SSEReader{BaseURL: srv.URL}
	if err := src.Stream(context.Background(), func(BobaEvent) error { return nil }); err == nil {
		t.Fatal("expected an error on a non-2xx SSE response")
	}
}

func TestHTTPHookRegistrar_RegistersCallbackURL(t *testing.T) {
	var gotPath, gotCallback string
	var gotEvents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			URL    string   `json:"url"`
			Events []string `json:"events"`
		}
		_ = json.Unmarshal(raw, &req)
		gotCallback = req.URL
		gotEvents = req.Events
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"hook-1"}`))
	}))
	defer srv.Close()

	reg := &HTTPHookRegistrar{BaseURL: srv.URL}
	out, err := reg.Register(context.Background(), "https://thready/hooks/boba", "download_complete")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if gotPath != DefaultHooksPath {
		t.Errorf("registered at path %q, want %q", gotPath, DefaultHooksPath)
	}
	if gotCallback != "https://thready/hooks/boba" {
		t.Errorf("callback url = %q", gotCallback)
	}
	if len(gotEvents) != 1 || gotEvents[0] != "download_complete" {
		t.Errorf("events = %v, want [download_complete]", gotEvents)
	}
	if out.ID != "hook-1" {
		t.Errorf("hook id = %q, want hook-1", out.ID)
	}
}

func TestHTTPHookRegistrar_Non2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	reg := &HTTPHookRegistrar{BaseURL: srv.URL}
	if _, err := reg.Register(context.Background(), "https://x/hook"); err == nil {
		t.Fatal("expected an error on a non-2xx hook registration")
	}
}

func TestHTTPHookRegistrar_HookIDFallbackKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hook_id":"hk-42"}`))
	}))
	defer srv.Close()

	reg := &HTTPHookRegistrar{BaseURL: srv.URL}
	out, err := reg.Register(context.Background(), "https://x/hook")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if out.ID != "hk-42" {
		t.Errorf("hook id = %q, want hk-42 (from hook_id fallback)", out.ID)
	}
}
