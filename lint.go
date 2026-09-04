package atomdown

import (
	"fmt"
	"regexp"
)

func lintDocument(document *Document, lineStarts []int, listItemCounts map[int]int) {
	if document.Declared && document.Version == "" {
		document.Diagnostics = append(document.Diagnostics, newDiagnostic(
			"missing-version", SeverityError,
			"Atomdown document marker requires a version attribute.", 0,
			"Add version=\"1\" to the atomdown marker.", lineStarts,
		))
	}
	if document.Version != "" && document.Version != "1" {
		document.Diagnostics = append(document.Diagnostics, newDiagnostic(
			"unsupported-version", SeverityError,
			fmt.Sprintf("Atomdown version %q is not supported.", document.Version), 0,
			"Use version 1.", lineStarts,
		))
	}
	if !document.Declared {
		document.Diagnostics = append(document.Diagnostics, newDiagnostic(
			"missing-version-directive", SeverityWarning,
			"Atomdown document has no version directive.", 0,
			"Run atomdown materialize, or add an atomdown version=\"1\" marker before the first atom.", lineStarts,
		))
	}

	seen := make(map[string]Position)
	slugs := make(map[string]Position)
	for _, atom := range document.Atoms {
		if atom.Implicit {
			document.Diagnostics = append(document.Diagnostics, Diagnostic{
				Code: "implicit-atom", Severity: SeverityWarning,
				Message:  "Top-level Markdown block has no persistent atom marker.",
				Position: atom.Content.Start,
				Fix:      "Insert a self-closing atom marker before this block.",
			})
			continue
		}
		lintID(document, "atom", atom.ID, atom.Marker.Start, seen)
		lintSlug(document, "atom", atom.Slug, atom.Marker.Start, slugs)
		if atom.Digest != "" && !digestPattern.MatchString(atom.Digest) {
			document.Diagnostics = append(document.Diagnostics, Diagnostic{
				Code: "invalid-digest", Severity: SeverityError,
				Message: fmt.Sprintf("atom digest %q is not a well-formed sha256 content digest.", atom.Digest), Position: atom.Marker.Start,
				Fix: "Remove the digest attribute and regenerate it with atomdown materialize --digest.",
			})
		}
	}
	for _, group := range document.Groups {
		lintID(document, "atom group", group.ID, group.Marker.Start, seen)
		lintSlug(document, "atom group", group.Slug, group.Marker.Start, slugs)
		if group.EndMarker != nil && len(group.AtomIDs) == 0 {
			document.Diagnostics = append(document.Diagnostics, Diagnostic{
				Code: "empty-group", Severity: SeverityError,
				Message: "Atom group contains no explicit atoms.", Position: group.Marker.Start,
				Fix: "Add atoms to the group or remove the group markers.",
			})
		}
	}

	lintListStructureSplits(document, listItemCounts)
}

// listMarkerPattern matches the leading bullet or ordered-list delimiter of
// a top-level list block's raw text. Two adjacent single-item lists using
// the same delimiter would be one CommonMark list if nothing sat between
// them; a captured group identifies which delimiter matched.
var listMarkerPattern = regexp.MustCompile(`^[ \t]{0,3}(?:([*+-])|[0-9]{1,9}([.)]))[ \t]`)

func listMarkerKey(text string) (string, bool) {
	match := listMarkerPattern.FindStringSubmatch(text)
	if match == nil {
		return "", false
	}
	if match[1] != "" {
		return "bullet:" + match[1], true
	}
	return "ordered:" + match[2], true
}

func isSingleItemListAtom(atom Atom, listItemCounts map[int]int) bool {
	if atom.NodeType != "List" {
		return false
	}
	count, ok := listItemCounts[atom.Content.Start.Offset]
	return ok && count == 1
}

// lintListStructureSplits finds a directive sitting between two adjacent
// list items that would otherwise be one CommonMark list. Two top-level
// atoms can only be single-item lists with nothing between them in the
// Atoms slice when something occupying its own block (an Atomdown
// directive) interrupted a single list while parsing; nothing else can
// produce that layout. materialize --split list-item produces exactly this
// layout deliberately and wraps it in one atom-group, so a run fully
// covered by one shared, non-empty group ID is not reported.
//
// This is a default-lint warning, not --strict-only and not an error. It is
// visible by default because it is a defect independent of partial
// Atomdown adoption (unlike an implicit atom): the rendered HTML silently
// changes from one <ul> to several. It stays a warning, not an error,
// because Atomdown's default lint philosophy is to stay permissive so
// mixed and partially-adopted documents keep passing; this diagnostic
// never causes lint to exit non-zero.
func lintListStructureSplits(document *Document, listItemCounts map[int]int) {
	atoms := document.Atoms
	for i := 0; i < len(atoms); {
		if !isSingleItemListAtom(atoms[i], listItemCounts) {
			i++
			continue
		}
		marker, ok := listMarkerKey(atoms[i].Text)
		if !ok {
			i++
			continue
		}

		runStart := i
		sameGroup := atoms[i].GroupID != ""
		groupID := atoms[i].GroupID
		j := i + 1
		for j < len(atoms) && isSingleItemListAtom(atoms[j], listItemCounts) {
			otherMarker, ok := listMarkerKey(atoms[j].Text)
			if !ok || otherMarker != marker {
				break
			}
			if atoms[j].GroupID == "" || atoms[j].GroupID != groupID {
				sameGroup = false
			}
			j++
		}

		if runLength := j - runStart; runLength >= 2 && !sameGroup {
			position := atoms[runStart+1].Content.Start
			if atoms[runStart+1].Marker != nil {
				position = atoms[runStart+1].Marker.Start
			}
			document.Diagnostics = append(document.Diagnostics, Diagnostic{
				Code: "directive-splits-list", Severity: SeverityWarning,
				Message: fmt.Sprintf(
					"A directive between list items splits one list into %d single-item lists, changing the document's rendered block structure.",
					runLength,
				),
				Position: position,
				Fix:      "Wrap the items in one atom-group (materialize --split list-item does this) to make the split deliberate.",
			})
		}
		i = j
	}
}

// lintSlug reports the two ways a slug can be unhelpful. Both are
// warnings, never errors, and the severity choice is the whole point of the
// rule.
//
// SPEC.md states that the slug is a readable alias and that the slug is not
// identity. Two consequences follow directly. A duplicate slug cannot be an
// error, because the format permits it and a conforming reader must accept
// a document that has one; making it an error would reject a valid
// document. And a slug outside the shape atomdown generates cannot be an
// error either, because Core puts no constraint on the value at all — an
// author is free to write "Q3 Findings" as a slug.
//
// They are still worth reporting, because the reason a slug exists is that
// a person can name one atom with it, and neither a duplicate nor an
// unpredictable spelling can do that. So:
//
//   - duplicate-slug is a default-lint warning. It is a real defect in the
//     document independent of how far Atomdown has been adopted: a selector
//     that hits it cannot resolve, and atomdown get reports it as
//     ambiguous. A reader that never uses a slug loses nothing by seeing
//     it.
//   - non-canonical-slug is a --strict-only warning. It is a style
//     preference, not a defect: the slug still resolves, uniquely, and it
//     is exactly the kind of loose value Core left room for on purpose.
//     Reporting it by default would nag every author who wrote a slug by
//     hand, which is the audience the whole feature is for.
//
// Neither ever makes lint exit non-zero, because neither is an error.
func lintSlug(document *Document, subject, slug string, position Position, slugs map[string]Position) {
	if slug == "" {
		return
	}
	if !IsCanonicalSlug(slug) {
		document.Diagnostics = append(document.Diagnostics, Diagnostic{
			Code: "non-canonical-slug", Severity: SeverityWarning,
			Message: fmt.Sprintf(
				"%s slug %q is not lowercase kebab-case within %d characters, so it is harder to type as a selector.",
				subject, slug, SlugMaxLength,
			),
			Position: position,
			Fix:      "Rewrite the slug in lowercase kebab-case, or let atomdown materialize --slugs generate one.",
		})
	}
	if first, exists := slugs[slug]; exists {
		document.Diagnostics = append(document.Diagnostics, Diagnostic{
			Code: "duplicate-slug", Severity: SeverityWarning,
			Message:  fmt.Sprintf("Slug %q was already used at line %d, so it names no single item.", slug, first.Line),
			Position: position,
			Fix:      "Give one of them a different slug. A slug is not identity, so changing it is safe.",
		})
		return
	}
	slugs[slug] = position
}

func lintID(document *Document, subject, id string, position Position, seen map[string]Position) {
	if id == "" {
		document.Diagnostics = append(document.Diagnostics, Diagnostic{
			Code: "missing-id", Severity: SeverityError,
			Message: fmt.Sprintf("%s marker requires an id attribute.", subject), Position: position,
			Fix: "Generate an ID with atomdown id.",
		})
		return
	}
	if !atomIDPattern.MatchString(id) {
		document.Diagnostics = append(document.Diagnostics, Diagnostic{
			Code: "invalid-id", Severity: SeverityError,
			Message: fmt.Sprintf("%s ID %q is not eight-character Crockford Base32.", subject, id), Position: position,
			Fix: "Generate a replacement with atomdown id.",
		})
	}
	if first, exists := seen[id]; exists {
		document.Diagnostics = append(document.Diagnostics, Diagnostic{
			Code: "duplicate-id", Severity: SeverityError,
			Message: fmt.Sprintf("ID %q was already used at line %d.", id, first.Line), Position: position,
			Fix: "Assign a new ID to the copied or duplicate item.",
		})
		return
	}
	seen[id] = position
}
