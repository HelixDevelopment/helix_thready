package threadyconfig

import (
	"strings"
	"testing"
)

func TestParseDotEnv_EdgeCases(t *testing.T) {
	const input = `
# a full-line comment
   # an indented comment

export EXPORTED=exported-value
PLAIN=plain
SPACED = trimmed
QUOTED="hello world"
SINGLE='it stays raw'
EQ_IN_VALUE=a=b=c
EQ_IN_QUOTES="k=v&x=y"
HASH_IN_QUOTES="value#notacomment"
INLINE=live # trailing comment stripped
COLOR=#B6E376
DSN=postgres://u:p@db:5432/thready?sslmode=require
ESCAPES="line1\nline2\ttab"
EMPTY=
PADDED_QUOTE=   "padded"
`
	got, err := ParseDotEnv(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseDotEnv returned error: %v", err)
	}

	want := map[string]string{
		"EXPORTED":       "exported-value",
		"PLAIN":          "plain",
		"SPACED":         "trimmed",
		"QUOTED":         "hello world",
		"SINGLE":         "it stays raw",
		"EQ_IN_VALUE":    "a=b=c",
		"EQ_IN_QUOTES":   "k=v&x=y",
		"HASH_IN_QUOTES": "value#notacomment",
		"INLINE":         "live",
		"COLOR":          "#B6E376",
		"DSN":            "postgres://u:p@db:5432/thready?sslmode=require",
		"ESCAPES":        "line1\nline2\ttab",
		"EMPTY":          "",
		"PADDED_QUOTE":   "padded",
	}
	for k, w := range want {
		g, ok := got[k]
		if !ok {
			t.Errorf("key %q missing from result", k)
			continue
		}
		if g != w {
			t.Errorf("key %q = %q, want %q", k, g, w)
		}
	}
	// Comments and blank lines must not appear as keys.
	if len(got) != len(want) {
		t.Errorf("got %d keys, want %d: %#v", len(got), len(want), got)
	}
}

func TestParseDotEnv_MissingEqualsIsError(t *testing.T) {
	const input = "GOOD=1\nTHIS_LINE_HAS_NO_EQUALS\nOTHER=2\n"
	_, err := ParseDotEnv(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for line without '=', got nil")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error should name line 2, got: %v", err)
	}
	if !strings.Contains(err.Error(), "'='") {
		t.Errorf("error should mention missing '=', got: %v", err)
	}
}

func TestParseDotEnv_InvalidKeyIsError(t *testing.T) {
	_, err := ParseDotEnv(strings.NewReader("1BAD=x\n"))
	if err == nil {
		t.Fatal("expected error for key starting with a digit, got nil")
	}
	if !strings.Contains(err.Error(), "invalid key") {
		t.Errorf("error should mention invalid key, got: %v", err)
	}
}

func TestParseDotEnv_Empty(t *testing.T) {
	got, err := ParseDotEnv(strings.NewReader("\n\n# only comments\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %#v", got)
	}
}
