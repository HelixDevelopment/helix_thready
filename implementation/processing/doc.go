// Package processing is the reusable Helix Thready Processing-Engine core
// (digital.vasic.processing): the shippable orchestrator that composes the
// pipeline modules at runtime for a single claimed post.
//
// It realises the runtime behaviour of the processing pipeline described in
// docs/public/research/mvp/architecture/processing-pipeline.md — claim a post
// exactly once, resolve and order the matching Skills by the deterministic stage
// precedence (download -> convert -> analyze -> research -> reply), run each step
// with retry/backoff and dead-lettering, emit per-step lifecycle events, and fire
// a completion callback carrying the final state.
//
// # Composition by seams, not by import
//
// The integration capstone (implementation/integration) already proved the real
// modules compose in a test. This package is the shippable orchestrator that
// composes them at RUNTIME. It imports NO sibling modules. Every collaborator is
// injected behind a small interface (a "seam"):
//
//   - Claimer      — idempotent single-claim per post id (exactly-once). The real
//     skill_dispatch.ClaimRegistry / a Postgres claim registry satisfy it.
//   - SkillSet     — resolve a post to the Skills that apply (hashtag/content-type
//     -> Skills). The real skill_dispatch.Registry.Resolve satisfies it.
//   - Skill        — a runnable step: Name/Kind/Run. skill_dispatch.Skill satisfies
//     it (StepRunner is an alias of Skill).
//   - EventEmitter — receives per-step lifecycle events. An event_bus_service
//     adapter satisfies it (mirrors skill_dispatch.StepEvent).
//   - Callbacker   — fires the completion callback. A callback_task.WebhookSink
//     adapter satisfies it (Completion mirrors callback_task.Envelope field-for-
//     field, so the adapter is a straight copy).
//
// The Post type mirrors the threadreader.Post contract (id, threadID, hashtags,
// text, attachments) so a thin adapter maps a threadreader thread onto it.
//
// Nothing here claims the real modules are imported; it deliberately depends only
// on the Go standard library so it can be shipped and unit-tested on its own, and
// the real modules plug in through thin adapters that the capstone already proved
// satisfy such seams.
package processing
