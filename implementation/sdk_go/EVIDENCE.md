# EVIDENCE — Helix Thready Go SDK (`digital.vasic.threadysdk`)

Real, captured build/vet/format/test output for the typed Go SDK client of the
Helix Thready REST `/v1` control API. Stdlib-only, Go 1.26, self-contained
(imports no sibling `implementation/*` module).

## Build discipline

A parent `implementation/go.work` exists that does **not** list this directory,
so every command runs with `GOWORK=off` — this is a standalone module with no
sibling imports.

```
$ cd implementation/sdk_go
$ GOWORK=off go build ./... && GOWORK=off go vet ./... && GOWORK=off gofmt -l . && GOWORK=off go test ./... -v -race -count=1
```

## Captured output (2026-07-22)

### Toolchain

```
$ go version
go version go1.26.4-X:nodwarf5 linux/amd64
```

### `GOWORK=off go build ./...`

```
(exit 0; no output = success)
```

### `GOWORK=off go vet ./...`

```
(exit 0; no output = success)
```

### `GOWORK=off gofmt -l .`

```
(exit 0; empty output = every file is gofmt-clean)
```

### `GOWORK=off go test ./... -v -race -count=1`

```
=== RUN   TestLogin_SendsCredentialsAndSubsequentCallUsesToken
--- PASS: TestLogin_SendsCredentialsAndSubsequentCallUsesToken (0.00s)
=== RUN   TestListChannels_InjectsBearerAndDecodesEnvelope
--- PASS: TestListChannels_InjectsBearerAndDecodesEnvelope (0.00s)
=== RUN   TestCreateChannel_SendsIdempotencyKeyAndBody
--- PASS: TestCreateChannel_SendsIdempotencyKeyAndBody (0.00s)
=== RUN   TestCreateChannel_WithIdempotencyKeyOverride
--- PASS: TestCreateChannel_WithIdempotencyKeyOverride (0.00s)
=== RUN   TestGetChannelThreads_PathAndDecode
--- PASS: TestGetChannelThreads_PathAndDecode (0.00s)
=== RUN   TestGetPost_DecodesTypedPost
--- PASS: TestGetPost_DecodesTypedPost (0.00s)
=== RUN   TestReprocess_ReturnsJobWithIdempotencyKey
--- PASS: TestReprocess_ReturnsJobWithIdempotencyKey (0.00s)
=== RUN   TestSearch_SendsBodyAndDecodesResults
--- PASS: TestSearch_SendsBodyAndDecodesResults (0.00s)
=== RUN   TestListSkills_DecodesEnvelope
--- PASS: TestListSkills_DecodesEnvelope (0.00s)
=== RUN   TestNon2xx_MapsToTypedAPIError
--- PASS: TestNon2xx_MapsToTypedAPIError (0.00s)
=== RUN   TestRetry_GET_503ThenSuccess
--- PASS: TestRetry_GET_503ThenSuccess (0.00s)
=== RUN   TestRetry_GET_ExhaustedReturnsAPIError
--- PASS: TestRetry_GET_ExhaustedReturnsAPIError (0.01s)
=== RUN   TestPOST_NotRetriedOn503
--- PASS: TestPOST_NotRetriedOn503 (0.00s)
=== RUN   TestSubscribeEvents_ReceivesDecodedSSEEvent
--- PASS: TestSubscribeEvents_ReceivesDecodedSSEEvent (0.00s)
=== RUN   TestSubscribeEvents_Non2xxReturnsAPIError
--- PASS: TestSubscribeEvents_Non2xxReturnsAPIError (0.00s)
=== RUN   TestAPIKeyAuth_SendsXAPIKeyHeader
--- PASS: TestAPIKeyAuth_SendsXAPIKeyHeader (0.00s)
=== RUN   TestNew_RequiresBaseURL
--- PASS: TestNew_RequiresBaseURL (0.00s)
=== RUN   TestAPIError_ErrorStringAndRetryable
--- PASS: TestAPIError_ErrorStringAndRetryable (0.00s)
PASS
ok  	digital.vasic.threadysdk	1.039s
```

**18 tests, all PASS, race detector clean (no data races reported).**

### Coverage

```
$ GOWORK=off go test ./... -race -count=1 -cover
ok  	digital.vasic.threadysdk	1.037s	coverage: 78.5% of statements
```

## What is (and isn't) proven

- **Tested against a contract-mock, not a live gateway.** The correct unit-test
  approach for a client SDK is to exercise it against a `net/http/httptest`
  server that mocks the gateway's `/v1` contract — asserting the exact
  method/path/headers the SDK **sends** and the typed struct it **decodes** from
  a canned response. These tests do **not** boot the real `rest_gateway`
  process, a database, or the network. The mock's request/response shapes are
  copied from the gateway's actual wire format (its handlers/DTOs) so the
  contract asserted here is the same one the running gateway serves.
- **Every task-required test path is exercised, none skipped:**
  - each method sends the right method + path (asserted server-side);
  - auth is injected — `Authorization: Bearer <jwt>` after Login/AccessToken,
    or `X-API-Key` for an API key (and the two are mutually exclusive);
  - `Idempotency-Key` is present on unsafe POSTs (CreateChannel, Reprocess),
    auto-generated (UUIDv4) or overridable via `WithIdempotencyKey`;
  - a non-2xx canonical envelope decodes to a typed `*APIError` with the right
    `code` / `status` / `request_id` (recovered via `errors.As`);
  - an idempotent GET retries `503`→`200` (asserted: exactly 2 server calls),
    and exhausts to a typed `APIError` after `1 + maxRetries` attempts;
  - unsafe POSTs are **not** retried on 503 (asserted: exactly 1 server call);
  - `SubscribeEvents` reads the SSE stream, ignores the `: subscribed`
    heartbeat comment, and decodes a framed `data:{…}` line into an `Event`;
    cancelling the context closes the channel (unsubscribe);
  - `Login` returns a token and a subsequent call carries it as the bearer.

## Files

| File | Role |
|------|------|
| `go.mod` | module `digital.vasic.threadysdk`, `go 1.26` (pre-existing, kept) |
| `errors.go` | `Code` taxonomy, `Detail`, typed `APIError` + `Retryable()` (pre-existing, kept) |
| `types.go` | typed request/response DTOs + list envelope + `PageMeta` |
| `client.go` | `Config`, `Client`, `New`, auth injection, `do()` (encode/decode, retry, backoff), `APIError` mapping |
| `methods.go` | typed methods over `/v1` + `SubscribeEvents` (SSE reader) |
| `client_test.go` | 18 TDD tests against an `httptest` contract-mock server |
| `README.md` | quickstart + method list |
| `EVIDENCE.md` | this file |

## Verdict

**READY** — builds, vets, is gofmt-clean, and all 18 race-enabled tests pass
under `GOWORK=off`. The SDK is a self-contained, stdlib-only typed client
verified against a contract-mock of the `/v1` surface.
