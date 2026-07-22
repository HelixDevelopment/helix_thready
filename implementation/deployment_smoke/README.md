# rest_gateway — deployment smoke test

A **real** deployment smoke test for the Helix Thready `rest_gateway`
(Go module `digital.vasic.restgateway`). It builds the runnable `cmd/gateway`
server, starts it, and drives the live `/v1` HTTP surface with genuine `curl`
round-trips — asserting the status code and body of each. Every result in
[`EVIDENCE.md`](./EVIDENCE.md) is captured from an actual run; nothing is
stubbed, mocked, or hand-written.

## What this proves

1. **The built binary actually serves the API.** `smoke.sh` compiles
   `./cmd/gateway`, launches it on a free port, waits for `/v1/health`, then
   exercises the real request path — public route, auth rejection, login, and an
   authenticated read — through the full middleware chain
   (request-id → access-log → panic-recovery → authn → RBAC role floor → scopes).
2. **It runs in a rootless Podman container.** `Containerfile` builds a
   multi-stage image (pinned Go toolchain → `CGO_ENABLED=0` static binary →
   `distroless/static:nonroot`, `USER 65532`, `EXPOSE 8080`). `podman_smoke.sh`
   builds and runs it rootless and curls `/v1/health` from the published port.
   This is the concrete evidence for the Constitution's rootless-Podman mandate
   (§11.4.161).

## Checks (host leg — `smoke.sh`)

| # | Request | Expect |
|---|---------|--------|
| 1 | `GET /v1/health` (public) | `200` + JSON `{"status":"ok",...}` |
| 2 | `GET /v1/channels` **without** `Authorization` | `401` `unauthenticated` |
| 3 | `POST /v1/auth/login` with seeded creds | `200` + `access_token` (signed HS256 JWT) |
| 4 | `GET /v1/channels` **with** `Authorization: Bearer <token>` | `200` + channel list |

**Seeded credentials** come straight from
[`rest_gateway/services.go`](../rest_gateway/services.go) (`NewInMemoryServices`):
the standard user `user@thready.test` / `userpassword-123` (role `user`, scope
`posts:read`, no TOTP required). The gateway's backing services are honest
in-memory stubs — real, observable end-to-end behaviour without the sibling
domain modules — so the smoke test needs no database or external dependency.

## Run it

```bash
# Host leg — build + serve + 4 asserted HTTP checks. Exits nonzero on any fail.
./smoke.sh

# Container leg — rootless Podman build + run + /v1/health curl (best-effort).
./podman_smoke.sh
```

`smoke.sh` is hermetic: it picks a free port, traps `EXIT` to kill the server,
and cleans up its temp binary. `podman_smoke.sh` stages a build context so the
literal `podman build -t thready-gateway-smoke .` runs, then `podman run --rm -d`.

### Exit codes

- `smoke.sh`: `0` = all checks passed, `1` = at least one check failed.
- `podman_smoke.sh`: `0` = container served the API; `3` = environment could not
  build/run the container (e.g. no registry egress) → leg **honestly SKIPPED**
  with the real error printed; `1` = container ran but the API assertion failed.

## Files

- `smoke.sh` — host build-and-serve smoke test (bash, `set -euo pipefail`).
- `Containerfile` — rootless-Podman multi-stage image (distroless, non-root).
- `podman_smoke.sh` — best-effort rootless container build + run + curl.
- `EVIDENCE.md` — captured transcripts of the real runs.
