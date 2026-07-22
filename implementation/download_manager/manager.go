package downloadmanager

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/url"
	"sync"
	"time"
)

// State is a job's lifecycle state.
type State string

const (
	StateQueued    State = "queued"    // accepted, awaiting a worker
	StateRunning   State = "running"   // actively downloading
	StatePaused    State = "paused"    // suspended by the caller; resumable
	StateSucceeded State = "succeeded" // completed and verified (terminal)
	StateFailed    State = "failed"    // permanent, non-retryable failure (terminal)
	StateRetrying  State = "retrying"  // waiting on backoff before the next attempt
	StateDead      State = "dead"      // retries exhausted (terminal)
)

// IsTerminal reports whether the state is final (fires the completion callback).
func (s State) IsTerminal() bool {
	return s == StateSucceeded || s == StateFailed || s == StateDead
}

// TaskSpec describes one download job.
type TaskSpec struct {
	URL            string // source URL (scheme selects the fetcher)
	DestPath       string // final on-disk destination
	Segments       int    // desired parallel segments (<=0 means 1)
	ExpectedSHA256 string // optional integrity digest (lowercase hex)
	MaxRetries     int    // per-job retry budget; <=0 inherits the manager default
}

// JobUpdate is an immutable snapshot of a job, delivered to callbacks and
// returned by Status/Wait.
type JobUpdate struct {
	JobID     string
	URL       string
	State     State
	BytesDone int64
	Total     int64
	Progress  float64 // 0..1; 0 when total is unknown
	Attempts  int
	Checksum  string
	Err       string
}

// Config configures a Manager.
type Config struct {
	Workers     int           // worker pool size (<=0 means 1)
	MaxRetries  int           // default retry budget per job (0 means no retries)
	BaseBackoff time.Duration // initial backoff (<=0 means 100ms)
	MaxBackoff  time.Duration // backoff ceiling (<=0 means 30s)
	Registry    *Registry     // scheme -> fetcher (nil means DefaultRegistry)

	// OnProgress, if set, is invoked on progress updates. Reported BytesDone is
	// monotonic non-decreasing per job.
	OnProgress func(JobUpdate)
	// OnComplete, if set, is invoked exactly once per job when it reaches a
	// terminal state (succeeded, failed, or dead).
	OnComplete func(JobUpdate)
}

func (c Config) workers() int {
	if c.Workers > 0 {
		return c.Workers
	}
	return 1
}

func (c Config) baseBackoff() time.Duration {
	if c.BaseBackoff > 0 {
		return c.BaseBackoff
	}
	return 100 * time.Millisecond
}

func (c Config) maxBackoff() time.Duration {
	if c.MaxBackoff > 0 {
		return c.MaxBackoff
	}
	return 30 * time.Second
}

// job is the internal, mutable job record.
type job struct {
	id   string
	spec TaskSpec

	mu        sync.Mutex
	state     State
	attempts  int
	bytesDone int64
	total     int64
	progress  float64
	checksum  string
	err       error

	ctx        context.Context
	cancel     context.CancelFunc
	pauseReq   bool
	done       chan struct{}
	completeOn sync.Once
	progMu     sync.Mutex // serializes progress callbacks -> monotonic delivery
	maxSeen    int64
}

func (j *job) snapshotLocked() JobUpdate {
	errStr := ""
	if j.err != nil {
		errStr = j.err.Error()
	}
	return JobUpdate{
		JobID:     j.id,
		URL:       j.spec.URL,
		State:     j.state,
		BytesDone: j.bytesDone,
		Total:     j.total,
		Progress:  j.progress,
		Attempts:  j.attempts,
		Checksum:  j.checksum,
		Err:       errStr,
	}
}

func (j *job) snapshot() JobUpdate {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.snapshotLocked()
}

// Manager is a bounded-worker download engine.
type Manager struct {
	cfg   Config
	queue chan *job

	mu      sync.Mutex
	jobs    map[string]*job
	nextID  int
	started bool
	stopped bool

	baseCtx    context.Context
	baseCancel context.CancelFunc
	workersWG  sync.WaitGroup
}

// New returns a Manager. Call Start before enqueuing.
func New(cfg Config) *Manager {
	if cfg.Registry == nil {
		cfg.Registry = DefaultRegistry()
	}
	baseCtx, baseCancel := context.WithCancel(context.Background())
	return &Manager{
		cfg:        cfg,
		queue:      make(chan *job, 4096),
		jobs:       make(map[string]*job),
		baseCtx:    baseCtx,
		baseCancel: baseCancel,
	}
}

// Start launches the worker pool. It is idempotent.
func (m *Manager) Start() {
	m.mu.Lock()
	if m.started || m.stopped {
		m.mu.Unlock()
		return
	}
	m.started = true
	n := m.cfg.workers()
	m.mu.Unlock()
	for i := 0; i < n; i++ {
		m.workersWG.Add(1)
		go m.worker()
	}
}

func (m *Manager) worker() {
	defer m.workersWG.Done()
	for j := range m.queue {
		m.runJob(j)
	}
}

// Enqueue accepts a new job and returns its id.
func (m *Manager) Enqueue(spec TaskSpec) (string, error) {
	if spec.URL == "" {
		return "", fmt.Errorf("downloadmanager: empty URL")
	}
	if spec.DestPath == "" {
		return "", fmt.Errorf("downloadmanager: empty DestPath")
	}
	if spec.MaxRetries <= 0 {
		spec.MaxRetries = m.cfg.MaxRetries
	}

	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return "", fmt.Errorf("downloadmanager: manager stopped")
	}
	m.nextID++
	id := fmt.Sprintf("job-%d", m.nextID)
	ctx, cancel := context.WithCancel(m.baseCtx)
	j := &job{
		id:     id,
		spec:   spec,
		state:  StateQueued,
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	m.jobs[id] = j
	// Send under the same lock as the m.stopped check and Shutdown's
	// close(m.queue), so the queue cannot be closed between the check and the
	// send (mirrors Resume). Workers drain m.queue via `range` without ever
	// acquiring m.mu, so holding it across the buffered send cannot deadlock.
	m.queue <- j
	m.mu.Unlock()
	return id, nil
}

// Status returns the latest snapshot for jobID.
func (m *Manager) Status(jobID string) (JobUpdate, bool) {
	m.mu.Lock()
	j, ok := m.jobs[jobID]
	m.mu.Unlock()
	if !ok {
		return JobUpdate{}, false
	}
	return j.snapshot(), true
}

// Wait blocks until jobID reaches a terminal state or ctx is done.
func (m *Manager) Wait(ctx context.Context, jobID string) (JobUpdate, error) {
	m.mu.Lock()
	j, ok := m.jobs[jobID]
	m.mu.Unlock()
	if !ok {
		return JobUpdate{}, fmt.Errorf("downloadmanager: unknown job %q", jobID)
	}
	select {
	case <-j.done:
		return j.snapshot(), nil
	case <-ctx.Done():
		return j.snapshot(), ctx.Err()
	}
}

// Pause suspends a queued or running job. On-disk partial data is preserved so
// Resume can continue. Terminal jobs are left unchanged.
func (m *Manager) Pause(jobID string) error {
	m.mu.Lock()
	j, ok := m.jobs[jobID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("downloadmanager: unknown job %q", jobID)
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.state.IsTerminal() || j.state == StatePaused {
		return nil
	}
	j.pauseReq = true
	j.cancel()
	return nil
}

// Resume re-queues a paused job. The fetcher continues from persisted state.
func (m *Manager) Resume(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("downloadmanager: unknown job %q", jobID)
	}
	if m.stopped {
		return fmt.Errorf("downloadmanager: manager stopped")
	}
	j.mu.Lock()
	if j.state != StatePaused {
		j.mu.Unlock()
		return fmt.Errorf("downloadmanager: job %q not paused (%s)", jobID, j.state)
	}
	j.pauseReq = false
	j.ctx, j.cancel = context.WithCancel(m.baseCtx)
	j.state = StateQueued
	j.mu.Unlock()

	m.queue <- j
	return nil
}

// Shutdown stops accepting work, cancels in-flight jobs, and waits for workers
// to drain (or until ctx is done).
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return nil
	}
	m.stopped = true
	close(m.queue)
	m.baseCancel()
	m.mu.Unlock()

	done := make(chan struct{})
	go func() {
		m.workersWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// runJob drives one job through attempts, retries and terminal transitions.
func (m *Manager) runJob(j *job) {
	// A job paused while still queued should not start.
	j.mu.Lock()
	if j.pauseReq {
		j.state = StatePaused
		j.mu.Unlock()
		return
	}
	j.mu.Unlock()

	budget := j.spec.MaxRetries
	for {
		j.mu.Lock()
		j.state = StateRunning
		j.attempts++
		attempt := j.attempts
		jobCtx := j.ctx
		j.mu.Unlock()

		res, err := m.doFetch(j, jobCtx)

		if err == nil {
			m.finish(j, StateSucceeded, res, nil)
			return
		}

		// Distinguish caller-driven pause / manager shutdown from real failures.
		j.mu.Lock()
		paused := j.pauseReq
		j.mu.Unlock()
		if paused {
			j.mu.Lock()
			j.state = StatePaused
			j.err = nil
			j.mu.Unlock()
			return
		}
		if m.baseCtx.Err() != nil {
			// Shutdown: leave the job resumable rather than marking it dead.
			j.mu.Lock()
			j.state = StatePaused
			j.mu.Unlock()
			return
		}

		if IsPermanent(err) {
			m.finish(j, StateFailed, res, err)
			return
		}
		if attempt > budget { // attempt counts tries: budget retries + 1 initial
			m.finish(j, StateDead, res, err)
			return
		}

		j.mu.Lock()
		j.state = StateRetrying
		j.err = err
		j.mu.Unlock()

		if !m.backoffSleep(jobCtx, attempt) {
			// Interrupted during backoff (pause or shutdown).
			j.mu.Lock()
			if j.pauseReq || m.baseCtx.Err() != nil {
				j.state = StatePaused
			}
			j.mu.Unlock()
			return
		}
	}
}

func (m *Manager) doFetch(j *job, ctx context.Context) (FetchResult, error) {
	u, err := url.Parse(j.spec.URL)
	if err != nil {
		return FetchResult{}, Permanent(fmt.Errorf("downloadmanager: parse url: %w", err))
	}
	f, ok := m.cfg.Registry.Fetcher(u.Scheme)
	if !ok {
		return FetchResult{}, Permanent(fmt.Errorf("downloadmanager: no fetcher for scheme %q", u.Scheme))
	}
	req := FetchRequest{
		URL:            u,
		DestPath:       j.spec.DestPath,
		Segments:       j.spec.Segments,
		ExpectedSHA256: j.spec.ExpectedSHA256,
		Progress: func(done, total int64) {
			m.reportProgress(j, done, total)
		},
	}
	return f.Fetch(ctx, req)
}

// reportProgress records progress and invokes OnProgress with monotonic bytes.
func (m *Manager) reportProgress(j *job, done, total int64) {
	j.progMu.Lock()
	defer j.progMu.Unlock()

	if done < j.maxSeen {
		done = j.maxSeen // enforce monotonicity across concurrent segments
	}
	j.maxSeen = done

	j.mu.Lock()
	j.bytesDone = done
	j.total = total
	if total > 0 {
		j.progress = float64(done) / float64(total)
	}
	upd := j.snapshotLocked()
	upd.State = StateRunning
	j.mu.Unlock()

	if m.cfg.OnProgress != nil {
		m.cfg.OnProgress(upd)
	}
}

// finish sets a terminal state and fires OnComplete exactly once.
func (m *Manager) finish(j *job, state State, res FetchResult, err error) {
	j.mu.Lock()
	j.state = state
	j.err = err
	if res.SHA256 != "" {
		j.checksum = res.SHA256
	}
	if state == StateSucceeded {
		if j.total > 0 {
			j.bytesDone = j.total
			j.progress = 1
		} else if res.BytesWritten > 0 {
			j.bytesDone = res.BytesWritten
			j.total = res.BytesWritten
			j.progress = 1
		}
	}
	upd := j.snapshotLocked()
	j.mu.Unlock()

	j.completeOn.Do(func() {
		// Fire the completion callback before signalling done, so a caller
		// unblocked by Wait is guaranteed the callback has already run.
		if m.cfg.OnComplete != nil {
			m.cfg.OnComplete(upd)
		}
		close(j.done)
	})
}

// backoffSleep waits exponentially (with full jitter), aborting if ctx is done.
// It reports true when the full delay elapsed, false when interrupted.
func (m *Manager) backoffSleep(ctx context.Context, attempt int) bool {
	d := m.backoffDuration(attempt)
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func (m *Manager) backoffDuration(attempt int) time.Duration {
	base := m.cfg.baseBackoff()
	maxB := m.cfg.maxBackoff()
	// attempt is the number of tries so far (>=1); grow as base * 2^(attempt-1).
	shift := attempt - 1
	if shift < 0 {
		shift = 0
	}
	if shift > 30 {
		shift = 30
	}
	d := base << uint(shift)
	if d <= 0 || d > maxB {
		d = maxB
	}
	// Full jitter in [0, d].
	return time.Duration(rand.Int64N(int64(d) + 1))
}
