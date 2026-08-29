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
func Strip(source []byte) []byte {
	lineStarts := sourceLineStarts(source)
	directives, _ := scanDirectives(source, lineStarts)
	removed := make([]byteRange, 0, len(directives))
	for _, item := range directives {
		end := item.rawRange.end
		if end < len(source) && source[end] == '\r' {
			end++
		}
		if end < len(source) && source[end] == '\n' {
			end++
		}
		removed = append(removed, byteRange{start: item.rawRange.start, end: end})
	}

	var output bytes.Buffer
	cursor := 0
	for _, item := range removed {
		output.Write(source[cursor:item.start])
		cursor = item.end
	}
	output.Write(source[cursor:])
	return output.Bytes()
}
