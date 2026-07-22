# EVIDENCE — Helix Thready Python SDK (`thready` / `client.py`)

Real, captured import + test output for the typed **Python** SDK client of the
Helix Thready REST `/v1` control API. Standard-library only (`urllib.request`,
`json`, `uuid`, `unittest`, `http.server`) — **no third-party packages, nothing
pip-installed** — and self-contained (imports no sibling `implementation/*`
module). It is the Python sibling of `implementation/sdk_go` and mirrors the same
`/v1` contract (`docs/public/research/mvp/api/openapi.yaml`).

## How it was run

```
$ cd implementation/sdk_py
$ python3 -c "import client"      # smoke-import: the module loads clean
$ python3 -m unittest -v          # full TDD suite against a real http.server mock
```

## Captured output (2026-07-22)

### Toolchain

```
$ python3 --version
Python 3.13.13
```

### `python3 -c "import client"`

```
(exit 0; no output = the module imports with no side effects / no missing deps)
```

Confirmed programmatically:

```
$ python3 -c "import client; print('import client: OK'); print('version', client.__version__)"
import client: OK
version 0.1.0
```

### `python3 -m unittest -v`

```
test_api_key_header_when_no_token (test_client.TestAuthInjection.test_api_key_header_when_no_token) ... ok
test_bearer_wins_over_api_key (test_client.TestAuthInjection.test_bearer_wins_over_api_key) ... ok
test_no_credential_sends_neither (test_client.TestAuthInjection.test_no_credential_sends_neither) ... ok
test_api_error_string_and_retryable (test_client.TestConstruction.test_api_error_string_and_retryable) ... ok
test_requires_base_url (test_client.TestConstruction.test_requires_base_url) ... ok
test_trailing_slash_trimmed (test_client.TestConstruction.test_trailing_slash_trimmed) ... ok
test_idempotency_key_override (test_client.TestCreateChannel.test_idempotency_key_override) ... ok
test_sends_idempotency_key_and_body (test_client.TestCreateChannel.test_sends_idempotency_key_and_body) ... ok
test_404_maps_to_typed_api_error (test_client.TestGetPost.test_404_maps_to_typed_api_error) ... ok
test_path_and_typed_decode (test_client.TestGetPost.test_path_and_typed_decode) ... ok
test_allow_insecure_http_override (test_client.TestInsecureTransportGuard.test_allow_insecure_http_override) ... ok
test_http_loopback_with_credentials_allowed (test_client.TestInsecureTransportGuard.test_http_loopback_with_credentials_allowed) ... ok
test_http_remote_with_api_key_raises (test_client.TestInsecureTransportGuard.test_http_remote_with_api_key_raises) ... ok
test_http_remote_with_credentials_raises (test_client.TestInsecureTransportGuard.test_http_remote_with_credentials_raises) ... ok
test_http_remote_without_credentials_does_not_raise_guard (test_client.TestInsecureTransportGuard.test_http_remote_without_credentials_does_not_raise_guard) ... ok
test_https_remote_with_credentials_allowed (test_client.TestInsecureTransportGuard.test_https_remote_with_credentials_allowed) ... ok
test_http_loopback_real_call_succeeds_with_token (test_client.TestInsecureTransportLoopbackEndToEnd.test_http_loopback_real_call_succeeds_with_token) ... ok
test_injects_bearer_and_decodes_envelope (test_client.TestListChannels.test_injects_bearer_and_decodes_envelope) ... ok
test_decodes_envelope (test_client.TestListSkills.test_decodes_envelope) ... ok
test_bad_credentials_maps_to_api_error (test_client.TestLogin.test_bad_credentials_maps_to_api_error) ... ok
test_sends_credentials_and_stores_token (test_client.TestLogin.test_sends_credentials_and_stores_token) ... ok
test_totp_included_when_supplied (test_client.TestLogin.test_totp_included_when_supplied) ... ok
test_returns_job_with_idempotency_key (test_client.TestReprocess.test_returns_job_with_idempotency_key) ... ok
test_get_429_then_success (test_client.TestRetry.test_get_429_then_success) ... ok
test_get_503_then_success_makes_two_requests (test_client.TestRetry.test_get_503_then_success_makes_two_requests) ... ok
test_get_exhausted_returns_api_error_after_four_attempts (test_client.TestRetry.test_get_exhausted_returns_api_error_after_four_attempts) ... ok
test_unsafe_post_not_retried_on_503 (test_client.TestRetry.test_unsafe_post_not_retried_on_503) ... ok
test_optional_fields_omitted (test_client.TestSearch.test_optional_fields_omitted) ... ok
test_sends_body_and_decodes_results (test_client.TestSearch.test_sends_body_and_decodes_results) ... ok

----------------------------------------------------------------------
Ran 29 tests in 0.383s

OK
```

**29 tests, all `ok`, final line `OK` (exit 0). No skips, no expected-failures.**

## What is (and isn't) proven

- **Tested against a real, in-process contract-mock — not a live gateway.** The
  honest unit-test approach for a client SDK is to drive it against a genuine
  `http.server.ThreadingHTTPServer` bound to a **free port on 127.0.0.1** in a
  background thread. That server records the exact method / path / headers / body
  each call sends and returns canned, contract-shaped JSON. There is a real TCP
  socket round-trip on every call (verified sub-millisecond on loopback). These
  tests do **not** boot the real `rest_gateway`, a database, or hit the network;
  the mock's request/response shapes are copied from the gateway's wire format
  (and match `implementation/sdk_go`) so the contract asserted here is the one the
  running gateway serves.
- **Every task-required test path is exercised, none skipped:**
  - each method sends the right method + path (asserted server-side from the
    recorded request);
  - auth is injected — `Authorization: Bearer <jwt>` when `access_token` is set
    (including after `login()` stores it), else `X-API-Key`; **bearer wins** when
    both are present; neither is sent when no credential is configured;
  - `Idempotency-Key` is present on the unsafe POSTs (`create_channel`,
    `reprocess`), auto-generated as a **UUIDv4** (asserted against the UUIDv4
    regex) or overridable via the `idempotency_key=` argument;
  - a non-2xx canonical envelope decodes to a typed `ApiError` with the right
    `code` / `status` / `request_id` / `message` (404 → `not_found`, 401 →
    `unauthenticated`);
  - an idempotent GET retries `503`→`200` (asserted: exactly **2** server calls)
    and also `429`→`200`, and exhausts to a typed `ApiError` after
    `1 + max_retries` = **4** attempts;
  - the unsafe POST `reprocess` is **not** retried on 503 (asserted: exactly
    **1** server call);
  - the **insecure-transport guard**: http + non-loopback host + credential →
    `InsecureTransportError` (both bearer and API-key, end-to-end via
    `list_channels()` — raised *before* any socket is opened); http + loopback
    (`127.0.0.1` / `localhost` / `::1`) + credential → allowed (and a real
    loopback call round-trips); https + credential → allowed; `allow_insecure_http=True`
    overrides the refusal; no-credential over remote http does **not** trip the
    guard (nothing to leak);
  - `login()` returns a `TokenPair` and a subsequent call carries the token as
    the bearer.

## Files

| File | Role |
|------|------|
| `client.py` | the SDK: typed DTOs (dataclasses), `ApiError` / `InsecureTransportError` / `TransportError`, `ThreadyClient` (auth injection, `_do()` encode/decode + retry/backoff, error mapping) and the typed `/v1` methods |
| `test_client.py` | 29 TDD tests against a real stdlib `http.server` mock `/v1` gateway on a free port |
| `README.md` | quickstart, method list, how to run the tests |
| `EVIDENCE.md` | this file |

## Verdict

**READY** — `import client` is clean and all **29** `unittest` tests pass
(`OK`, exit 0) against a real, socket-backed `http.server` mock of the `/v1`
surface. The SDK is a self-contained, standard-library-only typed client with the
insecure-transport credential guard enforced and covered.
