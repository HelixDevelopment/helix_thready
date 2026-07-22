package gateway

import (
	"io"
	"log/slog"
	"net/http"
)

// Server is the Helix Thready REST /v1 gateway: the HTTP surface composing a
// middleware chain over a set of injected Services. It is self-contained — its
// only dependencies are the Service interfaces in services.go — so it compiles
// and tests end-to-end without the sibling domain modules.
type Server struct {
	svc     Services
	signer  *Signer
	idem    *idemStore
	logger  *slog.Logger
	handler http.Handler
}

// Option customises a Server.
type Option func(*Server)

// WithLogger sets the structured access logger.
func WithLogger(l *slog.Logger) Option {
	return func(s *Server) {
		if l != nil {
			s.logger = l
		}
	}
}

// New builds a Server from the injected services and JWT signer.
func New(svc Services, signer *Signer, opts ...Option) *Server {
	s := &Server{
		svc:    svc,
		signer: signer,
		idem:   newIdemStore(),
		logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
	for _, o := range opts {
		o(s)
	}
	s.handler = s.routes()
	return s
}

// ServeHTTP makes Server an http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

// routes builds the /v1 router (Go 1.22 method+path ServeMux patterns) and wraps
// it in the global middleware chain (request-id -> access log -> panic recovery).
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// --- public (security: []) ---
	mux.HandleFunc("GET /v1/health", s.handleHealth)
	mux.HandleFunc("POST /v1/auth/login", s.handleLogin)

	// --- protected: role floor `user` ---
	mux.Handle("GET /v1/channels",
		s.secure(RoleUser, []string{"posts:read"}, s.handleListChannels))
	mux.Handle("GET /v1/channels/{id}/threads",
		s.secure(RoleUser, []string{"posts:read"}, s.handleChannelThreads))
	mux.Handle("GET /v1/posts/{id}",
		s.secure(RoleUser, []string{"posts:read"}, s.handleGetPost))
	mux.Handle("POST /v1/search",
		s.secure(RoleUser, []string{"search:read"}, s.handleSearch))
	mux.Handle("GET /v1/skills",
		s.secure(RoleUser, []string{"skills:read"}, s.handleListSkills))
	mux.Handle("GET /v1/events",
		s.secure(RoleUser, []string{"events:read"}, s.handleEvents))

	// --- protected: role floor `account_admin` (unsafe POSTs get idempotency) ---
	mux.Handle("POST /v1/channels",
		s.secureIdem(RoleAccountAdmin, []string{"posts:write"}, s.handleCreateChannel))
	mux.Handle("POST /v1/posts/{id}/reprocess",
		s.secureIdem(RoleAccountAdmin, []string{"posts:write"}, s.handleReprocess))

	// --- protected: role floor `account_admin` / `root` ---
	mux.Handle("GET /v1/accounts",
		s.secure(RoleAccountAdmin, []string{"accounts:admin"}, s.handleListAccounts))

	// --- fallback: JSON 404 for any unknown route ---
	mux.HandleFunc("/", s.handleNotFound)

	// Global chain: request-id (outer) -> access log -> panic recovery -> mux.
	return chain(mux, s.requestID, s.accessLog, s.recoverPanic)
}

// secure wraps a protected handler: authn (401) -> role floor (403) -> scopes (403).
func (s *Server) secure(role string, scopes []string, h http.HandlerFunc) http.Handler {
	return chain(h, s.authn, s.requireRole(role), s.requireScopes(scopes...))
}

// secureIdem is secure() plus Idempotency-Key acceptance for unsafe POSTs.
func (s *Server) secureIdem(role string, scopes []string, h http.HandlerFunc) http.Handler {
	return chain(h, s.authn, s.requireRole(role), s.requireScopes(scopes...), s.idempotency)
}
