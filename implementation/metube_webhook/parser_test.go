package metubewebhook

import (
	"testing"
)

func TestParseJobs_ObjectShape(t *testing.T) {
	body := []byte(`{"jobs":[
		{"id":"j1","status":"downloading","percent":42.5,"filename":"clip.mp4"},
		{"id":"j2","status":"finished","filename":"/out/done.mp4"}
	]}`)

	jobs, err := ParseJobs(body)
	if err != nil {
		t.Fatalf("ParseJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("want 2 jobs, got %d", len(jobs))
	}

	if jobs[0].ID != "j1" || jobs[0].State != StateDownloading {
		t.Errorf("job0 = %+v", jobs[0])
	}
	if got, want := jobs[0].Progress, 0.425; got != want {
		t.Errorf("job0 progress = %v, want %v", got, want)
	}
	if jobs[1].State != StateFinished || jobs[1].ResultPath != "/out/done.mp4" {
		t.Errorf("job1 = %+v", jobs[1])
	}
	// A finished job with no percent is fully complete.
	if got := jobs[1].Progress; got != 1.0 {
		t.Errorf("finished job progress = %v, want 1.0", got)
	}
}

func TestParseJobs_ArrayShape(t *testing.T) {
	body := []byte(`[{"id":"a","status":"pending"}]`)
	jobs, err := ParseJobs(body)
	if err != nil {
		t.Fatalf("ParseJobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != "a" || jobs[0].State != StatePending {
		t.Fatalf("unexpected: %+v", jobs)
	}
}

func TestParseJobs_ErrorMessageFromMsgFallback(t *testing.T) {
	body := []byte(`[{"id":"e","status":"error","msg":"ffmpeg failed"}]`)
	jobs, err := ParseJobs(body)
	if err != nil {
		t.Fatalf("ParseJobs: %v", err)
	}
	if jobs[0].State != StateError {
		t.Fatalf("state = %q, want error", jobs[0].State)
	}
	if jobs[0].Error != "ffmpeg failed" {
		t.Errorf("error = %q, want %q", jobs[0].Error, "ffmpeg failed")
	}
}

func TestParseJobs_ErrorFieldPreferredOverMsg(t *testing.T) {
	body := []byte(`[{"id":"e","status":"error","error":"real error","msg":"secondary"}]`)
	jobs, err := ParseJobs(body)
	if err != nil {
		t.Fatalf("ParseJobs: %v", err)
	}
	if jobs[0].Error != "real error" {
		t.Errorf("error = %q, want %q", jobs[0].Error, "real error")
	}
}

func TestParseJobs_ProgressClamped(t *testing.T) {
	body := []byte(`[
		{"id":"hi","status":"downloading","percent":150},
		{"id":"lo","status":"downloading","percent":-10}
	]`)
	jobs, err := ParseJobs(body)
	if err != nil {
		t.Fatalf("ParseJobs: %v", err)
	}
	if jobs[0].Progress != 1.0 {
		t.Errorf("high clamp = %v, want 1.0", jobs[0].Progress)
	}
	if jobs[1].Progress != 0.0 {
		t.Errorf("low clamp = %v, want 0.0", jobs[1].Progress)
	}
}

func TestParseJobs_StateMapping(t *testing.T) {
	cases := map[string]struct {
		state    JobState
		terminal bool
	}{
		"pending":        {StatePending, false},
		"downloading":    {StateDownloading, false},
		"postprocessing": {StatePostprocessing, false},
		"finished":       {StateFinished, true},
		"error":          {StateError, true},
	}
	for wire, want := range cases {
		body := []byte(`[{"id":"x","status":"` + wire + `"}]`)
		jobs, err := ParseJobs(body)
		if err != nil {
			t.Fatalf("ParseJobs(%s): %v", wire, err)
		}
		if jobs[0].State != want.state {
			t.Errorf("%s -> %q, want %q", wire, jobs[0].State, want.state)
		}
		if jobs[0].State.Terminal() != want.terminal {
			t.Errorf("%s terminal = %v, want %v", wire, jobs[0].State.Terminal(), want.terminal)
		}
		if !jobs[0].State.Valid() {
			t.Errorf("%s should be Valid()", wire)
		}
	}
}

func TestParseJobs_EmptyBody(t *testing.T) {
	if _, err := ParseJobs([]byte("   ")); err == nil {
		t.Fatal("expected error on empty body")
	}
}

func TestParseJobs_BadJSON(t *testing.T) {
	if _, err := ParseJobs([]byte(`{not json`)); err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}
