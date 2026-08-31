package atomdown

// DriftedAtom identifies one atom whose recorded content digest no longer
// matches its current block bytes.
type DriftedAtom struct {
	ID      string `json:"id"`
	Slug    string `json:"slug,omitempty"`
	Digest  string `json:"digest"`
	Current string `json:"current"`
}

// Drift parses source and reports every atom whose recorded Core content
// digest (written by a previous materialize --digest) no longer matches
// its current block content. It answers only "which atoms drifted"; it
// does not show what changed inside them; see SPEC.md "Content digest".
//
// An atom with no digest at all has no baseline to check against and is
// silently skipped: digests are opt-in, so an undigested atom is not
// drift, it is simply unmonitored.
func Drift(source []byte) []DriftedAtom {
	document := Parse(source)
	var drifted []DriftedAtom
	for _, atom := range document.Atoms {
		if atom.Implicit || atom.Digest == "" {
			continue
		}
		current := ContentDigest(atom.Text)
		if current != atom.Digest {
			drifted = append(drifted, DriftedAtom{ID: atom.ID, Slug: atom.Slug, Digest: atom.Digest, Current: current})
		}
	}
	return drifted
}
