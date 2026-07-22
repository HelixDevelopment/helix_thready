# Helix Thready — Server (`thready.server`)

A **real, runnable** Thready server. It serves the REST gateway's `/v1` HTTP
surface backed by the **actual sibling domain modules** — not the gateway's own
in-memory stubs. Every wired `gateway.Service` delegates to genuine sibling code.

- **Module path:** `thready.server`
- **Go:** 1.26 (workspace-resolved; local `replace` directives mirror
  `implementation/integration/go.mod`)
- **Composes:** `gateway.New(services, signer)` from `digital.vasic.restgateway`
- **Depends on (real):** `digital.vasic.userservice`, `digital.vasic.semsearch`,
  `digital.vasic.skilldispatch`, `digital.vasic.eventbusservice`

## What is real vs. an honest local store

| gateway.Service | Backing | Real sibling calls |
|-----------------|---------|--------------------|
| **Auth** | `digital.vasic.userservice` | `NewHasher`/`Hash` (PBKDF2 seed), `Verify` (real PBKDF2 compare), `NewTOTPFromBase32` + `TOTP.Verify` (real RFC 6238) |
| **Search** | `digital.vasic.semsearch` | `NewChunker`/`Chunk`, `NewDeterministicEmbedder`, `NewMemoryIndex`, `NewEngine`, `Engine.Index`, `Engine.Search` (real cosine-KNN + boost) |
| **Skills** | `digital.vasic.skilldispatch` | `NewRegistry`/`Register`, `OrderByPrecedence` (real stage precedence) |
| **Posts** | real in-memory store **+** `skilldispatch` | `Registry.Resolve` + `OrderByPrecedence` for `ProcessingJob.Precedence` |
| **Events** | `digital.vasic.eventbusservice` | `NewDefault`, `SubscribeAll`, `Publish`, `Unsubscribe` (real pub/sub) |
| **Channels** | **honest in-memory store** | none — gateway-level CRUD, no domain module (see EVIDENCE) |
| **Accounts** | **honest in-memory store** | none — gateway-level CRUD, no domain module (see EVIDENCE) |

No domain logic is reimplemented inline: password verification, TOTP validation,
search ranking, and skill precedence are all performed **by the sibling modules**.

## API

```go
// Assemble the real-wired gateway handler.
h, err := server.NewServer()   // returns (http.Handler, error)
```

The `cmd/thready-server` binary serves it on `$PORT` (default `8080`) with graceful
shutdown via `signal.NotifyContext`:

```
cd implementation/server
PORT=8080 go run ./cmd/thready-server
# GET /v1/health, POST /v1/auth/login, POST /v1/search, GET /v1/skills, ...
```

## Seed identities & MFA note

The three gateway seed identities (`root@thready.test`, `admin@thready.test`,
`user@thready.test`) are seeded as **real `userservice.User` records** with
PBKDF2-hashed passwords. The admin tiers (root, account_admin) are MFA-enrolled
with **real RFC 6238 TOTP secrets** exported as `server.SeedRootTOTPSecretB32` /
`server.SeedAdminTOTPSecretB32`.

The gateway's `SeedRootTOTP`/`SeedAdminTOTP` are **static** 6-digit codes and
cannot authenticate against a real time-based verifier, so this server provisions
genuine shared secrets instead. e2e drivers compute the live code with
`userservice.NewTOTPFromBase32(secret).Now()`. See `EVIDENCE.md`.

## Tests

End-to-end over `httptest.NewServer` against the real-wired handler:

1. `POST /v1/auth/login` correct password + live TOTP → **200 + token**; wrong
   password → **401**; wrong TOTP → **401** (both fail through the real verifiers).
2. `POST /v1/search` — a vector-DB query ranks `vectordb.md` top (real cosine);
   a disjoint telegram query ranks `telegram.md` top (negative control).
3. `GET /v1/skills` — real precedence order `download > convert > analyze >
   research > reply`.
4. `POST /v1/channels` then `GET /v1/channels` — the created channel is present.

```
cd implementation/server
go build ./...
go vet ./...
go test ./... -race -count=1
```

See [`EVIDENCE.md`](./EVIDENCE.md) for the verbatim captured run.

---

*Made with love ♥ by Helix Development.*
