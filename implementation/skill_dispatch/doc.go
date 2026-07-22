// Package skilldispatch is the Helix Thready Skill-dispatch / execution engine.
//
// It closes [GAP: 4.1] from the processing-pipeline architecture: HelixSkills
// stores Skills as knowledge units (a DAG) but has NO execution engine. This
// package is that missing layer — it maps a claimed post's hashtags/content-type
// to the matching Skill(s), orders them by the documented stage precedence
//
//	download > convert > analyze > research > reply
//
// runs each Skill with idempotent single-claim (a post is processed exactly once
// even under duplicate "new post" triggers), retries transient failures with
// exponential backoff up to a ceiling (then marks that step dead), and emits an
// event per step start / success / failure through an injected event sink.
//
// The design is composition over helix_skills: the Skill-Graph remains the
// knowledge/ordering source, and this package adds only the execution layer.
// It depends on the standard library only.
//
// The four moving parts map to the architecture doc (§3–§5):
//
//   - Registry     — register Skills; Resolve(post) returns every matching Skill
//     (categories are additive — a post can match many Skills at once).
//   - OrderByPrecedence — stable sort of resolved Skills into stage order so that
//     later stages consume earlier outputs.
//   - ClaimRegistry — the exactly-once guarantee: Claim(postID) succeeds once and
//     only once per post, under a lock, rejecting duplicate triggers.
//   - Dispatcher    — Process(ctx, post): claim → resolve → order → run each step
//     with retry/backoff, emitting step events and tracking overall post state.
package skilldispatch
