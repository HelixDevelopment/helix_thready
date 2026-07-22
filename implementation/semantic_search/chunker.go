package semsearch

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// Chunker splits source and generated materials into retrievable [Chunk]s.
// It dispatches by kind: a Go-AST symbol chunker for .go source (chunk =
// func/method/type/const/var), and a Markdown/paragraph chunker for docs.
type Chunker struct{}

// NewChunker returns a ready-to-use Chunker.
func NewChunker() *Chunker { return &Chunker{} }

// Chunk dispatches on the file extension: ".go" is parsed with go/ast, and
// everything else is treated as a Markdown/text document and split into
// paragraph (blank-line-separated) blocks.
func (c *Chunker) Chunk(filePath, content string) ([]Chunk, error) {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".go":
		return c.ChunkGo(filePath, content)
	default:
		return c.ChunkMarkdown(filePath, content), nil
	}
}

// ChunkGo parses Go source with the standard go/parser + go/ast and emits one
// chunk per top-level symbol declaration (func, method, type, const, var).
// Import declarations are skipped. The chunk's line span comes from the real
// token positions, so it is exact, not heuristic.
func (c *Chunker) ChunkGo(filePath, content string) ([]Chunk, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, content, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("semsearch: parse %q: %w", filePath, err)
	}
	lines := splitLines(content)
	var chunks []Chunk
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			kind := KindFunc
			symbol := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				kind = KindMethod
				symbol = recvTypeName(d.Recv.List[0].Type) + "." + d.Name.Name
			}
			chunks = append(chunks, mkGoChunk(fset, filePath, symbol, kind, d.Pos(), d.End(), lines))
		case *ast.GenDecl:
			var valKind Kind
			switch d.Tok {
			case token.CONST:
				valKind = KindConst
			case token.VAR:
				valKind = KindVar
			case token.TYPE:
				// handled per-spec with KindType below
			default:
				continue // import (and any other) declarations are not chunked
			}
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					chunks = append(chunks, mkGoChunk(fset, filePath, s.Name.Name, KindType, s.Pos(), s.End(), lines))
				case *ast.ValueSpec:
					names := make([]string, 0, len(s.Names))
					for _, n := range s.Names {
						names = append(names, n.Name)
					}
					chunks = append(chunks, mkGoChunk(fset, filePath, strings.Join(names, ","), valKind, s.Pos(), s.End(), lines))
				}
			}
		}
	}
	return chunks, nil
}

func mkGoChunk(fset *token.FileSet, filePath, symbol string, kind Kind, pos, end token.Pos, lines []string) Chunk {
	start := fset.Position(pos).Line
	stop := fset.Position(end).Line
	return Chunk{
		ID:        ChunkID(filePath, symbol, start),
		FilePath:  filePath,
		Symbol:    symbol,
		Kind:      kind,
		StartLine: start,
		EndLine:   stop,
		Content:   joinLines(lines, start, stop),
	}
}

// recvTypeName unwraps a method receiver expression (T, *T, generic T[P]) to
// the bare type name.
func recvTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return recvTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return recvTypeName(t.X)
	case *ast.IndexListExpr:
		return recvTypeName(t.X)
	default:
		return "?"
	}
}

// ChunkMarkdown splits a document into paragraph (blank-line-separated) blocks.
// Each block becomes one chunk. ATX headings ("# ...") set the Symbol for the
// heading block and are carried forward to following paragraphs, so a body
// paragraph is attributed to its nearest preceding heading.
func (c *Chunker) ChunkMarkdown(filePath, content string) []Chunk {
	lines := splitLines(content)
	n := len(lines)
	var chunks []Chunk
	heading := ""
	i := 0
	for i < n {
		for i < n && strings.TrimSpace(lines[i]) == "" {
			i++
		}
		if i >= n {
			break
		}
		start := i + 1 // 1-indexed
		var block []string
		for i < n && strings.TrimSpace(lines[i]) != "" {
			block = append(block, lines[i])
			i++
		}
		end := start + len(block) - 1
		symbol := heading
		if h := headingText(block[0]); h != "" {
			symbol = h
			heading = h
		}
		chunks = append(chunks, Chunk{
			ID:        ChunkID(filePath, symbol, start),
			FilePath:  filePath,
			Symbol:    symbol,
			Kind:      KindMarkdown,
			StartLine: start,
			EndLine:   end,
			Content:   strings.Join(block, "\n"),
		})
	}
	return chunks
}

// headingText returns the trimmed heading text for an ATX heading line, or ""
// if the line is not a heading.
func headingText(line string) string {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "#") {
		return ""
	}
	return strings.TrimSpace(strings.TrimLeft(t, "#"))
}

func splitLines(content string) []string {
	return strings.Split(content, "\n")
}

// joinLines returns lines[start-1 : stop] (1-indexed inclusive) joined by "\n",
// clamped to valid bounds.
func joinLines(lines []string, start, stop int) string {
	if start < 1 {
		start = 1
	}
	if stop > len(lines) {
		stop = len(lines)
	}
	if start > stop {
		return ""
	}
	return strings.Join(lines[start-1:stop], "\n")
}
