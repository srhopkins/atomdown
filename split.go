package atomdown

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// splitRule maps one CommonMark node name accepted by materialize --split to
// the internal goldmark node kinds it operates on. parentKindName is the
// Atom.NodeType value (a goldmark Kind().String()) that must match a
// top-level atom before this rule applies to it. childKind is the node kind
// split out into its own atom.
type splitRule struct {
	name           string
	parentKindName string
	childKind      ast.NodeKind
}

// splitRules lists the CommonMark node names materialize --split accepts.
// Atomdown Core 1 only supports splitting list items out of a list; add a
// rule here (and to SPEC.md / README.md) before accepting a new name.
var splitRules = []splitRule{
	{name: "list-item", parentKindName: "List", childKind: ast.KindListItem},
}

// acceptedSplitNodeNames returns the accepted --split values in a stable
// order, for use in error messages.
func acceptedSplitNodeNames() []string {
	names := make([]string, 0, len(splitRules))
	for _, rule := range splitRules {
		names = append(names, rule.name)
	}
	sort.Strings(names)
	return names
}

func splitRuleByName(name string) (splitRule, bool) {
	for _, rule := range splitRules {
		if rule.name == name {
			return rule, true
		}
	}
	return splitRule{}, false
}

// ParseSplitNodeTypes validates and deduplicates a materialize --split flag
// value: a comma-separated list of CommonMark node names. It returns an
// error naming the accepted values when a name is unknown or the value is
// empty.
func ParseSplitNodeTypes(value string) ([]string, error) {
	var names []string
	seen := make(map[string]bool)
	for _, part := range strings.Split(value, ",") {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		if _, ok := splitRuleByName(name); !ok {
			return nil, fmt.Errorf("unknown --split node type %q; accepted values: %s", name, strings.Join(acceptedSplitNodeNames(), ", "))
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("--split requires at least one node type; accepted values: %s", strings.Join(acceptedSplitNodeNames(), ", "))
	}
	return names, nil
}

// MaterializeSplit gives every child node named by nodeTypes its own atom,
// wrapped in one atom-group per split parent, then runs the ordinary
// Materialize pass so the result composes with default behavior (adding the
// document version directive and marking any other implicit atom, such as a
// heading) in one call. It returns the resulting Markdown and the number of
// atom markers it added.
//
// A parent block already split to one child per group (the common case on a
// second run) is left untouched, which makes --split idempotent.
func MaterializeSplit(source []byte, nodeTypes []string) ([]byte, int, error) {
	rules := make([]splitRule, 0, len(nodeTypes))
	for _, name := range nodeTypes {
		rule, ok := splitRuleByName(name)
		if !ok {
			return nil, 0, fmt.Errorf("unknown --split node type %q; accepted values: %s", name, strings.Join(acceptedSplitNodeNames(), ", "))
		}
		rules = append(rules, rule)
	}
	if len(rules) == 0 {
		return nil, 0, fmt.Errorf("--split requires at least one node type; accepted values: %s", strings.Join(acceptedSplitNodeNames(), ", "))
	}

	document := Parse(source)
	used := usedIDs(document)
	root := parseMarkdownTree(source)
	lineEnding := materializeLineEnding(source)

	var output bytes.Buffer
	cursor := 0
	added := 0

	for _, atom := range document.Atoms {
		rule, ok := matchingSplitRule(rules, atom.NodeType)
		if !ok {
			continue
		}
		items, ok := splitChildRanges(root, source, atom.Content.Start.Offset, atom.Content.End.Offset, rule.childKind)
		if !ok || len(items) <= 1 {
			// Not a match for this rule, or already split down to one child
			// per parent: nothing left to do here.
			continue
		}

		spanStart := atom.Content.Start.Offset
		if atom.Marker != nil {
			spanStart = atom.Marker.Start.Offset
		}
		if spanStart < cursor {
			return nil, 0, fmt.Errorf("materialize --split: overlapping atom at offset %d", spanStart)
		}

		groupID, err := newUniqueID(used)
		if err != nil {
			return nil, 0, err
		}

		output.Write(source[cursor:spanStart])
		fmt.Fprintf(&output, "<!-- <atom-group id=%q> -->%s", groupID, lineEnding)
		for _, item := range items {
			itemID, err := newUniqueID(used)
			if err != nil {
				return nil, 0, err
			}
			fmt.Fprintf(&output, "<!-- <atom id=%q/> -->%s", itemID, lineEnding)
			output.Write(source[item.start:item.end])
			output.WriteString(lineEnding)
			added++
		}
		output.WriteString("<!-- </atom-group> -->")
		cursor = atom.Content.End.Offset
	}
	output.Write(source[cursor:])

	// A plain materialize pass fills in everything --split does not touch:
	// the document version directive and any other implicit atom (a
	// heading, a paragraph) that sits alongside the split list.
	result, marked, err := Materialize(output.Bytes())
	if err != nil {
		return nil, 0, err
	}
	return result, added + marked, nil
}

func matchingSplitRule(rules []splitRule, nodeType string) (splitRule, bool) {
	for _, rule := range rules {
		if rule.parentKindName == nodeType {
			return rule, true
		}
	}
	return splitRule{}, false
}

// parseMarkdownTree parses source with the same goldmark configuration the
// rest of the package uses, so byte offsets line up with Parse's own view of
// the document.
func parseMarkdownTree(source []byte) ast.Node {
	markdown := goldmark.New(goldmark.WithExtensions(extension.GFM))
	return markdown.Parser().Parse(text.NewReader(source))
}

// splitChildRanges finds the top-level node starting at contentStart and
// returns the byte range of each direct child of kind childKind. It reports
// false when no top-level node starts exactly at contentStart.
func splitChildRanges(root ast.Node, source []byte, contentStart, contentEnd int, childKind ast.NodeKind) ([]byteRange, bool) {
	node, ok := findTopLevelNode(root, source, contentStart)
	if !ok {
		return nil, false
	}

	var children []ast.Node
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if child.Kind() == childKind {
			children = append(children, child)
		}
	}

	ranges := make([]byteRange, 0, len(children))
	for index, child := range children {
		start, ok := markdownNodeStart(child)
		if !ok {
			continue
		}
		start = sourceLineStart(source, start)
		end := contentEnd
		if index+1 < len(children) {
			if nextStart, ok := markdownNodeStart(children[index+1]); ok {
				end = sourceLineStart(source, nextStart)
			}
		}
		ranges = append(ranges, byteRange{start: start, end: trimBlockEnd(source, start, end)})
	}
	return ranges, true
}

func findTopLevelNode(root ast.Node, source []byte, contentStart int) (ast.Node, bool) {
	for node := root.FirstChild(); node != nil; node = node.NextSibling() {
		start, ok := markdownNodeStart(node)
		if !ok {
			continue
		}
		if sourceLineStart(source, start) == contentStart {
			return node, true
		}
	}
	return nil, false
}
