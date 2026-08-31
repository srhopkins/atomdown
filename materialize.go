package atomdown

import (
	"bytes"
	"fmt"
)

// Materialize inserts an explicit atom marker before every implicit atom and
// adds the document-level version directive when the source does not already
// declare one. Existing source bytes and explicit directives remain
// unchanged. It returns the resulting Markdown and the number of atom
// markers it added; adding the version directive is not counted.
func Materialize(source []byte) ([]byte, int, error) {
	document := Parse(source)
	used := make(map[string]struct{}, len(document.Atoms)+len(document.Groups))
	for _, atom := range document.Atoms {
		if atom.ID != "" {
			used[atom.ID] = struct{}{}
		}
	}
	for _, group := range document.Groups {
		if group.ID != "" {
			used[group.ID] = struct{}{}
		}
	}

	var output bytes.Buffer
	cursor := 0
	lineEnding := materializeLineEnding(source)
	marked := 0

	// Writers emit the document-level version directive; readers never
	// require it. Add it once, at the very top, when the source lacks one.
	// Never move or duplicate an existing directive.
	if !document.Declared {
		fmt.Fprintf(&output, "<!-- <atomdown version=%q/> -->%s", "1", lineEnding)
	}

	for _, atom := range document.Atoms {
		if !atom.Implicit {
			continue
		}
		offset := atom.Content.Start.Offset
		if offset < cursor || offset > len(source) {
			return nil, 0, fmt.Errorf("materialize atom at invalid offset %d", offset)
		}
		id, err := newUniqueID(used)
		if err != nil {
			return nil, 0, err
		}
		output.Write(source[cursor:offset])
		fmt.Fprintf(&output, "<!-- <atom id=%q/> -->%s", id, lineEnding)
		cursor = offset
		marked++
	}
	output.Write(source[cursor:])
	return output.Bytes(), marked, nil
}

func newUniqueID(used map[string]struct{}) (string, error) {
	for {
		id, err := NewID()
		if err != nil {
			return "", err
		}
		if _, exists := used[id]; exists {
			continue
		}
		used[id] = struct{}{}
		return id, nil
	}
}

func materializeLineEnding(source []byte) string {
	if bytes.Contains(source, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}
