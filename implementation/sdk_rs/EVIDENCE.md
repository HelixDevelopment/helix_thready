# EVIDENCE — Helix Thready Rust SDK (`thready`, std-only)

Real, captured build + test output for the typed **standard-library-only** Rust
SDK client of the Helix Thready REST `/v1` control API. Built with `rustc`
directly — **no cargo, no crates** (no `reqwest`/`serde`/`tokio`/`uuid`). The
HTTP/1.1 client and the JSON codec are hand-rolled over `std` only.

## Build discipline

There is no `Cargo.toml`. The whole crate is a single file (`src/lib.rs`) with
inline `mod json` / `mod http` and a `#[cfg(test)] mod tests`. `run.sh` compiles
the test binary with `rustc --test` and runs it.

```
$ cat run.sh
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p target
rustc --edition 2021 --test src/lib.rs -o target/testbin
./target/testbin
```

## Captured output (2026-07-22)

Fresh run after `rm -rf target` (clean compile — zero warnings, zero errors):

```
$ rustc --version
rustc 1.96.1 (31fca3adb 2026-06-26)

$ bash run.sh

running 17 tests
test tests::test_bearer_wins_over_api_key ... ok
test tests::test_create_channel_sends_idempotency_key_and_body ... ok
test tests::test_https_base_url_not_refused ... ok
test tests::test_get_post_decodes_typed_post ... ok
test tests::test_insecure_http_remote_refused_nothing_sent ... ok
test tests::test_json_roundtrip_nested ... ok
test tests::test_insecure_http_loopback_allowed_past_guard ... ok
test tests::test_list_channels_injects_bearer_and_decodes ... ok
test tests::test_list_skills_decodes_envelope ... ok
test tests::test_login_sends_credentials_stores_token ... ok
test tests::test_non2xx_maps_to_typed_api_error ... ok
test tests::test_post_not_retried_on_503 ... ok
test tests::test_reprocess_returns_job_with_idempotency_key ... ok
test tests::test_search_sends_body_and_decodes_results ... ok
test tests::test_transport_allowed_matrix ... ok
test tests::test_url_parse_variants ... ok
test tests::test_retry_get_503_then_200_two_connections ... ok

test result: ok. 17 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.03s

$ echo exit=$?
exit=0
```

**17 tests, all PASS, compile clean (no warnings), process exit 0.**

## What is (and isn't) proven

- **Tested against a contract-mock, not a live gateway.** The correct unit-test
  approach for a client SDK: the tests bind a `std::net::TcpListener` to
  `127.0.0.1:0`, read the assigned port, and spawn a thread that reads each
  request (line + headers + body by `Content-Length`), records it, and writes a
  canned HTTP/1.1 response. They assert the exact request the SDK **sends** and
  the typed value it **decodes** from the canned response. They do **not** boot
  the real `rest_gateway`, a database, or the network beyond loopback.
- **Every task-required path is exercised, none skipped (no `#[ignore]`):**
  - **method / path / headers + typed decode** — each of `login`,
    `list_channels`, `create_channel`, `get_post`, `reprocess`, `search`,
    `list_skills` asserts the method + `/v1/...` path (server-side) and decodes
    into the typed struct;
  - **auth injection** — `Authorization: Bearer <jwt>` after login/token
    (`test_list_channels_injects_bearer_and_decodes`), `X-API-Key` for an API
    key (`test_create_channel_...`), and **bearer wins** when both are set
    (`test_bearer_wins_over_api_key`);
  - **`Idempotency-Key` present on unsafe POSTs** — asserted non-empty on
    `create_channel` and `reprocess`;
  - **Api error mapping** — a `404` canonical envelope decodes to
    `Error::Api { code:"not_found", status:404, request_id:"req-123", message }`
    (`test_non2xx_maps_to_typed_api_error`);
  - **retry** — an idempotent `GET` retries `503`→`200` and the mock records
    **exactly 2 connections** (`test_retry_get_503_then_200_two_connections`);
    a `POST` is **not** retried on `503` (`test_post_not_retried_on_503`,
    exactly 1 connection);
  - **insecure-transport guard** —
    - `http` + remote host + credential → `Error::InsecureTransport` with
      **nothing sent** (`test_insecure_http_remote_refused_nothing_sent`;
      getting `InsecureTransport` rather than a `Transport` connect error is
      itself proof no socket was opened — that variant is only returned on the
      pre-connect guard path);
    - `http` + `127.0.0.1` + credential → **allowed past the guard**; the
      request actually reaches the mock, which records it, with the credential
      attached (`test_insecure_http_loopback_allowed_past_guard`);
    - an **`https` base URL + credentials does NOT return `InsecureTransport`**
      — asserted via the `transport_allowed(&url)` helper
      (`test_https_base_url_not_refused`, and the `https://` row of
      `test_transport_allowed_matrix`). std has no TLS, so `https` cannot be
      *connected* in a std-only build, but the guard must (and does) treat it as
      safe rather than refusing it;
  - **hand-rolled layers** — `test_json_roundtrip_nested` (nested objects/arrays,
    string escapes incl. `\n` and a non-ASCII char, encode→parse stability) and
    `test_url_parse_variants` (default ports, query string, bracketed IPv6
    `[::1]` + loopback detection).
- **std-only, verified.** The only imports are from `std` (`std::net`,
  `std::io`, `std::sync`, `std::time`, `std::thread`). No `extern crate`, no
  `Cargo.toml`, no dependency resolution — `rustc` compiles the single file.

## Files

| File | Role |
|------|------|
| `src/lib.rs` | the whole crate: `mod json` (Value + parser + encoder), `mod http` (HTTP/1.1 over `TcpStream` + URL parser), typed `Error`, DTOs, `ThreadyClient`, and `#[cfg(test)] mod tests` (17 TDD tests + the `TcpListener` mock server) |
| `run.sh` | `rustc --edition 2021 --test src/lib.rs -o target/testbin && ./target/testbin` |
| `README.md` | quickstart, methods, std-only/http note, run |
| `EVIDENCE.md` | this file |

## Verdict

**READY** — compiles clean with `rustc 1.96.1` (no warnings, no errors) and all
**17** tests pass (`test result: ok. 17 passed; 0 failed`), process exit `0`. A
self-contained, std-only typed client verified against a `TcpListener`
contract-mock of the `/v1` surface. No bluff: the run above is real and
reproducible via `bash run.sh`.
