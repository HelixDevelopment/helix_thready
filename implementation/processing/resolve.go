package processing

import "sort"

// SkillSet resolves a post to the Skills that apply to it — the knowledge/ordering
// seam. It is the runtime analogue of the helix_skills Skill-Graph resolve step:
// given a post, return the concrete Skills to run (hashtag/content-type -> Skills).
// The real skill_dispatch.Registry.Resolve satisfies this seam.
//
// Resolution is additive: a post may resolve to multiple Skills across multiple
// stages, which is exactly the "runs every matching Skill" semantic from the
// architecture (categories are additive, not exclusive). A SkillSet MAY return the
// Skills already ordered; the Processor applies OrderByPrecedence regardless, so a
// pre-ordered set is re-sorted idempotently.
type SkillSet interface {
	Resolve(post Post) []Skill
}

// SkillSetFunc adapts a plain function to the SkillSet seam.
type SkillSetFunc func(post Post) []Skill

// Resolve calls the underlying function.
func (f SkillSetFunc) Resolve(post Post) []Skill { return f(post) }

// OrderByPrecedence returns the Skills sorted into the deterministic stage order
//
//	download > convert > analyze > research > reply
//
// (processing-pipeline.md §5). The sort is STABLE: Skills of the same Kind keep
// their input order, so within a stage the order is deterministic. The input slice
// is not mutated; a new slice is returned.
func OrderByPrecedence(skills []Skill) []Skill {
	out := make([]Skill, len(skills))
	copy(out, skills)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Kind() < out[j].Kind()
	})
	return out
}
