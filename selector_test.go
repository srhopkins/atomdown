package atomdown

import (
	"errors"
	"strings"
	"testing"
)

const selectorFixture = `<!-- <atomdown version="1"/> -->

<!-- <atom-group id="3G7K9R5V" slug="findings"> -->

<!-- <atom id="4P8W2H6K" slug="first-claim"/> -->

First claim.

<!-- <atom id="9R3C7M5D" slug="second-claim"/> -->

Second claim.

<!-- </atom-group> -->

<!-- <atom id="5H8M2W6Y" slug="loose-claim"/> -->

Loose claim.
`

func TestResolveByIDAndSlug(t *testing.T) {
	document := Parse([]byte(selectorFixture))
	for _, testCase := range []struct {
		name      string
		selector  string
		wantID    string
		matchedBy string
	}{
		{name: "id", selector: "9R3C7M5D", wantID: "9R3C7M5D", matchedBy: "id"},
		{name: "atom slug", selector: "second-claim", wantID: "9R3C7M5D", matchedBy: "slug"},
		{name: "prefixed slug", selector: "slug:second-claim", wantID: "9R3C7M5D", matchedBy: "slug"},
		{name: "ungrouped atom", selector: "loose-claim", wantID: "5H8M2W6Y", matchedBy: "slug"},
		// A group slug resolves to the group's first atom, so naming a
		// section gets a caller to its head rather than to nothing.
		{name: "group slug", selector: "findings", wantID: "4P8W2H6K", matchedBy: "slug"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			selection, err := Resolve(document, testCase.selector)
			if err != nil {
				t.Fatal(err)
			}
			if selection.Atom.ID != testCase.wantID {
				t.Fatalf("resolved ID = %q, want %q", selection.Atom.ID, testCase.wantID)
			}
			if selection.MatchedBy != testCase.matchedBy {
				t.Fatalf("matched by %q, want %q", selection.MatchedBy, testCase.matchedBy)
			}
		})
	}
}

// TestResolvePrefersAnIDOverASlug is the precedence rule. An ID is identity
// and a slug is not, so a bare selector that is both must resolve to the
// atom whose ID it is. The slug: prefix is the escape hatch.
func TestResolvePrefersAnIDOverASlug(t *testing.T) {
	source := []byte(`<!-- <atomdown version="1"/> -->

<!-- <atom id="4P8W2H6K" slug="collides"/> -->

The atom whose ID is the contested value.

<!-- <atom id="9R3C7M5D" slug="4P8W2H6K"/> -->

The atom whose slug is the contested value.
`)
	document := Parse(source)

	byID, err := Resolve(document, "4P8W2H6K")
	if err != nil {
		t.Fatal(err)
	}
	if byID.Atom.ID != "4P8W2H6K" || byID.MatchedBy != "id" {
		t.Fatalf("bare selector resolved to %q by %q, want 4P8W2H6K by id", byID.Atom.ID, byID.MatchedBy)
	}

	bySlug, err := Resolve(document, "slug:4P8W2H6K")
	if err != nil {
		t.Fatal(err)
	}
	if bySlug.Atom.ID != "9R3C7M5D" || bySlug.MatchedBy != "slug" {
		t.Fatalf("slug selector resolved to %q by %q, want 9R3C7M5D by slug", bySlug.Atom.ID, bySlug.MatchedBy)
	}
}

// TestResolveRefusesAnAmbiguousSlug proves nothing is ever picked silently.
// A duplicate slug is a valid document, so the resolver has to handle it,
// and the only answer that lets a caller continue is the list of candidate
// IDs.
func TestResolveRefusesAnAmbiguousSlug(t *testing.T) {
	source := []byte(`<!-- <atomdown version="1"/> -->

<!-- <atom id="4P8W2H6K" slug="findings"/> -->

First.

<!-- <atom id="9R3C7M5D" slug="findings"/> -->

Second.
`)
	_, err := Resolve(Parse(source), "findings")
	var ambiguous *AmbiguousSelectorError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("error = %v, want an AmbiguousSelectorError", err)
	}
	if len(ambiguous.AtomIDs) != 2 {
		t.Fatalf("candidate IDs = %v, want two", ambiguous.AtomIDs)
	}
	for _, id := range []string{"4P8W2H6K", "9R3C7M5D"} {
		if !strings.Contains(err.Error(), id) {
			t.Fatalf("message %q does not name %q", err.Error(), id)
		}
	}
}

// TestResolveAtomSlugBeatsAGroupSlug proves an atom's own slug is the
// closer match, so a group is only consulted when no atom carries the name.
func TestResolveAtomSlugBeatsAGroupSlug(t *testing.T) {
	source := []byte(`<!-- <atomdown version="1"/> -->

<!-- <atom-group id="3G7K9R5V" slug="findings"> -->

<!-- <atom id="4P8W2H6K"/> -->

Group head.

<!-- <atom id="9R3C7M5D" slug="findings"/> -->

The atom that carries the name.

<!-- </atom-group> -->
`)
	selection, err := Resolve(Parse(source), "findings")
	if err != nil {
		t.Fatal(err)
	}
	if selection.Atom.ID != "9R3C7M5D" {
		t.Fatalf("resolved %q, want the atom 9R3C7M5D rather than the group head", selection.Atom.ID)
	}
}

func TestResolveRejectsUnknownAndEmptySelectors(t *testing.T) {
	document := Parse([]byte(selectorFixture))

	_, err := Resolve(document, "no-such-name")
	var unknown *UnknownSelectorError
	if !errors.As(err, &unknown) {
		t.Fatalf("error = %v, want an UnknownSelectorError", err)
	}
	// An implicit atom has no name of any kind, so a selector cannot reach
	// one; only an unmarked block is implicit and it carries no ID or slug.
	if _, err := Resolve(document, ""); err == nil {
		t.Fatal("an empty selector resolved")
	}
	if _, err := Resolve(document, "slug:"); err == nil {
		t.Fatal("a bare slug prefix resolved")
	}
}

// TestResolveIgnoresAnImplicitAtom keeps the resolver from returning a
// block that has no persistent name, which nothing could address a second
// time.
func TestResolveIgnoresAnImplicitAtom(t *testing.T) {
	document := Parse([]byte("Unmarked paragraph.\n\n<!-- <atom id=\"4P8W2H6K\" slug=\"marked\"/> -->\n\nMarked paragraph.\n"))
	selection, err := Resolve(document, "marked")
	if err != nil {
		t.Fatal(err)
	}
	if selection.Atom.ID != "4P8W2H6K" {
		t.Fatalf("resolved %q, want 4P8W2H6K", selection.Atom.ID)
	}
	if _, err := Resolve(document, ""); err == nil {
		t.Fatal("an empty selector resolved")
	}
}

// TestResolveChangesNothing pins the read-only contract, which a later
// move or group command depends on.
func TestResolveChangesNothing(t *testing.T) {
	source := []byte(selectorFixture)
	document := Parse(source)
	if _, err := Resolve(document, "findings"); err != nil {
		t.Fatal(err)
	}
	if string(source) != selectorFixture {
		t.Fatal("Resolve altered the source bytes")
	}
	if len(document.Atoms) != 3 {
		t.Fatalf("atom count = %d, want 3", len(document.Atoms))
	}
}
