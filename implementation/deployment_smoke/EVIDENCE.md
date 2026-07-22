<!--
  Title           : Helix Thready — rest_gateway deployment smoke test (EVIDENCE)
  Classification  : PUBLIC
  Location        : implementation/deployment_smoke/EVIDENCE.md
  Status          : Active — smoke PASS (12/12); container leg deferred
  Revision        : 1 (2026-07-22)
-->

# Deployment smoke test — real HTTP against the built gateway

## smoke.sh — real run (build binary -> serve /v1 -> curl -> assert)

```
== Helix Thready rest_gateway — deployment smoke test ==
host:        Linux 6.12.41-6.12-alt1 x86_64
go:          go version go1.26.4-X:nodwarf5 linux/amd64
date (UTC):  2026-07-22T14:56:33Z
-------------------------------------------------------------------------------
[build] GOWORK=off go build -o /tmp/.private/milos/thready-gateway.VLmvUe ./cmd/gateway  (in /home/milos/Factory/projects/tools_and_research/helix_thready/implementation/deployment_smoke/../rest_gateway)
[build] OK -> /tmp/.private/milos/thready-gateway.VLmvUe
-rwxr-xr-x 1 milos milos 9428679 Jul 22 19:56 /tmp/.private/milos/thready-gateway.VLmvUe
-------------------------------------------------------------------------------
[run] chosen free port: 40215
[run] started gateway pid=4002720 on http://127.0.0.1:40215
[run] health ready after 200ms (attempt 1)
-------------------------------------------------------------------------------
### CHECK 1 — GET /v1/health  (public; expect 200 + JSON)
HTTP 200  <-  GET http://127.0.0.1:40215/v1/health
body: {"service":"rest-gateway","status":"ok","time":"2023-11-14T22:13:20Z","version":"v1"}
PASS  health.status_code                          got=200 (want=200)
PASS  health.body_is_json                         got=object (want=object)
PASS  health.body_status_ok                       body contains "status":"ok"
-------------------------------------------------------------------------------
### CHECK 2 — GET /v1/channels WITHOUT auth  (protected; expect 401)
HTTP 401  <-  GET http://127.0.0.1:40215/v1/channels   (no Authorization header)
body: {"error":{"code":"unauthenticated","message":"missing or malformed Authorization header","request_id":"81274c6cd6da2730d1fa263b12c81cb9","status":401,"trace_id":"81274c6cd6da2730d1fa263b12c81cb9"}}
PASS  channels_noauth.status_code                 got=401 (want=401)
PASS  channels_noauth.err_code                    body contains "code":"unauthenticated"
-------------------------------------------------------------------------------
### CHECK 3 — POST /v1/auth/login  (seeded user; expect 200 + token)
request body: {"email":"user@thready.test","password":"userpassword-123"}   (seeded creds from rest_gateway/services.go: SeedUserEmail / SeedUserPass, no TOTP)
HTTP 200  <-  POST http://127.0.0.1:40215/v1/auth/login
token_type: Bearer   access_token[0:24]: eyJhbGciOiJIUzI1NiIsInR5...   (len=451)
PASS  login.status_code                           got=200 (want=200)
PASS  login.token_type_bearer                     got=Bearer (want=Bearer)
PASS  login.access_token_present            token len=451
PASS  login.token_is_jwt_3parts                   got=2 (want=2)
-------------------------------------------------------------------------------
### CHECK 4 — GET /v1/channels WITH Bearer token  (expect 200)
HTTP 200  <-  GET http://127.0.0.1:40215/v1/channels   (Authorization: Bearer <token>)
body: {"data":[{"id":"chan-1","account_id":"acct-a","name":"general","platform":"telegram","external_ref":"@thready_general","created_at":"2023-11-14T22:13:20Z"}],"meta":{"next_cursor":null,"total_estimate":1}}
PASS  channels_auth.status_code                   got=200 (want=200)
PASS  channels_auth.has_data                      body contains "data"
PASS  channels_auth.seed_channel                  body contains "chan-1"
-------------------------------------------------------------------------------
== server access log (structured JSON, proves the requests hit the server) ==
{"time":"2026-07-22T19:56:34.056112878+05:00","level":"INFO","msg":"rest-gateway listening","addr":":40215"}
{"time":"2026-07-22T19:56:34.060009223+05:00","level":"INFO","msg":"access","method":"GET","path":"/v1/health","status":200,"duration_ms":0,"request_id":"e978439e089eeda3ad78ec8fa56f445b"}
{"time":"2026-07-22T19:56:34.067348495+05:00","level":"INFO","msg":"access","method":"GET","path":"/v1/health","status":200,"duration_ms":0,"request_id":"9f0c0e4975c9b5ca9533d25db47c7336"}
{"time":"2026-07-22T19:56:34.086224748+05:00","level":"INFO","msg":"access","method":"GET","path":"/v1/channels","status":401,"duration_ms":0,"request_id":"81274c6cd6da2730d1fa263b12c81cb9"}
{"time":"2026-07-22T19:56:34.098043967+05:00","level":"INFO","msg":"access","method":"POST","path":"/v1/auth/login","status":200,"duration_ms":0,"request_id":"2d02d2efa91711abf4dd649e4b7d83d1"}
{"time":"2026-07-22T19:56:34.11816925+05:00","level":"INFO","msg":"access","method":"GET","path":"/v1/channels","status":200,"duration_ms":0,"request_id":"fddbc98cf389845360c6079872a024f2"}
-------------------------------------------------------------------------------
== SUMMARY ==
checks passed: 12
checks failed: 0
VERDICT: PASS
```

## Rootless Podman container leg (Constitution §11.4.161)

Containerfile (multi-stage build, non-root USER) and podman_smoke.sh are provided and ready.
HONEST STATUS: the rootless-Podman base-image pull did not complete within this
environment's time budget, so the container build/run is DEFERRED — NOT faked. Run
`bash podman_smoke.sh` on a host with a warm Podman image cache to capture that leg.
The HTTP surface itself is already proven real by the smoke.sh run above (12/12 PASS,
with the server's structured access log showing the requests hitting the process).
