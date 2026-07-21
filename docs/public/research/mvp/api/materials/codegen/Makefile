# =============================================================================
#  Helix Thready — SDK codegen Makefile (helix_proto pattern)
#  Location : docs/public/research/mvp/api/materials/codegen/Makefile
#  Pattern  : vasic-digital/helix_proto — `buf` -> Go/Rust (proto/event plane),
#             `openapi-generator` -> TS/Dart/Python/JVM/Swift/C++/C#/Ruby/PHP
#             (REST plane). See ../../sdk-strategy.md §2-§6.
#  Provenance: [DEFAULT — adjustable] targets mirror the helix_proto Makefile
#             gates named in sdk-strategy.md; [VERIFIED] buf/redocly present in
#             this env (buf 1.71.0, @redocly/cli 2.40.0), openapi-generator via
#             Docker or npx. Proto files are [OPEN: api-1] (produced in the
#             helix_thready_proto repo) — proto targets are runnable scaffolds
#             that no-op with a clear message until proto/ exists.
#
#  Usage:    make help
#            make openapi-lint openapi-generate-ts           # REST plane, no proto
#            make generate                                    # buf proto plane
#            make all                                         # full local gate
# =============================================================================

SHELL := /usr/bin/env bash

# ----- Inputs (override on the CLI: `make OPENAPI=/path/to/thready.v1.yaml`) --
# Default points at THIS docs area's canonical contract. In the real
# helix_thready_proto repo this is openapi/thready.v1.yaml.
OPENAPI      ?= ../../openapi.yaml
ASYNCAPI     ?= ../../asyncapi.yaml
PROTO_DIR    ?= proto
GEN_DIR      ?= gen
PREV_TAG     ?= .git\#tag=THREADY-latest   # buf breaking baseline (previous release)

# ----- Toolchain (auto-detected; override to pin a version) ------------------
BUF          ?= buf
REDOCLY      ?= npx --yes @redocly/cli@latest
# openapi-generator: prefer a local CLI, else Docker, else npx wrapper.
# NOTE (Docker fallback): the container mounts only MOUNT_ROOT as /local, so both
# the spec (OPENAPI) and the output (GEN_DIR) MUST live under it. The default
# OPENAPI=../../openapi.yaml sits ABOVE codegen/, so MOUNT_ROOT defaults to the
# api/ dir (two levels up) which contains both. Override MOUNT_ROOT/OPENAPI if you
# relocate the spec, or install openapi-generator-cli locally to avoid mounts.
MOUNT_ROOT   ?= ../..
OPENAPI_GEN  ?= $(shell command -v openapi-generator-cli 2>/dev/null || command -v openapi-generator 2>/dev/null)
ifeq ($(OPENAPI_GEN),)
OPENAPI_GEN  := docker run --rm -v $(abspath $(MOUNT_ROOT)):/local -w /local/materials/codegen openapitools/openapi-generator-cli:v7.9.0
# Re-root the relative OPENAPI (which is written relative to codegen/) onto /local.
GEN_IN       := /local/$(patsubst ../../%,%,$(OPENAPI))
GEN_OUT      := $(GEN_DIR)
else
GEN_IN       := $(OPENAPI)
GEN_OUT      := $(GEN_DIR)
endif

# Unknown enum values must round-trip as a pass-through variant (non-breaking
# enum growth) — sdk-strategy.md §5. Applied where the generator supports it.
GEN_COMMON   := --skip-validate-spec=false

.DEFAULT_GOAL := help

# =============================================================================
#  Meta
# =============================================================================
.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) \
	  | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-24s\033[0m %s\n", $$1, $$2}'

.PHONY: tools
tools: ## Print detected toolchain + versions
	@echo "buf              : $$($(BUF) --version 2>/dev/null || echo MISSING)"
	@echo "redocly          : $$($(REDOCLY) --version 2>/dev/null || echo MISSING)"
	@echo "openapi-generator: $(OPENAPI_GEN)"
	@echo "OPENAPI          : $(OPENAPI)"
	@echo "PROTO_DIR        : $(PROTO_DIR) (present: $$( [ -d $(PROTO_DIR) ] && echo yes || echo no ))"

# =============================================================================
#  Proto / event plane  —  buf  ->  Go (+Connect), Rust (+tonic), Dart
#  (helix_proto buf.gen.yaml: protocolbuffers/go, connectrpc/go, prost, tonic)
# =============================================================================
.PHONY: _require-proto
_require-proto:
	@if [ ! -d "$(PROTO_DIR)" ]; then \
	  echo "[skip] $(PROTO_DIR)/ not present — proto plane lives in the helix_thready_proto repo ([OPEN: api-1])."; \
	  echo "       Add proto/helix/thready/v1/*.proto (events, posts, assets, search) then re-run."; \
	  exit 0; \
	fi

.PHONY: lint
lint: ## buf lint the proto module (STANDARD ruleset)
	@$(MAKE) _require-proto
	@[ -d "$(PROTO_DIR)" ] && $(BUF) lint || true

.PHONY: breaking
breaking: ## buf breaking-change gate vs the previous release tag (versioning.md §5)
	@$(MAKE) _require-proto
	@[ -d "$(PROTO_DIR)" ] && $(BUF) breaking --against '$(PREV_TAG)' || true

.PHONY: generate
generate: ## buf generate Go/Rust/Dart stubs into $(GEN_DIR) (needs buf.gen.yaml)
	@$(MAKE) _require-proto
	@if [ -d "$(PROTO_DIR)" ]; then \
	  if [ ! -f buf.gen.yaml ]; then echo "ERROR: buf.gen.yaml missing (see README §buf.gen.yaml)"; exit 1; fi; \
	  $(BUF) generate; \
	  echo "[ok] buf generate -> $(GEN_DIR)/{go,rust,dart}"; \
	fi

.PHONY: rust-build
rust-build: ## cargo build the generated Rust crate (tonic) — drift/compile gate
	@if [ -f "$(GEN_DIR)/rust/Cargo.toml" ]; then cd $(GEN_DIR)/rust && cargo build; \
	 else echo "[skip] $(GEN_DIR)/rust not generated yet"; fi

# =============================================================================
#  REST plane  —  openapi-generator  ->  TS, Dart, Python, JVM, Swift, C++,
#  C#, Ruby, PHP  (+ redocly openapi-lint gate)
# =============================================================================
.PHONY: openapi-lint
openapi-lint: ## redocly lint the OpenAPI 3.1 contract (contract hygiene gate)
	@$(REDOCLY) lint $(OPENAPI)

.PHONY: openapi-generate-ts
openapi-generate-ts: ## openapi-generator -> TypeScript (typescript-fetch), then tsc --noEmit
	$(OPENAPI_GEN) generate -i $(GEN_IN) -g typescript-fetch -o $(GEN_OUT)/ts \
	  --additional-properties=supportsES6=true,npmName=@helix-thready/sdk $(GEN_COMMON)

.PHONY: openapi-generate-dart
openapi-generate-dart: ## openapi-generator -> Dart/Flutter (dart-dio)
	$(OPENAPI_GEN) generate -i $(GEN_IN) -g dart-dio -o $(GEN_OUT)/dart \
	  --additional-properties=pubName=helix_thready_sdk $(GEN_COMMON)

.PHONY: openapi-generate-python
openapi-generate-python: ## openapi-generator -> Python (sync + async)
	$(OPENAPI_GEN) generate -i $(GEN_IN) -g python -o $(GEN_OUT)/python \
	  --additional-properties=packageName=helix_thready,library=asyncio $(GEN_COMMON)

.PHONY: openapi-generate-kotlin
openapi-generate-kotlin: ## openapi-generator -> Kotlin (one JVM artifact; [OPEN: sdk-2])
	$(OPENAPI_GEN) generate -i $(GEN_IN) -g kotlin -o $(GEN_OUT)/kotlin \
	  --additional-properties=library=jvm-okhttp4,packageName=com.helix.thready $(GEN_COMMON)

.PHONY: openapi-generate-swift
openapi-generate-swift: ## openapi-generator -> Swift5 (iOS/macOS)
	$(OPENAPI_GEN) generate -i $(GEN_IN) -g swift5 -o $(GEN_OUT)/swift \
	  --additional-properties=projectName=HelixThready,responseAs=AsyncAwait $(GEN_COMMON)

.PHONY: openapi-generate-cpp
openapi-generate-cpp: ## openapi-generator -> C++ (cpp-restsdk); events use native protobuf
	$(OPENAPI_GEN) generate -i $(GEN_IN) -g cpp-restsdk -o $(GEN_OUT)/cpp $(GEN_COMMON)

.PHONY: openapi-generate-csharp
openapi-generate-csharp: ## openapi-generator -> C# (Mono/.NET)
	$(OPENAPI_GEN) generate -i $(GEN_IN) -g csharp -o $(GEN_OUT)/csharp \
	  --additional-properties=packageName=Helix.Thready,targetFramework=net8.0 $(GEN_COMMON)

.PHONY: openapi-generate-ruby
openapi-generate-ruby: ## openapi-generator -> Ruby
	$(OPENAPI_GEN) generate -i $(GEN_IN) -g ruby -o $(GEN_OUT)/ruby \
	  --additional-properties=gemName=helix_thready $(GEN_COMMON)

.PHONY: openapi-generate-php
openapi-generate-php: ## openapi-generator -> PHP
	$(OPENAPI_GEN) generate -i $(GEN_IN) -g php -o $(GEN_OUT)/php \
	  --additional-properties=invokerPackage=Helix\\Thready $(GEN_COMMON)

.PHONY: openapi-generate-all
openapi-generate-all: ## All REST-plane clients (TS, Dart, Python, Kotlin, Swift, C++, C#, Ruby, PHP)
openapi-generate-all: openapi-generate-ts openapi-generate-dart openapi-generate-python \
	openapi-generate-kotlin openapi-generate-swift openapi-generate-cpp \
	openapi-generate-csharp openapi-generate-ruby openapi-generate-php
	@echo "[ok] REST clients generated for 9 language targets (Zig is hand-written, [OPEN: api-3])."

# =============================================================================
#  Anti-drift & round-trip gates (sdk-strategy.md §6; contract-tests.md)
# =============================================================================
.PHONY: ts-check
ts-check: ## tsc --noEmit over the generated TS client (compile gate)
	@if [ -d "$(GEN_DIR)/ts" ]; then cd $(GEN_DIR)/ts && npx --yes typescript tsc --noEmit || tsc --noEmit; \
	 else echo "[skip] $(GEN_DIR)/ts not generated yet"; fi

.PHONY: check-no-handwritten
check-no-handwritten: ## Re-generate and assert gen/ was NOT hand-edited (drift guard)
	@echo "Regenerating and diffing $(GEN_DIR) — any diff means a hand-edit leaked into a generated core."
	@$(MAKE) --no-print-directory generate >/dev/null 2>&1 || true
	@$(MAKE) --no-print-directory openapi-generate-ts >/dev/null 2>&1 || true
	@if git -C . rev-parse >/dev/null 2>&1; then \
	  if ! git diff --quiet -- $(GEN_DIR) 2>/dev/null; then \
	    echo "FAIL: $(GEN_DIR) changed after regenerate — ergonomics belong in the thin layer, never in gen/."; \
	    git --no-pager diff --stat -- $(GEN_DIR); exit 1; \
	  else echo "[ok] no hand-written drift in $(GEN_DIR)"; fi; \
	else echo "[skip] not a git repo; cannot diff drift"; fi

.PHONY: roundtrip-test
roundtrip-test: ## Drive the generated TS client against a stub server incl. a 401 negative-control
	@if [ -x scripts/run_roundtrip_test.sh ]; then scripts/run_roundtrip_test.sh; \
	 else echo "[skip] scripts/run_roundtrip_test.sh not present (see README §round-trip)"; fi

# =============================================================================
#  Aggregate + housekeeping
# =============================================================================
.PHONY: all
all: ## Full local pre-tag gate (no server CI, §11.4.156)
all: lint breaking generate openapi-lint openapi-generate-all check-no-handwritten \
	rust-build ts-check roundtrip-test
	@echo "[ok] full codegen gate complete."

.PHONY: clean
clean: ## Remove generated output
	rm -rf $(GEN_DIR)
	@echo "[ok] removed $(GEN_DIR)"
