# Helix Thready — Ruby SDK client (`Thready::Client`)

A stdlib-only Ruby client for the Helix Thready REST `/v1` control API
(schema: `docs/public/research/mvp/api/openapi.yaml`; served by the
`implementation/rest_gateway` module; mirrors `implementation/sdk_go`).

- **Stdlib only.** No gems, no `bundle install`. The SDK (`lib/thready.rb`) uses
  only `net/http`, `json`, `uri`, `securerandom`, and `ipaddr`, and imports no
  sibling `implementation/*` module — it can be vendored on its own.
- **Auth injection.** A JWT `Authorization: Bearer …`, or an `X-API-Key` for
  non-interactive use. If both are set, the bearer wins. `login` refreshes the
  in-flight access token so later calls authenticate automatically.
- **Batteries included.** JSON encode/decode, canonical typed error mapping,
  transparent retries for idempotent GETs on `503`/`429`, and an automatic
  `Idempotency-Key` (`SecureRandom.uuid`) on unsafe POSTs.
- **Secure by default.** Refuses to attach credentials to plaintext `http`
  bound for a non-loopback host (raises `Thready::InsecureTransportError`
  *before* sending) unless `allow_insecure_http: true`.

## Requirements

Ruby 3.3+ (standard library only). No third-party dependencies — at runtime or
for tests.

> The test suite uses **no framework**: `test/test_client.rb` is a self-contained
> pure-stdlib harness (hand-rolled `assert_*` helpers + a `TCPServer` mock),
> consistent with the implementation/ track's "dependency-free, tests included"
> rule. See `EVIDENCE.md`.

## Quickstart

```ruby
require_relative "lib/thready"

client = Thready::Client.new(base_url: "https://thready.hxd3v.com")

# Log in — stores the access token on the client for subsequent calls.
client.login(email: "user@t1.example", password: "correct-horse-battery-x", totp: "123456")

# List channels for the tenant.
channels = client.list_channels
channels.each { |ch| puts "#{ch[:id]}  #{ch[:name]}  (#{ch[:platform]})" }

# Register a channel (unsafe POST — an Idempotency-Key is auto-attached).
ch = client.create_channel(name: "Alpha", platform: "telegram", external_ref: "@alpha")

# Fetch one post, then force a reprocess (returns the queued 202 job).
post = client.get_post("9b1e4c00-0000-4000-8000-000000000001")
job  = client.reprocess(post[:id])
puts job[:precedence].inspect   # ["download","convert","analyze","research","reply"]

# Semantic / hybrid search.
res = client.search(query: "great docs", mode: "hybrid", top_k: 20,
                    sources: %w[posts generated], rerank: true)
puts "#{res[:results].length} hits via #{res[:embedder]}"

# Skill-Graph knowledge units.
client.list_skills.each { |s| puts "#{s[:sort_order]}. #{s[:name]} (#{s[:kind]})" }
```

### API-key (non-interactive) client

```ruby
client = Thready::Client.new(base_url: "https://thready.hxd3v.com", api_key: "sk-…")
client.list_channels   # sent with "X-API-Key: sk-…"
```

### Local development over http

Loopback http (`127.0.0.1`, `::1`, `localhost`) always carries credentials.
For a remote http origin you must opt in explicitly:

```ruby
Thready::Client.new(base_url: "http://10.0.0.5:8080", api_key: "sk-…",
                    allow_insecure_http: true)
```

## Constructor

```ruby
Thready::Client.new(
  base_url:,                 # required — gateway origin, e.g. "https://thready.hxd3v.com"
  access_token: nil,         # JWT bearer (wins over api_key)
  api_key: nil,              # scoped API key -> X-API-Key
  timeout: 30,               # per-request open/read timeout (seconds)
  allow_insecure_http: false # allow creds over remote plaintext http
)
```

## Methods

| Method | HTTP | Path | Notes |
| --- | --- | --- | --- |
| `login(email:, password:, totp: nil)` | POST | `/v1/auth/login` | stores `access_token` |
| `list_channels` | GET | `/v1/channels` | returns `data` array; retried on 503/429 |
| `create_channel(name:, platform: nil, external_ref: nil, idempotency_key: nil)` | POST | `/v1/channels` | auto `Idempotency-Key` |
| `get_post(id)` | GET | `/v1/posts/{id}` | retried on 503/429 |
| `reprocess(id, idempotency_key: nil)` | POST | `/v1/posts/{id}/reprocess` | returns 202 job; auto `Idempotency-Key` |
| `search(query:, mode: nil, top_k: nil, sources: nil, rerank: nil)` | POST | `/v1/search` | unset optionals omitted |
| `list_skills` | GET | `/v1/skills` | returns `data` array |

All decoded bodies are Ruby `Hash`/`Array` with **symbol** keys
(`JSON.parse(…, symbolize_names: true)`).

## Errors

Every non-2xx response raises a typed `Thready::ApiError` parsed from the
canonical envelope `{"error":{"code","message","status","request_id"}}`:

```ruby
begin
  client.get_post("missing")
rescue Thready::ApiError => e
  e.code        # "not_found"
  e.status      # 404
  e.request_id  # "req-abc-123"
  e.retryable?  # false
end
```

Error class tree: `Thready::Error` < `StandardError`;
`Thready::ApiError` and `Thready::InsecureTransportError` < `Thready::Error`.

Codes mirror the gateway's canonical taxonomy (`invalid_argument`,
`unauthenticated`, `permission_denied`, `not_found`, `already_exists`,
`conflict`, `failed_precondition`, `unprocessable`, `rate_limited`,
`deadline_exceeded`, `unavailable`, `internal`). A non-envelope body degrades to
a status-derived code.

## Run the tests

```sh
cd implementation/sdk_rb
ruby -c lib/thready.rb            # => Syntax OK
ruby test/test_client.rb; echo exit=$?
# => PASS … (one line per test)
# => 27 tests, 88 assertions, 0 failures, 0 errors
# => exit=0
```

The suite (`test/test_client.rb`) starts a real stdlib `TCPServer` mock `/v1`
server on a background thread (OS-assigned port), so it exercises the actual
`Net::HTTP` request/response path with no external services and no stubbing.
See `EVIDENCE.md` for captured output.
