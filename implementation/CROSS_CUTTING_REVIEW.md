<!--
  Title           : Helix Thready — Whole-Branch Cross-Cutting Review
  Classification  : PUBLIC
  Location        : implementation/CROSS_CUTTING_REVIEW.md
  Status          : Active — v1.0
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready implementation review (cross-cutting pass)
  Scope           : The 17 self-contained Go modules + integration capstone
  Related         : ./README.md · ./integration/EVIDENCE.md · ../docs/public/research/mvp/api/event-bus-contract.md
  Method          : Read-only. Per-module anti-bluff reviews are NOT repeated here;
                    this pass covers only what a single-module review cannot see.
-->

# Helix Thready — Whole-Branch Cross-Cutting Review

**Overall verdict: GREEN, with one MUST-FIX and one design decision to record.**

Everything composes and is honest at the tree level: all 17 modules build
standalone (`GOWORK=off`), the `go.work` workspace + integration capstone build
and pass `-race` over the **real** modules (no re-stubbing), `go vet` and
`gofmt` are clean tree-wide, the HMAC signature scheme is uniform across every
emitter, and the `[BUILD-NEW]` stubs fail loudly rather than fake success. No
cross-module contract is *broken* in the compile/runtime sense. Two items need
attention: (a) a **MUST-FIX** secret-redaction gap in `config`
(`THREADY_NATS_URL`), and (b) an **Important** but non-breaking
completion-envelope schema divergence (`job_id`/`success` vs
`task_id`/`succeeded`) whose "single provider-agnostic shape" claim is only
half-delivered and should be either aligned or explicitly documented as two
families.

---

## Topic 1 — Shared completion-envelope contract

Three modules emit an HMAC-signed completion/status envelope:
`callback_task`, `metube_webhook`, `boba_adapter`.

### Signature scheme — UNIFORM (no finding)

All three use byte-for-byte the same wire scheme, so a downstream receiver
verifies **one** signature the same way for all of them:

| Aspect        | callback_task | metube_webhook | boba_adapter |
|---------------|---------------|----------------|--------------|
| Header        | `X-Thready-Signature` | `X-Thready-Signature` | `X-Thready-Signature` |
| Prefix        | `sha256=` | `sha256=` | `sha256=` |
| MAC           | `hmac.New(sha256.New, secret)` | same | same |
| Encoding      | lowercase hex | lowercase hex | lowercase hex |
| Compare       | `hmac.Equal` (constant-time) | same | same |
| Signed bytes  | exact `json.Marshal(env)` sent | same | same |

The integration capstone independently proves this: a bare `httptest` receiver
recomputes `sha256=` + `hex(hmac(body))` and matches both the `callback_task`
and the `metube_webhook` deliveries (`pipeline_test.go` §8,
`compose_test.go::TestMeTubeWebhookCompletionSigned`).

### Body schema — TWO FAMILIES (Important)

The envelope **bodies** do not share one schema. There are two families:

* **Family A — completion (terminal only):** `metube_webhook` + `boba_adapter`
  `{job_id, state, progress, result_ref, error, ts}` with
  `state ∈ {success, failure}`. Their `Envelope` structs are **byte-identical**
  to each other (same field order, tags, and `CompletionState` values), so a
  Boba/MeTube receiver is genuinely uniform.
* **Family B — status (full lifecycle):** `callback_task`
  `{task_id, state, progress, result_ref, error, ts}` with
  `state ∈ {queued, running, succeeded, failed, retrying, dead}` — and it can be
  emitted mid-lifecycle, not only at terminal.

Two concrete divergences between the families:

1. **Identifier key:** `job_id` (A) vs `task_id` (B) — a different JSON key.
2. **State vocabulary:** `success`/`failure` (A) vs `succeeded`/`failed`(+four
   non-terminal states) (B).

**Impact — is this a real interoperability defect?** Yes, partially, and it is
worth stating plainly. `callback_task`'s own package doc claims it defines "one
canonical async-job contract … applied uniformly across the 3rd-party
integrations (Boba, MeTube, Download Manager) so the Processing Engine consumes
a single completion shape regardless of the underlying provider." But
`metube_webhook` and `boba_adapter` **do not route through `callback_task`** —
each `Poller`/`Bridge` builds its own `{job_id,…}` envelope and POSTs it
directly to a sink. So end-to-end there is **not** a single body shape: a
receiver that wants to ingest both families must branch on `job_id` vs `task_id`
and map two state vocabularies. It cannot deserialize both with one struct.

This is a schema/interop wrinkle, **not** a silent break — the signature is
uniform, and each family is internally consistent (Family A is byte-identical
across its two members). The families are also semantically distinct: a
*completion* signal (always terminal, 2 states) vs a *status* signal (lifecycle,
6 states).

**Documentation accuracy (Minor):** `boba_adapter/webhook.go` is honest — it
explicitly calls out that `callback_task` uses `task_id` and a
`succeeded`/`failed` vocabulary and flags it as a "pre-existing sibling
divergence, out of scope here." But `metube_webhook/webhook.go`'s comment is
**inaccurate**: it says its `Envelope` is "matching the canonical callback_task
shape `{job_id, state, …}`" — `callback_task`'s canonical shape uses `task_id`,
not `job_id`. That comment mis-describes its sibling and should be corrected.

### Verdict + recommendation

Real divergence, ranked **Important** (it lands on whoever writes the downstream
completion receiver), but non-breaking and cheap to resolve. **Pick one:**

* **Preferred — align to the declared canonical.** Since `callback_task`
  advertises itself as *the* canonical contract, rename Family A's `job_id →
  task_id` and map `success/failure → succeeded/failed`. That makes
  `callback_task`'s "single completion shape regardless of provider" promise
  actually true, and downstream code needs one struct + one state enum.
* **Alternative — bless the two-family split.** If completion-vs-status is a
  deliberate distinction, then (1) correct `callback_task`'s package doc to stop
  claiming a single provider-agnostic shape, (2) fix `metube_webhook`'s
  inaccurate `{job_id}` comment, and (3) add a shared contract doc that defines
  both families and, ideally, standardizes on **one identifier field name**
  (`task_id`) even if the state vocabularies stay different.

Cheapest correct path: standardize the identifier key to `task_id` across all
three and document that completion adapters emit only the terminal subset of
states.

---

## Topic 2 — Post-type contract across messenger adapters

### Field alignment — EXACT (no mismatch)

`threadreader.Post`, `telegramadapter.Post`, and `maxadapter.Post` are
field-for-field identical, and so are their `Attachment` types:

```
Post:       ID string · ThreadID string · ParentID string · AuthorID string
            Text string · TimestampUnix int64 · IsForwarded bool
            Attachments []Attachment
Attachment: ID string · MIME string · FileName string · SHA256 string
```

No field, type, or ordering mismatch. `TimestampUnix` is documented as Unix
**seconds** in all three (max_adapter normalizes OneMe's epoch-ms in
`ParseHistory`; telegram_adapter's `tg.Message.Date` is already seconds).

### The types are DISTINCT (caveat, honestly documented)

They are three separate Go types in three separate modules, so an adapter does
**not** literally satisfy `threadreader.MessageSource`. Note that
`telegram_adapter/client.go`'s compile-time assertion
`var _ MessageSource = (*TelegramThreadReader)(nil)` is against the adapter's
**own local** `MessageSource` (which returns `[]telegramadapter.Post`), not
`threadreader.MessageSource` (which returns `[]threadreader.Post`). Feeding
threadreader therefore requires an explicit per-`Post` conversion shim.

The source comments mostly use the accurate word "shape" ("satisfies the
threadreader.MessageSource **shape**"), which is honest. The phrase "the bridge
is a one-liner" (max_adapter) slightly understates it — it is a small
field-copy loop including the attachments slice — but the bridge does exist and
is real.

### Integration wiring — REAL and PROVEN

The capstone genuinely wires **telegram_adapter → threadreader** using the
adapter's **real, tested** mapper (not the `[BUILD-NEW]` `FetchThread` stub):

```
pipeline_test.go:  telegramadapter.MapMessages(tgMsgs)         // real mapper
                 → telegramToThreadReader(p)  (explicit bridge, incl. Attachments)
                 → threadreader.NewAssembler(bot).Assemble(trPosts)
```

`compose_test.go::TestMaxAdapterFeedsThreadReader` does the same for
`max_adapter` (`ParseHistory → maxToThreadReader → Assembler`). The bridge
comment in `pipeline_test.go` is explicit and honest: "telegramadapter.Post
mirrors threadreader.Post field-for-field but is a DISTINCT type in a DISTINCT
module, so the wiring layer converts explicitly." Both paths pass under `-race`.

**Result:** field-for-field aligned, no mismatch; composition proven with the
real mapping halves + explicit bridges. Only caveat is the Minor phrasing
overstatement above.

---

## Topic 3 — go.work integration reality

Re-ran, this session, on `go1.26.4`, tesseract 5.3.0 + ImageMagick present:

```
$ cd implementation/integration && go test ./... -race -count=1
ok  	thready.integration	1.21s

$ ... -v  (relevant lines)
--- PASS: TestMaxAdapterFeedsThreadReader
--- PASS: TestMeTubeWebhookCompletionSigned
--- PASS: TestRestGatewayComposes
--- PASS: TestThreadyPipelineEndToEnd (0.19s)     ← full capstone, NOT skipped
```

* **Composes the REAL modules.** `integration/go.mod` `require`s the
  `digital.vasic.*` modules and `replace`s each to its `../<dir>` sibling; the
  tests import and drive the actual packages — real `download_manager` (segmented
  HTTP range download), real `asset_service.ContentStore` (sha256 store +
  `ErrIntegrity` tamper detection), real `ocr.TesseractProvider` (real
  `tesseract`), real `semsearch` cosine retrieval, real `event_bus_service`
  durable ordered replay, real `callback_task` HMAC delivery, real `metering`
  billing. Nothing under test is re-stubbed.
* **`go.work` lists 15** — the 14 modules + `integration` — exactly as expected.
  `go work sync` is clean; `go vet ./integration/...` is clean; all 17 modules
  build standalone under `GOWORK=off`.

### Finding (Important, tree-level): 3 modules live OUTSIDE the workspace

`go.work` and the integration capstone **exclude** `boba_adapter`, `config`, and
`sdk_go` (the three newest, still git-untracked). Consequences:

* **`boba_adapter`** — the *third* completion-envelope emitter (Topic 1) — is not
  exercised by any cross-module test. Its "byte-identical to metube_webhook"
  property rests on code inspection + its own unit tests, not on the capstone.
* **`config`** (the central configuration surface) and **`sdk_go`** are likewise
  never composed with the rest.

Not a correctness defect (all three build + test standalone), but it means "the
workspace" is not "the whole tree," and the capstone proves 14/17. Recommend
either adding these to `go.work` (with at least a token integration touchpoint
for `boba_adapter`) or documenting in `README.md` that they are intentionally
standalone.

---

## Topic 4 — Consistency across the tree

* **HMAC schemes — consistent and correct.** The three completion emitters are
  identical (Topic 1 table). Other HMAC uses are appropriate, not divergent:
  `user_service/totp.go` uses `sha1` (mandated by RFC 6238 TOTP), and
  `user_service/jwt.go` + `rest_gateway/token.go` use `sha256` for HS256 JWT
  signing. No inconsistency.
* **Error-wrapping — consistent where used.** `fmt.Errorf` with `%w` and a
  package-name prefix (`"callbacktask: …: %w"`) is the dominant style
  (asset_service 41/45 wrap, callback_task 8/9, metube_webhook 8/12,
  semantic_search 10/13, boba_adapter 12/16). Modules with zero `fmt.Errorf`
  (`event_bus_service`, `metering`, `rest_gateway`) use sentinel `errors.New` /
  typed errors — acceptable. `config` uses a validation aggregator (`addf`) that
  emits leaf user-facing messages where wrapping is not applicable. No defect.
* **`[BUILD-NEW]` stub honesty — uniform.** Every live stub returns
  `fmt.Errorf("%w: …", ErrNotImplemented, …)` and fails loudly; none fabricate a
  success. Verified in asset_service, download_manager, telegram_adapter,
  max_adapter, and ocr_adapter (hybrid fallback).
* **Module-path naming — one cosmetic outlier.** 16/17 follow
  `digital.vasic.<compact>` (integration is `thready.integration`). The
  messenger/vision adapters use a `…adapter` suffix consistently —
  `telegramadapter`, `maxadapter`, `bobaadapter` — **except** `ocr_adapter`,
  whose module is `digital.vasic.ocr` and package is `ocr` (drops "adapter").
  Cosmetic Minor.
* **`gofmt` — clean** across all 17 modules + `integration`.

---

## Topic 5 — Minor-findings triage (MUST-FIX vs OK)

### MUST-FIX before promotion to own repos

| # | Module | Finding | Why it must be fixed |
|---|--------|---------|----------------------|
| 1 | **config** | `THREADY_NATS_URL` (`EventBus.NATSURL`) is **not** redacted by `Config.Redacted()`/`String()`, while the sibling `Cache.RedisURL` **is**. | NATS URLs can embed credentials (`nats://user:pass@host:4222`). The module's headline guarantee is that `String()` "redacts credential-bearing DSNs/URLs" — so this is an internal contradiction and a real (conditional) secret leak into any log that prints the config. One-line fix: `r.EventBus.NATSURL = maskVal(c.EventBus.NATSURL)`. Apply the same to `Observability.OTLPEndpoint` for consistency (lower risk — OTLP auth is header-based — so *recommended*, not strictly must). |

### OK / acceptable-as-is (documented, or already fixed)

| Module | Finding | Disposition |
|--------|---------|-------------|
| metube_webhook | Doc comment claims its envelope matches "the canonical callback_task shape `{job_id,…}`" — callback_task actually uses `task_id`. | Minor **doc** fix; bundle with the Topic-1 decision. Not a code defect. |
| ocr_adapter | `EVIDENCE.md` records `go test -v -count=1` **without** `-race`. | Doc gap only — I ran `-race` this session: `ok digital.vasic.ocr` (exit 0). Optionally re-record with `-race`. OK. |
| user_service | Deferred minors: PBKDF2 (not Argon2id), `rand` error handling, O(n) APIKey scan, `MaskKey`, `counterAt` wrap, `RevokeAll` `>=` assert. | OK for MVP — PBKDF2 is an acceptable, cost-tunable KDF; the rest are defensive/cosmetic. Revisit Argon2id if the threat model hardens. |
| boba_adapter | Magnet/Torrent `result_ref` fallback was untested. | **Already fixed** (test added in "Review fixes"). OK. |
| event_bus_service | `EventID` idempotency-key path was untested. | **Already fixed** (`TestEventID_`, stable at `-count=20`). OK. |
| download_manager | Shutdown / after-shutdown races. | **Already fixed** (2 new tests, `-race -count=20`). OK. |
| (several) | Untested defensive branches. | OK — defensive, low-risk; documented per-module. |
| ocr_adapter | Module/package name drops the `adapter` suffix. | Cosmetic Minor. OK. |
| boba_adapter / config / sdk_go | Outside `go.work` / integration. | Important for `boba_adapter` (envelope story, Topic 3), Minor for `config`/`sdk_go`. Document or include. |

---

## MUST-FIX list (consolidated)

1. **`config`: redact `THREADY_NATS_URL`** in `Config.Redacted()` (and, for
   consistency, `OTEL_EXPORTER_OTLP_ENDPOINT`). It is credential-capable and
   currently prints in cleartext from `String()`, contradicting the module's own
   redaction contract and diverging from how `RedisURL` is handled. One-line fix
   + one assertion in `TestConfig_RedactionHidesSecrets`.

### Recommended (not blocking)

2. Resolve the completion-envelope divergence (Topic 1): align Family A's
   `job_id/success` to the canonical `task_id/succeeded`, **or** document the two
   families and fix `callback_task`'s package doc + `metube_webhook`'s inaccurate
   comment.
3. Bring `boba_adapter` (and, if desired, `config` / `sdk_go`) into `go.work` +
   an integration touchpoint, or document them as intentionally standalone.

---

*Method note: this pass is read-only except for authoring this file. It did not
re-run per-module reviews; it re-ran the workspace build/vet/fmt, the
`integration` suite under `-race`, and `ocr_adapter` under `-race`, and read the
envelope/Post/HMAC/config source and every module's `EVIDENCE.md`.*
