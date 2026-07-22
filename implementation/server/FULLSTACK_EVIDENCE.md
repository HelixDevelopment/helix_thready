# FULLSTACK EVIDENCE — real CLI binary ↔ real server binary ↔ real domain modules

Real, captured transcript of `implementation/server/fullstack_smoke.sh`. Nothing
below is paraphrased or hand-edited — it is a verbatim run.

The smoke proves the WHOLE stack composes for real, with no mocking:

```
 real `thready` CLI binary  ──HTTP──▶  real `thready-server` binary
      (cli/cmd/thready,                    (server/cmd/thready-server,
       over the sdk_go transport)           real /v1 gateway wired over the REAL
                                            domain modules: user_service,
                                            semantic_search, skill_dispatch,
                                            event_bus_service)
```

## Environment

```
$ go version
go version go1.26.4-X:nodwarf5 linux/amd64
```

## What the script does

1. Builds BOTH binaries offline:
   - `thready-server` — `server/cmd/thready-server`, workspace build (`GOPROXY=off`).
   - `thready` — `cli/cmd/thready`, `GOWORK=off` build (cli+sdk_go are not
     workspace members; the cli module's `replace … => ../sdk_go` resolves it).
2. Picks a FREE loopback port, exports a throwaway `THREADY_JWT_SECRET` (the
   server is fail-closed and refuses to boot without it), starts the server in
   the background on `PORT`, and waits for `GET /v1/health` to answer `200`.
3. Drives the server with the REAL `thready` CLI over `http://127.0.0.1:PORT`
   (loopback → the sdk_go transport guard attaches the bearer token over
   plaintext http only because the host is loopback):
   - `login` with the **password-only** seed user, password passed via
     `THREADY_PASSWORD` (never argv), token captured from the `--json` output;
   - `channels list`, `skills`, `search "vector database"`, with the obtained
     bearer token passed via `THREADY_TOKEN`.
4. Asserts the real output (skills list + a real search hit appear), shuts the
   server down cleanly (kill + wait, via an `EXIT`/`INT`/`TERM` trap that also
   fires on error), prints a PASS/FAIL summary, and exits non-zero on any failure.

## Verbatim run

```
$ cd implementation/server && ./fullstack_smoke.sh ; echo "SCRIPT_EXIT=$?"

=== build binaries ===
PASS  build thready-server
PASS  build thready CLI

=== start thready-server ===
chosen free port: 52501
server pid: 99443
PASS  GET /v1/health -> 200

=== CLI: login (password-only seed user, no TOTP) ===
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IjIwMjYtMDctYTFiMmMzZDQifQ.eyJzdWIiOiJ1c2VyLXN0ZCIsInJvbGUiOiJ1c2VyIiwiYWNjb3VudF9pZCI6ImFjY3QtYSIsInNjb3BlcyI6WyJwb3N0czpyZWFkIiwicG9zdHM6d3JpdGUiLCJhc3NldHM6cmVhZCIsInNlYXJjaDpyZWFkIiwic2tpbGxzOnJlYWQiLCJldmVudHM6cmVhZCJdLCJpc3MiOiJ0aHJlYWR5LXVzZXItc2VydmljZSIsImF1ZCI6InRocmVhZHktYXBpIiwiaWF0IjoxNzg0NzM2NDcxLCJleHAiOjE3ODQ3MzczNzEsInRva2VuX3VzZSI6ImFjY2VzcyJ9.dvyFXGpe00VtoioGjzSde_WinbuceiJwnh-PEAWbTJU",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IjIwMjYtMDctYTFiMmMzZDQifQ.eyJzdWIiOiJ1c2VyLXN0ZCIsInJvbGUiOiJ1c2VyIiwiYWNjb3VudF9pZCI6ImFjY3QtYSIsInNjb3BlcyI6WyJwb3N0czpyZWFkIiwicG9zdHM6d3JpdGUiLCJhc3NldHM6cmVhZCIsInNlYXJjaDpyZWFkIiwic2tpbGxzOnJlYWQiLCJldmVudHM6cmVhZCJdLCJpc3MiOiJ0aHJlYWR5LXVzZXItc2VydmljZSIsImF1ZCI6InRocmVhZHktYXBpIiwiaWF0IjoxNzg0NzM2NDcxLCJleHAiOjE3ODUzNDEyNzEsInRva2VuX3VzZSI6InJlZnJlc2gifQ.676pnBXhk7TOLldlY6_0Tvo9gbtQ06WNZMAa6gTIc6M",
  "token_type": "Bearer",
  "expires_in": 900
}
PASS  login obtained an access token (len=451)

=== CLI: channels list ===
ID      NAME     PLATFORM  EXTERNAL_REF      CREATED_AT
chan-1  general  telegram  @thready_general  2023-11-14T22:13:20Z
PASS  channels list returned the seed channel 'general'

=== CLI: skills ===
ID                    NAME            KIND      SORT_ORDER
skill-video.download  video.download  download  0
skill-media.convert   media.convert   convert   1
skill-vision.analyze  vision.analyze  analyze   2
skill-tech.research   tech.research   research  3
skill-thread.reply    thread.reply    reply     4
PASS  skills list appears (video.download … thread.reply)

=== CLI: search "vector database" ===
SOURCE_ID    KIND      SCORE  SNIPPET
vectordb.md  markdown  0.551  self hosted vector database benchmarks pgvector cosine similarity nearest neighbor embeddings index recall
telegram.md  markdown  0.050  telegram messenger bot channel adapter chat group message webhook long polling updates
skills.md    markdown  0.046  skill dispatch precedence download convert analyze research reply pipeline stage execution engine registry
(3 results in 0ms via semsearch/hash-deterministic)
PASS  search returned a real hit (top: vectordb.md)

=== shutdown ===
server stopped (wait rc=0)
PASS  server shut down cleanly

=== server log ===
2026/07/22 21:07:51 thready-server: listening on :52501 (real-wired /v1 surface)
2026/07/22 21:07:51 thready-server: shutdown signal received, draining...
2026/07/22 21:07:51 thready-server: stopped cleanly

=== SUMMARY ===
PASS=8  FAIL=0
RESULT: PASS — full stack composed end-to-end for real.
SCRIPT_EXIT=0
```

Port / PID / token / timestamps vary per run (free-port allocation, per-run
signing secret, RFC 7519 `iat`/`exp`); the assertions above are stable.

## What each PASS proves (real behaviour, real binaries)

| Step | Proves |
|------|--------|
| build thready-server / thready CLI | Both real entrypoints compile and link — the server as a workspace member, the CLI over the real sdk_go via its filesystem `replace`. |
| `GET /v1/health -> 200` | The real server binary is listening and serving `/v1` over TCP. |
| login (password-only, no TOTP) | The **password-only** seed user (`user@thready.test`) authenticates with email+password ONLY through the server's real-wired auth adapter (`realAuth` → `userservice.Verify`, real PBKDF2); it is provisioned with no TOTP secret (`MFAEnabled=false`), so no RFC 6238 code is needed. The token is a genuine HS256 JWT minted by the real gateway signer. |
| channels list → `general` | Authorized real request (role `user` + scope `posts:read`); the seed channel round-trips CLI → sdk_go → gateway → real channel store → back. |
| skills → 5 skills | Real `skill_dispatch` registry returned via `OrderByPrecedence` (download > convert > analyze > research > reply). |
| search `"vector database"` → `vectordb.md` | Real `semantic_search` cosine-KNN over the deterministic embedder ranks `vectordb.md` top; the honest embedder label `semsearch/hash-deterministic` is surfaced. |
| clean shutdown | The server drains and stops cleanly (SIGTERM → graceful `Shutdown`); the trap guarantees teardown on success and on error. |

## Honest notes / adjustments

1. **`channels` uses the `channels list` subcommand.** The CLI's `channels`
   command requires a subcommand (`list` | `add`); bare `channels` prints a usage
   error by design. The smoke calls `channels list`.

2. **The standard user is authorized for all three commands — no 403 needed.**
   Its scopes (`posts:read`, `search:read`, `skills:read`, …) cover `channels`
   (`posts:read`), `skills` (`skills:read`), and `search` (`search:read`) against
   the gateway RBAC. So every command runs on the authorized path; the smoke
   asserts real success, not a 403.

3. **Token flows between CLI invocations via env.** Each `thready` invocation is a
   separate process and the CLI does not persist a token to disk. The smoke
   captures the access token from `login --json` and passes it to the following
   commands via `THREADY_TOKEN` (the same env var `cmd/thready/main.go` reads) —
   the realistic non-interactive pattern. The password is passed via
   `THREADY_PASSWORD` (never argv).

4. **Loopback is what makes plaintext http legal.** The sdk_go client refuses to
   attach a bearer token to a plaintext-http request bound for a NON-loopback
   host (`ErrInsecureTransport`). The smoke targets `http://127.0.0.1:PORT`, a
   loopback host, so the guard permits the credential without setting
   `THREADY_ALLOW_INSECURE_HTTP`.

5. **Offline builds.** `GOPROXY` in this environment points at the public proxy;
   the smoke builds with `GOPROXY=off` (server, workspace mode) and
   `GOWORK=off GOPROXY=off` (CLI, module mode) so no network is touched — every
   dependency resolves from the local workspace / `replace` directives.

6. **Fail-closed signing secret.** The server refuses to boot without
   `THREADY_JWT_SECRET`; the smoke exports a throwaway per-run value (16 random
   bytes from `/dev/urandom`) before starting it. This is the same fail-closed
   behaviour asserted by `server`'s `TestNewServer_FailsClosedWithoutSecret`.

---

*Made with love ♥ by Helix Development.*
