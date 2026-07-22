# Helix Thready — Python SDK

A typed, **standard-library-only** Python client for the Helix Thready REST
`/v1` control API. It is the Python sibling of [`../sdk_go`](../sdk_go) and
mirrors the same `/v1` contract
(`docs/public/research/mvp/api/openapi.yaml`, realized by
[`../rest_gateway`](../rest_gateway)).

- **Zero dependencies.** Only `urllib.request`, `json`, `uuid` at runtime;
  `unittest` + `http.server` for the tests. Nothing to `pip install`.
- **Self-contained.** Imports no sibling `implementation/*` module — vendor
  `client.py` on its own.
- **Typed.** Requests/responses are dataclasses; every non-2xx becomes a typed
  `ApiError`.
- **Safe by default.** Refuses to put a credential on cleartext HTTP to a
  non-loopback host.

Requires Python 3.8+ (developed and verified on 3.13).

## Install

Copy `client.py` into your project (or add this directory to `PYTHONPATH`).
There is no package to install.

```python
from client import ThreadyClient, ApiError, InsecureTransportError
```

## Quickstart

```python
from client import ThreadyClient, ApiError

# 1. Construct against the gateway origin (the /v1 prefix is added per-path).
client = ThreadyClient("https://thready.hxd3v.com")

# 2a. Interactive: log in. The returned access token is stored on the client,
#     so every later call authenticates automatically as `Authorization: Bearer`.
tokens = client.login("user@thready.test", "correct-horse-battery")
print(tokens.access_token, tokens.expires_in)

# 2b. Or non-interactive: hand it a scoped API key (sent as `X-API-Key`).
client = ThreadyClient("https://thready.hxd3v.com", api_key="sk-…")

# 3. Call the typed methods.
for ch in client.list_channels():
    print(ch.id, ch.name, ch.platform)

ch = client.create_channel("release", platform="telegram", external_ref="@rel")

post = client.get_post("9b1e4c00-0000-4000-8000-000000000001")
print(post.status, post.hashtags)

job = client.reprocess(post.id)          # 202 -> queued Job
print(job.job_id, job.precedence)        # ["download","convert","analyze","research","reply"]

results = client.search(
    "vector database benchmarks",
    mode="hybrid",
    sources=["posts", "generated"],
    top_k=20,
    rerank=True,
)
print(results.embedder, results.took_ms)
for hit in results.results:
    print(hit.source_id, hit.score, hit.snippet)

for skill in client.list_skills():
    print(skill.name, skill.sort_order)

# 4. Typed error handling.
try:
    client.get_post("does-not-exist")
except ApiError as e:
    print(e.code, e.status, e.request_id, e.message)   # e.g. not_found 404 req-… "post not found"
    if e.retryable():
        ...  # rate_limited / unavailable / deadline_exceeded
```

## Constructor

```python
ThreadyClient(
    base_url,                      # gateway origin, e.g. "https://thready.hxd3v.com"
    access_token=None,             # JWT -> "Authorization: Bearer <jwt>"
    api_key=None,                  # API key -> "X-API-Key: <key>"
    timeout=30.0,                  # per-request timeout, seconds
    allow_insecure_http=False,     # permit credentials over cleartext http to remote hosts
)
```

Tunables after construction (defaults mirror the Go SDK): `client.max_retries`
(3), `client.backoff_base` (0.025 s), `client.backoff_max` (2.0 s).

## Methods

| Method | HTTP | Notes |
|--------|------|-------|
| `login(email, password, totp=None)` | `POST /v1/auth/login` | returns `TokenPair`; stores the access token on the client |
| `list_channels()` | `GET /v1/channels` | returns `list[Channel]` |
| `create_channel(name, platform="", external_ref="", idempotency_key=None)` | `POST /v1/channels` | unsafe POST → auto `Idempotency-Key` (UUIDv4); returns `Channel` |
| `get_channel_threads(channel_id)` | `GET /v1/channels/{id}/threads` | returns `list[Thread]` |
| `get_post(post_id)` | `GET /v1/posts/{id}` | returns `Post` |
| `reprocess(post_id, idempotency_key=None)` | `POST /v1/posts/{id}/reprocess` | unsafe POST → auto `Idempotency-Key`; returns `Job` |
| `search(query, mode=None, top_k=None, sources=None, rerank=None)` | `POST /v1/search` | returns `SearchResults` |
| `list_skills()` | `GET /v1/skills` | returns `list[Skill]` |

### Behaviour

- **Auth injection.** A bearer `access_token` (set at construction or after
  `login()`) is sent as `Authorization: Bearer <jwt>`. Otherwise an `api_key` is
  sent as `X-API-Key`. If both are set, **bearer wins**. A call with no
  credential (e.g. `login()` on a fresh client) sends neither.
- **JSON encode/decode.** Request bodies are JSON; 2xx responses decode into the
  typed dataclasses; `204 No Content` decodes to `None`.
- **Typed errors.** Any non-2xx response is raised as an `ApiError` parsed from
  the canonical envelope `{"error":{"code","message","status","request_id",…}}`,
  exposing `.code`, `.message`, `.status`, `.request_id`, `.trace_id`,
  `.retry_after`, `.details`, and `.retryable()`.
- **Retries.** Idempotent `GET`s are retried on `503`/`429` (and transient
  transport errors) with capped exponential backoff — `1 + max_retries` attempts
  total. Unsafe `POST`s are **never** retried.
- **Idempotency.** `create_channel` and `reprocess` auto-stamp a UUIDv4
  `Idempotency-Key`; pass `idempotency_key=` to supply your own.
- **Insecure-transport guard.** Attaching a credential over cleartext `http` to a
  **non-loopback** host raises `InsecureTransportError` *before any bytes leave
  the process*. `https` (any host) and `http` to `127.0.0.1` / `localhost` /
  `::1` are always allowed; pass `allow_insecure_http=True` to override the
  refusal.

## Types

`TokenPair`, `Channel`, `Thread`, `Post`, `Job`, `SearchHit`, `SearchResults`,
`Skill` (dataclasses). Errors: `ThreadyError` (base), `ApiError`,
`TransportError`, `InsecureTransportError`.

## Run the tests

```
cd implementation/sdk_py
python3 -c "import client"     # smoke import
python3 -m unittest -v         # 29 tests, expect "OK"
```

The suite starts a real `http.server` mock of the `/v1` gateway on a free
loopback port and asserts both the request each method sends (method, path,
injected auth header, `Idempotency-Key`) and the typed value it decodes back —
plus `ApiError` mapping, GET retry/exhaustion, POST-not-retried, and the full
insecure-transport matrix. See [`EVIDENCE.md`](./EVIDENCE.md) for the captured
run.
