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
	used := usedIDs(document)

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

// usedIDs collects every atom and atom-group ID already present in a parsed
// document, so a caller minting new IDs never collides with an existing one.
func usedIDs(document Document) map[string]struct{} {
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
	return used
}

// generateID is the ID source every call to newUniqueID draws from.
// Production code always uses NewID; a test overrides this package-level
// variable to force a deterministic collision and prove newUniqueID
// recovers from it, since 40 real random bits will never collide inside a
// single test run. Restore the original value (via defer) before the test
// returns.
var generateID = NewID

// maxIDCollisionRetries bounds newUniqueID's retry loop. A real collision
// against 40 random bits is astronomically unlikely within one document, so
// this bound exists only so a pathological generator (or a test forcing
// every attempt to collide) fails with a clear error instead of looping
// forever.
const maxIDCollisionRetries = 100

// newUniqueID mints an ID that is not already in used, regenerating on
// collision up to maxIDCollisionRetries times, and records the returned ID
// in used so a subsequent call in the same batch also avoids it.
func newUniqueID(used map[string]struct{}) (string, error) {
	for attempt := 0; attempt < maxIDCollisionRetries; attempt++ {
		id, err := generateID()
		if err != nil {
			return "", err
		}
		if _, exists := used[id]; exists {
			continue
		}
		used[id] = struct{}{}
		return id, nil
	}
	return "", fmt.Errorf("generate unique Atomdown ID: exhausted %d attempts against %d existing IDs", maxIDCollisionRetries, len(used))
}

func materializeLineEnding(source []byte) string {
	if bytes.Contains(source, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}
