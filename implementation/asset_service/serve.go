package assetservice

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

// PrincipalFunc extracts an authenticated [Principal] from a request (e.g. by
// verifying a bearer token). Returning an error denies the request with 401.
// The Asset Service does not authenticate on its own — this is where an upstream
// auth layer (digital.vasic.userservice) plugs in.
type PrincipalFunc func(r *http.Request) (Principal, error)

// Handler serves assets over HTTP through the [Resolver], enforcing the
// "never a direct file path" rule: the URL carries an opaque asset id, the
// bytes come back RBAC-gated and integrity-verified, and Range requests are
// honored via http.ServeContent (HTTP 206 Partial Content).
//
// Routes handled (path suffix after the mount point):
//
//	GET {id}                 -> raw original, Range-capable
//	GET {id}/web             -> the "…-web" rendition (if present)
type Handler struct {
	resolver *Resolver
	auth     PrincipalFunc
}

// NewHandler builds a Handler over resolver, using auth to identify callers.
// If auth is nil, every request is treated as an empty (unauthenticated)
// principal — which the deny-by-default [Authorizer] will reject.
func NewHandler(resolver *Resolver, auth PrincipalFunc) *Handler {
	if auth == nil {
		auth = func(*http.Request) (Principal, error) { return Principal{}, nil }
	}
	return &Handler{resolver: resolver, auth: auth}
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id, rendition := parseAssetPath(r.URL.Path)
	if id == "" {
		http.Error(w, "missing asset id", http.StatusBadRequest)
		return
	}

	principal, err := h.auth(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var (
		reader      seekReadCloser
		name        string
		contentType string
		resolveErr  error
	)
	if rendition == "" {
		var a Asset
		reader, a, resolveErr = h.resolver.Resolve(r.Context(), principal, id)
		if resolveErr == nil {
			name = a.OriginalName
			contentType = a.ContentType
		}
	} else {
		var rd Rendition
		reader, rd, resolveErr = h.resolver.ResolveRendition(r.Context(), principal, id, rendition)
		if resolveErr == nil {
			name = rd.Name
			contentType = rd.ContentType
		}
	}
	if resolveErr != nil {
		writeResolveError(w, resolveErr)
		return
	}
	defer reader.Close()

	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	// http.ServeContent handles Range parsing, Content-Range, 206 vs 200,
	// If-Range, and HEAD — all from the seekable reader. modTime is fixed
	// (content-addressed bytes are immutable) so conditional requests are sane.
	http.ServeContent(w, r, name, time.Unix(0, 0).UTC(), reader)
}

// seekReadCloser is the reader type ServeContent needs (io.ReadSeeker) plus the
// Close we own.
type seekReadCloser interface {
	Read([]byte) (int, error)
	Seek(int64, int) (int64, error)
	Close() error
}

// writeResolveError maps resolver sentinel errors to HTTP status codes.
func writeResolveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		http.Error(w, "forbidden", http.StatusForbidden)
	case errors.Is(err, ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, ErrIntegrity):
		http.Error(w, "integrity error", http.StatusInternalServerError)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// parseAssetPath extracts the asset id and optional rendition from the request
// path. It reads the LAST one or two non-empty segments so the handler works
// regardless of the mount prefix (e.g. "/v1/assets/{id}" or "/{id}/web").
func parseAssetPath(p string) (id, rendition string) {
	parts := make([]string, 0, 4)
	for _, seg := range strings.Split(p, "/") {
		if seg != "" {
			parts = append(parts, seg)
		}
	}
	if len(parts) == 0 {
		return "", ""
	}
	last := parts[len(parts)-1]
	if last == "web" && len(parts) >= 2 {
		return parts[len(parts)-2], "web"
	}
	return last, ""
}
