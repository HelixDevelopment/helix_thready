# EVIDENCE — rest_gateway deployment smoke test

Captured physical evidence of the Helix Thready `rest_gateway` actually serving
its `/v1` API. **Every transcript below is copied verbatim from a real run** on
this host — no hand-edited status codes, no fabricated bodies. Reproduce with
`./smoke.sh` and `./podman_smoke.sh`.

- Host: `Linux 6.12.41-6.12-alt1 x86_64`
- Go: `go version go1.26.4-X:nodwarf5 linux/amd64`
- Podman: `5.7.1`, rootless (uid 1000, `Host.Security.Rootless = true`, pasta networking)
- Captured (UTC): `2026-07-22`

---

## Leg 1 — Host build + serve (`smoke.sh`) — VERDICT: PASS (12/12)

Builds `./cmd/gateway` with `GOWORK=off go build`, starts it on a free port,
polls `/v1/health`, then drives four real, asserted HTTP round-trips through the
full middleware chain. Seeded credentials read from `rest_gateway/services.go`
(`SeedUserEmail=user@thready.test`, `SeedUserPass=userpassword-123`, role `user`,
scope `posts:read`, no TOTP).

```
== Helix Thready rest_gateway — deployment smoke test ==
host:        Linux 6.12.41-6.12-alt1 x86_64
go:          go version go1.26.4-X:nodwarf5 linux/amd64
date (UTC):  2026-07-22T16:06:48Z
-------------------------------------------------------------------------------
[build] GOWORK=off go build -o /tmp/.private/milos/thready-gateway.MJLpN9 ./cmd/gateway  (in .../implementation/rest_gateway)
[build] OK -> /tmp/.private/milos/thready-gateway.MJLpN9
-rwxr-xr-x 1 milos milos 9429419 Jul 22 21:06 /tmp/.private/milos/thready-gateway.MJLpN9
-------------------------------------------------------------------------------
[run] chosen free port: 42105
[run] started gateway pid=61318 on http://127.0.0.1:42105
[run] health ready after 200ms (attempt 1)
-------------------------------------------------------------------------------
### CHECK 1 — GET /v1/health  (public; expect 200 + JSON)
HTTP 200  <-  GET http://127.0.0.1:42105/v1/health
body: {"service":"rest-gateway","status":"ok","time":"2023-11-14T22:13:20Z","version":"v1"}
PASS  health.status_code                          got=200 (want=200)
PASS  health.body_is_json                         got=object (want=object)
PASS  health.body_status_ok                       body contains "status":"ok"
-------------------------------------------------------------------------------
### CHECK 2 — GET /v1/channels WITHOUT auth  (protected; expect 401)
HTTP 401  <-  GET http://127.0.0.1:42105/v1/channels   (no Authorization header)
body: {"error":{"code":"unauthenticated","message":"missing or malformed Authorization header","request_id":"996f9e0fd815fa3ecc058a27bf100a56","status":401,"trace_id":"996f9e0fd815fa3ecc058a27bf100a56"}}
PASS  channels_noauth.status_code                 got=401 (want=401)
PASS  channels_noauth.err_code                    body contains "code":"unauthenticated"
-------------------------------------------------------------------------------
### CHECK 3 — POST /v1/auth/login  (seeded user; expect 200 + token)
request body: {"email":"user@thready.test","password":"userpassword-123"}   (seeded creds from rest_gateway/services.go: SeedUserEmail / SeedUserPass, no TOTP)
HTTP 200  <-  POST http://127.0.0.1:42105/v1/auth/login
token_type: Bearer   access_token[0:24]: eyJhbGciOiJIUzI1NiIsInR5...   (len=451)
PASS  login.status_code                           got=200 (want=200)
PASS  login.token_type_bearer                     got=Bearer (want=Bearer)
PASS  login.access_token_present            token len=451
PASS  login.token_is_jwt_3parts                   got=2 (want=2)
-------------------------------------------------------------------------------
### CHECK 4 — GET /v1/channels WITH Bearer token  (expect 200)
HTTP 200  <-  GET http://127.0.0.1:42105/v1/channels   (Authorization: Bearer <token>)
body: {"data":[{"id":"chan-1","account_id":"acct-a","name":"general","platform":"telegram","external_ref":"@thready_general","created_at":"2023-11-14T22:13:20Z"}],"meta":{"next_cursor":null,"total_estimate":1}}
PASS  channels_auth.status_code                   got=200 (want=200)
PASS  channels_auth.has_data                      body contains "data"
PASS  channels_auth.seed_channel                  body contains "chan-1"
-------------------------------------------------------------------------------
== server access log (structured JSON, proves the requests hit the server) ==
{"time":"2026-07-22T21:06:49.2267477+05:00","level":"INFO","msg":"rest-gateway listening","addr":":42105"}
{"time":"2026-07-22T21:06:49.24167289+05:00","level":"INFO","msg":"access","method":"GET","path":"/v1/health","status":200,"duration_ms":0,"request_id":"36c0531c471167150be1442a80e21418"}
{"time":"2026-07-22T21:06:49.249823904+05:00","level":"INFO","msg":"access","method":"GET","path":"/v1/health","status":200,"duration_ms":0,"request_id":"28a8401d6d9a4d4506c2c214cdede48e"}
{"time":"2026-07-22T21:06:49.269481006+05:00","level":"INFO","msg":"access","method":"GET","path":"/v1/channels","status":401,"duration_ms":0,"request_id":"996f9e0fd815fa3ecc058a27bf100a56"}
{"time":"2026-07-22T21:06:49.283127816+05:00","level":"INFO","msg":"access","method":"POST","path":"/v1/auth/login","status":200,"duration_ms":0,"request_id":"e6698688c4b705dff1aeababf922ae7e"}
{"time":"2026-07-22T21:06:49.300571958+05:00","level":"INFO","msg":"access","method":"GET","path":"/v1/channels","status":200,"duration_ms":0,"request_id":"313855c0a90e0b5b6d36751c7c1c2fd3"}
-------------------------------------------------------------------------------
== SUMMARY ==
checks passed: 12
checks failed: 0
VERDICT: PASS
```

The server's own structured access log (bottom block) independently confirms
each HTTP request reached the process with the exact status the client observed
(`GET /v1/health` 200, `GET /v1/channels` 401, `POST /v1/auth/login` 200,
`GET /v1/channels` 200). `smoke.sh` exited `0`.

---

## Leg 2 — Rootless Podman container (`podman_smoke.sh`) — VERDICT: PASS

The multi-stage `Containerfile` (pinned Go toolchain → `CGO_ENABLED=0` static
binary → `gcr.io/distroless/static-debian12:nonroot`, `USER 65532`, `EXPOSE
8080`) built rootless and ran, and `/v1/health` was curled from the published
port. This is the direct evidence for the Constitution's rootless-Podman mandate
(§11.4.161). Transcript from a full (uncached) build run:

```
== rest_gateway — rootless Podman container smoke (best-effort) ==
podman:      podman version 5.7.1
uid:         1000   rootless: true
date (UTC):  2026-07-22T16:06:01Z
-------------------------------------------------------------------------------
[build] podman build -t thready-gateway-smoke .   (context=/tmp/.private/milos/thready-podman-ctx.7i7niT)
[build] OK
[1/2] STEP 4/6: WORKDIR /src/rest_gateway
--> a64e1402eea1
[1/2] STEP 5/6: ENV GOWORK=off     CGO_ENABLED=0     GOOS=linux     GOTOOLCHAIN=local     GOPROXY=off
--> b1641d8c2ba2
[1/2] STEP 6/6: RUN go build -trimpath -ldflags='-s -w' -o /out/gateway ./cmd/gateway
--> d3cab192888e
[2/2] STEP 1/6: FROM gcr.io/distroless/static-debian12:nonroot
[2/2] STEP 2/6: COPY --from=build /out/gateway /usr/local/bin/gateway
--> c60fde17a161
[2/2] STEP 3/6: ENV GATEWAY_ADDR=:8080
--> a254e015971e
[2/2] STEP 4/6: EXPOSE 8080
--> d05d2bb583cd
[2/2] STEP 5/6: USER 65532:65532
--> 0ae6cb980922
[2/2] STEP 6/6: ENTRYPOINT ["/usr/local/bin/gateway"]
[2/2] COMMIT thready-gateway-smoke
--> 8550c53a60fe
Successfully tagged localhost/thready-gateway-smoke:latest
8550c53a60fe290b6ecde4046f18345e0041db0c688affa5b6a4d6d0f3b503dc
-------------------------------------------------------------------------------
[run] podman run --rm -d -p 41843:8080 --name thready-gateway-smoke-run thready-gateway-smoke
[run] container id: 35dbfc0e8af51ffe42c4e6b775a9d55705cdf885a46d6ef5060717796b72f77c
[run] in-container id: exec-unavailable-on-distroless
[run] image USER directive:
USER=65532:65532
-------------------------------------------------------------------------------
[run] health ready after 200ms (attempt 1)
-------------------------------------------------------------------------------
### CONTAINER CHECK — GET /v1/health from the running rootless container
HTTP 200  <-  GET http://127.0.0.1:41843/v1/health   (served from container thready-gateway-smoke-run)
body: {"service":"rest-gateway","status":"ok","time":"2023-11-14T22:13:20Z","version":"v1"}
container logs:
{"time":"2026-07-22T16:06:34.337453968Z","level":"INFO","msg":"rest-gateway listening","addr":":8080"}
{"time":"2026-07-22T16:06:34.60891531Z","level":"INFO","msg":"access","method":"GET","path":"/v1/health","status":200,"duration_ms":0,"request_id":"92ff126417a375ae8c8b190d81997df9"}
{"time":"2026-07-22T16:06:34.6165137Z","level":"INFO","msg":"access","method":"GET","path":"/v1/health","status":200,"duration_ms":0,"request_id":"970a54db5a3083e591fd8f8b110a42f8"}
{"time":"2026-07-22T16:06:34.622047713Z","level":"INFO","msg":"access","method":"GET","path":"/v1/health","status":200,"duration_ms":0,"request_id":"6b7e6533b2140a6d748387c5773dbe10"}
-------------------------------------------------------------------------------
PASS  container.health_200_json
VERDICT: PASS (rootless podman container served the live API)
```

`podman_smoke.sh` exited `0`. The container's own log (served from inside the
distroless image) shows `rest-gateway listening addr=:8080` and the `GET
/v1/health` 200 lines — genuine proof the same binary serves from within a
non-root container. A re-run reproduced this from cache (`--> Using cache ...`),
confirming determinism.

### Image metadata (real `podman image inspect`)

```
User=65532:65532  ExposedPorts=map[8080/tcp:{}]  Entrypoint=[/usr/local/bin/gateway]  Env=[... GATEWAY_ADDR=:8080]
localhost/thready-gateway-smoke:latest  size=9.47 MB
```

`User=65532:65532` proves the process runs **non-root** inside the container
(rootless mandate). The 9.47 MB size confirms the distroless-static base carries
only the CGO-free static binary — a dynamically-linked binary could not even
start on distroless (no libc), so the successful `/v1/health` 200 is itself proof
the binary is static. `--rm` + an `EXIT` trap leave no residual container.

---

## Honesty notes (what really happened during bring-up)

Nothing here is idealized; two real obstacles were hit and resolved:

1. **First container attempt was KILLED at the time budget, not faked.** The
   initial run had to pull two large base images
   (`docker.io/library/golang:1.26` ≈ 900 MB and
   `gcr.io/distroless/static-debian12:nonroot`); the cumulative pull exceeded the
   run's time limit and the task was killed with the build still in progress. It
   was **not** reported as passing. After the base images were present locally,
   the leg completed and passed (transcript above). Registry egress to both
   `docker.io` and `gcr.io` works from this host.

2. **A real bug in the first `Containerfile` was found and fixed.** The build
   `RUN` step originally ended with `&& /out/gateway -h`, intended as a smoke of
   the binary. But `cmd/gateway/main.go` ignores its arguments and calls
   `ListenAndServe()`, so that command started the server and **blocked the image
   build forever** (observed as an orphaned in-container `go build`/gateway
   process holding the build). The fix: drop the in-build invocation entirely and
   pin `GOTOOLCHAIN=local` + `GOPROXY=off` so the stdlib-only module builds
   hermetically (in-container build then completes in ~27 s). The binary is
   exercised for real by `podman_smoke.sh` after the image is built, which is the
   correct place to do it.

## Overall verdict

| Leg | Ran? | Result |
|-----|------|--------|
| Host build + 4 asserted HTTP checks (`smoke.sh`) | yes | **PASS** — 12/12 checks, exit 0 |
| Rootless Podman build + run + `/v1/health` (`podman_smoke.sh`) | yes | **PASS** — non-root container served 200, exit 0 |

**The built `rest_gateway` binary genuinely serves the `/v1` API, both natively
and from inside a rootless, non-root Podman container.**
