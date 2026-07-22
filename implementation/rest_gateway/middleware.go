package gateway

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"sync"
	"time"
)

// ----- context plumbing -----

type ctxKey int

const (
	ctxRequestID ctxKey = iota
	ctxPrincipal
)

func requestIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(ctxRequestID).(string); ok {
		return v
	}
	return ""
}

func principalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxPrincipal).(Principal)
	return p, ok
}

// ----- RBAC roles -----

const (
	RoleUser         = "user"
	RoleAccountAdmin = "account_admin"
	RoleRoot         = "root"
)

// roleRank encodes the role floor ordering: root > account_admin > user.
func roleRank(role string) int {
	switch role {
	case RoleRoot:
		return 3
	case RoleAccountAdmin:
		return 2
	case RoleUser:
		return 1
	default:
		return 0
	}
}

// ----- request-id -----

// requestID assigns a stable id to every request (honoring an inbound
// X-Request-Id when present) and echoes it on the response. It runs first so
// every downstream log line and error envelope is correlatable.
func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-Id")
		if rid == "" {
			rid = newRequestID()
		}
		w.Header().Set("X-Request-Id", rid)
		ctx := context.WithValue(r.Context(), ctxRequestID, rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fall back to a time-derived id; ids are for correlation, not security.
		return "req-" + itoa(int(time.Now().UnixNano()))
	}
	return hex.EncodeToString(b[:])
}

// ----- access log -----

// statusRecorder captures the status code for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wrote {
		s.status = code
		s.wrote = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wrote {
		s.WriteHeader(http.StatusOK)
	}
	return s.ResponseWriter.Write(b)
}

// Flush lets the SSE handler stream through the access-log wrapper.
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// accessLog emits one structured log record per request (method, path, status,
// duration, request_id).
func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.logger.Info("access",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", requestIDFrom(r.Context()),
		)
	})
}

// ----- panic recovery -----

// recoverPanic turns a handler panic into a 500 internal error rendered through
// the canonical envelope (with the request_id), instead of a dropped connection.
func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.logger.Error("panic recovered",
					"request_id", requestIDFrom(r.Context()),
					"panic", rec,
				)
				writeError(w, r, CodeInternal, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ----- authn (JWT bearer) -----

// authn requires a valid Bearer JWT. Missing/invalid/expired -> 401. On success
// the resolved Principal is placed in the request context.
func (s *Server) authn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		h := r.Header.Get("Authorization")
		if len(h) <= len(prefix) || h[:len(prefix)] != prefix {
			writeError(w, r, CodeUnauthenticated, "missing or malformed Authorization header")
			return
		}
		claims, err := s.signer.Verify(h[len(prefix):])
		if err != nil {
			writeError(w, r, CodeUnauthenticated, "invalid or expired token")
			return
		}
		if claims.TokenType == "refresh" {
			writeError(w, r, CodeUnauthenticated, "refresh token is not valid for API access")
			return
		}
		p := Principal{
			UserID:    claims.Sub,
			Role:      claims.Role,
			AccountID: claims.AccountID,
			Scopes:    claims.Scopes,
		}
		ctx := context.WithValue(r.Context(), ctxPrincipal, p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ----- authz: role floor -----

// requireRole enforces a role floor. Insufficient tier -> 403.
func (s *Server) requireRole(min string) func(http.Handler) http.Handler {
	floor := roleRank(min)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := principalFrom(r.Context())
			if !ok {
				writeError(w, r, CodeUnauthenticated, "authentication required")
				return
			}
			if roleRank(p.Role) < floor {
				writeError(w, r, CodePermissionDenied, "role '"+p.Role+"' is below the required tier '"+min+"'")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ----- authz: scopes -----

// requireScopes enforces that the principal holds every required scope.
func (s *Server) requireScopes(required ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := principalFrom(r.Context())
			if !ok {
				writeError(w, r, CodeUnauthenticated, "authentication required")
				return
			}
			held := map[string]struct{}{}
			for _, sc := range p.Scopes {
				held[sc] = struct{}{}
			}
			for _, need := range required {
				if _, has := held[need]; !has {
					writeError(w, r, CodePermissionDenied, "missing required scope '"+need+"'")
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ----- idempotency -----

// idemRecord is a cached response for an (principal, method, path, key) tuple.
type idemRecord struct {
	bodyHash [32]byte
	status   int
	body     []byte
}

// idemStore is the in-memory idempotency ledger (the API-plane-owned table in
// production). Entries would expire after 24h; the in-memory version keeps them
// for process lifetime, which is sufficient for the contract behaviour.
type idemStore struct {
	mu      sync.Mutex
	entries map[string]idemRecord
}

func newIdemStore() *idemStore { return &idemStore{entries: map[string]idemRecord{}} }

func (s *idemStore) get(key string) (idemRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.entries[key]
	return r, ok
}

func (s *idemStore) put(key string, r idemRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = r
}

// idempotency implements Idempotency-Key acceptance on unsafe POSTs. A replay
// with the same key + body returns the cached original response WITHOUT calling
// the handler again (so no duplicate side effect). A same-key/different-body
// replay is rejected with 409 conflict.
func (s *Server) idempotency(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}
		p, _ := principalFrom(r.Context())

		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, r, CodeInvalidArgument, "unable to read request body")
			return
		}
		_ = r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))
		bodyHash := sha256.Sum256(body)

		storeKey := p.UserID + " " + r.Method + " " + r.URL.Path + " " + key
		if rec, ok := s.idem.get(storeKey); ok {
			if rec.bodyHash != bodyHash {
				writeError(w, r, CodeConflict, "Idempotency-Key reused with a different request body")
				return
			}
			// Replay the cached original result; the handler never runs again.
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Idempotency-Replayed", "true")
			w.WriteHeader(rec.status)
			_, _ = w.Write(rec.body)
			return
		}

		cw := &captureWriter{ResponseWriter: w, status: http.StatusOK, buf: &bytes.Buffer{}}
		next.ServeHTTP(cw, r)
		// Only cache successful (2xx) responses so a transient failure can be retried.
		if cw.status >= 200 && cw.status < 300 {
			s.idem.put(storeKey, idemRecord{bodyHash: bodyHash, status: cw.status, body: cw.buf.Bytes()})
		}
	})
}

// captureWriter writes through to the client while buffering the body/status so
// the idempotency layer can cache the original response for replay.
type captureWriter struct {
	http.ResponseWriter
	status int
	buf    *bytes.Buffer
	wrote  bool
}

func (c *captureWriter) WriteHeader(code int) {
	if !c.wrote {
		c.status = code
		c.wrote = true
	}
	c.ResponseWriter.WriteHeader(code)
}

func (c *captureWriter) Write(b []byte) (int, error) {
	if !c.wrote {
		c.WriteHeader(http.StatusOK)
	}
	c.buf.Write(b)
	return c.ResponseWriter.Write(b)
}

// chain composes middlewares so mw[0] is the OUTERMOST (runs first).
func chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}
