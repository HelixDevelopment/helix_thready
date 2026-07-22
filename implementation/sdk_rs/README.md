# Helix Thready — Rust SDK client (`thready`, std-only)

A typed, **standard-library-only** Rust client for the Helix Thready REST `/v1`
control API (schema: `docs/public/research/mvp/api/openapi.yaml`; served by the
`implementation/rest_gateway` module; typed surface mirrors the sibling Go SDK
in `implementation/sdk_go`).

- **std only.** No `cargo`, no crates — no `reqwest`, `serde`, `tokio`, or
  `uuid`. Built directly with `rustc`. Because `std` ships neither an HTTP client
  nor a JSON codec, **both are hand-rolled** here (that is the point):
  - a minimal blocking **HTTP/1.1** client over `std::net::TcpStream`
    (`mod http`);
  - a minimal **JSON** `Value` enum with a recursive-descent parser + compact
    encoder (`mod json`).
- **Typed end to end.** Request/response structs mirror the gateway's wire
  shapes; every non-2xx maps to a single typed `Error`.
- **Batteries included.** Bearer-wins auth injection, canonical error mapping,
  transparent retries for idempotent GETs, an automatic `Idempotency-Key` on
  unsafe POSTs, and an insecure-transport guard.

## std-only transport note (http works, https needs a crate)

`std` has **no TLS**, so the only transport this crate can actually *speak* is
plaintext `http`. Talking to an `https` origin requires an external TLS crate
(e.g. `rustls`/`native-tls`), which is out of scope for a std-only build.

Crucially, the insecure-transport guard **still treats an `https` base URL as
safe** — it never returns `Error::InsecureTransport` for `https`. The guard's
job is to refuse leaking a credential over *plaintext http* to a remote host;
`https` is precisely the case it must not refuse. See
`ThreadyClient::transport_allowed`.

## Build & test

No `Cargo.toml`. Build the test binary with `rustc` directly and run it:

```sh
cd implementation/sdk_rs
bash run.sh
# => rustc --edition 2021 --test src/lib.rs -o target/testbin && ./target/testbin
# => test result: ok. 17 passed; 0 failed
```

To build the library object (non-test) you would use
`rustc --edition 2021 --crate-type=lib src/lib.rs`.

## Quickstart

```rust
use thready::{ThreadyClient, LoginRequest, SearchRequest, Error};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // base_url is the gateway origin (no trailing /v1); the methods add /v1/...
    // Start with an API key, or leave both empty and call login() below.
    let client = ThreadyClient::new(
        "http://127.0.0.1:8080", // origin
        "",                      // access_token (JWT bearer)
        "sk-...",                // api_key (X-API-Key)
        false,                   // allow_insecure_http
    )?;

    // Password login stores the returned access token on the client, so every
    // later call authenticates automatically as "Authorization: Bearer <jwt>".
    let mut login = LoginRequest::new("user@thready.test", "userpassword-123");
    login.totp = Some("123456".into()); // required for admin tiers; else omit
    client.login(&login)?;

    let channels = client.list_channels()?;
    for ch in &channels {
        println!("channel {} ({}) on {}", ch.id, ch.name, ch.platform);
    }

    let mut q = SearchRequest::new("release notes");
    q.mode = Some("hybrid".into());
    q.sources = vec!["posts".into(), "generated".into()];
    q.top_k = Some(10);
    q.rerank = true;
    let results = client.search(&q)?;
    println!("{} hits via {}", results.results.len(), results.embedder);

    // Every non-2xx is one typed error:
    match client.get_post("does-not-exist") {
        Ok(p) => println!("{}", p.body),
        Err(Error::Api { code, status, request_id, .. }) => {
            eprintln!("thready {code} ({status}) [request_id={request_id}]");
        }
        Err(e) => eprintln!("{e}"),
    }
    Ok(())
}
```

## Configuration

`ThreadyClient::new(base_url, access_token, api_key, allow_insecure_http)`:

| Arg | Meaning |
|-----|---------|
| `base_url` | Gateway origin, e.g. `http://127.0.0.1:8080` (required; trailing slash trimmed). The methods append `/v1/...`. |
| `access_token` | JWT bearer access token → sent as `Authorization: Bearer …`. |
| `api_key` | Scoped API key → sent as `X-API-Key: …` (non-interactive use). |
| `allow_insecure_http` | Permit attaching a credential to plaintext http bound for a **non-loopback** host. Default `false`. |

If both `access_token` and `api_key` are set, the **bearer token wins**. A
successful `login` updates the in-flight access token (`access_token()` reads it).

## Methods

| Method | HTTP | Returns |
|--------|------|---------|
| `login(&LoginRequest)` | `POST /v1/auth/login` | `TokenPair` (also stored on the client) |
| `list_channels()` | `GET /v1/channels` | `Vec<Channel>` |
| `create_channel(&CreateChannelRequest)` | `POST /v1/channels` | `Channel` (sends `Idempotency-Key`) |
| `get_post(post_id)` | `GET /v1/posts/{id}` | `Post` |
| `reprocess(post_id)` | `POST /v1/posts/{id}/reprocess` | `Job` (sends `Idempotency-Key`) |
| `search(&SearchRequest)` | `POST /v1/search` | `SearchResults` |
| `list_skills()` | `GET /v1/skills` | `Vec<Skill>` |

### Idempotency

`create_channel` and `reprocess` are unsafe POSTs and always send an
`Idempotency-Key`. The key is a **unique** value minted from
`SystemTime::now()` nanoseconds plus a process-global `AtomicU64` counter,
formatted as hex (`idem-<nanos_hex>-<counter_hex>`). It is a unique key, **not**
a UUID — `std` has no UUID and no crates are allowed.

### Retries

Idempotent `GET`s are retried transparently on transient `503`/`429` (and
transport errors) with capped exponential backoff (base 25 ms, cap 2 s, up to 3
retries). Unsafe `POST`s are **never** retried.

### Errors

Every non-2xx decodes from the canonical
`{"error":{"code","message","request_id"}}` envelope into `Error::Api`:

```rust
Error::Api { code: String, message: String, status: u16, request_id: String }
```

`status` is backfilled from the HTTP status line and `request_id` from the
`X-Request-Id` header when the envelope omits them. Other variants:
`Error::InsecureTransport`, `Error::Transport`, `Error::Decode`, `Error::Config`.

### Insecure-transport guard

Before connecting, if a credential is present and the request is plaintext
`http` to a **non-loopback** host, the client returns `Error::InsecureTransport`
and sends nothing — unless `allow_insecure_http` is set. `https` (any host) and
`http` to a loopback host (`127.0.0.1`, `::1`, `localhost`) are always allowed.

## Testing note

The tests exercise the client against a `std::net::TcpListener` mock `/v1`
server bound to port 0 (the assigned port is read back) in a spawned thread that
records each request's line/headers/body and writes canned HTTP/1.1 responses —
the correct unit-test strategy for a client library. They assert the exact
request the SDK sends (method, path, headers, body) and the typed value it
decodes back; they do **not** boot the live gateway. See `EVIDENCE.md` for the
captured `bash run.sh` output.
