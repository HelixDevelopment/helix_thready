<!--
  Title           : Helix Thready — Integration Capstone (end-to-end pipeline)
  Classification  : PUBLIC
  Location        : implementation/integration/README.md
  Status          : Active — GREEN (-race)
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready implementation track
  Related         : ../README.md, ./EVIDENCE.md, ../go.work
-->

# Helix Thready — Integration Capstone

`thready.integration` is the proof that the 14 self-contained implementation modules
**compose into the real end-to-end Thready processing pipeline** — using the *actual* modules
(imported via the parent `../go.work` workspace), not re-stubbed copies.

## What the pipeline test proves (`pipeline_test.go`, `compose_test.go`)

A realistic thread flows through the real modules, end to end, with real crypto / OCR / HTTP /
cosine / events / metering:

1. **`telegram_adapter`** maps a thread (root `#Video #Research` + organic replies adding
   `#ToDownload`) → **`threadreader`** assembles the organic thread + extracts hashtags.
2. **`skill_dispatch`** resolves + orders Skills by precedence (download → … → reply) and
   enforces **idempotent single-claim** (same post processed twice ⇒ each skill runs once).
3. A download Skill fetches bytes from a real `httptest` server via **`download_manager`**
   (sha256-verified) → stores them in **`asset_service`** (content-addressed, integrity-verified,
   tamper-detected).
4. An OCR Skill runs **`ocr_adapter`** on a PNG generated at test time (real `tesseract`).
5. **`semantic_search`** indexes the post + OCR text + a doc; a query retrieves the expected
   chunk via real cosine KNN.
6. Pipeline events fan through **`event_bus_service`** (durable ordered replay asserted).
7. **`callback_task`** fires an HMAC-signed completion webhook to an `httptest` receiver that
   **independently recomputes** the signature.
8. **`metering`** records usage → an invoice line is asserted.

Final assertions: processed exactly once · asset stored+verified · OCR text indexed+searchable ·
events in order · callback delivered with a valid HMAC · usage metered. **No committed module
required a change** — see [EVIDENCE.md](./EVIDENCE.md).

## Run

```
cd implementation            # the go.work workspace ties the modules together
go work sync
go build ./...
cd integration && go test ./... -race -count=1
```

Requires the same toolchain the modules use (Go 1.26, plus `tesseract` + ImageMagick `convert`
for the OCR leg). All other legs use `net/http/httptest` and stdlib crypto — no external services.

---

*Made with love ♥ by Helix Development.*
