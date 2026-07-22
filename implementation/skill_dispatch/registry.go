package skilldispatch

import "sync"

// Registry holds the set of registered Skills and resolves the ones that apply
// to a post. It is the execution-layer analogue of the helix_skills Skill-Graph
// resolve step: given a post, return the concrete Skills to run. It is safe for
// concurrent use.
type Registry struct {
	mu     sync.RWMutex
	skills []Skill
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a Skill. Registration order is preserved and is the tie-breaker
// used by OrderByPrecedence for Skills of the same Kind (a stable sort), so the
// order in which Skills are registered is deterministic and meaningful.
func (r *Registry) Register(skills ...Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills = append(r.skills, skills...)
}

// Resolve returns every registered Skill whose Match reports true for the post,
// in registration order. Matching is additive: a post may resolve to multiple
// Skills across multiple stages, which is exactly the "runs every matching Skill"
// semantic from the architecture (categories are additive, not exclusive).
func (r *Registry) Resolve(post Post) []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Skill
	for _, s := range r.skills {
		if s.Match(post) {
			out = append(out, s)
		}
	}
	return out
}

// Len reports the number of registered Skills.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.skills)
}
