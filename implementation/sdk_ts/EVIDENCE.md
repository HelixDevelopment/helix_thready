# EVIDENCE — Helix Thready TypeScript / JS SDK (`@thready/sdk`)

Real, captured syntax-check + test output for the typed TypeScript/ESM SDK
client of the Helix Thready REST `/v1` control API. Stdlib-only (Node built-ins
`node:http`, `node:https`, `node:net`, `node:crypto`), no external dependencies,
no build step. Verified on Node **v24.18.0**.

## Build/test discipline

The runtime is ESM (`src/client.mjs`); the TypeScript deliverable is the
hand-written `src/client.d.ts` typings, so there is nothing to transpile — the
client and its tests run directly under `node`.

```
$ cd implementation/sdk_ts
$ node --check src/client.mjs && node --test
```

## Captured output (2026-07-22)

### Toolchain

```
$ node --version
v24.18.0
```

### `node --check src/client.mjs`

```
(exit 0; no output = the ESM module parses cleanly)
```

### `node --test`

```
✔ login sends credentials and a subsequent call carries the bearer token (14.4525ms)
✔ listChannels injects the bearer header and decodes the list envelope (2.260978ms)
✔ createChannel sends an auto Idempotency-Key + JSON body and decodes the channel (1.851399ms)
✔ createChannel honours an explicit Idempotency-Key override (1.523323ms)
✔ getPost hits GET /v1/posts/{id} and decodes a typed post (1.370783ms)
✔ reprocess hits POST /v1/posts/{id}/reprocess, sends a key, decodes the job (1.343832ms)
✔ search POSTs the body (topK→top_k) and decodes ranked results (2.236641ms)
✔ listSkills decodes the list envelope (1.258863ms)
✔ API-key auth sends X-API-Key and NOT Authorization (1.308478ms)
✔ bearer wins when both an access token and an API key are set (1.311984ms)
✔ a 404 canonical envelope maps to a typed ApiError (code/status/requestId) (1.397473ms)
✔ a non-envelope error body degrades to a status-derived ApiError (1.06473ms)
✔ an idempotent GET retries 503 → 200 (exactly 2 server calls) (2.492588ms)
✔ an idempotent GET retries 429 then succeeds (2.339196ms)
✔ a GET that stays 503 exhausts retries and throws the typed ApiError (1 + maxRetries calls) (9.543388ms)
✔ an unsafe POST is NOT retried on 503 (exactly 1 server call) (1.211902ms)
✔ the constructor requires baseUrl (0.305672ms)
✔ ApiError renders a log-correlatable string and reports retryability (0.134303ms)
✔ insecure-transport guard: http + remote host + credentials is refused before any send (0.254134ms)
✔ insecure-transport guard: http + 127.0.0.1 + credentials is allowed and sends the bearer (2.172324ms)
✔ insecure-transport guard: http + localhost + credentials is allowed (loopback) (1.431034ms)
✔ insecure-transport guard: https + remote + credentials is allowed through the guard (151.373052ms)
✔ insecure-transport guard: allowInsecureHttp opts into http + remote + credentials (151.672316ms)
✔ insecure-transport guard: no credential ⇒ no refusal even on remote http (11.075114ms)
ℹ tests 24
ℹ suites 0
ℹ pass 24
ℹ fail 0
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 458.417667
```

**24 tests, all pass, 0 fail, 0 skipped.**

The two ~151 ms tests are the `https + remote` and `allowInsecureHttp + remote`
guard cases: they intentionally let the request through the guard to the
transport, which then fails fast with a bounded (`timeoutMs: 150`) NON-guard
network error to a non-routable literal IP (proving the guard did **not**
refuse). Everything else is sub-10 ms against a real loopback mock server.

## What is (and isn't) proven

- **Tested against a contract-mock, not a live gateway.** The correct unit-test
  approach for a client SDK is to exercise it against a **real `node:http`**
  server (bound to a free `127.0.0.1` port) that mocks the gateway's `/v1`
  contract — asserting the exact method/path/headers/body the SDK **sends** and
  the typed value it **decodes** from a canned response. These tests do **not**
  boot the real `rest_gateway` process, a database, or an external network. The
  mock's request/response shapes mirror the gateway's actual wire format (the
  same shapes the sibling Go SDK's tests assert), so the contract verified here
  is the one the running gateway serves.
- **Every task-required path is exercised, none skipped:**
  - each method sends the right method + path (asserted server-side):
    `login`→`POST /v1/auth/login`, `listChannels`→`GET /v1/channels`,
    `createChannel`→`POST /v1/channels`, `getPost`→`GET /v1/posts/{id}`,
    `reprocess`→`POST /v1/posts/{id}/reprocess`, `search`→`POST /v1/search`,
    `listSkills`→`GET /v1/skills`;
  - auth is injected — `Authorization: Bearer <jwt>` after `login()`/`accessToken`,
    or `X-API-Key` for an API key, and **bearer wins** when both are set;
  - `Idempotency-Key` is present on unsafe POSTs (`createChannel`, `reprocess`),
    auto-generated as a UUIDv4 (asserted by regex) or overridable per call;
  - a non-2xx canonical envelope decodes to a typed `ApiError` with the right
    `code` / `status` / `requestId` (404 → `not_found`), and a non-envelope body
    degrades to a status-derived `ApiError`;
  - an idempotent GET retries `503`→`200` (asserted: exactly **2** server calls)
    and `429`→`200`, and exhausts to a typed `ApiError` after `1 + maxRetries`
    (= **4**) attempts;
  - an unsafe POST is **not** retried on 503 (asserted: exactly **1** call);
  - the insecure-transport guard: `http` + remote + credentials →
    `InsecureTransportError` with **nothing sent**; `http` + `127.0.0.1`/`localhost`
    → allowed (bearer header verified); `https` → allowed; `allowInsecureHttp`
    opts back in; no-credential calls are never refused.

## Files

| File | Role |
|------|------|
| `package.json` | `{"type":"module"}`, no dependencies; `test` = `node --test` |
| `src/client.mjs` | `ThreadyClient`, `ApiError`, `InsecureTransportError`, `Code`; auth injection, JSON encode/decode over `node:http(s)`, retry/backoff, error mapping, transport guard |
| `src/client.d.ts` | hand-written TypeScript typings for the public API (the "TS" deliverable) |
| `test/client.test.mjs` | 24 `node:test` + `node:assert` tests against a real `node:http` `/v1` contract-mock |
| `README.md` | quickstart, config, method table, `node --test` |
| `EVIDENCE.md` | this file |

## Verdict

**READY** — the ESM module parses (`node --check`) and all **24** `node:test`
tests pass (0 fail, 0 skipped) with zero external dependencies and no build
step. The SDK is a self-contained, stdlib-only typed client verified against a
real `node:http` contract-mock of the `/v1` surface.
