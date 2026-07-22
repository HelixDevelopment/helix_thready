# Helix Thready — REST API Gateway (`digital.vasic.restgateway`)

The **HTTP surface** for Helix Thready: a self-contained Go module that mounts
the `/v1` REST API described in `docs/public/research/mvp/api/` (`openapi.yaml`,
`authn-authz.md`, `event-bus-contract.md`, `error-model.md`). It implements the
edge concerns — routing, JWT auth, RBAC, idempotency, SSE event streaming, and a
single consistent error envelope — over a set of **injected service interfaces**.

- **Module path:** `digital.vasic.restgateway`
- **Go:** 1.26, **stdlib only** (no third-party dependencies)
- **Router:** `net/http` with Go 1.22+ method+path `ServeMux` patterns
- **JWT:** hand-rolled HS256 verify/sign via `crypto/hmac` + `encoding/base64`,
  with an algorithm pin on verify (rejects `alg:none` / algorithm-confusion)

> **Self-contained by design.** The gateway defines its own `Services`
> interfaces (Auth, Channels, Posts, Search, Skills, Accounts, Events) and ships
> honest **in-memory** implementations so the whole surface is testable
> end-to-end. Wiring these to the sibling `implementation/*` domain modules is a
> later `go.work` step. See `EVIDENCE.md` for the no-bluff scope statement.

## Endpoints

| Method & path | Auth | RBAC floor | Notes |
|---------------|------|-----------|-------|
| `GET  /v1/health` | none | — | Liveness probe |
| `POST /v1/auth/login` | none | — | → access + refresh JWT (`TokenPair`) |
| `GET  /v1/channels` | Bearer | `user` | List channels (tenant-scoped; root sees all) |
| `POST /v1/channels` | Bearer | `account_admin` | Register a channel — accepts `Idempotency-Key` |
| `GET  /v1/channels/{id}/threads` | Bearer | `user` | Thread list for a channel |
| `GET  /v1/posts/{id}` | Bearer | `user` | Single post |
| `POST /v1/posts/{id}/reprocess` | Bearer | `account_admin` | → **202** `ProcessingJob` — accepts `Idempotency-Key` |
| `POST /v1/search` | Bearer | `user` | → **200** ranked results (`SearchResponse`) |
| `GET  /v1/skills` | Bearer | `user` | Skill-Graph list |
| `GET  /v1/events` | Bearer | `user` | **SSE** stream (`text/event-stream`) |
| `GET  /v1/accounts` | Bearer | `account_admin` / `root` | Root sees all; account-admin sees own |
| any unknown route | — | — | → **404** JSON error envelope |

## Middleware chain

Applied in order per request (global → per-route):

1. **request-id** — assigns/echoes `X-Request-Id`; seeds the error/log correlation id.
2. **structured access log** — one `slog` JSON record per request (method, path, status, duration, request_id).
3. **panic recovery** — a handler panic becomes `500 internal` through the envelope.
4. **JWT bearer auth** — missing/invalid/expired/tampered → `401 unauthenticated`; resolves the `Principal` into the request context.
5. **RBAC role floor** — insufficient tier → `403 permission_denied` (root > account_admin > user).
6. **scope enforcement** — missing required scope → `403 permission_denied`.
7. **Idempotency-Key** (unsafe POSTs) — replay same key + body returns the cached original result (no duplicate side effect); same key + different body → `409 conflict`.

## Authentication & RBAC

- **Login:** `POST /v1/auth/login {email, password, totp?}` → `{access_token, refresh_token, token_type:"Bearer", expires_in:900, refresh_expires_in:604800}`. Admin tiers (root, account_admin) require a TOTP code.
- **Bearer:** send `Authorization: Bearer <access_token>` on every protected route.
- **Roles:** `root` (all tenants) > `account_admin` (own account) > `user`. A route requires **both** its role floor and its scope set.
- **Scopes:** `posts:read`, `posts:write`, `search:read`, `skills:read`, `events:read`, `accounts:admin`, `root:admin`, … (mirrors authn-authz.md §7).

### Seeded credentials (in-memory backend)

| Email | Password | TOTP | Role | Account |
|-------|----------|------|------|---------|
| `root@thready.test` | `rootpassword-12` | `123456` | root | acct-a |
| `admin@thready.test` | `adminpassword-12` | `654321` | account_admin | acct-a |
| `user@thready.test` | `userpassword-123` | — | user | acct-a |

## Error model

Every non-2xx response is the single envelope (see `error-model.md`):

```json
{ "error": { "code": "permission_denied", "message": "...", "request_id": "…",
             "status": 403, "trace_id": "…", "details": [] } }
```

`code` is the stable machine value application code branches on; the HTTP status
is a mirror. Codes: `invalid_argument`, `unprocessable`, `unauthenticated`,
`permission_denied`, `not_found`, `already_exists`, `conflict`,
`failed_precondition`, `rate_limited`, `deadline_exceeded`, `unavailable`,
`internal`.

## Layout

```
rest_gateway/
├── go.mod                 # module digital.vasic.restgateway (go 1.26)
├── server.go              # Server, routing (/v1 ServeMux), middleware composition
├── middleware.go          # request-id, access log, recovery, authn, RBAC, scopes, idempotency
├── handlers.go            # per-endpoint handlers (incl. SSE), JSON helpers
├── services.go            # Service interfaces + seeded in-memory implementations
├── token.go               # hand-rolled HS256 JWT sign/verify (stdlib only)
├── errors.go              # error codes, status mirror, envelope writers
├── gateway_test.go        # end-to-end tests via net/http/httptest (-race)
├── cmd/gateway/main.go    # runnable standalone server (seeded in-memory backend)
├── EVIDENCE.md            # captured build/vet/fmt/test output + honest verdict
└── README.md
```

## Run

```bash
cd implementation/rest_gateway

# Build + verify
go build ./...
go vet ./...
gofmt -l .            # empty output = clean

# Tests (end-to-end, race detector, no cache)
go test ./... -v -race -count=1

# Run the standalone server (in-memory backend)
GATEWAY_ADDR=127.0.0.1:8080 go run ./cmd/gateway
# then, e.g.:
curl -s http://127.0.0.1:8080/v1/health
```

---

*Made with love ♥ by Helix Development.*
