package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// listEnvelope is the standard list wrapper: {"data":[...], "meta":{...}}.
type listEnvelope struct {
	Data any      `json:"data"`
	Meta listMeta `json:"meta"`
}

type listMeta struct {
	NextCursor    *string `json:"next_cursor"`
	TotalEstimate *int    `json:"total_estimate"`
}

func newList(data any, total int) listEnvelope {
	return listEnvelope{Data: data, Meta: listMeta{NextCursor: nil, TotalEstimate: &total}}
}

// decodeJSON strictly decodes a JSON request body, rejecting unknown fields and
// trailing data with a 400 invalid_argument.
func decodeJSON(r *http.Request, dst any) *apiError {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return newError(CodeInvalidArgument, "malformed JSON body")
	}
	return nil
}

// ----- GET /v1/health -----

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "rest-gateway",
		"version": "v1",
		"time":    time.Unix(1_700_000_000, 0).UTC().Format(time.RFC3339),
	})
}

// ----- POST /v1/auth/login -----

// loginRequest is the credential body.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	TOTP     string `json:"totp"`
}

// tokenPair is the login/refresh success body.
type tokenPair struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
}

const (
	accessTTL  = 15 * time.Minute
	refreshTTL = 7 * 24 * time.Hour
)

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if aerr := decodeJSON(r, &req); aerr != nil {
		writeError(w, r, aerr.Code, aerr.Message, aerr.Details...)
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, r, CodeInvalidArgument, "email and password are required",
			Detail{Field: "email", Reason: "required"})
		return
	}
	principal, ok := s.svc.Auth.Authenticate(req.Email, req.Password, req.TOTP)
	if !ok {
		writeError(w, r, CodeUnauthenticated, "invalid credentials")
		return
	}
	access, err := s.signer.Sign(Claims{
		Sub: principal.UserID, Role: principal.Role, AccountID: principal.AccountID,
		Scopes: principal.Scopes, TokenType: "access",
	}, accessTTL)
	if err != nil {
		writeError(w, r, CodeInternal, "failed to mint access token")
		return
	}
	refresh, err := s.signer.Sign(Claims{
		Sub: principal.UserID, Role: principal.Role, AccountID: principal.AccountID,
		Scopes: principal.Scopes, TokenType: "refresh",
	}, refreshTTL)
	if err != nil {
		writeError(w, r, CodeInternal, "failed to mint refresh token")
		return
	}
	writeJSON(w, http.StatusOK, tokenPair{
		AccessToken:      access,
		RefreshToken:     refresh,
		TokenType:        "Bearer",
		ExpiresIn:        int(accessTTL.Seconds()),
		RefreshExpiresIn: int(refreshTTL.Seconds()),
	})
}

// ----- GET /v1/channels -----

func (s *Server) handleListChannels(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	scope := p.AccountID
	if p.Role == RoleRoot {
		scope = "" // root sees all tenants
	}
	channels := s.svc.Channels.List(scope)
	writeJSON(w, http.StatusOK, newList(channels, len(channels)))
}

// ----- POST /v1/channels -----

func (s *Server) handleCreateChannel(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	var in ChannelInput
	if aerr := decodeJSON(r, &in); aerr != nil {
		writeError(w, r, aerr.Code, aerr.Message, aerr.Details...)
		return
	}
	if in.Name == "" {
		writeError(w, r, CodeInvalidArgument, "channel name is required",
			Detail{Field: "name", Reason: "required"})
		return
	}
	ch, err := s.svc.Channels.Create(p.AccountID, in)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, ch)
}

// ----- GET /v1/channels/{id}/threads -----

func (s *Server) handleChannelThreads(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	threads, ok := s.svc.Channels.Threads(id)
	if !ok {
		writeError(w, r, CodeNotFound, "channel not found")
		return
	}
	writeJSON(w, http.StatusOK, newList(threads, len(threads)))
}

// ----- GET /v1/posts/{id} -----

func (s *Server) handleGetPost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	post, ok := s.svc.Posts.Get(id)
	if !ok {
		writeError(w, r, CodeNotFound, "post not found")
		return
	}
	writeJSON(w, http.StatusOK, post)
}

// ----- POST /v1/posts/{id}/reprocess -----

func (s *Server) handleReprocess(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, err := s.svc.Posts.Reprocess(id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

// ----- POST /v1/search -----

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if aerr := decodeJSON(r, &req); aerr != nil {
		writeError(w, r, aerr.Code, aerr.Message, aerr.Details...)
		return
	}
	if req.Query == "" {
		writeError(w, r, CodeInvalidArgument, "query is required",
			Detail{Field: "query", Reason: "required"})
		return
	}
	if req.TopK < 0 || req.TopK > 100 {
		writeError(w, r, CodeInvalidArgument, "top_k must be between 1 and 100",
			Detail{Field: "top_k", Issue: "must be 1..100", Reason: "out_of_range"})
		return
	}
	res, err := s.svc.Search.Search(req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ----- GET /v1/skills -----

func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	skills := s.svc.Skills.List()
	writeJSON(w, http.StatusOK, newList(skills, len(skills)))
}

// ----- GET /v1/accounts (root / account_admin only) -----

func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	accounts := s.svc.Accounts.List(p)
	writeJSON(w, http.StatusOK, newList(accounts, len(accounts)))
}

// ----- GET /v1/events (SSE stream) -----

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, r, CodeInternal, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	ch, cancel := s.svc.Events.Subscribe(r.Context())
	defer cancel()

	// Initial comment confirms the subscription is live (per event-bus-contract.md §5).
	fmt.Fprintf(w, ": subscribed\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, open := <-ch:
			if !open {
				return
			}
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			// Standard id:/event:/data: SSE framing.
			fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", ev.ID, ev.Type, data)
			flusher.Flush()
		}
	}
}

// ----- 404 fallback -----

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, CodeNotFound, "no route matches "+r.Method+" "+r.URL.Path)
}

// writeServiceError renders an error returned by a service. A structured
// *apiError keeps its code; anything else becomes 500 internal (never leaking
// internals to the client).
func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var aerr *apiError
	if errors.As(err, &aerr) {
		writeError(w, r, aerr.Code, aerr.Message, aerr.Details...)
		return
	}
	writeError(w, r, CodeInternal, "internal server error")
}
