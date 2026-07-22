# Helix Thready — headless CLI (`digital.vasic.threadycli`)

`thready` is the headless command-line front end for Helix Thready. It drives the
same `/v1` control API that `sdk_go` (the typed Go client) and `rest_gateway`
(the HTTP surface) implement, for pipeline and automation use. Output is a
human-readable table by default and machine-readable JSON with `--json`.

Standalone Go module, Go 1.26. The **command layer is stdlib-only**; only the
thin production adapter and the `cmd/thready` entrypoint wrap the sibling
`sdk_go` module. A parent `implementation/go.work` does not list this directory,
so **all `go` commands run with `GOWORK=off`**.

## Design

```
Run(args, APIClient, stdout, stderr) int      // cli.go — dispatch + subcommands
        │  depends only on ↓
APIClient interface + CLI-local DTOs           // client.go
        ▲                       ▲
   fakeClient (tests)      *SDKAdapter          // adapter.go — wraps sdk_go
                                 │ delegates to
                        digital.vasic.threadysdk.Client
```

- The command layer depends only on the `APIClient` interface, so it is
  unit-tested in isolation against an in-memory **fake** (no gateway, no SDK).
- `*SDKAdapter` is the production `APIClient`; it delegates each call to the real
  `sdk_go` client with field-for-field DTO conversion. It is compile-checked
  against the real SDK (wired via `replace ../sdk_go`).
- `cmd/thready/main.go` constructs a real SDK client from the environment and
  hands `os.Args` to `Run`.

## Commands

| Command | Description | Key flags |
|---|---|---|
| `login` | Authenticate; prints the access token | `--email` `--password` `--totp` |
| `channels list` | List registered channels | |
| `channels add` | Register a channel | `--name` (required) `--platform` `--external-ref` |
| `post get <id>` | Fetch a single post | |
| `post reprocess <id>` | Queue a fresh processing run (gateway → 202) | |
| `search <query>` | Search posts + generated materials | `--mode` `--sources` `--top-k` `--rerank` |
| `skills` | List Skill-Graph knowledge units | |
| `whoami` | Show the authenticated identity | |
| `help` | Show usage | |

Every command also accepts `--json` to emit JSON instead of a table.

### Exit codes

| Code | Meaning |
|---|---|
| `0` | success |
| `1` | runtime / API error (the request reached the client and failed) |
| `2` | usage error (unknown command, bad flags, missing required argument) |

## Configuration (environment, read by `cmd/thready`)

| Variable | Default | Meaning |
|---|---|---|
| `THREADY_BASE_URL` | `http://127.0.0.1:8080` | gateway origin |
| `THREADY_TOKEN` | — | JWT bearer access token (or obtain one via `login`) |
| `THREADY_API_KEY` | — | scoped API key for non-interactive use |

## Usage examples

```
thready login --email me@example.com --password s3cret
thready channels list
thready channels add --name "Design QA" --platform telegram --external-ref -100123
thready post get post_42
thready post reprocess post_42
thready search "vector db" --mode semantic --top-k 5 --sources posts,generated --rerank
thready skills --json
thready whoami
```

Flags and positional arguments may be given in any order —
`thready post get post_42 --json` and `thready post get --json post_42` are
equivalent.

## Build & run

```
cd implementation/cli
GOWORK=off go build ./...
GOWORK=off go build -o ./thready ./cmd/thready
./thready help
```

## Test

```
cd implementation/cli
GOWORK=off go test ./... -v -race -count=1
```

21 test functions cover: each subcommand calls the right `APIClient` method with
the parsed flags/args; table and JSON rendering; `--json` produces valid JSON;
unknown/missing commands and missing required flags return usage errors on
stderr; API errors map to exit 1; and the adapter satisfies `APIClient`. See
[`EVIDENCE.md`](./EVIDENCE.md) for the captured, reproducible run.

## Notes

- `whoami` is resolved client-side: `sdk_go` exposes no server whoami endpoint,
  so `*SDKAdapter` decodes the standard claims (`sub`, `email`, `tier`) from the
  JWT it already holds. When the gateway adds `GET /v1/auth/me`, swap the adapter
  body to call it.
- This module is intended to be promoted to its own repository later, per the
  implementation-phase decoupling plan; it stays self-contained (only the
  `replace ../sdk_go` couples it to a sibling, which becomes a normal module
  requirement post-promotion).
```
