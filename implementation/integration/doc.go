// Package integration is the Helix Thready INTEGRATION CAPSTONE.
//
// It contains no production code of its own. Its job is to wire the fourteen
// committed implementation modules together through a go.work workspace and
// prove — with a REAL, race-clean, end-to-end test over the REAL modules (no
// re-stubbing the modules under test) — that they COMPOSE into the actual
// Thready processing pipeline:
//
//	identity/RBAC (userservice)
//	  -> ingest (telegramadapter.MapMessages -> threadreader.Assembler + ExtractHashtags)
//	  -> dispatch (skilldispatch: precedence + idempotent single-claim)
//	       -> download (downloadmanager, sha256-verified) -> store (assetservice content store)
//	       -> OCR (ocr_adapter over a real tesseract on a real PNG)
//	  -> index + semantic query (semantic_search, real cosine)
//	  -> events (event_bus_service durable ordered replay)
//	  -> completion callback (callback_task HMAC webhook, independently verified)
//	  -> usage metering + invoice (metering)
//
// A second composition test exercises the remaining adapters/surfaces
// (max_adapter, metube_webhook, rest_gateway) against the same seams.
//
// See pipeline_test.go, compose_test.go, README.md and EVIDENCE.md.
package integration
