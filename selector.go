package atomdown

import (
	"fmt"
	"strings"
)

// SlugSelectorPrefix forces a selector to be read as a slug. Without it, a
// selector is tried as an ID first.
const SlugSelectorPrefix = "slug:"

// Selection is one resolved atom and the selector that found it.
type Selection struct {
	// Atom is the atom the selector names.
	Atom Atom
	// GroupID is the atom group the atom belongs to, empty when it belongs
	// to none.
	GroupID string
	// MatchedBy is "id" or "slug": which kind of name resolved. A caller
	// that rewrites a document can record the resolved ID instead of the
	// slug it was given, because only the ID is identity.
	MatchedBy string
}

// AmbiguousSelectorError reports a slug that names more than one atom.
//
// Core permits a duplicate slug: SPEC.md says the slug is a readable alias
// and not identity, so a reader must accept a document that has two of
// them. A resolver therefore cannot treat uniqueness as given, and it must
// not pick one silently either — the whole value of naming an atom by slug
// is that the person knows which atom they named. So it fails and names
// every candidate ID, which is the one answer that lets the caller
// continue with an ID.
type AmbiguousSelectorError struct {
	Selector string
	AtomIDs  []string
}

func (e *AmbiguousSelectorError) Error() string {
	return fmt.Sprintf("slug %q names %d atoms: %s; select one by ID", e.Selector, len(e.AtomIDs), strings.Join(e.AtomIDs, ", "))
}

// UnknownSelectorError reports a selector that names nothing in the
// document.
type UnknownSelectorError struct{ Selector string }

func (e *UnknownSelectorError) Error() string {
	return fmt.Sprintf("no atom matches %q", e.Selector)
}

// Resolve finds the one atom a selector names.
//
// A selector is an atom ID, an atom or atom-group slug, or a slug with the
// "slug:" prefix. Precedence is fixed:
//
//  1. A bare selector is matched against atom IDs first. An ID is identity
//     and a slug is not, so an ID always wins. A slug that happens to look
//     like an ID can still be reached with the "slug:" prefix.
//  2. A bare selector that matches no ID is matched against slugs: first
//     an atom's own slug, then an atom-group's slug, which resolves to the
//     group's first atom.
//  3. A "slug:" selector skips step 1 entirely.
//
// A slug matching several atoms returns *AmbiguousSelectorError naming
// every candidate ID; nothing is ever picked silently. A selector matching
// nothing returns *UnknownSelectorError.
//
// Resolve reads the document and changes nothing. A later command that
// moves or regroups an atom resolves its target through this one function,
// so every command accepts the same selector spellings.
func Resolve(document Document, selector string) (Selection, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return Selection{}, fmt.Errorf("selector is empty")
	}

	slugOnly := strings.HasPrefix(selector, SlugSelectorPrefix)
	name := strings.TrimPrefix(selector, SlugSelectorPrefix)
	if name == "" {
		return Selection{}, fmt.Errorf("selector %q names no slug", selector)
	}

	if !slugOnly {
		for _, atom := range document.Atoms {
			if !atom.Implicit && atom.ID == name {
				return Selection{Atom: atom, GroupID: atom.GroupID, MatchedBy: "id"}, nil
			}
		}
	}

	var matches []Atom
	for _, atom := range document.Atoms {
		if !atom.Implicit && atom.Slug == name {
			matches = append(matches, atom)
		}
	}
	// A group slug resolves to the group's first atom, so naming a section
	// gets a caller to its head rather than to nothing. An atom's own slug
	// is a closer match than the group's, so a group is only consulted when
	// no atom carries the slug.
	if len(matches) == 0 {
		for _, group := range document.Groups {
			if group.Slug != name {
				continue
			}
			if atom, exists := firstAtomOfGroup(document, group.ID); exists {
				matches = append(matches, atom)
			}
		}
	}

	switch len(matches) {
	case 0:
		return Selection{}, &UnknownSelectorError{Selector: selector}
	case 1:
		return Selection{Atom: matches[0], GroupID: matches[0].GroupID, MatchedBy: "slug"}, nil
	default:
		ids := make([]string, 0, len(matches))
		for _, atom := range matches {
			ids = append(ids, atom.ID)
		}
		return Selection{}, &AmbiguousSelectorError{Selector: selector, AtomIDs: ids}
	}
}

func firstAtomOfGroup(document Document, groupID string) (Atom, bool) {
	for _, atom := range document.Atoms {
		if !atom.Implicit && atom.GroupID == groupID {
			return atom, true
		}
	}
	return Atom{}, false
}
