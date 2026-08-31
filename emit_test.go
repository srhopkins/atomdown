package atomdown

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

func TestEmitCanonicalDocument(t *testing.T) {
	document := Document{
		Declared:   true,
		Version:    "1",
		Attributes: []Attribute{{Name: "acme-profile", Value: "draft"}},
		Atoms: []Atom{
			{
				ID:         "4P8W2H6K",
				Attributes: []Attribute{{Name: "acme-status", Value: "approved"}, {Name: "acme-owner", Value: "research"}},
				Text:       "The product launched in March.",
				GroupID:    "7K3M9X2D",
			},
			{
				ID:         "9R3C7M5D",
				Attributes: []Attribute{{Name: "acme-status", Value: "needs-source"}},
				Text:       "The regional rollout continued through April.",
				GroupID:    "7K3M9X2D",
			},
		},
		Groups: []AtomGroup{{
			ID:      "7K3M9X2D",
			Slug:    "research-findings",
			AtomIDs: []string{"4P8W2H6K", "9R3C7M5D"},
		}},
	}

	output, err := Emit(document)
	if err != nil {
		t.Fatal(err)
	}
	const expected = `<!-- <atomdown version="1" acme-profile="draft"/> -->

<!-- <atom-group id="7K3M9X2D" slug="research-findings"> -->

<!-- <atom id="4P8W2H6K" acme-status="approved" acme-owner="research"/> -->

The product launched in March.

<!-- <atom id="9R3C7M5D" acme-status="needs-source"/> -->

The regional rollout continued through April.

<!-- </atom-group> -->
`
	if string(output) != expected {
		t.Fatalf("unexpected output:\n%s", output)
	}
	if reparsed := Parse(output); reparsed.HasErrors() {
		t.Fatalf("emitted document has errors: %+v", reparsed.Diagnostics)
	}
}

func TestEmitImplicitAtomHasNoMarker(t *testing.T) {
	output, err := Emit(Document{Atoms: []Atom{{Text: "# Unmarked", Implicit: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(output), "<atom ") {
		t.Fatalf("implicit atom received a marker:\n%s", output)
	}
	if string(output) != "# Unmarked\n\n" {
		t.Fatalf("unexpected output %q", output)
	}
}

func TestEmitGeneratesMissingIDs(t *testing.T) {
	document := Document{
		Atoms: []Atom{{Text: "Needs an ID."}},
	}

	output, err := Emit(document)
	if err != nil {
		t.Fatal(err)
	}
	atomPattern := regexp.MustCompile(`<!-- <atom id="[0-9A-HJKMNP-TV-Z]{8}"/> -->`)
	if !atomPattern.Match(output) {
		t.Fatalf("missing generated atom ID:\n%s", output)
	}
}

func TestEmitUsesAtomIDsForGeneratedGroupID(t *testing.T) {
	document := Document{
		Atoms:  []Atom{{ID: "4P8W2H6K", Text: "Grouped."}},
		Groups: []AtomGroup{{AtomIDs: []string{"4P8W2H6K"}}},
	}
	output, err := Emit(document)
	if err != nil {
		t.Fatal(err)
	}
	groupPattern := regexp.MustCompile(`<!-- <atom-group id="[0-9A-HJKMNP-TV-Z]{8}"> -->`)
	if !groupPattern.Match(output) {
		t.Fatalf("missing generated group ID:\n%s", output)
	}
	if !strings.Contains(string(output), "<!-- </atom-group> -->") {
		t.Fatalf("missing group close:\n%s", output)
	}
}

func TestEmitPreservesSlugAndEscapesAttributes(t *testing.T) {
	document := Document{Atoms: []Atom{{
		ID:         "4P8W2H6K",
		Slug:       "claim",
		Attributes: []Attribute{{Name: "acme-note", Value: `A&B "quoted"`}},
		Text:       "Text.",
	}}}
	output, err := Emit(document)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), `slug="claim" acme-note="A&amp;B &#34;quoted&#34;"`) {
		t.Fatalf("identity fields were not preserved:\n%s", output)
	}
	reparsed := Parse(output)
	if reparsed.HasErrors() || reparsed.Atoms[0].Attributes[0].Value != `A&B "quoted"` {
		t.Fatalf("attribute did not round trip: %+v", reparsed)
	}
}

func TestEmitRejectsDiscontiguousGroup(t *testing.T) {
	document := Document{
		Atoms: []Atom{
			{ID: "4P8W2H6K", Text: "First.", GroupID: "7K3M9X2D"},
			{ID: "9R3C7M5D", Text: "Middle."},
			{ID: "2D6K4P8W", Text: "Last.", GroupID: "7K3M9X2D"},
		},
		Groups: []AtomGroup{{ID: "7K3M9X2D"}},
	}
	if _, err := Emit(document); err == nil {
		t.Fatal("expected discontiguous group error")
	}
}

// TestEmitRegeneratesIDOnCollision proves Emit mints IDs through the checked
// path rather than calling NewID directly. Emit previously bypassed the
// uniqueness check, so a regression here is silent: real 40-bit IDs never
// collide inside one test run, and every other emit test would still pass.
func TestEmitRegeneratesIDOnCollision(t *testing.T) {
	original := generateID
	defer func() { generateID = original }()

	// Hand out the same ID twice, then a distinct one. An unchecked Emit
	// assigns the duplicate to both atoms; a checked Emit skips it.
	queue := []string{"AAAAAAA1", "AAAAAAA1", "BBBBBBB2"}
	index := 0
	generateID = func() (string, error) {
		if index >= len(queue) {
			return "", errors.New("test generator exhausted")
		}
		id := queue[index]
		index++
		return id, nil
	}

	document := Document{Atoms: []Atom{{Text: "First."}, {Text: "Second."}}}
	output, err := Emit(document)
	if err != nil {
		t.Fatal(err)
	}

	ids := regexp.MustCompile(`<atom id="([0-9A-HJKMNP-TV-Z]{8})"`).FindAllStringSubmatch(string(output), -1)
	if len(ids) != 2 {
		t.Fatalf("expected 2 atom ids, got %d:\n%s", len(ids), output)
	}
	if ids[0][1] == ids[1][1] {
		t.Fatalf("Emit assigned the same id twice (%s); it is not using the collision-checked path:\n%s", ids[0][1], output)
	}
}

// TestEmitErrorsWhenCollisionRetriesAreExhausted proves Emit surfaces a clear
// error rather than looping forever when every generated ID collides.
func TestEmitErrorsWhenCollisionRetriesAreExhausted(t *testing.T) {
	original := generateID
	defer func() { generateID = original }()
	generateID = func() (string, error) { return "AAAAAAA1", nil }

	document := Document{Atoms: []Atom{{Text: "First."}, {Text: "Second."}}}
	if _, err := Emit(document); err == nil {
		t.Fatal("expected an error when every generated id collides")
	}
}
