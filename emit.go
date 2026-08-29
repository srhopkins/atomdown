package atomdown

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

// Emit reconstructs canonical marked Markdown from a parsed document model.
// Parse-only fields such as positions, node types, and diagnostics are ignored.
func Emit(document Document) ([]byte, error) {
	atoms := append([]Atom(nil), document.Atoms...)
	for index := range atoms {
		if !atoms[index].Implicit && atoms[index].ID == "" {
			id, err := NewID()
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
			id, err := NewID()
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
		marker, err := sourceMarker("atomdown", true, []Attribute{{Name: "version", Value: version}}, document.Attributes)
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
				output.WriteString("<!-- </atom-group> -->\n\n")
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
				marker, err := sourceMarker("atom-group", false, attributes, group.Attributes)
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
			marker, err := sourceMarker("atom", true, attributes, atom.Attributes)
			if err != nil {
				return nil, err
			}
			output.WriteString(marker)
			output.WriteString("\n\n")
		}
		writeBlock(&output, atom.Text)
	}
	if openGroup >= 0 {
		output.WriteString("<!-- </atom-group> -->\n")
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

func sourceMarker(name string, selfClosing bool, core, extensions []Attribute) (string, error) {
	seen := make(map[string]bool, len(core)+len(extensions))
	var source strings.Builder
	source.WriteString("<!-- <")
	source.WriteString(name)
	for _, attribute := range append(core, extensions...) {
		if attribute.Name == "" {
			return "", fmt.Errorf("%s has an empty attribute name", name)
		}
		if seen[attribute.Name] {
			return "", fmt.Errorf("%s has duplicate attribute %q", name, attribute.Name)
		}
		seen[attribute.Name] = true
		source.WriteByte(' ')
		source.WriteString(attribute.Name)
		source.WriteString(`="`)
		if err := xml.EscapeText(&source, []byte(attribute.Value)); err != nil {
			return "", fmt.Errorf("escape %s attribute %q: %w", name, attribute.Name, err)
		}
		source.WriteByte('"')
	}
	if selfClosing {
		source.WriteByte('/')
	}
	source.WriteString("> -->")

	body := strings.TrimSuffix(strings.TrimPrefix(source.String(), "<!-- "), " -->")
	decoder := xml.NewDecoder(strings.NewReader(body))
	if _, err := decoder.Token(); err != nil {
		return "", fmt.Errorf("invalid %s marker: %w", name, err)
	}
	return source.String(), nil
}

func writeBlock(output *bytes.Buffer, text string) {
	output.WriteString(strings.TrimRight(text, "\r\n"))
	output.WriteString("\n\n")
}
