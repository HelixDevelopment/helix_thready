package threadyconfig

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ParseDotEnv parses a .env-format stream into a map. It supports:
//   - blank lines and full-line comments (a line whose first non-space rune is '#')
//   - an optional "export " prefix
//   - KEY=VALUE with surrounding whitespace trimmed from KEY
//   - '=' inside the value (only the first '=' splits)
//   - single- and double-quoted values (quotes stripped; content preserved
//     verbatim, including '=', '#', and spaces). Double-quoted values honour
//     the \n, \t, \r, \", \\ escapes.
//   - trailing inline comments on UNQUOTED values (a " #" and everything after)
//
// A line that has no '=' (and is not blank/comment) is a hard error naming the
// 1-based line number, as is an empty or malformed key.
func ParseDotEnv(r io.Reader) (map[string]string, error) {
	out := make(map[string]string)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	lineNo := 0
	for sc.Scan() {
		lineNo++
		raw := sc.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if rest, ok := strings.CutPrefix(trimmed, "export "); ok {
			trimmed = strings.TrimSpace(rest)
		}

		eq := strings.IndexByte(trimmed, '=')
		if eq < 0 {
			return nil, fmt.Errorf("line %d: missing '=' in %q", lineNo, raw)
		}
		key := strings.TrimSpace(trimmed[:eq])
		if key == "" {
			return nil, fmt.Errorf("line %d: empty key in %q", lineNo, raw)
		}
		if !validKey(key) {
			return nil, fmt.Errorf("line %d: invalid key %q", lineNo, key)
		}
		out[key] = parseDotEnvValue(trimmed[eq+1:])
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read .env: %w", err)
	}
	return out, nil
}

func parseDotEnvValue(v string) string {
	v = strings.TrimLeft(v, " \t")
	if len(v) >= 1 && (v[0] == '"' || v[0] == '\'') {
		quote := v[0]
		if end := strings.IndexByte(v[1:], quote); end >= 0 {
			inner := v[1 : 1+end]
			if quote == '"' {
				inner = unescapeDouble(inner)
			}
			return inner
		}
		// No closing quote: fall through and treat the raw text (minus the
		// opening quote) as an unquoted value.
		v = v[1:]
	}
	// Unquoted: strip a trailing inline comment introduced by " #".
	if i := strings.Index(v, " #"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimRight(v, " \t")
}

func unescapeDouble(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			default:
				b.WriteByte(s[i+1])
			}
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func validKey(k string) bool {
	for i := 0; i < len(k); i++ {
		c := k[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c == '_':
			// always allowed
		case c >= '0' && c <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
