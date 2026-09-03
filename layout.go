package atomdown

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

// directiveLayout records the whitespace skeleton of one authored directive.
//
// A directive can span several source lines: see SPEC.md "Directive line
// span". Whitespace inside a directive carries no meaning to a reader, so
// every writer in this package treats it as the author's property and copies
// it rather than choosing a shape of its own. One type describes that shape
// for every writer, so emit and materialize --digest cannot drift apart.
type directiveLayout struct {
	// head is "<!--", the whitespace the author put before the element, and
	// the element's opening bracket and name: "<!-- <atom" for a one-line
	// directive, "<!--\n  <atom" for a wrapped one.
	head string
	// separator is the exact whitespace run that preceded the first
	// attribute. A writer repeats it before every attribute, so an attribute
	// it adds lands in the same column as the attributes already there.
	separator string
	// tail is the whitespace run before the closing token, the closing token
	// itself ("/>" or ">"), and every byte through the final "-->".
	tail string
	// insertAt is the offset, inside the marker text, of the first byte of
	// the whitespace run that tail begins with. A writer that must keep the
	// existing attribute bytes exactly splices a new attribute in there.
	insertAt int
	// wrapped reports whether the author spread the directive over more than
	// one source line.
	wrapped bool
	// selfClosing reports whether the element closes with "/>". Only an
	// atom-group opening marker does not.
	selfClosing bool
}

// parseDirectiveLayout reads the skeleton of one directive's marker text.
// raw runs from the opening "<!--" through the closing "-->", which is
// exactly the span Parse records in Atom.Marker and AtomGroup.Marker.
//
// It re-derives the element's extent by decoding the directive's trimmed body
// as one raw XML token, the same way scanDirectives does. After that token,
// encoding/xml reports an InputOffset immediately past the closing token, so
// the split is exact even when an attribute value contains a literal "/" or
// ">".
func parseDirectiveLayout(raw []byte) (directiveLayout, error) {
	commentStart := bytes.Index(raw, []byte("<!--"))
	commentEnd := bytes.LastIndex(raw, []byte("-->"))
	if commentStart < 0 || commentEnd < 0 || commentEnd < commentStart {
		return directiveLayout{}, fmt.Errorf("marker is not a well-formed Atomdown comment")
	}
	commentStart += len("<!--")

	body := raw[commentStart:commentEnd]
	trimmed := bytes.TrimSpace(body)
	leading := len(body) - len(bytes.TrimLeft(body, " \t\r\n"))

	decoder := xml.NewDecoder(bytes.NewReader(trimmed))
	if _, err := decoder.RawToken(); err != nil {
		return directiveLayout{}, fmt.Errorf("parse directive marker: %w", err)
	}
	tagEnd := int(decoder.InputOffset())
	if tagEnd < 2 || tagEnd > len(trimmed) || trimmed[tagEnd-1] != '>' {
		return directiveLayout{}, fmt.Errorf("directive marker does not end in a closing token")
	}
	closeLength := 1
	if trimmed[tagEnd-2] == '/' {
		closeLength = 2
	}

	element := trimmed[:tagEnd]
	nameStart := 1
	if len(element) > 1 && element[1] == '/' {
		nameStart = 2
	}
	nameEnd := nameStart
	for nameEnd < len(element) && !isDirectiveNameBreak(element[nameEnd]) {
		nameEnd++
	}
	if nameEnd == nameStart {
		return directiveLayout{}, fmt.Errorf("directive marker has no element name")
	}

	// The attribute region is everything between the element name and the
	// closing token. Its leading whitespace run is the author's attribute
	// separator; its trailing whitespace run belongs to the closing token.
	region := element[nameEnd : tagEnd-closeLength]
	separator := " "
	beforeClose := len(region)
	if len(bytes.TrimSpace(region)) > 0 {
		// XML requires whitespace between the element name and its first
		// attribute, so this run is never empty when an attribute exists.
		separator = string(region[:len(region)-len(bytes.TrimLeft(region, " \t\r\n"))])
		beforeClose = len(region) - len(bytes.TrimRight(region, " \t\r\n"))
	}

	elementStart := commentStart + leading
	insertAt := elementStart + tagEnd - closeLength - beforeClose
	return directiveLayout{
		head:      string(raw[:elementStart]) + string(element[:nameEnd]),
		separator: separator,
		tail:      string(region[len(region)-beforeClose:]) + string(element[tagEnd-closeLength:]) + string(raw[elementStart+tagEnd:]),
		insertAt:  insertAt,

		wrapped:     bytes.ContainsRune(raw, '\n'),
		selfClosing: closeLength == 2,
	}, nil
}

func isDirectiveNameBreak(value byte) bool {
	switch value {
	case ' ', '\t', '\r', '\n', '/', '>':
		return true
	}
	return false
}

// flatLayout is the canonical one-line skeleton: the shape every Atomdown
// writer used before authored layout was preserved, and the shape
// emit --flatten restores.
func flatLayout(name string, selfClosing bool) directiveLayout {
	closing := "> -->"
	if selfClosing {
		closing = "/> -->"
	}
	return directiveLayout{head: "<!-- <" + name, separator: " ", tail: closing, selfClosing: selfClosing}
}

// render writes one directive with the given attributes into this skeleton.
// It repeats separator before every attribute, so a directive the author
// wrapped stays wrapped and a new attribute lines up with its siblings.
func (layout directiveLayout) render(name string, attributes []Attribute) (string, error) {
	seen := make(map[string]bool, len(attributes))
	var source strings.Builder
	source.WriteString(layout.head)
	for _, attribute := range attributes {
		if attribute.Name == "" {
			return "", fmt.Errorf("%s has an empty attribute name", name)
		}
		if seen[attribute.Name] {
			return "", fmt.Errorf("%s has duplicate attribute %q", name, attribute.Name)
		}
		seen[attribute.Name] = true
		source.WriteString(layout.separator)
		source.WriteString(attribute.Name)
		source.WriteString(`="`)
		if err := xml.EscapeText(&source, []byte(attribute.Value)); err != nil {
			return "", fmt.Errorf("escape %s attribute %q: %w", name, attribute.Name, err)
		}
		source.WriteByte('"')
	}
	source.WriteString(layout.tail)

	marker := source.String()
	if err := checkDirectiveMarker(marker); err != nil {
		return "", fmt.Errorf("invalid %s marker: %w", name, err)
	}
	return marker, nil
}

// checkDirectiveMarker proves a rendered marker is still one well-formed XML
// element, so a writer can never produce a directive the parser rejects.
func checkDirectiveMarker(marker string) error {
	start := strings.Index(marker, "<!--")
	end := strings.LastIndex(marker, "-->")
	if start < 0 || end < start {
		return fmt.Errorf("marker is not an HTML comment")
	}
	body := strings.TrimSpace(marker[start+len("<!--") : end])
	decoder := xml.NewDecoder(strings.NewReader(body))
	if _, err := decoder.Token(); err != nil {
		return err
	}
	return nil
}
