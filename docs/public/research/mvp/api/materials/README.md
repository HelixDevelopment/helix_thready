<!--
  Title           : Helix Thready — API Materials (Index)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/api/materials/README.md
  Status          : Active — v1.0
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready documentation swarm (API & SDKs — materials)
  Related         : ../openapi.yaml, ../event-bus-contract.md, ../sdk-strategy.md,
                    ../index.md, ../../CONVENTIONS.md,
                    ./examples/README.md, ./codegen/README.md, ./validation.md
-->

# Helix Thready — API Materials (Index)

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (API & SDKs — materials) | Initial materials pack: request collections, codegen harness, OpenAPI validation report |

Concrete, hands-on artifacts derived from the API area’s schema-first contracts
([`../openapi.yaml`](../openapi.yaml), [`../event-bus-contract.md`](../event-bus-contract.md)).
Where the parent area *specifies* the contract, this folder makes it **executable**:
copy-pasteable requests, a working codegen Makefile, and a validation record.

## Contents

| Path | What it is | Runnable? |
|------|-----------|-----------|
| [`examples/`](./examples/README.md) | Per-endpoint-group request collections — **curl** (`*.curl.sh`) **and HTTPie** (`*.http`) for all 10 groups (auth, accounts, channels, posts, processing, assets, search, skills, billing, events), plus `env.example`. | ✅ curl scripts pass `bash -n`; need a token + base URL to hit a server. |
| [`codegen/`](./codegen/README.md) | The **buf + openapi-generator** codegen harness (`Makefile`, `buf.gen.yaml`, `buf.yaml`) implementing the `helix_proto` pattern → 11 SDK languages. | ✅ REST/lint targets run now; ⚠️ proto targets skip cleanly until `proto/` exists (`[OPEN: api-1]`). |
| [`validation.md`](./validation.md) | Structural validation of `../openapi.yaml` with `@redocly/cli` 2.40.0 + a PyYAML hand-lint of the required 3.1 fields. | — record only. |

## Headline findings

- **Validation:** `../openapi.yaml` is a structurally valid OpenAPI 3.1 skeleton (46 paths /
  59 ops / 40 schemas / 1 webhook, all `$ref`s resolve) but currently **FAILS lint with 2
  errors** — two `description:` values contain an unquoted comma, so YAML mis-parses each into
  a spurious schema property (independently confirmed with PyYAML). Both are one-line quote
  fixes in `../openapi.yaml`; raised as `[OPEN: val-1]` because this pack writes only inside
  `materials/`. The 39 warnings are style/convention advisories (details in
  [`validation.md`](./validation.md)).
- **Codegen:** the REST-plane and lint targets are real and were parse/dry-run verified
  (`make help`, `make tools`, `make -n`); the proto plane is a **verified-pattern scaffold**
  that is inert until the `helix_thready_proto` repo provides `proto/…`. No SDK was actually
  generated in this environment — see the anti-bluff note in
  [`codegen/README.md` §8](./codegen/README.md).

## How this maps to the parent area

| Parent doc | Materials that exercise it |
|-----------|----------------------------|
| [`../openapi.yaml`](../openapi.yaml) | `examples/*` (every operation), `codegen/` (`openapi-generate-*`), `validation.md` |
| [`../event-bus-contract.md`](../event-bus-contract.md) | `examples/events.*` (catalog, sticky, sinks, SSE, WS note) |
| [`../authn-authz.md`](../authn-authz.md) | `examples/auth.*`, the Bearer/HMAC/idempotency conventions |
| [`../sdk-strategy.md`](../sdk-strategy.md) | `codegen/Makefile` + `buf.gen.yaml` + `buf.yaml` (helix_proto pattern) |

## Open items (rolled up)

- `[OPEN: val-1]` Fix the two `description` mis-parses in `../openapi.yaml` (out of this
  pack’s write scope; materials-only).
- `[OPEN: val-3]` Only redocly ran here; re-run Spectral + `openapi-generator validate` with
  network for independent structural opinions.
- `[OPEN: api-1]` Proto plane produced in `helix_thready_proto`; codegen proto targets are
  ready-but-inert until then.
- `[OPEN: ex-1 / ex-2]` WebSocket is not curl/HTTPie-able (SSE shown; WS via `websocat`); no
  request was run against a live server.

This folder obeys [`../../CONVENTIONS.md`](../../CONVENTIONS.md) and never contradicts the
authoritative sources (final answered request, subsystem gap register). Facts tagged
`[VERIFIED]` were observed in-tool; proposals are `[DEFAULT — adjustable]`; unresolved work is
tracked `[OPEN: …]`.

---

*Made with love ♥ by Helix Development.*
