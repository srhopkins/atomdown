package atomdown

import (
	"bytes"
	"encoding/xml"
	"errors"
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

// errExtraDirectiveContent reports a recognized Atomdown comment that holds
// something besides its one directive element. A directive may wrap across
// several source lines, so this is the defect that replaces the old
// "one source line" rule: whitespace inside the comment is free, and any
// other content inside it is not. It has its own diagnostic code
// (extra-directive-content) because the repair is specific -- delete the
// stray content -- and different from repairing malformed XML.
var errExtraDirectiveContent = errors.New("Atomdown comment must contain exactly one XML directive and no other content")

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
	digest     string
	version    string
	attributes []Attribute
	rawRange   byteRange
}

type byteRange struct {
	start int
	end   int
}

type markdownBlock struct {
	start     int
	end       int
	nodeType  string
	isMarker  bool
	itemCount int
}

// Parse builds an Atomdown document model from Markdown source.
func Parse(source []byte) Document {
	lineStarts := sourceLineStarts(source)
	directives, diagnostics := scanDirectives(source, lineStarts)
	blocks := scanMarkdownBlocks(source, directives)
	listItemCounts := make(map[int]int, len(blocks))
	for _, block := range blocks {
		if block.nodeType == "List" {
			listItemCounts[block.start] = block.itemCount
		}
	}

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
		document.MarkerSource = string(source[item.rawRange.start:item.rawRange.end])
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
		if blockIndex < 0 {
			document.Diagnostics = append(document.Diagnostics, newDiagnostic(
				"orphan-atom", SeverityError,
				"Atom marker is not followed by a Markdown block.", item.rawRange.start,
				"Add content after the marker or remove the marker.", lineStarts,
			))
			continue
		}
		if blocker, blocked := firstBlockingDirective(directives, directiveIndex, blocks[blockIndex].start); blocked {
			if blocker.kind == directiveAtom {
				// A block does exist here; the next atom marker claims it
				// first. Reporting "not followed by a Markdown block" would
				// be false, so this gets its own, accurate diagnostic.
				document.Diagnostics = append(document.Diagnostics, newDiagnostic(
					"shadowed-atom", SeverityError,
					fmt.Sprintf(
						"Atom marker has no Markdown block of its own: the atom marker at line %d claims the next block instead.",
						makePosition(blocker.rawRange.start, lineStarts).Line,
					),
					item.rawRange.start,
					"Remove one of the stacked atom markers, or put a Markdown block between them.", lineStarts,
				))
			} else {
				document.Diagnostics = append(document.Diagnostics, newDiagnostic(
					"orphan-atom", SeverityError,
					"Atom marker is not followed by a Markdown block.", item.rawRange.start,
					"Add content after the marker or remove the marker.", lineStarts,
				))
			}
			continue
		}

		// assigned[blockIndex] can never already be set here: nextContentBlock
		// only returns blocks at or after this directive's own end, directives
		// are processed in source order, and any earlier directive that could
		// reach the same block would have been reported above as blocking
		// (or blocked by) this one first. There is no reachable path to a
		// genuine double assignment, so there is nothing to guard here.
		block := blocks[blockIndex]
		atom := Atom{
			ID: item.id, Slug: item.slug, Digest: item.digest, Attributes: item.attributes,
			Marker:       rangePointer(makeRange(item.rawRange, lineStarts)),
			MarkerSource: string(source[item.rawRange.start:item.rawRange.end]),
			Content:      makeRange(byteRange{block.start, block.end}, lineStarts),
			NodeType:     block.nodeType,
			Text:         string(bytes.TrimRight(source[block.start:block.end], "\r\n")),
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
	applyGroups(&document, source, directives, lineStarts)
	lintDocument(&document, lineStarts, listItemCounts)
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
			switch {
			case errors.Is(err, errExtraDirectiveContent):
				diagnostics = append(diagnostics, newDiagnostic(
					"extra-directive-content", SeverityError,
					err.Error(), start,
					"Remove everything inside the comment except the one directive element.", lineStarts,
				))
			case err != nil:
				diagnostics = append(diagnostics, newDiagnostic(
					"invalid-directive", SeverityError, err.Error(), start,
					"Correct the XML-shaped Atomdown directive.", lineStarts,
				))
			case !directiveOccupiesLines(source, start, end):
				diagnostics = append(diagnostics, newDiagnostic(
					"inline-directive", SeverityError,
					"Atomdown directive must occupy its own source lines.", start,
					"Move the directive to separate lines outside the Markdown block.", lineStarts,
				))
			default:
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

// directiveOccupiesLines reports whether a directive spanning
// source[start:end] has nothing but whitespace outside itself on every
// source line it touches. A directive may wrap across several lines, so
// this checks each spanned line, not only the first and the last:
//
//   - On the first line, everything before "<!--" must be whitespace.
//   - On the last line, everything after "-->" must be whitespace.
//   - Every line in between lies wholly inside [start, end), because a
//     comment is one contiguous byte range, so no part of an interior line
//     sits outside the directive. The loop still walks those lines, so the
//     rule reads as "no line may carry other content" rather than as two
//     special cases. Content inside the comment that is not part of the one
//     directive element is a separate defect; parseDirective reports it as
//     errExtraDirectiveContent.
func directiveOccupiesLines(source []byte, start, end int) bool {
	firstLineStart := bytes.LastIndexByte(source[:start], '\n') + 1
	lastLineEnd := len(source)
	if relative := bytes.IndexByte(source[end:], '\n'); relative >= 0 {
		lastLineEnd = end + relative
	}

	for cursor := firstLineStart; cursor < lastLineEnd; {
		lineEnd := lastLineEnd
		if relative := bytes.IndexByte(source[cursor:lastLineEnd], '\n'); relative >= 0 {
			lineEnd = cursor + relative
		}
		if len(bytes.TrimSpace(outsideDirective(source, cursor, lineEnd, start, end))) != 0 {
			return false
		}
		if lineEnd >= lastLineEnd {
			break
		}
		cursor = lineEnd + 1
	}
	return true
}

// outsideDirective returns the part of source[lineStart:lineEnd) that lies
// outside the directive's [start, end) byte range. A line is either before
// the directive's own bytes, after them, or wholly inside them, so at most
// one side can be non-empty on any single line.
func outsideDirective(source []byte, lineStart, lineEnd, start, end int) []byte {
	if lineStart < start {
		return source[lineStart:min(lineEnd, start)]
	}
	if lineEnd > end {
		return source[max(lineStart, end):lineEnd]
	}
	return nil
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
			case key == "digest" && result.kind == directiveAtom:
				result.digest = attribute.Value
			case key == "version" && result.kind == directiveDocument:
				result.version = attribute.Value
			default:
				result.attributes = append(result.attributes, Attribute{Name: key, Value: attribute.Value})
			}
		}
		if decoder.InputOffset() != int64(len(trimmed)) {
			return directive{}, true, errExtraDirectiveContent
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
			return directive{}, true, errExtraDirectiveContent
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

	var topLevel []ast.Node
	for node := root.FirstChild(); node != nil; node = node.NextSibling() {
		topLevel = append(topLevel, node)
	}
	starts := resolveTopLevelStarts(source, topLevel)

	var blocks []markdownBlock
	for index, node := range topLevel {
		start, ok := starts[index]
		if !ok {
			continue
		}
		start = sourceLineStart(source, start)
		itemCount := 0
		if node.Kind() == ast.KindList {
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				itemCount++
			}
		}
		blocks = append(blocks, markdownBlock{start: start, nodeType: node.Kind().String(), itemCount: itemCount})
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

// markdownNodeEnd returns the byte offset just past a node's last source
// line. It mirrors markdownNodeStart: a node with no Lines() of its own (a
// container such as List or Blockquote) falls back to its last descendant
// that has one. A ThematicBreak has neither Lines() nor children, so it
// reports false; callers resolve a thematic break's extent separately, in
// resolveTopLevelStarts below.
func markdownNodeEnd(node ast.Node) (int, bool) {
	lines := node.Lines()
	if lines != nil && lines.Len() > 0 {
		return lines.At(lines.Len() - 1).Stop, true
	}
	for child := node.LastChild(); child != nil; child = child.PreviousSibling() {
		if end, ok := markdownNodeEnd(child); ok {
			return end, true
		}
	}
	return 0, false
}

// resolveTopLevelStarts finds the start offset of every top-level node,
// including a ThematicBreak, which goldmark's parser never attaches source
// Lines() to (see ast.NewThematicBreak and parser.thematicBreakPraser.Open).
// Without a resolved start, scanMarkdownBlocks previously skipped a
// thematic break entirely: the block before it silently absorbed the "---"
// line and everything up to the next real block, and a break could never
// be targeted by its own atom marker.
//
// A ThematicBreak's start is found by scanning the source gap between the
// nearest preceding sibling with a known end and the nearest following
// sibling with a known start. Every non-blank line in that gap must be a
// thematic break line: if goldmark had parsed a gap line as anything else,
// that line would have produced its own node and narrowed the gap. So the
// non-blank lines in a gap, taken in order, correspond one-to-one with the
// run of ThematicBreak nodes that gap contains.
func resolveTopLevelStarts(source []byte, nodes []ast.Node) map[int]int {
	starts := make(map[int]int, len(nodes))
	for index, node := range nodes {
		if start, ok := markdownNodeStart(node); ok {
			starts[index] = start
		}
	}
	for index := 0; index < len(nodes); {
		if _, ok := starts[index]; ok {
			index++
			continue
		}
		runEnd := index
		for runEnd < len(nodes) {
			if _, ok := starts[runEnd]; ok {
				break
			}
			runEnd++
		}
		lower := 0
		if index > 0 {
			if end, ok := markdownNodeEnd(nodes[index-1]); ok {
				lower = end
			}
		}
		upper := len(source)
		if runEnd < len(nodes) {
			upper = starts[runEnd]
		}
		lines := nonBlankLineOffsets(source, lower, upper)
		for offset := index; offset < runEnd && offset-index < len(lines); offset++ {
			starts[offset] = lines[offset-index]
		}
		index = runEnd
	}
	return starts
}

// nonBlankLineOffsets returns the start offset of every non-blank line in
// source[lower:upper).
func nonBlankLineOffsets(source []byte, lower, upper int) []int {
	var offsets []int
	cursor := lower
	for cursor < upper {
		lineEnd := upper
		nextCursor := upper
		if relative := bytes.IndexByte(source[cursor:upper], '\n'); relative >= 0 {
			lineEnd = cursor + relative
			nextCursor = lineEnd + 1
		}
		line := bytes.TrimRight(source[cursor:lineEnd], "\r")
		if len(bytes.TrimSpace(line)) > 0 {
			offsets = append(offsets, cursor)
		}
		cursor = nextCursor
	}
	return offsets
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

// firstBlockingDirective reports the nearest directive that sits between
// `current` and the candidate block at blockStart, if one exists. A
// group-start directive never blocks: it can open right after an atom
// directive without claiming that atom's target block.
func firstBlockingDirective(directives []directive, current int, blockStart int) (directive, bool) {
	for index := current + 1; index < len(directives); index++ {
		item := directives[index]
		if item.rawRange.start >= blockStart {
			return directive{}, false
		}
		if item.kind == directiveAtom || item.kind == directiveGroupEnd || item.kind == directiveDocument {
			return item, true
		}
	}
	return directive{}, false
}

func applyGroups(document *Document, source []byte, directives []directive, lineStarts []int) {
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
				Marker:       makeRange(item.rawRange, lineStarts),
				MarkerSource: string(source[item.rawRange.start:item.rawRange.end]),
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
			open.EndMarkerSource = string(source[item.rawRange.start:item.rawRange.end])
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
