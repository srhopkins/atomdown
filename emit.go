package atomdown

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

// EmitOptions selects how Emit writes each directive.
type EmitOptions struct {
	// Flatten rewrites every directive to one source line, discarding the
	// line wrapping and indentation the author gave it. It is the opt-in
	// canonicalizing writer: emit --flatten on the command line.
	Flatten bool
}

// Emit reconstructs marked Markdown from a parsed document model.
// Parse-only fields such as positions, node types, and diagnostics are ignored.
//
// Emit preserves each directive's authored source layout. A directive whose
// attributes are unchanged is written back byte for byte, so line wrapping,
// interior newlines and indentation survive a parse and write cycle. See
// EmitWithOptions for the rule that applies to a directive the caller
// modified, and for the flattening option.
func Emit(document Document) ([]byte, error) {
	return EmitWithOptions(document, EmitOptions{})
}

// EmitWithOptions is Emit with explicit options.
//
// Layout rules, in the order they apply to one directive:
//
//  1. With EmitOptions.Flatten, the directive is written as one line in
//     canonical attribute order. Nothing of the authored shape survives.
//  2. When the model carries no authored marker source — a directive Emit is
//     writing for the first time — it is written as one line.
//  3. When the attribute set is unchanged (the same names carrying the same
//     values, in any order), the authored bytes are written back exactly.
//  4. When the attribute set changed, the directive is rebuilt into the
//     authored skeleton: a wrapped directive stays wrapped, each attribute
//     keeps the author's separator and indentation, and the closing token
//     keeps its own line. Attribute order becomes canonical, because the
//     model records no authored order for an attribute that just arrived.
//     A directive the author wrote on one line stays on one line.
//
// Rule 4 is the deliberate limit of what a writer can honor. The authored
// bytes describe one attribute set; once that set changes there is no
// authored text for the new state, so the skeleton is kept and the attribute
// sequence is rebuilt.
func EmitWithOptions(document Document, options EmitOptions) ([]byte, error) {
	// Collect every ID already present before minting any new one, so a
	// generated atom or group ID never collides with one the document
	// already has.
	used := usedIDs(document)

	atoms := append([]Atom(nil), document.Atoms...)
	for index := range atoms {
		if !atoms[index].Implicit && atoms[index].ID == "" {
			id, err := newUniqueID(used)
			if err != nil {
				return nil, err
			}
			atoms[index].ID = id
		}
	}

	groups := append([]AtomGroup(nil), document.Groups...)
	groupByID := make(map[string]int, len(groups))
	for index := range groups {
		if groups[index].ID == "" {
			id, err := newUniqueID(used)
			if err != nil {
				return nil, err
			}
			groups[index].ID = id
		}
		if _, exists := groupByID[groups[index].ID]; exists {
			return nil, fmt.Errorf("duplicate atom group ID %q", groups[index].ID)
		}
		groupByID[groups[index].ID] = index
	}

	memberships, err := resolveGroupMemberships(atoms, groups, groupByID)
	if err != nil {
		return nil, err
	}

	var output bytes.Buffer
	if document.Declared || document.Version != "" {
		version := document.Version
		if version == "" {
			version = "1"
		}
		marker, err := directiveText("atomdown", true, document.MarkerSource, options,
			[]Attribute{{Name: "version", Value: version}}, document.Attributes)
		if err != nil {
			return nil, err
		}
		output.WriteString(marker)
		output.WriteString("\n\n")
	}

	openGroup := -1
	closedGroups := make(map[int]bool, len(groups))
	for index, atom := range atoms {
		groupIndex := memberships[index]
		if groupIndex != openGroup {
			if openGroup >= 0 {
				output.WriteString(groupCloseText(groups[openGroup], options))
				output.WriteString("\n\n")
				closedGroups[openGroup] = true
			}
			openGroup = groupIndex
			if openGroup >= 0 {
				if closedGroups[openGroup] {
					return nil, fmt.Errorf("atom group %q is not contiguous", groups[openGroup].ID)
				}
				group := groups[openGroup]
				attributes := []Attribute{{Name: "id", Value: group.ID}}
				if group.Slug != "" {
					attributes = append(attributes, Attribute{Name: "slug", Value: group.Slug})
				}
				marker, err := directiveText("atom-group", false, group.MarkerSource, options, attributes, group.Attributes)
				if err != nil {
					return nil, err
				}
				output.WriteString(marker)
				output.WriteString("\n\n")
			}
		}

		if !atom.Implicit {
			attributes := []Attribute{{Name: "id", Value: atom.ID}}
			if atom.Slug != "" {
				attributes = append(attributes, Attribute{Name: "slug", Value: atom.Slug})
			}
			if atom.Digest != "" {
				attributes = append(attributes, Attribute{Name: "digest", Value: atom.Digest})
			}
			marker, err := directiveText("atom", true, atom.MarkerSource, options, attributes, atom.Attributes)
			if err != nil {
				return nil, err
			}
			output.WriteString(marker)
			output.WriteString("\n\n")
		}
		writeBlock(&output, atom.Text)
	}
	if openGroup >= 0 {
		output.WriteString(groupCloseText(groups[openGroup], options))
		output.WriteString("\n")
	}

	return output.Bytes(), nil
}

func resolveGroupMemberships(atoms []Atom, groups []AtomGroup, groupByID map[string]int) ([]int, error) {
	memberships := make([]int, len(atoms))
	for index := range memberships {
		memberships[index] = -1
	}

	atomIndexes := make(map[string][]int, len(atoms))
	for index, atom := range atoms {
		if atom.ID != "" {
			atomIndexes[atom.ID] = append(atomIndexes[atom.ID], index)
		}
		if atom.GroupID == "" {
			continue
		}
		groupIndex, exists := groupByID[atom.GroupID]
		if !exists {
			return nil, fmt.Errorf("atom %q references unknown group %q", atom.ID, atom.GroupID)
		}
		memberships[index] = groupIndex
	}

	for groupIndex, group := range groups {
		for _, atomID := range group.AtomIDs {
			indexes := atomIndexes[atomID]
			if len(indexes) == 0 {
				return nil, fmt.Errorf("atom group %q references unknown atom %q", group.ID, atomID)
			}
			if len(indexes) > 1 {
				return nil, fmt.Errorf("atom group %q references duplicate atom ID %q", group.ID, atomID)
			}
			atomIndex := indexes[0]
			if memberships[atomIndex] >= 0 && memberships[atomIndex] != groupIndex {
				return nil, fmt.Errorf("atom %q belongs to conflicting groups", atomID)
			}
			memberships[atomIndex] = groupIndex
		}
	}
	return memberships, nil
}

// directiveText writes one directive, keeping as much of the author's source
// layout as the requested attribute set allows. EmitWithOptions documents the
// rule this implements.
func directiveText(name string, selfClosing bool, markerSource string, options EmitOptions, core, extensions []Attribute) (string, error) {
	attributes := make([]Attribute, 0, len(core)+len(extensions))
	attributes = append(attributes, core...)
	attributes = append(attributes, extensions...)

	if options.Flatten || markerSource == "" {
		return flatLayout(name, selfClosing).render(name, attributes)
	}

	layout, err := parseDirectiveLayout([]byte(markerSource))
	if err != nil {
		// The recorded bytes are not a directive this package can read, so
		// they describe no shape worth keeping. A caller that hand-built the
		// model can put anything in this field; the identity fields still
		// have to reach the output.
		return flatLayout(name, selfClosing).render(name, attributes)
	}

	authored, err := markerAttributeValues(markerSource)
	if err == nil && sameAttributeValues(authored, attributes) {
		return markerSource, nil
	}
	return layout.render(name, attributes)
}

// groupCloseText writes an atom group's closing directive. The directive
// carries no attributes, so an authored one is always written back exactly.
func groupCloseText(group AtomGroup, options EmitOptions) string {
	if options.Flatten || group.EndMarkerSource == "" {
		return "<!-- </atom-group> -->"
	}
	return group.EndMarkerSource
}

// markerAttributeValues reads every attribute of an authored directive,
// identity attributes included, as one name-to-value map. It decodes the
// marker the way parseDirective does, so an escaped value compares equal to
// the model value the parser produced from it.
func markerAttributeValues(markerSource string) (map[string]string, error) {
	start := strings.Index(markerSource, "<!--")
	end := strings.LastIndex(markerSource, "-->")
	if start < 0 || end < start {
		return nil, fmt.Errorf("marker is not an HTML comment")
	}
	body := strings.TrimSpace(markerSource[start+len("<!--") : end])
	decoder := xml.NewDecoder(strings.NewReader(body))
	token, err := decoder.RawToken()
	if err != nil {
		return nil, err
	}
	element, isStart := token.(xml.StartElement)
	if !isStart {
		return map[string]string{}, nil
	}
	values := make(map[string]string, len(element.Attr))
	for _, attribute := range element.Attr {
		values[xmlAttributeName(attribute.Name)] = attribute.Value
	}
	return values, nil
}

// sameAttributeValues reports whether an authored directive already carries
// exactly the attributes a writer is about to write. Order does not matter:
// the author's order is part of the authored bytes, and reordering an
// unchanged set would be the same silent reflow this rule exists to prevent.
func sameAttributeValues(authored map[string]string, attributes []Attribute) bool {
	if len(authored) != len(attributes) {
		return false
	}
	for _, attribute := range attributes {
		value, exists := authored[attribute.Name]
		if !exists || value != attribute.Value {
			return false
		}
	}
	return true
}

func writeBlock(output *bytes.Buffer, text string) {
	output.WriteString(strings.TrimRight(text, "\r\n"))
	output.WriteString("\n\n")
}
