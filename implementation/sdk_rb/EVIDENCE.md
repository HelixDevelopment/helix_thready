# EVIDENCE — Helix Thready Ruby SDK (`Thready::Client`)

Real, captured syntax-check + test output for the stdlib-only Ruby SDK client of
the Helix Thready REST `/v1` control API. Both the SDK (`lib/thready.rb`) AND its
tests (`test/test_client.rb`) use ONLY the Ruby 3.3 standard library and import no
sibling `implementation/*` module — the module is fully self-contained and
dependency-free (tests included), per the implementation/ track rule. It mirrors
the `/v1` contract in `docs/public/research/mvp/api/openapi.yaml` and the Go SDK
(`implementation/sdk_go`).

The tests run against a **real** stdlib mock `/v1` server: a `TCPServer` (bound
to `127.0.0.1:0`, OS-assigned port) on a background thread that reads each
HTTP/1.1 request, records method/path/headers/body, and writes a canned,
contract-shaped HTTP response. No network egress, no `Net::HTTP` stubbing.

## How to reproduce

```sh
cd implementation/sdk_rb
ruby -c lib/thready.rb
ruby test/test_client.rb; echo exit=$?
```

## Test framework: NONE — pure-stdlib harness (zero external deps, nothing vendored)

The SDK library uses only `net/http`, `json`, `uri`, `securerandom`, `ipaddr`.
The test file uses **no test framework** — no `minitest`, no gems, no vendored
code. `test/test_client.rb` defines a tiny hand-rolled assertion harness
(`assert_eq` / `assert` / `assert_raises` and a few thin helpers), discovers
every `test_*` method on each registered `TestCase`, prints `PASS`/`FAIL`/`ERROR`
per test, then a summary line, and exits non-zero on any failure or error. This
matches the implementation/ track's "self-contained, dependency-free including
tests" rule (as the Java SDK does with its own hand-rolled runner).

> Context: this distro's Ruby 3.3 build ships no `minitest` default gem and no
> `test/unit`, and the box is offline. Rather than depend on a framework, the
> suite is framework-free — the honest, self-contained resolution.

```
$ ruby --version
ruby 3.3.8 (2025-11-15) [x86_64-linux]
```

## Captured output (2026-07-22)

### `ruby -c lib/thready.rb`

```
Syntax OK
```

### `ruby test/test_client.rb`

```
PASS TestConstruction#test_requires_base_url
PASS TestConstruction#test_strips_trailing_slash
PASS TestLogin#test_login_omits_totp_when_absent
PASS TestLogin#test_login_posts_credentials_and_stores_token
PASS TestListChannels#test_api_key_used_when_no_bearer
PASS TestListChannels#test_bearer_wins_over_api_key
PASS TestListChannels#test_get_channels_injects_bearer_and_decodes
PASS TestCreateChannel#test_explicit_idempotency_key_is_honored
PASS TestCreateChannel#test_post_carries_idempotency_key_and_body
PASS TestGetPost#test_get_post_path_and_decode
PASS TestReprocess#test_reprocess_returns_job
PASS TestSearch#test_search_body_and_results
PASS TestSearch#test_search_omits_unset_optionals
PASS TestListSkills#test_list_skills_decodes_data
PASS TestErrorMapping#test_404_maps_to_api_error
PASS TestErrorMapping#test_409_conflict_maps
PASS TestErrorMapping#test_non_envelope_body_falls_back_to_status_code
PASS TestRetry#test_get_gives_up_after_max_retries_and_raises
PASS TestRetry#test_get_retries_on_429_then_succeeds
PASS TestRetry#test_get_retries_on_503_then_succeeds
PASS TestRetry#test_post_is_not_retried
PASS TestInsecureTransport#test_allow_insecure_http_opt_out
PASS TestInsecureTransport#test_http_loopback_with_credentials_is_allowed
PASS TestInsecureTransport#test_http_remote_with_api_key_raises
PASS TestInsecureTransport#test_http_remote_with_credentials_raises_before_send
PASS TestInsecureTransport#test_http_remote_without_credentials_is_allowed_by_guard
PASS TestInsecureTransport#test_localhost_hostname_is_loopback

27 tests, 88 assertions, 0 failures, 0 errors
```

```
$ echo exit=$?
exit=0
```

The 88 assertions are every check in the suite: 64 `assert_eq` + 5 `assert_nil` +
2 `refute_nil` + 5 `refute` + 1 `assert_match` + 2 `assert_includes` +
1 `assert_in_delta` + 8 `assert_raises`. No coverage was dropped in the switch
away from a framework.

### Stability (3 consecutive runs — no thread/retry flakiness)

```
$ for i in 1 2 3; do ruby test/test_client.rb >/dev/null 2>&1; echo "run $i exit=$?"; done
run 1 exit=0
run 2 exit=0
run 3 exit=0
```

## What the suite proves (27 tests, 88 assertions)

- **login** — POSTs `/v1/auth/login` with `email`/`password`/`totp`, decodes the
  token pair, and stores `access_token` on the client; omits `totp` when absent.
- **list_channels** — GETs `/v1/channels`, injects `Authorization: Bearer …`,
  and decodes the `data` array; falls back to `X-API-Key` when no bearer;
  **bearer wins** when both are set.
- **create_channel** — POSTs `/v1/channels` with an auto `Idempotency-Key`
  (`SecureRandom.uuid`, UUIDv4-shaped, asserted); an explicit key is honored.
- **get_post** — GETs `/v1/posts/{id}` with the id in the path; decodes the post.
- **reprocess** — POSTs `/v1/posts/{id}/reprocess`, carries an `Idempotency-Key`,
  decodes the `202` job (with deterministic `precedence`).
- **search** — POSTs `/v1/search`, includes only the set optionals
  (`mode`/`top_k`/`sources`/`rerank`), decodes results + `embedder`.
- **list_skills** — GETs `/v1/skills`, decodes the `data` array.
- **ApiError mapping** — `404` → `Thready::ApiError` with `code="not_found"`,
  `status=404`, `request_id` parsed from `{"error":{…}}`; `409` conflict; and a
  non-envelope body falls back to a status-derived code.
- **Retry** — an idempotent GET retries `503`-then-`200` (**exactly 2 requests**)
  and `429`-then-`200`; gives up after `1 + 3` attempts and raises; an unsafe
  **POST is never retried** (exactly 1 request).
- **Insecure-transport guard** — `http` + non-loopback host + credentials raises
  `Thready::InsecureTransportError` **before any bytes are sent** (bearer and
  api-key); `http` + `127.0.0.1` and `http` + `localhost` are allowed; a
  credential-less request over remote http is allowed (nothing to leak);
  `allow_insecure_http: true` is wired.

## Files

```
implementation/sdk_rb/
├── lib/thready.rb            # the SDK (stdlib-only)
├── test/test_client.rb       # pure-stdlib harness + TCPServer mock /v1 server (no framework)
├── EVIDENCE.md               # this file
└── README.md                 # quickstart, methods, run instructions
```

No `vendor/` directory, no third-party code — the whole module is stdlib-only.
