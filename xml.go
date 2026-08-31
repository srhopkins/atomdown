package atomdown

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"
)

type normalizedDocument struct {
	XMLName    xml.Name         `xml:"atomdown"`
	Version    string           `xml:"version,attr"`
	Attributes []xml.Attr       `xml:",any,attr"`
	Items      []normalizedItem `xml:"-"`
}

type normalizedItem struct {
	Atom  *normalizedAtom
	Group *normalizedGroup
}

type normalizedAtom struct {
	XMLName    xml.Name   `xml:"atom"`
	ID         string     `xml:"id,attr"`
	Slug       string     `xml:"slug,attr,omitempty"`
	Digest     string     `xml:"digest,attr,omitempty"`
	Attributes []xml.Attr `xml:",any,attr"`
}

type normalizedGroup struct {
	XMLName    xml.Name         `xml:"atom-group"`
	ID         string           `xml:"id,attr"`
	Slug       string           `xml:"slug,attr,omitempty"`
	Attributes []xml.Attr       `xml:",any,attr"`
	Atoms      []normalizedAtom `xml:"atom"`
}

// NormalizedXML returns the Atomdown metadata model as a conventional XML document.
// Markdown content remains in the source file and is represented by atom IDs.
func NormalizedXML(document Document) ([]byte, error) {
	version := document.Version
	if version == "" {
		version = "1"
	}
	root := normalizedDocument{Version: version, Attributes: toXMLAttributes(document.Attributes)}

	groupByID := make(map[string]AtomGroup)
	for _, group := range document.Groups {
		groupByID[group.ID] = group
	}
	groupItems := make(map[string]int)
	for _, atom := range document.Atoms {
		if atom.Implicit {
			continue
		}
		normalized := normalizedAtom{ID: atom.ID, Slug: atom.Slug, Digest: atom.Digest, Attributes: toXMLAttributes(atom.Attributes)}
		if atom.GroupID == "" {
			root.Items = append(root.Items, normalizedItem{Atom: &normalized})
			continue
		}
		itemIndex, exists := groupItems[atom.GroupID]
		if !exists {
			group := groupByID[atom.GroupID]
			normalizedGroupValue := normalizedGroup{
				ID: group.ID, Slug: group.Slug, Attributes: toXMLAttributes(group.Attributes),
			}
			root.Items = append(root.Items, normalizedItem{Group: &normalizedGroupValue})
			itemIndex = len(root.Items) - 1
			groupItems[atom.GroupID] = itemIndex
		}
		root.Items[itemIndex].Group.Atoms = append(root.Items[itemIndex].Group.Atoms, normalized)
	}

	var output bytes.Buffer
	output.WriteString(xml.Header)
	encoder := xml.NewEncoder(&output)
	encoder.Indent("", "  ")
	start := xml.StartElement{Name: xml.Name{Local: "atomdown"}}
	start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "version"}, Value: root.Version})
	start.Attr = append(start.Attr, root.Attributes...)
	if err := encoder.EncodeToken(start); err != nil {
		return nil, fmt.Errorf("encode Atomdown root: %w", err)
	}
	for _, item := range root.Items {
		if item.Atom != nil {
			if err := encoder.Encode(item.Atom); err != nil {
				return nil, fmt.Errorf("encode atom: %w", err)
			}
		}
		if item.Group != nil {
			if err := encoder.Encode(item.Group); err != nil {
				return nil, fmt.Errorf("encode atom group: %w", err)
			}
		}
	}
	if err := encoder.EncodeToken(start.End()); err != nil {
		return nil, fmt.Errorf("close Atomdown root: %w", err)
	}
	if err := encoder.Flush(); err != nil {
		return nil, fmt.Errorf("flush normalized XML: %w", err)
	}
	output.WriteByte('\n')
	return output.Bytes(), nil
}

func toXMLAttributes(attributes []Attribute) []xml.Attr {
	result := make([]xml.Attr, 0, len(attributes))
	sorted := append([]Attribute(nil), attributes...)
	sort.Slice(sorted, func(left, right int) bool { return sorted[left].Name < sorted[right].Name })
	for _, attribute := range sorted {
		result = append(result, xml.Attr{Name: xml.Name{Local: attribute.Name}, Value: attribute.Value})
	}
	return result
}
