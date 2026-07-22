# Helix Thready — TypeScript / JavaScript SDK (`@thready/sdk`)

A typed, **stdlib-only** client for the Helix Thready REST `/v1` control API
(schema: `docs/public/research/mvp/api/openapi.yaml`; served by the
`implementation/rest_gateway` module). It speaks the identical `/v1` wire
contract as the sibling Go SDK (`implementation/sdk_go`) and Python SDK
(`implementation/sdk_py`).

- **No dependencies.** Runtime imports only Node built-ins (`node:http`,
  `node:https`, `node:net`, `node:crypto`). There is nothing to `npm install`.
- **No build step.** The client is authored in ESM (`src/client.mjs`); the
  TypeScript deliverable is the hand-written `src/client.d.ts` typings, so you
  run and test the SDK directly with `node`.
- **Batteries included.** Auth injection (bearer JWT or API key), JSON
  encode/decode, canonical error mapping to a typed `ApiError`, transparent
  retries for idempotent GETs, an automatic `Idempotency-Key` on unsafe POSTs,
  and a credential-leak transport guard.

Requires Node 18+ (developed and verified on Node 24).

## Quickstart

```js
import { ThreadyClient, ApiError } from "./src/client.mjs";

const client = new ThreadyClient({
  baseUrl: "https://thready.hxd3v.com", // gateway origin (methods add /v1/…)
  timeoutMs: 15_000,
  // Either start with an API key…
  apiKey: "sk-…",
  // …or leave credentials empty and call login() below.
});

// Password login stores the returned access token on the client, so every
// later call authenticates automatically as "Authorization: Bearer <jwt>".
await client.login({
  email: "user@thready.test",
  password: "userpassword-123",
  // totp: "123456", // required for admin tiers
});

try {
  const channels = await client.listChannels();
  for (const ch of channels) {
    console.log(`channel ${ch.id} (${ch.name}) on ${ch.platform}`);
  }

  const results = await client.search({
    query: "self-hosted vector database benchmarks",
    mode: "hybrid",
    sources: ["posts", "generated"],
    topK: 20,
    rerank: true,
  });
  console.log(`${results.results.length} hits from ${results.embedder}`);
} catch (err) {
  if (err instanceof ApiError) {
    // Every non-2xx maps to a typed ApiError.
    console.error(`thready ${err.code} (${err.status}): ${err.message} [request_id=${err.requestId}]`);
  } else {
    throw err;
  }
}
```

## Configuration

`new ThreadyClient({ … })`:

| Field | Meaning |
|-------|---------|
| `baseUrl` | Gateway origin, e.g. `https://thready.hxd3v.com` (required; trailing slash trimmed). Methods append the versioned path `/v1/…`. |
| `accessToken` | JWT bearer access token → sent as `Authorization: Bearer …`. |
| `apiKey` | Scoped API key → sent as `X-API-Key: …` (for non-interactive use). |
| `timeoutMs` | Per-request timeout in milliseconds (default `30000`), bounding the connect phase too. |
| `allowInsecureHttp` | Permit attaching credentials over plaintext http to a non-loopback host. Default `false`. |

If both `accessToken` and `apiKey` are set, **the bearer token wins**. A
successful `login()` updates the in-flight access token.

## Methods

| Method | HTTP | Returns |
|--------|------|---------|
| `login({email, password, totp?})` | `POST /v1/auth/login` | `TokenPair` (also stored on the client) |
| `listChannels()` | `GET /v1/channels` | `Channel[]` |
| `createChannel({name, platform?, externalRef?}, {idempotencyKey?}?)` | `POST /v1/channels` | `Channel` (sends `Idempotency-Key`) |
| `getPost(postId)` | `GET /v1/posts/{id}` | `Post` |
| `reprocess(postId, {idempotencyKey?}?)` | `POST /v1/posts/{id}/reprocess` | `Job` (sends `Idempotency-Key`) |
| `search({query, mode?, topK?, sources?, rerank?})` | `POST /v1/search` | `SearchResults` |
| `listSkills()` | `GET /v1/skills` | `Skill[]` |

Returned objects use the gateway's **wire field names** (snake_case, e.g.
`access_token`, `external_ref`, `top_k`/`took_ms`), matching the Go SDK's JSON
tags so a decode needs no transformation layer. See `src/client.d.ts` for the
full typed surface.

### Idempotency

`createChannel` and `reprocess` are unsafe POSTs and always send an
`Idempotency-Key`. A fresh UUIDv4 (`node:crypto` `randomUUID`) is generated per
call unless you supply your own for cross-process idempotency:

```js
const job = await client.reprocess("post-1", { idempotencyKey: "my-stable-key" });
```

### Retries

Idempotent `GET`s are retried transparently on transient `503`/`429` (and
transport errors) with capped exponential backoff. Unsafe methods (`POST`) are
**never** retried. After retries are exhausted the original typed `ApiError` is
thrown.

### Errors

Every non-2xx response decodes from the canonical
`{"error":{"code","message","status","request_id",…}}` envelope into a typed
`ApiError` (`code`, `message`, `status`, `requestId`, `traceId`, `retryAfter`,
`details`). Recover it with `err instanceof ApiError`; `err.retryable()` reports
whether the code is one the SDK considers transiently retryable.

### Security — transport guard

The SDK refuses to attach a credential (bearer token or API key) to a
plaintext-`http` request bound for a **non-loopback** host, throwing
`InsecureTransportError` **before** any bytes leave the process — so a secret is
never leaked to an on-path observer. `https` (any host) and `http` to a loopback
host (`127.0.0.1`, `::1`, `localhost`) are always allowed. Set
`allowInsecureHttp: true` to opt out on a trusted network.

## Testing

```sh
cd implementation/sdk_ts
node --check src/client.mjs   # syntax check
node --test                   # run the suite
```

The tests exercise the SDK against a **real `node:http` server** that mocks the
`/v1` contract on a free loopback port — the correct unit-test strategy for a
client library. They assert the exact request the SDK sends (method, path,
headers, body) and the typed value it decodes back; they do **not** boot the
live gateway. See `EVIDENCE.md` for the captured `node --test` run.
