<!--
  Title           : Helix Thready — API Versioning Policy
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/api/versioning.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-21)
  Author          : Helix Thready documentation swarm (API & SDKs)
  Related         : ./openapi.yaml, ./rest-endpoints.md, ./sdk-strategy.md, ./error-model.md
-->

# Helix Thready — API Versioning Policy

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-21 | swarm (API & SDKs) | Initial draft: URL-path `/v1`, additive-vs-breaking, buf breaking gate |
| 2 | 2026-07-21 | swarm (API & SDKs) | Linked the breaking-change gate to its RED-first skeleton in contract-tests.md |

## Table of Contents

1. [Decision](#1-decision)
2. [What `/v1` covers](#2-what-v1-covers)
3. [Additive vs breaking](#3-additive-vs-breaking)
4. [REST ↔ Protobuf parity](#4-rest--protobuf-parity)
5. [Breaking-change gates (no server CI)](#5-breaking-change-gates-no-server-ci)
6. [Deprecation & Sunset](#6-deprecation--sunset)
7. [SDK & release alignment](#7-sdk--release-alignment)
8. [Open items](#8-open-items)

## 1. Decision

**URL-path versioning: `/v1`.** Resolves **Q29** `[DEFAULT — adjustable]`. Every REST
route is mounted under `/v1` (`https://thready.hxd3v.com/v1/...`). The parallel Protobuf
contract uses the matching package suffix **`helix.thready.v1`**, exactly as `helix_proto`
pins `helix.<domain>.v1` `[RESEARCH: helix_proto]`. A future incompatible surface ships as
`/v2` + `helix.thready.v2`, served **side-by-side** with `/v1` during a deprecation window.

Rationale: path versioning is unambiguous in logs, caches, proxies and SDK base URLs;
it is the org convention (HelixVPN `openapi/helix.v1.yaml` is served at `/v1`).

## 2. What `/v1` covers

`/v1` is the whole authenticated REST surface in `openapi.yaml` **plus** the real-time
endpoints in [event-bus-contract.md](./event-bus-contract.md) (`/v1/events/ws`,
`/v1/events/stream`). The unauthenticated ops endpoints (`/healthz`, `/readyz`,
`/metrics`) are intentionally **unversioned** — they are operational, not part of the
product contract, matching the `helix_proto` decision to keep ops routes out of the
versioned slice.

## 3. Additive vs breaking

Within a major version we make **additive-only** changes. The contract distinguishes:

**Non-breaking (allowed within `/v1`, no version bump):**

- Add a new endpoint, a new optional request field, or a new response field.
- Add a new enum value **only where the client is documented to tolerate unknowns**
  (e.g. `Error.code`, `ContentType`, event `type`). SDKs treat unknown enum values as a
  pass-through/`unknown` variant — see [sdk-strategy.md](./sdk-strategy.md).
- Add a new optional query parameter or header.
- Relax a validation constraint; add a new error `code`.

**Breaking (requires `/v2`):**

- Remove/rename a field, endpoint, or enum value; change a field's type or cardinality.
- Make an optional field required, or tighten validation on existing input.
- Change default behaviour, pagination semantics, or an auth requirement.
- Change the meaning of an existing error `code` or status mapping.

> Adding an enum value is treated as **potentially breaking for naive clients**. Because
> Thready's SDKs are generated with unknown-tolerant enums, it is additive **for
> conformant SDK users** and called out in the changelog regardless.

## 4. REST ↔ Protobuf parity

Two schema-first contracts describe one system (see [sdk-strategy.md](./sdk-strategy.md)):

| Plane | Contract | Version token |
|-------|----------|---------------|
| REST `/v1` | `openapi/thready.v1.yaml` | path `/v1` + `info.version` semver |
| Events / DTO wire | `proto/helix/thready/v1/*.proto` | package `helix.thready.v1` |

The two move together: a change that is breaking in one is breaking in the other, and both
bump to `v2` at the same time. Shared DTOs (e.g. the event envelope) are defined once in
Protobuf and mirrored in the OpenAPI `components.schemas` so the shapes cannot drift; a
contract test asserts the JSON shapes match (see [testing/](../testing/index.md)).

## 5. Breaking-change gates (no server CI)

Server-side CI/CD is forbidden `[CONSTITUTION §11.4.156]`, so version discipline is
enforced by **local git-hooks + pre-tag full-suite retest** `[CONSTITUTION §11.4.75/40]`,
reusing the exact `helix_proto` gates `[VERIFIED: helix_proto Makefile]`:

- **`buf breaking`** — the proto contract is diffed against the previous tag with
  `buf breaking --against '.git#tag=<prev>'`; a breaking change inside `v1` **fails the
  commit** (helix_proto's `buf.yaml` sets `breaking: use: [FILE]`, and its
  `scripts/buf_breaking_gate.sh` wires it). Renumbering/removing fields inside `v1` is
  rejected.
- **OpenAPI diff** — `openapi.yaml` is linted (`openapi-lint`) and diffed against the
  previous tag; removals/renames fail. (`oasdiff`-style check, run locally as part of the
  Makefile `all` target modelled on helix_proto's `lint → breaking → generate →
  check-no-handwritten → rust-build → openapi-lint → openapi-generate-ts → roundtrip-test`.)
- **`check-no-handwritten`** — asserts generated SDK cores were not hand-edited (drift
  guard), exactly as helix_proto does.

Only after these pass GREEN, a project-prefixed tag `THREADY-<version>` is cut and pushed
to all four upstreams `[CONSTITUTION §11.4.151/§2.1]`. The RED-first skeleton that proves the
`buf breaking` and OpenAPI-diff gates actually reject a removed field lives in
[contract-tests.md](./contract-tests.md) §full-automation.

## 6. Deprecation & Sunset

When `/v2` ships, `/v1` enters a deprecation window (`[DEFAULT — adjustable]` ≥ 6 months):

- Deprecated operations set `deprecated: true` in OpenAPI and return an RFC 8594
  **`Sunset: <date>`** response header (declared as `components.headers.Sunset`), plus a
  `Deprecation: true` header.
- The changelog + each SDK release note the deprecation; SDKs emit a one-time warning when
  a deprecated endpoint is called.
- After the window, `/v1` is removed in a tagged release; the proto `v1` package is
  likewise retired.

## 7. SDK & release alignment

- `info.version` in `openapi.yaml` follows **semver**: MINOR for additive, PATCH for
  fixes, MAJOR only alongside a new URL path segment (`/v2`).
- SDKs are versioned to track the contract's MAJOR.MINOR and published per language
  registry on each `THREADY-<version>` tag (see [sdk-strategy.md](./sdk-strategy.md)).
- Tags are mirrored onto every owned submodule and all four upstreams
  `[CONSTITUTION §11.4.151/§2.1]`; releases carry a changelog + multi-format doc export
  (Docs Chain).

## 8. Open items

- `[OPEN: ver-1]` The concrete OpenAPI-diff tool (`oasdiff` vs a custom Go differ) is
  selected with the tooling pack; the *policy* (additive-only, local gate) is fixed here.
- `[OPEN: ver-2]` Whether to expose a machine-readable `/v1/version` endpoint (build +
  contract hash) is `[DEFAULT — adjustable]`; proposed yes for support diagnosability.

---

*Made with love ♥ by Helix Development.*
