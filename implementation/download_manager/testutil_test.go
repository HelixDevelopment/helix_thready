package downloadmanager

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// rangeServer is a real HTTP handler that serves a fixed byte slice with genuine
// HTTP Range support, optional per-chunk delay/flush (to make interruption
// deterministic), and an optional "ignore ranges" mode.
type rangeServer struct {
	data     []byte
	etag     string
	delay    time.Duration // sleep between flushed chunks
	chunk    int           // write granularity; <=0 means whole body at once
	noRanges bool          // if true, always answer 200 with the full body
}

func (rs *rangeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	total := int64(len(rs.data))
	if rs.etag != "" {
		w.Header().Set("ETag", rs.etag)
	}
	w.Header().Set("Content-Type", "application/octet-stream")

	rangeHdr := r.Header.Get("Range")
	if rs.noRanges || rangeHdr == "" {
		if !rs.noRanges {
			w.Header().Set("Accept-Ranges", "bytes")
		}
		w.Header().Set("Content-Length", strconv.FormatInt(total, 10))
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		rs.writeBody(w, rs.data)
		return
	}

	w.Header().Set("Accept-Ranges", "bytes")
	start, end, ok := parseTestRange(rangeHdr, total)
	if !ok {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", total))
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	seg := rs.data[start : end+1]
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
	w.Header().Set("Content-Length", strconv.Itoa(len(seg)))
	w.WriteHeader(http.StatusPartialContent)
	if r.Method == http.MethodHead {
		return
	}
	rs.writeBody(w, seg)
}

func (rs *rangeServer) writeBody(w http.ResponseWriter, b []byte) {
	cs := rs.chunk
	if cs <= 0 {
		cs = len(b)
	}
	if cs <= 0 {
		return
	}
	fl, _ := w.(http.Flusher)
	for off := 0; off < len(b); off += cs {
		end := off + cs
		if end > len(b) {
			end = len(b)
		}
		if _, err := w.Write(b[off:end]); err != nil {
			return // client went away (e.g. cancelled context)
		}
		if fl != nil {
			fl.Flush()
		}
		if rs.delay > 0 {
			time.Sleep(rs.delay)
		}
	}
}

// parseTestRange parses a single "bytes=start-end" / "bytes=start-" / "bytes=-N"
// header into inclusive absolute offsets.
func parseTestRange(h string, total int64) (int64, int64, bool) {
	const p = "bytes="
	if !strings.HasPrefix(h, p) {
		return 0, 0, false
	}
	spec := h[len(p):]
	if strings.Contains(spec, ",") {
		return 0, 0, false
	}
	dash := strings.IndexByte(spec, '-')
	if dash < 0 {
		return 0, 0, false
	}
	startS, endS := spec[:dash], spec[dash+1:]
	if startS == "" { // suffix range: bytes=-N
		n, err := strconv.ParseInt(endS, 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, false
		}
		if n > total {
			n = total
		}
		return total - n, total - 1, true
	}
	start, err := strconv.ParseInt(startS, 10, 64)
	if err != nil {
		return 0, 0, false
	}
	end := total - 1
	if endS != "" {
		end, err = strconv.ParseInt(endS, 10, 64)
		if err != nil {
			return 0, 0, false
		}
	}
	if start < 0 || start >= total {
		return 0, 0, false
	}
	if end >= total {
		end = total - 1
	}
	if end < start {
		return 0, 0, false
	}
	return start, end, true
}

// flakyHandler fails the first `fail` requests with 500, then delegates.
type flakyHandler struct {
	inner http.Handler
	fail  int64
	count atomic.Int64
}

func (f *flakyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if f.count.Add(1) <= f.fail {
		http.Error(w, "temporary failure", http.StatusInternalServerError)
		return
	}
	f.inner.ServeHTTP(w, r)
}

// failHandler always returns 500.
func failHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "permanent test failure", http.StatusInternalServerError)
	})
}

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return b
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
