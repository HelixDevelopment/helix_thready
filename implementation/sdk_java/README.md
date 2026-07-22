# Helix Thready — Java SDK (`sdk_java`)

A real, tested Java client for the Helix Thready REST `/v1` control API
(`docs/public/research/mvp/api/openapi.yaml`; realized by `implementation/rest_gateway`).
It mirrors the `/v1` contract and the Go SDK (`implementation/sdk_go`).

- **JDK 21 standard library only** — `java.net.http.HttpClient` for transport,
  `com.sun.net.httpserver.HttpServer` for the mock in tests. No Maven/Gradle,
  no external jars (no Jackson/Gson/JUnit). Self-contained and vendorable.
- **Package** `digital.vasic.thready` → `src/digital/vasic/thready/`.

## Quickstart

```java
import digital.vasic.thready.*;
import java.util.List;

// baseUrl may be an origin ("https://thready.hxd3v.com") or include "/v1"
// ("https://thready.hxd3v.com/v1"); a trailing slash and "/v1" are normalized away.
ThreadyClient client = new ThreadyClient(
        "https://thready.hxd3v.com", // baseUrl (required)
        null,                        // accessToken (JWT bearer, nullable)
        null,                        // apiKey     (X-API-Key, nullable)
        false);                      // allowInsecureHttp

// Log in — the returned access token is stored and auto-attached to later calls.
TokenPair tokens = client.login("user@thready.test", "correct-horse-battery", /* totp */ null);

List<Channel> channels = client.listChannels();
Channel created = client.createChannel("release", "telegram", "@rel"); // auto Idempotency-Key
Post post = client.getPost("9b1e4c00-0000-4000-8000-000000000001");
ProcessingJob job = client.reprocess(post.id());                       // 202 + auto Idempotency-Key
SearchResult hits = client.search("vector database benchmarks",
        "hybrid", 20, List.of("posts", "generated"), true);
List<Skill> skills = client.listSkills();
```

### Authentication with an API key instead of a JWT

```java
ThreadyClient client = new ThreadyClient("https://thready.hxd3v.com", null, "sk-…", false);
```

Auth injection rule: a bearer JWT wins when present (`Authorization: Bearer <jwt>`);
otherwise the API key is sent as `X-API-Key: <key>`.

### Error handling

Every non-2xx response is mapped to a typed `ApiException` decoded from the canonical
envelope `{"error":{"code","message","status","request_id","trace_id"}}`:

```java
try {
    client.getPost("does-not-exist");
} catch (ApiException e) {
    e.code();       // "not_found"
    e.status();     // 404
    e.requestId();  // "req-abc-123"
    e.getMessage(); // "post not found"
    e.retryable();  // false
}
```

## Methods

| Method | HTTP | Notes |
|--------|------|-------|
| `login(email, password, totp)` | `POST /v1/auth/login` | Stores the returned access token on the client. `totp` may be null. |
| `listChannels()` | `GET /v1/channels` | Decodes the `{data,meta}` envelope. |
| `createChannel(name, platform, externalRef[, idempotencyKey])` | `POST /v1/channels` | Auto `Idempotency-Key` (UUID) unless overridden. |
| `getPost(id)` | `GET /v1/posts/{id}` | |
| `reprocess(id[, idempotencyKey])` | `POST /v1/posts/{id}/reprocess` | 202 Accepted → `ProcessingJob`. Auto `Idempotency-Key`. |
| `search(query, mode, topK, sources, rerank)` | `POST /v1/search` | `mode`∈`semantic|keyword|hybrid`; `sources`⊆`posts|generated|assets`. |
| `listSkills()` | `GET /v1/skills` | Decodes the `{data}` envelope. |

## Behavior

- **Retries.** Idempotent GETs retry on transient `503`/`429` (and transport errors)
  with capped exponential backoff: 1 initial attempt + 3 retries. Unsafe POSTs are
  never retried.
- **Idempotency.** Unsafe POSTs (`createChannel`, `reprocess`) stamp a fresh
  `Idempotency-Key` (`java.util.UUID`) unless you pass one explicitly.
- **Insecure-transport guard.** Before sending, the SDK refuses to attach a credential
  to a plaintext-`http` request bound for a non-loopback host, throwing
  `InsecureTransportException` **before any bytes leave the process**. `https` (any host)
  and `http` to a loopback host (`127.0.0.1`, `::1`, `localhost`) are always allowed.
  Pass `allowInsecureHttp = true` to opt out on a trusted network.

## Layout

```
sdk_java/
├── run.sh                      # javac -d out … && java … ThreadyClientTest
├── README.md
├── EVIDENCE.md                 # real captured PASS run (exit 0)
└── src/digital/vasic/thready/
    ├── ThreadyClient.java      # client: auth, retry, idempotency, error mapping, transport guard
    ├── Json.java               # hand-rolled JSON encoder + recursive-descent parser
    ├── ApiException.java       # typed error (code, message, status, requestId, traceId)
    ├── InsecureTransportException.java
    ├── TokenPair.java  Channel.java  Post.java  ProcessingJob.java
    ├── SearchResult.java  SearchHit.java  Skill.java   # typed model records
    └── ThreadyClientTest.java  # JUnit-free runner + com.sun.net.httpserver mock gateway
```

## Run the tests

```
cd implementation/sdk_java
bash run.sh
echo exit=$?    # 0
```

`run.sh` compiles all sources into `out/` and runs the in-file assertion runner
(`ThreadyClientTest`), which stands up a `com.sun.net.httpserver.HttpServer` mock of the
`/v1` gateway on a free loopback port, exercises each method/behavior, prints
`PASS`/`FAIL <name>` and a `N passed / M failed` summary, and exits non-zero if any test
fails. Current status: **20 passed / 0 failed** (see `EVIDENCE.md`).
