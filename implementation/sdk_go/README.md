# Helix Thready — Go SDK client (`digital.vasic.threadysdk`)

A typed, stdlib-only Go client for the Helix Thready REST `/v1` control API
(schema: `docs/public/research/mvp/api/openapi.yaml`; served by the
`implementation/rest_gateway` module).

- **Stdlib only.** No third-party dependencies; no sibling `implementation/*`
  imports. It talks to the gateway purely over HTTP and can be vendored alone.
- **Typed end to end.** Request and response structs mirror the gateway's wire
  shapes, so a decode is one line and errors are a single typed value.
- **Batteries included.** Auth injection, JSON encode/decode, canonical error
  mapping, transparent retries for idempotent GETs, automatic `Idempotency-Key`
  on unsafe POSTs, and an SSE event subscription.

## Install / build

This is a standalone module. A parent `implementation/go.work` intentionally
does **not** include it, so run every Go command with `GOWORK=off`:

```sh
cd implementation/sdk_go
GOWORK=off go build ./...
GOWORK=off go test ./... -race -count=1
```

Import path: `digital.vasic.threadysdk` (package `thready`).

## Quickstart

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	thready "digital.vasic.threadysdk"
)

func main() {
	client, err := thready.New(thready.Config{
		BaseURL: "https://thready.hxd3v.com/v1",
		Timeout: 15 * time.Second,
		// Either start with an API key…
		APIKey: "sk-…",
		// …or leave credentials empty and call Login below.
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Password login stores the returned access token on the client, so every
	// later call authenticates automatically as "Authorization: Bearer <jwt>".
	if _, err := client.Login(ctx, thready.LoginRequest{
		Email:    "user@thready.test",
		Password: "userpassword-123",
		// TOTP is required for admin tiers:
		// TOTP: "123456",
	}); err != nil {
		log.Fatal(err)
	}

	channels, err := client.ListChannels(ctx)
	if err != nil {
		// Every non-2xx maps to a typed *APIError.
		var apiErr *thready.APIError
		if errors.As(err, &apiErr) {
			log.Fatalf("thready %s (%d): %s [request_id=%s]",
				apiErr.Code, apiErr.Status, apiErr.Message, apiErr.RequestID)
		}
		log.Fatal(err)
	}
	for _, ch := range channels {
		fmt.Printf("channel %s (%s) on %s\n", ch.ID, ch.Name, ch.Platform)
	}

	// Subscribe to the live event stream (Server-Sent Events). Cancel the
	// context to unsubscribe; the channel closes when the stream ends.
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	events, err := client.SubscribeEvents(streamCtx)
	if err != nil {
		log.Fatal(err)
	}
	for ev := range events {
		fmt.Printf("event %s type=%s payload=%v\n", ev.ID, ev.Type, ev.Payload)
	}
}
```

## Configuration

`thready.Config`:

| Field | Meaning |
|-------|---------|
| `BaseURL` | Gateway origin, e.g. `https://thready.hxd3v.com/v1` (required; trailing slash trimmed). |
| `AccessToken` | JWT bearer access token → sent as `Authorization: Bearer …`. |
| `APIKey` | Scoped API key → sent as `X-API-Key: …` (for non-interactive use). |
| `HTTPClient` | Optional `*http.Client` override. |
| `Timeout` | Per-request timeout when `HTTPClient` is nil (default 30s). Not applied to the long-lived `SubscribeEvents` stream, which is bounded by its context. |

If both `AccessToken` and `APIKey` are set, the bearer token wins. A successful
`Login` updates the in-flight access token.

## Methods

| Method | HTTP | Returns |
|--------|------|---------|
| `Login(ctx, LoginRequest)` | `POST /v1/auth/login` | `*TokenPair` (also stored on the client) |
| `ListChannels(ctx)` | `GET /v1/channels` | `[]Channel` |
| `CreateChannel(ctx, CreateChannelRequest, …opts)` | `POST /v1/channels` | `*Channel` (sends `Idempotency-Key`) |
| `GetChannelThreads(ctx, channelID)` | `GET /v1/channels/{id}/threads` | `[]Thread` |
| `GetPost(ctx, postID)` | `GET /v1/posts/{id}` | `*Post` |
| `Reprocess(ctx, postID, …opts)` | `POST /v1/posts/{id}/reprocess` | `*Job` (sends `Idempotency-Key`) |
| `Search(ctx, SearchRequest)` | `POST /v1/search` | `*SearchResults` |
| `ListSkills(ctx)` | `GET /v1/skills` | `[]Skill` |
| `SubscribeEvents(ctx)` | `GET /v1/events` (SSE) | `<-chan Event` |

### Idempotency

`CreateChannel` and `Reprocess` are unsafe POSTs and always send an
`Idempotency-Key`. A fresh UUIDv4 is generated per call unless you supply your
own for cross-process idempotency:

```go
job, err := client.Reprocess(ctx, "post-1",
	thready.WithIdempotencyKey("my-stable-key"))
```

### Retries

Idempotent `GET`s are retried transparently on transient `503`/`429` (and
transport errors) with capped exponential backoff (respecting context
cancellation). Unsafe methods (`POST`) are **never** retried. After retries are
exhausted the original typed `*APIError` is returned.

### Errors

Every non-2xx response decodes from the canonical
`{"error":{"code","message","status","request_id",…}}` envelope into a typed
`*APIError`:

```go
type APIError struct {
	Code       Code     // stable machine code (e.g. not_found, rate_limited)
	Message    string
	Status     int      // mirrored HTTP status
	RequestID  string   // for support / log correlation
	TraceID    string
	RetryAfter *int
	Details    []Detail
}
```

Recover it with `errors.As`; `apiErr.Retryable()` reports whether the code is
one the SDK considers transiently retryable.

## Testing note

The SDK's tests exercise it against a `net/http/httptest` server that mocks the
`/v1` contract — the correct unit-test strategy for a client library. They
assert the exact request the SDK sends (method, path, headers, body) and the
typed value it decodes back; they do **not** boot the live gateway. See
`EVIDENCE.md` for the captured `build` / `vet` / `gofmt` / `test -race` run.
