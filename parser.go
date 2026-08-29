package atomdown

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

var atomIDPattern = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{8}$`)

type directiveKind int

const (
	directiveDocument directiveKind = iota + 1
	directiveAtom
	directiveGroupStart
	directiveGroupEnd
)

type directive struct {
	kind       directiveKind
	name       string
	id         string
	slug       string
	version    string
	attributes []Attribute
	rawRange   byteRange
}

type byteRange struct {
	start int
	end   int
}

type markdownBlock struct {
	start    int
	end      int
	nodeType string
	isMarker bool
}

// Parse builds an Atomdown document model from Markdown source.
func Parse(source []byte) Document {
	lineStarts := sourceLineStarts(source)
	directives, diagnostics := scanDirectives(source, lineStarts)
	blocks := scanMarkdownBlocks(source, directives)

	document := Document{Atoms: make([]Atom, 0), Diagnostics: diagnostics}
	for _, item := range directives {
		if item.kind != directiveDocument {
			continue
		}
		if document.Declared {
			document.Diagnostics = append(document.Diagnostics, newDiagnostic(
				"duplicate-document-marker", SeverityError,
				"Only one atomdown document marker is allowed.", item.rawRange.start,
				"Remove the duplicate marker.", lineStarts,
			))
			continue
		}
		document.Declared = true
		document.Version = item.version
		document.Attributes = item.attributes
	}

	assigned := make(map[int]int)
	atomDirectiveIndexes := make([]int, 0)
	for index, item := range directives {
		if item.kind == directiveAtom {
			atomDirectiveIndexes = append(atomDirectiveIndexes, index)
		}
	}

	for _, directiveIndex := range atomDirectiveIndexes {
		item := directives[directiveIndex]
		blockIndex := nextContentBlock(blocks, item.rawRange.end)
		if blockIndex < 0 || blockedByDirective(directives, directiveIndex, blocks[blockIndex].start) {
			document.Diagnostics = append(document.Diagnostics, newDiagnostic(
				"orphan-atom", SeverityError,
				"Atom marker is not followed by a Markdown block.", item.rawRange.start,
				"Add content after the marker or remove the marker.", lineStarts,
			))
			continue
		}
		if previous, exists := assigned[blockIndex]; exists {
			document.Diagnostics = append(document.Diagnostics, newDiagnostic(
				"duplicate-assignment", SeverityError,
				fmt.Sprintf("Markdown block is already assigned to atom %q.", document.Atoms[previous].ID),
				item.rawRange.start, "Remove one of the atom markers.", lineStarts,
			))
			continue
		}

		block := blocks[blockIndex]
		atom := Atom{
			ID: item.id, Slug: item.slug, Attributes: item.attributes,
			Marker:   rangePointer(makeRange(item.rawRange, lineStarts)),
			Content:  makeRange(byteRange{block.start, block.end}, lineStarts),
			NodeType: block.nodeType,
			Text:     string(bytes.TrimRight(source[block.start:block.end], "\r\n")),
		}
		assigned[blockIndex] = len(document.Atoms)
		document.Atoms = append(document.Atoms, atom)
	}

	for blockIndex, block := range blocks {
		if block.isMarker {
			continue
		}
		if _, exists := assigned[blockIndex]; exists {
			continue
		}
		document.Atoms = append(document.Atoms, Atom{
			Implicit: true,
			Content:  makeRange(byteRange{block.start, block.end}, lineStarts),
			NodeType: block.nodeType,
			Text:     string(bytes.TrimRight(source[block.start:block.end], "\r\n")),
		})
	}

	sort.SliceStable(document.Atoms, func(left, right int) bool {
		return document.Atoms[left].Content.Start.Offset < document.Atoms[right].Content.Start.Offset
	})
	applyGroups(&document, directives, lineStarts)
	lintDocument(&document, lineStarts)
	return document
}

func scanDirectives(source []byte, lineStarts []int) ([]directive, []Diagnostic) {
	codeRanges := markdownCodeRanges(source)
	var directives []directive
	var diagnostics []Diagnostic
	for cursor := 0; cursor < len(source); {
		relativeStart := bytes.Index(source[cursor:], []byte("<!--"))
		if relativeStart < 0 {
			break
		}
		start := cursor + relativeStart
		if rangeContainsOffset(codeRanges, start) {
			cursor = start + 4
			continue
		}
		relativeEnd := bytes.Index(source[start+4:], []byte("-->"))
		if relativeEnd < 0 {
			diagnostics = append(diagnostics, newDiagnostic(
				"unterminated-comment", SeverityError,
				"HTML comment is not terminated.", start,
				"Add a closing --> token.", lineStarts,
			))
			break
		}
		end := start + 4 + relativeEnd + 3
		body := bytes.TrimSpace(source[start+4 : end-3])
		if bytes.Contains(body, []byte("--")) {
			diagnostics = append(diagnostics, newDiagnostic(
				"invalid-xml-comment", SeverityError,
				"XML comments cannot contain --.", start,
				"Remove or encode the double hyphen.", lineStarts,
			))
		}
		if parsed, recognized, err := parseDirective(body, byteRange{start, end}); recognized {
			if err != nil {
				diagnostics = append(diagnostics, newDiagnostic(
					"invalid-directive", SeverityError, err.Error(), start,
					"Correct the XML-shaped Atomdown directive.", lineStarts,
				))
			} else if !directiveOccupiesLine(source, start, end) {
				diagnostics = append(diagnostics, newDiagnostic(
					"inline-directive", SeverityError,
					"Atomdown directive must occupy its own source line.", start,
					"Move the directive to a separate line outside the Markdown block.", lineStarts,
				))
			} else {
				directives = append(directives, parsed)
			}
		}
		cursor = end
	}
	return directives, diagnostics
}

func markdownCodeRanges(source []byte) []byteRange {
	markdown := goldmark.New(goldmark.WithExtensions(extension.GFM))
	root := markdown.Parser().Parse(text.NewReader(source))
	var ranges []byteRange
	_ = ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || (node.Kind() != ast.KindFencedCodeBlock && node.Kind() != ast.KindCodeBlock) {
			return ast.WalkContinue, nil
		}
		lines := node.Lines()
		for index := 0; index < lines.Len(); index++ {
			line := lines.At(index)
			ranges = append(ranges, byteRange{start: line.Start, end: line.Stop})
		}
		return ast.WalkSkipChildren, nil
	})
	sort.Slice(ranges, func(left, right int) bool {
		return ranges[left].start < ranges[right].start
	})
	return ranges
}

func rangeContainsOffset(ranges []byteRange, offset int) bool {
	index := sort.Search(len(ranges), func(index int) bool {
		return ranges[index].end > offset
	})
	return index < len(ranges) && ranges[index].start <= offset
}

func directiveOccupiesLine(source []byte, start, end int) bool {
	lineStart := bytes.LastIndexByte(source[:start], '\n') + 1
	lineEnd := len(source)
	if relative := bytes.IndexByte(source[end:], '\n'); relative >= 0 {
		lineEnd = end + relative
	}
	return len(bytes.TrimSpace(source[lineStart:start])) == 0 && len(bytes.TrimSpace(source[end:lineEnd])) == 0
}

func parseDirective(body []byte, sourceRange byteRange) (directive, bool, error) {
	trimmed := strings.TrimSpace(string(body))
	if !strings.HasPrefix(trimmed, "<") {
		return directive{}, false, nil
	}

	decoder := xml.NewDecoder(strings.NewReader(trimmed))
	token, err := decoder.RawToken()
	if err != nil {
		return directive{}, looksLikeAtomdown(trimmed), fmt.Errorf("invalid XML directive: %w", err)
	}

	result := directive{rawRange: sourceRange}
	switch typed := token.(type) {
	case xml.StartElement:
		result.name = typed.Name.Local
		switch typed.Name.Local {
		case "atomdown":
			result.kind = directiveDocument
		case "atom":
			result.kind = directiveAtom
		case "atom-group":
			result.kind = directiveGroupStart
		default:
			return directive{}, false, nil
		}
		seenAttributes := make(map[string]struct{}, len(typed.Attr))
		for _, attribute := range typed.Attr {
			key := xmlAttributeName(attribute.Name)
			if _, exists := seenAttributes[key]; exists {
				return directive{}, true, fmt.Errorf("duplicate XML attribute %q", key)
			}
			seenAttributes[key] = struct{}{}
			switch {
			case key == "id" && (result.kind == directiveAtom || result.kind == directiveGroupStart):
				result.id = attribute.Value
			case key == "slug" && (result.kind == directiveAtom || result.kind == directiveGroupStart):
				result.slug = attribute.Value
			case key == "version" && result.kind == directiveDocument:
				result.version = attribute.Value
			default:
				result.attributes = append(result.attributes, Attribute{Name: key, Value: attribute.Value})
			}
		}
		if decoder.InputOffset() != int64(len(trimmed)) {
			return directive{}, true, fmt.Errorf("Atomdown comment must contain exactly one XML directive")
		}
		if result.kind == directiveGroupStart && strings.HasSuffix(strings.TrimSpace(trimmed), "/>") {
			return directive{}, true, fmt.Errorf("atom-group opening marker must not be self-closing")
		}
		if result.kind != directiveGroupStart && !strings.HasSuffix(strings.TrimSpace(trimmed), "/>") {
			return directive{}, true, fmt.Errorf("%s marker must be self-closing", result.name)
		}
	case xml.EndElement:
		if typed.Name.Local != "atom-group" {
			return directive{}, false, nil
		}
		result.kind = directiveGroupEnd
		result.name = typed.Name.Local
		if decoder.InputOffset() != int64(len(trimmed)) {
			return directive{}, true, fmt.Errorf("Atomdown comment must contain exactly one XML directive")
		}
	default:
		return directive{}, looksLikeAtomdown(trimmed), fmt.Errorf("Atomdown directive must contain one XML element")
	}

	return result, true, nil
}

func looksLikeAtomdown(value string) bool {
	return strings.HasPrefix(value, "<atom") || strings.HasPrefix(value, "</atom")
}

func xmlAttributeName(name xml.Name) string {
	if name.Space == "" {
		return name.Local
	}
	// RawToken leaves the source prefix in Space (not a resolved URI).
	return name.Space + ":" + name.Local
}

func scanMarkdownBlocks(source []byte, directives []directive) []markdownBlock {
	markdown := goldmark.New(goldmark.WithExtensions(extension.GFM))
	root := markdown.Parser().Parse(text.NewReader(source))
	var blocks []markdownBlock
	for node := root.FirstChild(); node != nil; node = node.NextSibling() {
		start, ok := markdownNodeStart(node)
		if !ok {
			continue
		}
		start = sourceLineStart(source, start)
		blocks = append(blocks, markdownBlock{start: start, nodeType: node.Kind().String()})
	}
	for index := range blocks {
		end := len(source)
		if index+1 < len(blocks) {
			end = blocks[index+1].start
		}
		blocks[index].end = trimBlockEnd(source, blocks[index].start, end)
		for _, item := range directives {
			if blocks[index].start >= item.rawRange.start && blocks[index].start < item.rawRange.end {
				blocks[index].isMarker = true
				break
			}
		}
	}
	return blocks
}

func markdownNodeStart(node ast.Node) (int, bool) {
	lines := node.Lines()
	if lines != nil && lines.Len() > 0 {
		return lines.At(0).Start, true
	}
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if start, ok := markdownNodeStart(child); ok {
			return start, true
		}
	}
	return 0, false
}

func sourceLineStart(source []byte, offset int) int {
	return bytes.LastIndexByte(source[:offset], '\n') + 1
}

func trimBlockEnd(source []byte, start, end int) int {
	for end > start && (source[end-1] == '\n' || source[end-1] == '\r') {
		end--
	}
	return end
}

func nextContentBlock(blocks []markdownBlock, offset int) int {
	for index, block := range blocks {
		if block.start >= offset && !block.isMarker {
			return index
		}
	}
	return -1
}

func blockedByDirective(directives []directive, current int, blockStart int) bool {
	for index := current + 1; index < len(directives); index++ {
		item := directives[index]
		if item.rawRange.start >= blockStart {
			return false
		}
		if item.kind == directiveAtom || item.kind == directiveGroupEnd || item.kind == directiveDocument {
			return true
		}
	}
	return false
}

func applyGroups(document *Document, directives []directive, lineStarts []int) {
	var open *AtomGroup
	for _, item := range directives {
		switch item.kind {
		case directiveGroupStart:
			if open != nil {
				document.Diagnostics = append(document.Diagnostics, newDiagnostic(
					"nested-group", SeverityError,
					"Atomdown 1 does not allow nested atom groups.", item.rawRange.start,
					"Close the current group before opening another.", lineStarts,
				))
				continue
			}
			group := AtomGroup{
				ID: item.id, Slug: item.slug, Attributes: item.attributes,
				Marker: makeRange(item.rawRange, lineStarts),
			}
			document.Groups = append(document.Groups, group)
			open = &document.Groups[len(document.Groups)-1]
		case directiveGroupEnd:
			if open == nil {
				document.Diagnostics = append(document.Diagnostics, newDiagnostic(
					"unmatched-group-close", SeverityError,
					"Atom group closing marker has no matching opening marker.", item.rawRange.start,
					"Remove it or add a preceding atom-group marker.", lineStarts,
				))
				continue
			}
			closing := makeRange(item.rawRange, lineStarts)
			open.EndMarker = &closing
			for atomIndex := range document.Atoms {
				atom := &document.Atoms[atomIndex]
				if atom.Implicit || atom.Marker == nil {
					continue
				}
				if atom.Marker.Start.Offset > open.Marker.End.Offset && atom.Marker.Start.Offset < closing.Start.Offset {
					atom.GroupID = open.ID
					open.AtomIDs = append(open.AtomIDs, atom.ID)
				}
			}
			open = nil
		}
	}
	if open != nil {
		document.Diagnostics = append(document.Diagnostics, newDiagnostic(
			"unclosed-group", SeverityError,
			"Atom group is not closed.", open.Marker.Start.Offset,
			"Add <!-- </atom-group> --> after the final atom.", lineStarts,
		))
	}
}

func sourceLineStarts(source []byte) []int {
	starts := []int{0}
	for index, value := range source {
		if value == '\n' && index+1 < len(source) {
			starts = append(starts, index+1)
		}
	}
	return starts
}

func makePosition(offset int, lineStarts []int) Position {
	lineIndex := sort.Search(len(lineStarts), func(index int) bool { return lineStarts[index] > offset }) - 1
	if lineIndex < 0 {
		lineIndex = 0
	}
	return Position{Offset: offset, Line: lineIndex + 1, Column: offset - lineStarts[lineIndex] + 1}
}

func makeRange(value byteRange, lineStarts []int) Range {
	return Range{Start: makePosition(value.start, lineStarts), End: makePosition(value.end, lineStarts)}
}

func rangePointer(value Range) *Range { return &value }

func newDiagnostic(code string, severity Severity, message string, offset int, fix string, lineStarts []int) Diagnostic {
	return Diagnostic{Code: code, Severity: severity, Message: message, Position: makePosition(offset, lineStarts), Fix: fix}
}
