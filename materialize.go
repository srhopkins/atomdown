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

// MaterializeDigest runs the same pass as Materialize (adding the version
// directive when missing, and a fresh marker before each implicit atom) and
// additionally writes a Core content digest to every atom that does not
// already carry one: see SPEC.md "Content digest". A newly materialized
// implicit atom gets its digest in the new marker; an existing explicit
// atom without one gets a digest attribute appended to its existing marker,
// with every other byte of that marker left untouched.
//
// An atom that already carries a digest attribute is never touched, even
// when its content has since changed. Nothing refreshes an existing
// digest automatically — that is the rule the whole feature depends on: a
// digest means "reviewed as of these bytes" only if nothing but an
// explicit, deliberate action can change it. Use atomdown drift to find an
// atom whose content no longer matches its recorded digest.
//
// It returns the resulting Markdown, the number of atom markers it added
// (implicit atoms newly materialized, same count Materialize reports), and
// the number of atoms that received a digest (that count plus every
// previously explicit atom that gained one).
func MaterializeDigest(source []byte) ([]byte, int, int, error) {
	document := Parse(source)
	used := usedIDs(document)

	var output bytes.Buffer
	cursor := 0
	lineEnding := materializeLineEnding(source)
	marked := 0
	digested := 0

	if !document.Declared {
		fmt.Fprintf(&output, "<!-- <atomdown version=%q/> -->%s", "1", lineEnding)
	}

	for _, atom := range document.Atoms {
		if atom.Implicit {
			offset := atom.Content.Start.Offset
			if offset < cursor || offset > len(source) {
				return nil, 0, 0, fmt.Errorf("materialize atom at invalid offset %d", offset)
			}
			id, err := newUniqueID(used)
			if err != nil {
				return nil, 0, 0, err
			}
			digest := ContentDigest(atom.Text)
			output.Write(source[cursor:offset])
			fmt.Fprintf(&output, "<!-- <atom id=%q digest=%q/> -->%s", id, digest, lineEnding)
			cursor = offset
			marked++
			digested++
			continue
		}

		if atom.Digest != "" || atom.Marker == nil {
			// Either already digested (never auto-refreshed) or, in
			// principle, a marker-less explicit atom (cannot occur in a
			// document Parse produced), so leave the source bytes as they
			// are; a later copy carries them through unchanged.
			continue
		}

		digest := ContentDigest(atom.Text)
		doctoredMarker, err := insertDigestAttribute(source, *atom.Marker, digest)
		if err != nil {
			return nil, 0, 0, err
		}
		if atom.Marker.Start.Offset < cursor {
			return nil, 0, 0, fmt.Errorf("materialize --digest: overlapping marker at offset %d", atom.Marker.Start.Offset)
		}
		output.Write(source[cursor:atom.Marker.Start.Offset])
		output.Write(doctoredMarker)
		cursor = atom.Marker.End.Offset
		digested++
	}
	output.Write(source[cursor:])
	return output.Bytes(), marked, digested, nil
}

// insertDigestAttribute returns the byte-for-byte marker text for an
// existing atom directive with one digest attribute appended after the
// attributes already there. Every other byte of the marker — attribute order,
// spacing, quoting of existing values — is preserved exactly, because
// materialize --digest must not otherwise touch a marker it did not itself
// just write. That is why this splices bytes instead of re-rendering the
// marker: re-rendering would re-escape values the author wrote by hand.
//
// parseDirectiveLayout supplies both the exact insertion point and the
// author's own attribute separator. In a one-line marker that separator is a
// single space, so the result is what it always was. In a wrapped marker the
// separator is the newline and indentation the author used, so the digest
// lands in the same column as its siblings and the closing token keeps its
// own line.
func insertDigestAttribute(source []byte, marker Range, digest string) ([]byte, error) {
	raw := append([]byte(nil), source[marker.Start.Offset:marker.End.Offset]...)

	layout, err := parseDirectiveLayout(raw)
	if err != nil {
		return nil, fmt.Errorf("materialize --digest: %w", err)
	}
	if !layout.selfClosing {
		return nil, fmt.Errorf("materialize --digest: atom marker must be self-closing")
	}

	var result bytes.Buffer
	result.Write(raw[:layout.insertAt])
	result.WriteString(layout.separator)
	fmt.Fprintf(&result, "digest=%q", digest)
	result.Write(raw[layout.insertAt:])
	return result.Bytes(), nil
}

func materializeLineEnding(source []byte) string {
	if bytes.Contains(source, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}
