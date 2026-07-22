package skilldispatch

import (
	"reflect"
	"testing"
)

func indexOf(names []string, want string) int {
	for i, n := range names {
		if n == want {
			return i
		}
	}
	return -1
}

// A shuffled set of one Skill per stage must come out in strict stage order.
func TestOrderByPrecedence_StageOrder(t *testing.T) {
	in := []Skill{
		&fakeSkill{name: "reply", kind: KindReply},
		&fakeSkill{name: "research", kind: KindResearch},
		&fakeSkill{name: "download", kind: KindDownload},
		&fakeSkill{name: "analyze", kind: KindAnalyze},
		&fakeSkill{name: "convert", kind: KindConvert},
	}
	got := skillNames(OrderByPrecedence(in))
	want := []string{"download", "convert", "analyze", "research", "reply"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

// The load-bearing precedence assertion: download runs strictly before research.
func TestOrderByPrecedence_DownloadBeforeResearch(t *testing.T) {
	in := []Skill{
		&fakeSkill{name: "tech.research", kind: KindResearch},
		&fakeSkill{name: "video.download", kind: KindDownload},
	}
	got := skillNames(OrderByPrecedence(in))
	di, ri := indexOf(got, "video.download"), indexOf(got, "tech.research")
	if di < 0 || ri < 0 {
		t.Fatalf("missing skills in %v", got)
	}
	if di >= ri {
		t.Fatalf("download index %d not before research index %d (order %v)", di, ri, got)
	}
}

// Skills of the same Kind keep their input order (stable sort).
func TestOrderByPrecedence_StableWithinKind(t *testing.T) {
	in := []Skill{
		&fakeSkill{name: "dl.a", kind: KindDownload},
		&fakeSkill{name: "dl.b", kind: KindDownload},
		&fakeSkill{name: "dl.c", kind: KindDownload},
	}
	got := skillNames(OrderByPrecedence(in))
	want := []string{"dl.a", "dl.b", "dl.c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("within-kind order = %v, want %v (must be stable)", got, want)
	}
}

// OrderByPrecedence must not mutate its input slice.
func TestOrderByPrecedence_DoesNotMutateInput(t *testing.T) {
	in := []Skill{
		&fakeSkill{name: "reply", kind: KindReply},
		&fakeSkill{name: "download", kind: KindDownload},
	}
	before := skillNames(in)
	_ = OrderByPrecedence(in)
	after := skillNames(in)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("input mutated: before %v, after %v", before, after)
	}
}
