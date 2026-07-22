package threadreader

import "unicode"

// ExtractHashtags returns every "#Tag" token in text, unicode-aware, deduped, with
// case preserved and first-seen order kept.
//
// Rules:
//   - A tag starts at '#' only when it is at a word boundary — the '#' must NOT be
//     preceded by a tag character. This rejects "foo#bar" and URL fragments like
//     "example.com#section" while accepting "(#tag)", "hi #tag", and a leading "#tag".
//   - A tag body is one or more unicode letters, unicode digits, or underscores.
//     It stops at the first non-tag character, so adjacent punctuation such as
//     "#Video." or "#tag!" yields "Video" / "tag".
//   - A lone '#' (no body) and "C#" (no trailing body) yield nothing.
//   - Dedup is exact (case-sensitive): "#Tag" and "#tag" are distinct, matching the
//     case-preserved contract; identical repeats are collapsed to the first.
func ExtractHashtags(text string) []string {
	runes := []rune(text)
	seen := make(map[string]struct{})
	var out []string

	for i := 0; i < len(runes); i++ {
		if runes[i] != '#' {
			continue
		}
		// Word-boundary guard: skip a '#' glued to the end of a preceding token.
		if i > 0 && isTagChar(runes[i-1]) {
			continue
		}
		j := i + 1
		for j < len(runes) && isTagChar(runes[j]) {
			j++
		}
		if j > i+1 { // at least one body rune
			tag := string(runes[i+1 : j])
			if _, ok := seen[tag]; !ok {
				seen[tag] = struct{}{}
				out = append(out, tag)
			}
		}
		i = j - 1 // continue scanning after the consumed token
	}
	return out
}

// isTagChar reports whether r may appear in a hashtag body: any unicode letter or
// digit (so Cyrillic, CJK, accented letters all work), or an underscore.
func isTagChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
