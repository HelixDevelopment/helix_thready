package downloadmanager

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// defaultChunkSize is the read/write granularity for streaming a segment.
const defaultChunkSize = 32 * 1024

// HTTPFetcher is a real HTTP(S) fetcher supporting segmented parallel Range
// downloads, resume from a persisted partial file, and SHA-256 verification.
type HTTPFetcher struct {
	// Client is the HTTP client used for all requests. If nil, a default is used.
	Client *http.Client
	// ChunkSize is the streaming buffer size. If <= 0, defaultChunkSize is used.
	ChunkSize int
}

// NewHTTPFetcher returns an HTTPFetcher with sane defaults.
func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{Client: &http.Client{}, ChunkSize: defaultChunkSize}
}

func (h *HTTPFetcher) client() *http.Client {
	if h.Client != nil {
		return h.Client
	}
	return http.DefaultClient
}

func (h *HTTPFetcher) chunkSize() int {
	if h.ChunkSize > 0 {
		return h.ChunkSize
	}
	return defaultChunkSize
}

// Schemes reports the schemes handled by this fetcher.
func (h *HTTPFetcher) Schemes() []string { return []string{"http", "https"} }

// --- persisted resume state ---

type segState struct {
	Start int64 `json:"start"` // absolute inclusive start offset
	End   int64 `json:"end"`   // absolute inclusive end offset (-1 == until EOF)
	Done  int64 `json:"done"`  // bytes committed, relative to Start
}

func (s segState) complete() bool {
	if s.End < 0 {
		return false // unknown length: never provably complete mid-stream
	}
	return s.Done >= (s.End - s.Start + 1)
}

type dlState struct {
	URL          string     `json:"url"`
	Size         int64      `json:"size"`
	ETag         string     `json:"etag"`
	AcceptRanges bool       `json:"accept_ranges"`
	Segments     []segState `json:"segments"`
}

func (st *dlState) matches(u string, info ResourceInfo) bool {
	if st.URL != u || st.Size != info.Size || st.AcceptRanges != info.AcceptRanges {
		return false
	}
	// If both sides advertise an ETag, they must agree.
	if st.ETag != "" && info.ETag != "" && st.ETag != info.ETag {
		return false
	}
	return true
}

func loadState(path string) *dlState {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var st dlState
	if json.Unmarshal(data, &st) != nil {
		return nil
	}
	return &st
}

func newState(u string, info ResourceInfo, segments int) *dlState {
	st := &dlState{URL: u, Size: info.Size, ETag: info.ETag, AcceptRanges: info.AcceptRanges}
	if !info.AcceptRanges || info.Size <= 0 || segments <= 1 {
		end := int64(-1)
		if info.Size > 0 {
			end = info.Size - 1
		}
		st.Segments = []segState{{Start: 0, End: end, Done: 0}}
		return st
	}
	if segments < 1 {
		segments = 1
	}
	chunk := (info.Size + int64(segments) - 1) / int64(segments)
	for start := int64(0); start < info.Size; start += chunk {
		end := start + chunk - 1
		if end >= info.Size {
			end = info.Size - 1
		}
		st.Segments = append(st.Segments, segState{Start: start, End: end})
	}
	return st
}

// --- session ---

type httpSession struct {
	h         *HTTPFetcher
	req       FetchRequest
	statePath string
	partPath  string

	file *os.File

	mu         sync.Mutex // guards st segment Done fields + state persistence
	st         *dlState
	prog       sync.Mutex // serializes progress callbacks (keeps reported bytes monotonic)
	downloaded atomic.Int64
	chunkTick  atomic.Int64
	resumed    bool
}

// Fetch downloads req.URL to req.DestPath.
func (h *HTTPFetcher) Fetch(ctx context.Context, req FetchRequest) (FetchResult, error) {
	if req.URL == nil {
		return FetchResult{}, Permanent(fmt.Errorf("downloadmanager: nil URL"))
	}
	if req.Segments <= 0 {
		req.Segments = 1
	}
	if req.DestPath == "" {
		return FetchResult{}, Permanent(fmt.Errorf("downloadmanager: empty DestPath"))
	}

	info, err := h.probe(ctx, req.URL)
	if err != nil {
		return FetchResult{}, err
	}

	s := &httpSession{
		h:         h,
		req:       req,
		statePath: req.DestPath + ".dlstate",
		partPath:  req.DestPath + ".part",
	}

	fresh := true
	if loaded := loadState(s.statePath); loaded != nil && loaded.matches(req.URL.String(), info) {
		s.st = loaded
		fresh = false
	} else {
		s.st = newState(req.URL.String(), info, req.Segments)
	}

	// Servers without range support cannot be resumed; always restart clean.
	if !info.AcceptRanges {
		fresh = true
		s.st = newState(req.URL.String(), info, req.Segments)
	}

	f, err := os.OpenFile(s.partPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return FetchResult{}, err
	}
	s.file = f

	if fresh {
		_ = os.Remove(s.statePath)
		if info.Size > 0 {
			if err := f.Truncate(info.Size); err != nil {
				f.Close()
				return FetchResult{}, err
			}
		} else {
			_ = f.Truncate(0)
		}
	}

	// Seed the cumulative counter from any already-committed bytes.
	var already int64
	for i := range s.st.Segments {
		already += s.st.Segments[i].Done
		if s.st.Segments[i].Done > 0 {
			s.resumed = true
		}
	}
	s.downloaded.Store(already)
	s.reportProgress()

	runErr := s.run(ctx)

	if runErr != nil {
		s.persist()
		f.Close()
		return FetchResult{BytesWritten: s.downloaded.Load(), Resumed: s.resumed}, runErr
	}

	if err := f.Sync(); err != nil {
		f.Close()
		return FetchResult{}, err
	}
	if err := f.Close(); err != nil {
		return FetchResult{}, err
	}

	sum, err := sha256File(s.partPath)
	if err != nil {
		return FetchResult{}, err
	}
	if req.ExpectedSHA256 != "" && !strings.EqualFold(sum, req.ExpectedSHA256) {
		return FetchResult{SHA256: sum}, Permanent(
			fmt.Errorf("downloadmanager: checksum mismatch: got %s want %s", sum, req.ExpectedSHA256))
	}

	_ = os.Remove(s.statePath)
	if err := os.Rename(s.partPath, req.DestPath); err != nil {
		return FetchResult{}, err
	}
	return FetchResult{BytesWritten: s.downloaded.Load(), SHA256: sum, Resumed: s.resumed}, nil
}

// run downloads every not-yet-complete segment in parallel and cancels the
// remaining work on the first error.
func (s *httpSession) run(ctx context.Context) error {
	gctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error

	for i := range s.st.Segments {
		if s.st.Segments[i].complete() {
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if err := s.runSegment(gctx, idx); err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
					cancel()
				}
				errMu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	return firstErr
}

func (s *httpSession) runSegment(ctx context.Context, idx int) error {
	s.mu.Lock()
	start := s.st.Segments[idx].Start
	end := s.st.Segments[idx].End
	done := s.st.Segments[idx].Done
	accept := s.st.AcceptRanges
	s.mu.Unlock()

	absStart := start + done
	if end >= 0 && absStart > end {
		return nil // already complete
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.req.URL.String(), nil)
	if err != nil {
		return Permanent(err)
	}
	if accept {
		if end >= 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", absStart, end))
		} else {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", absStart))
		}
	}

	resp, err := s.h.client().Do(req)
	if err != nil {
		return err // retryable (network/context)
	}
	defer resp.Body.Close()

	if err := classifyStatus(resp.StatusCode, accept); err != nil {
		return err
	}

	buf := make([]byte, s.h.chunkSize())
	off := absStart
	for {
		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := s.file.WriteAt(buf[:n], off); wErr != nil {
				return wErr
			}
			off += int64(n)
			s.mu.Lock()
			s.st.Segments[idx].Done = off - start
			s.mu.Unlock()
			s.downloaded.Add(int64(n))
			s.reportProgress()
			s.maybePersist()
		}
		if rErr == io.EOF {
			break
		}
		if rErr != nil {
			return rErr // retryable; includes context cancellation
		}
	}
	s.persist()
	return nil
}

func (s *httpSession) reportProgress() {
	if s.req.Progress == nil {
		return
	}
	s.prog.Lock()
	d := s.downloaded.Load()
	t := s.st.Size
	s.req.Progress(d, t)
	s.prog.Unlock()
}

func (s *httpSession) maybePersist() {
	if s.chunkTick.Add(1)%16 == 0 {
		s.persist()
	}
}

func (s *httpSession) persist() {
	s.mu.Lock()
	data, err := json.Marshal(s.st)
	s.mu.Unlock()
	if err != nil {
		return
	}
	tmp := s.statePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, s.statePath)
}

// probe discovers size and range support using a one-byte range GET.
func (h *HTTPFetcher) probe(ctx context.Context, u *url.URL) (ResourceInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return ResourceInfo{}, Permanent(err)
	}
	req.Header.Set("Range", "bytes=0-0")
	resp, err := h.client().Do(req)
	if err != nil {
		return ResourceInfo{}, err // retryable
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	switch {
	case resp.StatusCode == http.StatusPartialContent: // 206
		total := parseContentRangeTotal(resp.Header.Get("Content-Range"))
		return ResourceInfo{Size: total, AcceptRanges: true, ETag: resp.Header.Get("ETag")}, nil
	case resp.StatusCode == http.StatusOK: // 200 -> no range support
		return ResourceInfo{Size: resp.ContentLength, AcceptRanges: false, ETag: resp.Header.Get("ETag")}, nil
	default:
		return ResourceInfo{}, classifyStatus(resp.StatusCode, true)
	}
}

// classifyStatus turns an HTTP status into nil (ok), a permanent error, or a
// retryable error. When ranges were requested, a 206 (and 200 fallback) is ok.
func classifyStatus(code int, rangedRequest bool) error {
	switch {
	case code == http.StatusOK || code == http.StatusPartialContent:
		return nil
	case code == http.StatusRequestedRangeNotSatisfiable:
		return Permanent(fmt.Errorf("downloadmanager: http %d (range not satisfiable)", code))
	case code >= 400 && code < 500 && code != http.StatusRequestTimeout && code != http.StatusTooManyRequests:
		return Permanent(fmt.Errorf("downloadmanager: http %d", code))
	default:
		// 5xx, 408, 429, and anything else -> retryable.
		return fmt.Errorf("downloadmanager: http %d", code)
	}
}

func parseContentRangeTotal(h string) int64 {
	// Format: "bytes 0-0/12345"
	i := strings.LastIndex(h, "/")
	if i < 0 {
		return -1
	}
	tail := strings.TrimSpace(h[i+1:])
	if tail == "*" || tail == "" {
		return -1
	}
	v, err := strconv.ParseInt(tail, 10, 64)
	if err != nil {
		return -1
	}
	return v
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hsh := sha256.New()
	if _, err := io.Copy(hsh, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hsh.Sum(nil)), nil
}
