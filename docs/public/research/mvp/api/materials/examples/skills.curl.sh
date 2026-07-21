#!/usr/bin/env bash
# =============================================================================
#  Helix Thready — SKILLS request collection (curl)
#  Tag: skills  |  Source of truth: ../../openapi.yaml
#  x-thready-maturity: foundation. The Skill-Graph is a knowledge DAG (read here);
#  execution is the separate build_new dispatch engine (see /posts/{id}/process).
#  registerSkill requires scope skills:write.
# =============================================================================
set -uo pipefail
: "${THREADY_BASE:=https://dev.thready.hxd3v.com/v1}"
AUTH=(-H "Authorization: Bearer $THREADY_TOKEN")

# --- GET /skills — list Skill-Graph knowledge units --------------------------
curl -sS "$THREADY_BASE/skills?limit=50" "${AUTH[@]}"

# --- POST /skills — register a Skill + its typed Skill-Graph edges ------------
curl -sS -X POST "$THREADY_BASE/skills" "${AUTH[@]}" \
  -H "Idempotency-Key: $THREADY_IDEMPOTENCY_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
        "id":"video.download",
        "name":"Video Download",
        "version":"1.0.0",
        "kind":"atomic",
        "edges":[
          {"type":"requires","target":"asset.store"},
          {"type":"recommends","target":"video.convert"}
        ],
        "binds_content_types":["video","movie","series"],
        "sort_order":10
      }'

# --- GET /skills/{skillId} — a single Skill and its edges --------------------
curl -sS "$THREADY_BASE/skills/$THREADY_SKILL_ID" "${AUTH[@]}"
