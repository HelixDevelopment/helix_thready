package downloadmanager

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"sync"
)

// ResourceInfo describes a remote resource as discovered by a probe.
type ResourceInfo struct {
	// Size is the total length in bytes, or -1 if the server did not report it.
	Size int64
	// AcceptRanges is true when the server supports HTTP Range requests
	// (and therefore segmented/resumable transfer).
	AcceptRanges bool
	// ETag is the entity tag, if any, used to validate a resumed transfer.
	ETag string
}

// FetchRequest is a single download instruction handed to a [Fetcher].
type FetchRequest struct {
	// URL is the fully-parsed source URL.
	URL *url.URL
	// DestPath is the final on-disk destination. A sibling "<DestPath>.part"
	// and "<DestPath>.dlstate" file are used for in-progress data and resume
	// bookkeeping, then atomically renamed on success.
	DestPath string
	// Segments is the desired number of parallel byte-range segments. Values
	// <= 0 mean 1. Ignored when the server does not support ranges.
	Segments int
	// ExpectedSHA256 is an optional lowercase hex digest. When set, the
	// completed file is verified against it; a mismatch is a permanent error.
	ExpectedSHA256 string
	// Progress, if non-nil, is invoked with cumulative bytes written and the
	// total size (-1 if unknown). It may be called from multiple goroutines;
	// the HTTP fetcher serializes its calls so reported bytes are monotonic.
	Progress func(bytesDone, total int64)
}

// FetchResult reports the outcome of a successful fetch.
type FetchResult struct {
	// BytesWritten is the total number of bytes committed to disk.
	BytesWritten int64
	// SHA256 is the lowercase hex digest of the completed file.
	SHA256 string
	// Resumed is true when the fetch continued from a previously-interrupted
	// partial download rather than starting from zero.
	Resumed bool
}

// Fetcher fetches the bytes of a resource for one or more URL schemes.
// Implementations must honor context cancellation and, where the transport
// allows, support segmented and resumable transfer.
type Fetcher interface {
	// Schemes returns the lowercase URL schemes this fetcher handles.
	Schemes() []string
	// Fetch downloads req.URL to req.DestPath, returning the result or an error.
	// Retryable failures should be returned as plain errors; failures that will
	// never succeed on retry should be wrapped with [Permanent].
	Fetch(ctx context.Context, req FetchRequest) (FetchResult, error)
}

// Registry maps URL schemes to fetchers.
type Registry struct {
	mu       sync.RWMutex
	byScheme map[string]Fetcher
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{byScheme: make(map[string]Fetcher)}
}

// Register associates f with every scheme it reports via Schemes().
func (r *Registry) Register(f Fetcher) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range f.Schemes() {
		r.byScheme[s] = f
	}
}

// Fetcher returns the fetcher registered for scheme, if any.
func (r *Registry) Fetcher(scheme string) (Fetcher, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.byScheme[scheme]
	return f, ok
}

// Schemes returns the sorted list of registered schemes.
func (r *Registry) Schemes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.byScheme))
	for s := range r.byScheme {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// DefaultRegistry returns a registry wired with the real HTTP(S) fetcher plus
// honest stubs for ftp, smb, nfs and webdav (each returning ErrNotImplemented).
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(NewHTTPFetcher())
	r.Register(NewStubFetcher("ftp"))
	r.Register(NewStubFetcher("smb"))
	r.Register(NewStubFetcher("nfs"))
	r.Register(NewStubFetcher("webdav"))
	return r
}

// stubFetcher is an honest placeholder for a not-yet-implemented transport.
// It is the documented reuse point for digital.vasic.filesystem (FTP/SMB/NFS/
// WebDav) per the subsystem design; until that module is wired in it refuses
// work rather than pretending to succeed.
type stubFetcher struct{ schemes []string }

// NewStubFetcher returns a Fetcher that always fails with ErrNotImplemented for
// the given schemes.
func NewStubFetcher(schemes ...string) Fetcher {
	cp := append([]string(nil), schemes...)
	return &stubFetcher{schemes: cp}
}

func (s *stubFetcher) Schemes() []string { return s.schemes }

func (s *stubFetcher) Fetch(_ context.Context, req FetchRequest) (FetchResult, error) {
	scheme := ""
	if req.URL != nil {
		scheme = req.URL.Scheme
	}
	return FetchResult{}, fmt.Errorf("%w: %q", ErrNotImplemented, scheme)
}
