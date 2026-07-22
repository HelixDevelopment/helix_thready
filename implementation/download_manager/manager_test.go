package downloadmanager

import (
	"bytes"
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func waitCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func waitForState(t *testing.T, m *Manager, id string, want State, timeout time.Duration) JobUpdate {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if u, ok := m.Status(id); ok && u.State == want {
			return u
		}
		time.Sleep(2 * time.Millisecond)
	}
	u, _ := m.Status(id)
	t.Fatalf("job %s did not reach state %q within %s (last=%q)", id, want, timeout, u.State)
	return JobUpdate{}
}

// Test 4: server returns 500 twice, then 200 -> succeeds within the retry budget.
func TestRetryThenSucceed(t *testing.T) {
	data := randomBytes(t, 96*1024)
	want := sha256Hex(data)

	flaky := &flakyHandler{inner: &rangeServer{data: data, etag: `"retry"`}, fail: 2}
	srv := httptest.NewServer(flaky)
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "retry.bin")
	m := New(Config{
		Workers:     2,
		MaxRetries:  3,
		BaseBackoff: time.Millisecond,
		MaxBackoff:  5 * time.Millisecond,
	})
	m.Start()
	defer m.Shutdown(context.Background())

	id, err := m.Enqueue(TaskSpec{URL: srv.URL + "/retry.bin", DestPath: dest, Segments: 1, ExpectedSHA256: want})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	upd, err := m.Wait(waitCtx(t), id)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if upd.State != StateSucceeded {
		t.Fatalf("state = %q (err=%q), want succeeded", upd.State, upd.Err)
	}
	if upd.Attempts != 3 {
		t.Errorf("attempts = %d, want 3 (two failures then success)", upd.Attempts)
	}
	if upd.Checksum != want {
		t.Errorf("checksum = %s, want %s", upd.Checksum, want)
	}
	got, _ := os.ReadFile(dest)
	if !bytes.Equal(got, data) {
		t.Fatal("downloaded bytes differ from source")
	}
}

// Test 5: progress callback fires with monotonically non-decreasing bytes.
func TestProgressCallbackMonotonic(t *testing.T) {
	data := randomBytes(t, 256*1024)

	srv := httptest.NewServer(&rangeServer{data: data, delay: 500 * time.Microsecond, chunk: 8 * 1024})
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "prog.bin")

	var mu sync.Mutex
	var seq []int64
	var last int64 = -1
	violation := false

	m := New(Config{
		Workers: 1,
		OnProgress: func(u JobUpdate) {
			mu.Lock()
			if u.BytesDone < last {
				violation = true
			}
			last = u.BytesDone
			seq = append(seq, u.BytesDone)
			mu.Unlock()
		},
	})
	m.Start()
	defer m.Shutdown(context.Background())

	id, err := m.Enqueue(TaskSpec{URL: srv.URL + "/prog.bin", DestPath: dest, Segments: 1})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	upd, err := m.Wait(waitCtx(t), id)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if upd.State != StateSucceeded {
		t.Fatalf("state = %q, want succeeded", upd.State)
	}

	mu.Lock()
	defer mu.Unlock()
	if violation {
		t.Fatal("progress bytes were not monotonic")
	}
	if len(seq) < 2 {
		t.Fatalf("expected multiple progress events, got %d", len(seq))
	}
	if seq[len(seq)-1] != int64(len(data)) {
		t.Errorf("final progress = %d, want %d", seq[len(seq)-1], len(data))
	}
}

// Test 6: completion callback fires exactly once with the final state.
func TestCompletionCallbackFiresOnce(t *testing.T) {
	data := randomBytes(t, 128*1024)
	want := sha256Hex(data)

	srv := httptest.NewServer(&rangeServer{data: data})
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "complete.bin")

	var count atomic.Int64
	var mu sync.Mutex
	var final JobUpdate

	m := New(Config{
		Workers: 2,
		OnComplete: func(u JobUpdate) {
			count.Add(1)
			mu.Lock()
			final = u
			mu.Unlock()
		},
	})
	m.Start()
	defer m.Shutdown(context.Background())

	id, err := m.Enqueue(TaskSpec{URL: srv.URL + "/complete.bin", DestPath: dest, Segments: 4, ExpectedSHA256: want})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	upd, err := m.Wait(waitCtx(t), id)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if upd.State != StateSucceeded {
		t.Fatalf("state = %q, want succeeded", upd.State)
	}
	// A brief window in case of any stray extra callbacks.
	time.Sleep(20 * time.Millisecond)
	if n := count.Load(); n != 1 {
		t.Fatalf("completion callback fired %d times, want 1", n)
	}
	mu.Lock()
	defer mu.Unlock()
	if final.State != StateSucceeded {
		t.Errorf("final callback state = %q, want succeeded", final.State)
	}
	if final.Checksum != want {
		t.Errorf("final checksum = %s, want %s", final.Checksum, want)
	}
}

// Test 7: failure past the retry budget -> dead state + completion callback.
func TestFailurePastMaxRetriesDead(t *testing.T) {
	srv := httptest.NewServer(failHandler())
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "dead.bin")

	var count atomic.Int64
	var mu sync.Mutex
	var final JobUpdate

	m := New(Config{
		Workers:     1,
		MaxRetries:  2,
		BaseBackoff: time.Millisecond,
		MaxBackoff:  3 * time.Millisecond,
		OnComplete: func(u JobUpdate) {
			count.Add(1)
			mu.Lock()
			final = u
			mu.Unlock()
		},
	})
	m.Start()
	defer m.Shutdown(context.Background())

	id, err := m.Enqueue(TaskSpec{URL: srv.URL + "/dead.bin", DestPath: dest, Segments: 1})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	upd, err := m.Wait(waitCtx(t), id)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if upd.State != StateDead {
		t.Fatalf("state = %q, want dead", upd.State)
	}
	if upd.Attempts != 3 {
		t.Errorf("attempts = %d, want 3 (initial + 2 retries)", upd.Attempts)
	}
	if n := count.Load(); n != 1 {
		t.Errorf("completion callback fired %d times, want 1", n)
	}
	mu.Lock()
	defer mu.Unlock()
	if final.State != StateDead {
		t.Errorf("final callback state = %q, want dead", final.State)
	}
	if final.Err == "" {
		t.Error("expected a non-empty error on the dead job")
	}
}

// A permanent error (unimplemented scheme) -> failed state, no retries.
func TestPermanentErrorFailedState(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "ftp.bin")

	var count atomic.Int64
	m := New(Config{
		Workers:    1,
		MaxRetries: 5,
		OnComplete: func(JobUpdate) { count.Add(1) },
	})
	m.Start()
	defer m.Shutdown(context.Background())

	id, err := m.Enqueue(TaskSpec{URL: "ftp://example.invalid/file", DestPath: dest})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	upd, err := m.Wait(waitCtx(t), id)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if upd.State != StateFailed {
		t.Fatalf("state = %q, want failed", upd.State)
	}
	if upd.Attempts != 1 {
		t.Errorf("attempts = %d, want 1 (permanent errors are not retried)", upd.Attempts)
	}
	if n := count.Load(); n != 1 {
		t.Errorf("completion callback fired %d times, want 1", n)
	}
}

// Pause mid-download, then Resume to completion (continues from persisted state).
func TestPauseResume(t *testing.T) {
	data := randomBytes(t, 512*1024)
	want := sha256Hex(data)

	srv := httptest.NewServer(&rangeServer{data: data, etag: `"pr"`, delay: time.Millisecond, chunk: 8 * 1024})
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "pauseresume.bin")

	var mgr *Manager
	var jobID atomic.Value
	var pauseOnce sync.Once

	mgr = New(Config{
		Workers: 1,
		OnProgress: func(u JobUpdate) {
			if u.Total > 0 && u.BytesDone*4 >= u.Total {
				if v, ok := jobID.Load().(string); ok {
					pauseOnce.Do(func() { _ = mgr.Pause(v) })
				}
			}
		},
	})
	mgr.Start()
	defer mgr.Shutdown(context.Background())

	id, err := mgr.Enqueue(TaskSpec{URL: srv.URL + "/pr.bin", DestPath: dest, Segments: 1, ExpectedSHA256: want})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	jobID.Store(id)

	// Wait for the job to actually pause.
	waitForState(t, mgr, id, StatePaused, 10*time.Second)

	st := loadState(dest + ".dlstate")
	if st == nil {
		t.Fatal("expected persisted state while paused")
	}
	var partial int64
	for _, s := range st.Segments {
		partial += s.Done
	}
	if partial <= 0 || partial >= int64(len(data)) {
		t.Fatalf("expected partial progress while paused, got %d of %d", partial, len(data))
	}

	if err := mgr.Resume(id); err != nil {
		t.Fatalf("resume: %v", err)
	}
	upd, err := mgr.Wait(waitCtx(t), id)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if upd.State != StateSucceeded {
		t.Fatalf("state = %q (err=%q), want succeeded", upd.State, upd.Err)
	}
	if upd.Checksum != want {
		t.Errorf("checksum = %s, want %s", upd.Checksum, want)
	}
	got, _ := os.ReadFile(dest)
	if !bytes.Equal(got, data) {
		t.Fatal("resumed file differs from source")
	}
}
