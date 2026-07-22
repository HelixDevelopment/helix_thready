# EVIDENCE — Helix Thready headless CLI (`digital.vasic.threadycli`)

Real, captured build / vet / format / test output for the Helix Thready headless
command-line front end. Stdlib-only command layer, Go 1.26. The module wires the
sibling `sdk_go` module (`digital.vasic.threadysdk`) via a **filesystem
`replace ../sdk_go`** so the production adapter and `cmd/thready` entrypoint are
compile-checked against the real SDK with no network and no `go.sum`.

## Verdict

**READY.** `go build`, `go vet`, `gofmt -l` are all clean, and the full test
suite (21 test functions) is green under `-race`. The binary runs end-to-end.

## Honest scope note (how it is tested — the correct approach)

- The **command layer** — `Run` and every subcommand handler — depends only on
  the `APIClient` interface and CLI-local DTOs. It is unit-tested against an
  **in-memory fake `APIClient`** that records calls and returns canned data. The
  tests assert the parsed flags/args reach the client, the right method is
  called, the table/JSON output is correct, and the exit code is correct. This is
  the intended design: the command logic is verified in isolation, without a
  live gateway or the SDK.
- The **real adapter** (`adapter.go`, `*SDKAdapter`) is the thin production
  `APIClient` that delegates to the actual `digital.vasic.threadysdk` client
  (field-for-field DTO conversion). It is **compile-checked against the real
  SDK** (built via the `replace`, plus a `var _ APIClient = (*SDKAdapter)(nil)`
  guard and a test that pins the same). It is not exercised against a live
  gateway here because that would require a running server; the end-to-end run
  below drives it against an unreachable origin to prove the real SDK network
  path is wired.
- `Whoami` is an honest CLI-level convenience: `sdk_go` exposes no server
  whoami/introspection endpoint, so the adapter decodes the standard claims
  (`sub`, `email`, `tier`) from the SDK-held JWT locally. Documented in
  `adapter.go` — swap the body to call `GET /v1/auth/me` once the gateway grows
  it.

## Build discipline

A parent `implementation/go.work` exists that does **not** list this directory,
so every command runs with `GOWORK=off`.

```
$ cd implementation/cli
$ GOWORK=off go build ./... && GOWORK=off go vet ./... && GOWORK=off gofmt -l . && GOWORK=off go test ./... -v -race -count=1
```

## Captured output (2026-07-22)

### Toolchain

```
$ go version
go version go1.26.4-X:nodwarf5 linux/amd64
```

### `GOWORK=off go build ./...`

```
(exit 0; no output = success)
```

### `GOWORK=off go vet ./...`

```
(exit 0; no output = success)
```

### `GOWORK=off gofmt -l .`

```
(exit 0; no output = every file already gofmt-clean)
```

### `GOWORK=off go test ./... -v -race -count=1`

```
=== RUN   TestChannelsList_TableAndCall
--- PASS: TestChannelsList_TableAndCall (0.00s)
=== RUN   TestChannelsAdd_CreateCalledWithName
--- PASS: TestChannelsAdd_CreateCalledWithName (0.00s)
=== RUN   TestChannelsAdd_MissingNameIsUsageError
--- PASS: TestChannelsAdd_MissingNameIsUsageError (0.00s)
=== RUN   TestPostGet_MethodAndID
--- PASS: TestPostGet_MethodAndID (0.00s)
=== RUN   TestPostGet_FlagAfterPositional
--- PASS: TestPostGet_FlagAfterPositional (0.00s)
=== RUN   TestPostReprocess_202AndJob
--- PASS: TestPostReprocess_202AndJob (0.00s)
=== RUN   TestSearch_ParsedOptsAndResults
--- PASS: TestSearch_ParsedOptsAndResults (0.00s)
=== RUN   TestSearch_MissingQueryIsUsageError
--- PASS: TestSearch_MissingQueryIsUsageError (0.00s)
=== RUN   TestJSONFlag_ProducesValidJSON
--- PASS: TestJSONFlag_ProducesValidJSON (0.00s)
=== RUN   TestLogin_StoresPrintsToken
--- PASS: TestLogin_StoresPrintsToken (0.00s)
=== RUN   TestLogin_MissingCredsIsUsageError
--- PASS: TestLogin_MissingCredsIsUsageError (0.00s)
=== RUN   TestSkills_TableAndCall
--- PASS: TestSkills_TableAndCall (0.00s)
=== RUN   TestWhoami_Call
--- PASS: TestWhoami_Call (0.00s)
=== RUN   TestUnknownCommand_NonzeroUsageOnStderr
--- PASS: TestUnknownCommand_NonzeroUsageOnStderr (0.00s)
=== RUN   TestNoArgs_UsageOnStderr
--- PASS: TestNoArgs_UsageOnStderr (0.00s)
=== RUN   TestHelp_UsageOnStdoutExitZero
--- PASS: TestHelp_UsageOnStdoutExitZero (0.00s)
=== RUN   TestAPIError_ExitCodeOne
--- PASS: TestAPIError_ExitCodeOne (0.00s)
=== RUN   TestChannels_MissingSubcommand
--- PASS: TestChannels_MissingSubcommand (0.00s)
=== RUN   TestPost_UnknownSubcommand
--- PASS: TestPost_UnknownSubcommand (0.00s)
=== RUN   TestSplitCSV
--- PASS: TestSplitCSV (0.00s)
=== RUN   TestSDKAdapterSatisfiesInterface
--- PASS: TestSDKAdapterSatisfiesInterface (0.00s)
PASS
ok  	digital.vasic.threadycli	1.016s
?   	digital.vasic.threadycli/cmd/thready	[no test files]
```

Aggregate: **21 test functions, 21 PASS, 0 FAIL, 0 SKIP**, clean under `-race`.

## End-to-end binary run (real SDK adapter path)

Built and driven as the actual `thready` binary. `help` writes usage to stdout
and exits 0; an unknown command writes an error + usage to stderr and exits 2; a
`login` against an unreachable gateway exercises the **real sdk_go client**
(hence the `thready:`-prefixed SDK error) and exits 1.

```
$ GOWORK=off go build -o ./thready ./cmd/thready

$ ./thready help                       # → usage on stdout
(exit 0)

$ ./thready frobnicate                 # → "unknown command" + usage on stderr
thready: unknown command "frobnicate"
(exit 2)

$ THREADY_BASE_URL=http://127.0.0.1:1 ./thready login --email a@b.c --password x
thready: thready: POST /v1/auth/login: Post "http://127.0.0.1:1/v1/auth/login": dial tcp 127.0.0.1:1: connect: connection refused
(exit 1)
```

## Reproduce

```
cd implementation/cli
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off gofmt -l .
GOWORK=off go test ./... -v -race -count=1
```
