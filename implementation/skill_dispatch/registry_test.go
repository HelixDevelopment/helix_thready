package skilldispatch

import (
	"reflect"
	"testing"
)

func skillNames(skills []Skill) []string {
	out := make([]string, len(skills))
	for i, s := range skills {
		out[i] = s.Name()
	}
	return out
}

// #Video resolves to the download skill; #Research to the research skill; a post
// tagged with both resolves to BOTH (categories are additive, not exclusive).
func TestRegistry_Resolve_ByHashtag(t *testing.T) {
	dl := &fakeSkill{name: "video.download", kind: KindDownload, tags: []string{"Video", "ToDownload"}}
	rs := &fakeSkill{name: "tech.research", kind: KindResearch, tags: []string{"Research", "Technology"}}

	reg := NewRegistry()
	reg.Register(dl, rs)

	cases := []struct {
		name string
		post Post
		want []string
	}{
		{"video only", Post{ID: "p1", Hashtags: []string{"Video"}}, []string{"video.download"}},
		{"research only", Post{ID: "p2", Hashtags: []string{"Research"}}, []string{"tech.research"}},
		{"both", Post{ID: "p3", Hashtags: []string{"Research", "Video"}}, []string{"video.download", "tech.research"}},
		{"none", Post{ID: "p4", Hashtags: []string{"Music"}}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := skillNames(reg.Resolve(tc.post))
			if tc.want == nil {
				if len(got) != 0 {
					t.Fatalf("want no skills, got %v", got)
				}
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("resolve = %v, want %v", got, tc.want)
			}
		})
	}
}

// Hashtag matching is case-insensitive and tolerant of a leading '#'.
func TestRegistry_Resolve_HashtagNormalization(t *testing.T) {
	rs := &fakeSkill{name: "tech.research", kind: KindResearch, tags: []string{"Research"}}
	reg := NewRegistry()
	reg.Register(rs)

	post := Post{ID: "p", Hashtags: []string{"#research"}}
	if got := skillNames(reg.Resolve(post)); !reflect.DeepEqual(got, []string{"tech.research"}) {
		t.Fatalf("normalized resolve = %v, want [tech.research]", got)
	}
}

func TestRegistry_Len(t *testing.T) {
	reg := NewRegistry()
	if reg.Len() != 0 {
		t.Fatalf("empty registry len = %d, want 0", reg.Len())
	}
	reg.Register(&fakeSkill{name: "a", kind: KindReply})
	reg.Register(&fakeSkill{name: "b", kind: KindReply})
	if reg.Len() != 2 {
		t.Fatalf("registry len = %d, want 2", reg.Len())
	}
}
