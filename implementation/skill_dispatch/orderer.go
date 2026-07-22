package skilldispatch

import "sort"

// OrderByPrecedence returns the Skills sorted into the deterministic stage order
//
//	download > convert > analyze > research > reply
//
// (processing-pipeline.md §5). The sort is STABLE: Skills of the same Kind keep
// their input (registration/resolve) order, so within a stage the order is the
// order they were registered — independent Skills in a stage may run concurrently
// in a fuller implementation, but their recorded order here is deterministic.
//
// The input slice is not mutated; a new slice is returned.
func OrderByPrecedence(skills []Skill) []Skill {
	out := make([]Skill, len(skills))
	copy(out, skills)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Kind() < out[j].Kind()
	})
	return out
}
