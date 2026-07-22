package metubewebhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPStatusSource_EndToEnd(t *testing.T) {
	var gotPath string
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jobs":[
			{"id":"d1","status":"downloading","percent":10},
			{"id":"f1","status":"finished","filename":"/out/f1.mp4"}
		]}`))
	}))
	defer mock.Close()

	src := &HTTPStatusSource{BaseURL: mock.URL}
	jobs, err := src.Jobs(context.Background())
	if err != nil {
		t.Fatalf("Jobs: %v", err)
	}
	if gotPath != DefaultJobsPath {
		t.Errorf("polled path = %q, want %q", gotPath, DefaultJobsPath)
	}
	if len(jobs) != 2 {
		t.Fatalf("want 2 jobs, got %d", len(jobs))
	}
	if jobs[1].ID != "f1" || jobs[1].State != StateFinished || jobs[1].ResultPath != "/out/f1.mp4" {
		t.Errorf("finished job = %+v", jobs[1])
	}
}

func TestHTTPStatusSource_Non2xxIsError(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer mock.Close()

	src := &HTTPStatusSource{BaseURL: mock.URL}
	if _, err := src.Jobs(context.Background()); err == nil {
		t.Fatal("expected error on non-2xx MeTube response")
	}
}

func TestHTTPStatusSource_CustomPath(t *testing.T) {
	var gotPath string
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`[]`))
	}))
	defer mock.Close()

	src := &HTTPStatusSource{BaseURL: mock.URL + "/", Path: "/custom/jobs"}
	if _, err := src.Jobs(context.Background()); err != nil {
		t.Fatalf("Jobs: %v", err)
	}
	if gotPath != "/custom/jobs" {
		t.Errorf("path = %q, want /custom/jobs", gotPath)
	}
}
