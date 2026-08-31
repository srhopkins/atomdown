package atomdown

import "fmt"

func lintDocument(document *Document, lineStarts []int) {
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
	}
	for _, group := range document.Groups {
		lintID(document, "atom group", group.ID, group.Marker.Start, seen)
		if group.EndMarker != nil && len(group.AtomIDs) == 0 {
			document.Diagnostics = append(document.Diagnostics, Diagnostic{
				Code: "empty-group", Severity: SeverityError,
				Message: "Atom group contains no explicit atoms.", Position: group.Marker.Start,
				Fix: "Add atoms to the group or remove the group markers.",
			})
		}
	}
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
