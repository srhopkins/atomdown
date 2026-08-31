package atomdown

import (
	"bytes"
	"sort"
)

// TokenKind identifies one lossless source segment.
type TokenKind string

const (
	TokenMarkdown   TokenKind = "markdown"
	TokenDirective  TokenKind = "atomdown-directive"
	TokenWhitespace TokenKind = "whitespace"
)

// DirectiveToken is the public XML-shaped view of an Atomdown directive.
type DirectiveToken struct {
	Element    string      `json:"element"`
	Operation  string      `json:"operation"`
	ID         string      `json:"id,omitempty"`
	Slug       string      `json:"slug,omitempty"`
	Version    string      `json:"version,omitempty"`
	Attributes []Attribute `json:"attributes,omitempty"`
}

// Token is one lossless Markdown, whitespace, or Atomdown source segment.
// Concatenating Raw for every token reconstructs the original document.
type Token struct {
	Kind      TokenKind       `json:"kind"`
	Range     Range           `json:"range"`
	Raw       string          `json:"raw"`
	NodeType  string          `json:"nodeType,omitempty"`
	Directive *DirectiveToken `json:"directive,omitempty"`
}

// TokenStream contains a lossless ordered source stream and lexical diagnostics.
type TokenStream struct {
	Tokens      []Token      `json:"tokens"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

type tokenSpan struct {
	rangeValue byteRange
	kind       TokenKind
	nodeType   string
	directive  *DirectiveToken
}

// Tokenize returns a lossless ordered stream containing Markdown blocks,
// Atomdown directives, and interstitial whitespace.
func Tokenize(source []byte) TokenStream {
	lineStarts := sourceLineStarts(source)
	directives, diagnostics := scanDirectives(source, lineStarts)
	blocks := scanMarkdownBlocks(source, directives)
	spans := make([]tokenSpan, 0, len(directives)+len(blocks))

	for _, item := range directives {
		token := publicDirective(item)
		spans = append(spans, tokenSpan{
			rangeValue: item.rawRange,
			kind:       TokenDirective,
			directive:  &token,
		})
	}
	for _, block := range blocks {
		if block.isMarker {
			continue
		}
		spans = append(spans, tokenSpan{
			rangeValue: byteRange{start: block.start, end: block.end},
			kind:       TokenMarkdown,
			nodeType:   block.nodeType,
		})
	}

	sort.SliceStable(spans, func(left, right int) bool {
		if spans[left].rangeValue.start == spans[right].rangeValue.start {
			return spans[left].rangeValue.end < spans[right].rangeValue.end
		}
		return spans[left].rangeValue.start < spans[right].rangeValue.start
	})

	stream := TokenStream{Diagnostics: diagnostics}
	cursor := 0
	for _, span := range spans {
		if span.rangeValue.start < cursor {
			continue
		}
		if span.rangeValue.start > cursor {
			stream.Tokens = append(stream.Tokens, rawToken(source, byteRange{cursor, span.rangeValue.start}, lineStarts))
		}
		stream.Tokens = append(stream.Tokens, Token{
			Kind: span.kind, Range: makeRange(span.rangeValue, lineStarts),
			Raw:      string(source[span.rangeValue.start:span.rangeValue.end]),
			NodeType: span.nodeType, Directive: span.directive,
		})
		cursor = span.rangeValue.end
	}
	if cursor < len(source) {
		stream.Tokens = append(stream.Tokens, rawToken(source, byteRange{cursor, len(source)}, lineStarts))
	}
	return stream
}

func rawToken(source []byte, value byteRange, lineStarts []int) Token {
	kind := TokenMarkdown
	if len(bytes.TrimSpace(source[value.start:value.end])) == 0 {
		kind = TokenWhitespace
	}
	return Token{Kind: kind, Range: makeRange(value, lineStarts), Raw: string(source[value.start:value.end])}
}

func publicDirective(item directive) DirectiveToken {
	operation := "open"
	if item.kind == directiveAtom || item.kind == directiveDocument {
		operation = "empty"
	}
	if item.kind == directiveGroupEnd {
		operation = "close"
	}
	return DirectiveToken{
		Element: item.name, Operation: operation, ID: item.id,
		Slug: item.slug, Version: item.version, Attributes: item.attributes,
	}
}

// Strip returns the pure Markdown projection. It removes only recognized,
// valid Atomdown directive lines and preserves all non-directive content.
//
// A removed directive takes its whole line with it, including any leading
// or trailing whitespace on that line, so trailing spaces after "-->" never
// leave a whitespace-only line behind. Strip also removes the blank-line
// scaffolding a directive leaves when it sat between two blank lines used
// only to separate it from its neighbors: a run of blank lines introduced
// solely by removed directives collapses to at most one, and to none at the
// very start or end of the document. A blank line the author actually wrote
// is never touched, even inside the same run -- a deliberate double blank
// line (which can change a list from tight to loose) survives untouched.
func Strip(source []byte) []byte {
	lineStarts := sourceLineStarts(source)
	directives, _ := scanDirectives(source, lineStarts)

	lines := splitSourceLines(source, lineStarts)
	removedLine := make([]bool, len(lines))
	for _, item := range directives {
		removedLine[lineIndexForOffset(lineStarts, item.rawRange.start)] = true
	}

	blank := make([]bool, len(lines))
	contentLine := make([]bool, len(lines))
	for index, line := range lines {
		blank[index] = len(bytes.TrimSpace(line)) == 0
		contentLine[index] = !blank[index] && !removedLine[index]
	}

	// hasContentBefore[i] / hasContentAfter[i] answer "does a real content
	// line exist strictly before / at-or-after line i", so a blank run
	// touching either edge of the document (nothing but removed directives
	// on that side) can be told apart from one sitting between two blocks.
	hasContentBefore := make([]bool, len(lines)+1)
	for index := 0; index < len(lines); index++ {
		hasContentBefore[index+1] = hasContentBefore[index] || contentLine[index]
	}
	hasContentAfter := make([]bool, len(lines)+1)
	for index := len(lines) - 1; index >= 0; index-- {
		hasContentAfter[index] = hasContentAfter[index+1] || contentLine[index]
	}

	// A kept blank line is scaffolding ("artifact") only when it sat right
	// next to a removed directive line. A blank line the author wrote next
	// to other ordinary content is never marked, so it is never touched.
	artifact := make([]bool, len(lines))
	for index := range lines {
		if !blank[index] || removedLine[index] {
			continue
		}
		before := index > 0 && removedLine[index-1]
		after := index+1 < len(lines) && removedLine[index+1]
		artifact[index] = before || after
	}

	keep := make([]bool, len(lines))
	for index := range lines {
		keep[index] = !removedLine[index]
	}

	for index := 0; index < len(lines); {
		if removedLine[index] || !blank[index] {
			index++
			continue
		}
		start := index
		genuine := false
		for index < len(lines) && (removedLine[index] || blank[index]) {
			if !removedLine[index] && !artifact[index] {
				genuine = true
			}
			index++
		}
		end := index

		switch {
		case genuine:
			// At least one blank line in this run is the author's own; keep
			// every genuine blank line exactly as written and drop only the
			// redundant scaffolding next to it.
			for i := start; i < end; i++ {
				if !removedLine[i] && artifact[i] {
					keep[i] = false
				}
			}
		case !hasContentBefore[start] || !hasContentAfter[end]:
			// Every blank line here is scaffolding, and the run touches the
			// start or end of the document: no separator is needed at all.
			for i := start; i < end; i++ {
				keep[i] = false
			}
		default:
			// Every blank line here is scaffolding, but it still separates
			// two real blocks: keep exactly one blank line as that
			// separator and drop the rest.
			kept := false
			for i := start; i < end; i++ {
				if removedLine[i] {
					continue
				}
				if kept {
					keep[i] = false
				} else {
					kept = true
				}
			}
		}
	}

	var output bytes.Buffer
	for index, line := range lines {
		if keep[index] {
			output.Write(line)
		}
	}
	return output.Bytes()
}

// splitSourceLines splits source into one slice per line, each including its
// own trailing line ending (absent only for a final line with none), so
// concatenating every line reconstructs source exactly.
func splitSourceLines(source []byte, lineStarts []int) [][]byte {
	lines := make([][]byte, len(lineStarts))
	for index, start := range lineStarts {
		end := len(source)
		if index+1 < len(lineStarts) {
			end = lineStarts[index+1]
		}
		lines[index] = source[start:end]
	}
	return lines
}

// lineIndexForOffset returns the 0-indexed line containing offset.
func lineIndexForOffset(lineStarts []int, offset int) int {
	index := sort.Search(len(lineStarts), func(candidate int) bool { return lineStarts[candidate] > offset }) - 1
	if index < 0 {
		index = 0
	}
	return index
}
