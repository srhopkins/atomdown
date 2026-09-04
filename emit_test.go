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

// TestWrappedDirectiveRoundTrip covers design requirement 5 for a directive
// that spans several source lines: both the unknown attributes and the
// author's own line breaks survive a parse and write cycle. Emit used to
// flatten the directive back to one line, byte-identical to the unwrapped
// form and with no diagnostic, so an author who wrapped a directive for
// readability lost the wrapping to any tool that round-tripped the document.
func TestWrappedDirectiveRoundTrip(t *testing.T) {
	source := []byte("<!-- <atomdown version=\"1\"/> -->\n\n<!--\n  <atom\n    id=\"4P8W2H6K\"\n    slug=\"claim\"\n    acme-approved-by=\"ada\"\n  />\n-->\n\nSome content.\n")

	stream := Tokenize(source)
	var reconstructed strings.Builder
	for _, token := range stream.Tokens {
		reconstructed.WriteString(token.Raw)
	}
	if reconstructed.String() != string(source) {
		t.Fatalf("token stream did not reconstruct the wrapped source: %q", reconstructed.String())
	}

	document := Parse(source)
	if document.HasErrors() {
		t.Fatalf("unexpected errors: %#v", document.Diagnostics)
	}
	output, err := Emit(document)
	if err != nil {
		t.Fatal(err)
	}
	want := "<!-- <atomdown version=\"1\"/> -->\n\n<!--\n  <atom\n    id=\"4P8W2H6K\"\n    slug=\"claim\"\n    acme-approved-by=\"ada\"\n  />\n-->\n\nSome content.\n\n"
	if string(output) != want {
		t.Fatalf("Emit() = %q, want %q", output, want)
	}

	// The emitted document parses back to the same model, so the cycle is
	// stable and the unknown attribute is never lost.
	second := Parse(output)
	if len(second.Atoms) != 1 || len(second.Atoms[0].Attributes) != 1 ||
		second.Atoms[0].Attributes[0].Name != "acme-approved-by" ||
		second.Atoms[0].Attributes[0].Value != "ada" {
		t.Fatalf("unknown attribute did not survive the cycle: %#v", second.Atoms)
	}

	// --flatten is the opt-in way back to the canonical one-line form.
	flattened, err := EmitWithOptions(document, EmitOptions{Flatten: true})
	if err != nil {
		t.Fatal(err)
	}
	wantFlat := "<!-- <atomdown version=\"1\"/> -->\n\n<!-- <atom id=\"4P8W2H6K\" slug=\"claim\" acme-approved-by=\"ada\"/> -->\n\nSome content.\n\n"
	if string(flattened) != wantFlat {
		t.Fatalf("EmitWithOptions(Flatten) = %q, want %q", flattened, wantFlat)
	}
}

// directiveMarkers returns every HTML comment in a document, exactly as it
// appears in the bytes. Emit normalizes the blank lines between blocks, so a
// whole-file comparison would answer a different question. These spans are
// the directive text the layout rule is about.
func directiveMarkers(source string) []string {
	var markers []string
	for cursor := 0; cursor < len(source); {
		start := strings.Index(source[cursor:], "<!--")
		if start < 0 {
			break
		}
		start += cursor
		end := strings.Index(source[start:], "-->")
		if end < 0 {
			break
		}
		end = start + end + len("-->")
		markers = append(markers, source[start:end])
		cursor = end
	}
	return markers
}

// assertDirectivesRoundTrip proves a parse and emit cycle returns every
// directive's bytes unchanged. This is the guarantee the whole layout rule
// exists to provide, so every layout case below asserts it the same way.
func assertDirectivesRoundTrip(t *testing.T, source string) string {
	t.Helper()
	document := Parse([]byte(source))
	if document.HasErrors() {
		t.Fatalf("unexpected errors: %#v", document.Diagnostics)
	}
	output, err := Emit(document)
	if err != nil {
		t.Fatal(err)
	}
	want := directiveMarkers(source)
	got := directiveMarkers(string(output))
	if len(want) == 0 {
		t.Fatalf("test source has no directive to compare")
	}
	if len(got) != len(want) {
		t.Fatalf("emitted %d directives, want %d:\n%s", len(got), len(want), output)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("directive %d = %q, want the authored bytes %q", index, got[index], want[index])
		}
	}
	return string(output)
}

// TestEmitPreservesAuthoredDirectiveLayout is the round-trip guarantee across
// every layout the parser accepts. Each case is a shape an author can write
// and expect back unchanged.
func TestEmitPreservesAuthoredDirectiveLayout(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{
			name:   "single line",
			source: "<!-- <atomdown version=\"1\"/> -->\n\n<!-- <atom id=\"4P8W2H6K\" slug=\"claim\"/> -->\n\nParagraph.\n",
		},
		{
			name:   "wrapped",
			source: "<!-- <atomdown version=\"1\"/> -->\n\n<!--\n  <atom\n    id=\"4P8W2H6K\"\n    slug=\"claim\"\n  />\n-->\n\nParagraph.\n",
		},
		{
			name:   "wrapped with CRLF line endings",
			source: "<!-- <atomdown version=\"1\"/> -->\r\n\r\n<!--\r\n  <atom\r\n    id=\"4P8W2H6K\"\r\n    slug=\"claim\"\r\n  />\r\n-->\r\n\r\nParagraph.\r\n",
		},
		{
			name:   "single line with CRLF line endings",
			source: "<!-- <atomdown version=\"1\"/> -->\r\n\r\n<!-- <atom id=\"4P8W2H6K\"/> -->\r\n\r\nParagraph.\r\n",
		},
		{
			name:   "wrapped atom-group opening marker",
			source: "<!-- <atomdown version=\"1\"/> -->\n\n<!-- <atom-group\n       id=\"7K3M9X2D\"\n       slug=\"findings\"> -->\n\n<!-- <atom id=\"4P8W2H6K\"/> -->\n\nParagraph.\n\n<!-- </atom-group> -->\n",
		},
		{
			name:   "wrapped directive at the very start of the file",
			source: "<!--\n  <atom\n    id=\"4P8W2H6K\"\n  />\n-->\n\nParagraph.\n",
		},
		{
			name:   "wrapped group close at the very end of the file",
			source: "<!-- <atom-group id=\"7K3M9X2D\"> -->\n\n<!-- <atom id=\"4P8W2H6K\"/> -->\n\nParagraph.\n\n<!--\n  </atom-group>\n-->\n",
		},
		{
			name:   "wrapped directive between two list items",
			source: "- one\n\n<!-- <atom\n       id=\"4P8W2H6K\"/> -->\n\n- two\n",
		},
		{
			name:   "wrapped directive inside a tight list",
			source: "- one\n<!-- <atom\n     id=\"4P8W2H6K\"/> -->\n- two\n",
		},
		{
			// The closing token is found by decoding the element, not by
			// searching for "/>", so a value that contains one cannot move
			// the split point.
			name:   "wrapped with a literal closing token inside a value",
			source: "<!--\n  <atom\n    id=\"4P8W2H6K\"\n    acme-note=\"a/>b\"\n  />\n-->\n\nParagraph.\n",
		},
		{
			name:   "wrapped with a blank line inside the comment",
			source: "<!--\n\n  <atom id=\"4P8W2H6K\"/>\n\n-->\n\nParagraph.\n",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assertDirectivesRoundTrip(t, testCase.source)
		})
	}
}

// TestEmitFlattenCanonicalizesEveryDirective proves --flatten is the opt-in
// way back to one line per directive, for every wrapped shape.
func TestEmitFlattenCanonicalizesEveryDirective(t *testing.T) {
	source := []byte("<!--\n  <atomdown version=\"1\"/>\n-->\n\n<!-- <atom-group\n       id=\"7K3M9X2D\"> -->\n\n<!--\n  <atom\n    id=\"4P8W2H6K\"\n    slug=\"claim\"\n  />\n-->\n\nParagraph.\n\n<!--\n  </atom-group>\n-->\n")
	document := Parse(source)
	if document.HasErrors() {
		t.Fatalf("unexpected errors: %#v", document.Diagnostics)
	}
	output, err := EmitWithOptions(document, EmitOptions{Flatten: true})
	if err != nil {
		t.Fatal(err)
	}
	want := "<!-- <atomdown version=\"1\"/> -->\n\n<!-- <atom-group id=\"7K3M9X2D\"> -->\n\n<!-- <atom id=\"4P8W2H6K\" slug=\"claim\"/> -->\n\nParagraph.\n\n<!-- </atom-group> -->\n"
	if string(output) != want {
		t.Fatalf("EmitWithOptions(Flatten) = %q,\nwant %q", output, want)
	}
	for _, marker := range directiveMarkers(string(output)) {
		if strings.Contains(marker, "\n") {
			t.Fatalf("--flatten left a directive spanning lines: %q", marker)
		}
	}
}

// TestEmitReordersNothingWhenAttributesAreUnchanged proves the unchanged test
// is about the attribute set, not its order. A writer that reordered an
// untouched directive into canonical order would be the same silent reflow
// the layout rule exists to prevent.
func TestEmitReordersNothingWhenAttributesAreUnchanged(t *testing.T) {
	source := "<!-- <atom acme-owner=\"ada\" slug=\"claim\" id=\"4P8W2H6K\"/> -->\n\nParagraph.\n"
	assertDirectivesRoundTrip(t, source)
}

// TestEmitKeepsWrappingWhenAnAttributeChanges covers the modified-directive
// rule. There is no authored text for an attribute set the author never
// wrote, so the authored skeleton is kept and only the attribute sequence is
// rebuilt: the directive stays wrapped, at the author's indentation, with the
// closing token on its own line.
func TestEmitKeepsWrappingWhenAnAttributeChanges(t *testing.T) {
	source := []byte("<!--\n  <atom\n    id=\"4P8W2H6K\"\n    acme-owner=\"ada\"\n  />\n-->\n\nParagraph.\n")
	document := Parse(source)
	if document.HasErrors() {
		t.Fatalf("unexpected errors: %#v", document.Diagnostics)
	}

	added := document
	added.Atoms = append([]Atom(nil), document.Atoms...)
	added.Atoms[0].Attributes = []Attribute{{Name: "acme-owner", Value: "ada"}, {Name: "acme-status", Value: "approved"}}
	output, err := Emit(added)
	if err != nil {
		t.Fatal(err)
	}
	want := "<!--\n  <atom\n    id=\"4P8W2H6K\"\n    acme-owner=\"ada\"\n    acme-status=\"approved\"\n  />\n-->\n\nParagraph.\n\n"
	if string(output) != want {
		t.Fatalf("added attribute: Emit() = %q,\nwant %q", output, want)
	}

	changed := document
	changed.Atoms = append([]Atom(nil), document.Atoms...)
	changed.Atoms[0].Attributes = []Attribute{{Name: "acme-owner", Value: "grace"}}
	output, err = Emit(changed)
	if err != nil {
		t.Fatal(err)
	}
	want = "<!--\n  <atom\n    id=\"4P8W2H6K\"\n    acme-owner=\"grace\"\n  />\n-->\n\nParagraph.\n\n"
	if string(output) != want {
		t.Fatalf("changed value: Emit() = %q,\nwant %q", output, want)
	}

	removed := document
	removed.Atoms = append([]Atom(nil), document.Atoms...)
	removed.Atoms[0].Attributes = nil
	output, err = Emit(removed)
	if err != nil {
		t.Fatal(err)
	}
	want = "<!--\n  <atom\n    id=\"4P8W2H6K\"\n  />\n-->\n\nParagraph.\n\n"
	if string(output) != want {
		t.Fatalf("removed attribute: Emit() = %q,\nwant %q", output, want)
	}
}

// TestEmitKeepsAModifiedOneLineDirectiveOnOneLine is the other half of the
// modified-directive rule: a shape the author did not use is never invented.
func TestEmitKeepsAModifiedOneLineDirectiveOnOneLine(t *testing.T) {
	document := Parse([]byte("<!-- <atom id=\"4P8W2H6K\"/> -->\n\nParagraph.\n"))
	document.Atoms[0].Attributes = []Attribute{{Name: "acme-status", Value: "approved"}}
	output, err := Emit(document)
	if err != nil {
		t.Fatal(err)
	}
	want := "<!-- <atom id=\"4P8W2H6K\" acme-status=\"approved\"/> -->\n\nParagraph.\n\n"
	if string(output) != want {
		t.Fatalf("Emit() = %q,\nwant %q", output, want)
	}
}

// TestEmitKeepsWrappedIndentationWhenASlugArrives proves the rule covers a
// Core identity attribute too, not only an extension attribute.
func TestEmitKeepsWrappedIndentationWhenASlugArrives(t *testing.T) {
	document := Parse([]byte("<!--\n\t<atom\n\t\tid=\"4P8W2H6K\"\n\t/>\n-->\n\nParagraph.\n"))
	if document.HasErrors() {
		t.Fatalf("unexpected errors: %#v", document.Diagnostics)
	}
	document.Atoms[0].Slug = "claim"
	output, err := Emit(document)
	if err != nil {
		t.Fatal(err)
	}
	want := "<!--\n\t<atom\n\t\tid=\"4P8W2H6K\"\n\t\tslug=\"claim\"\n\t/>\n-->\n\nParagraph.\n\n"
	if string(output) != want {
		t.Fatalf("Emit() = %q,\nwant %q", output, want)
	}
}

// TestEmitFallsBackToOneLineForUnreadableMarkerSource proves a hand-built
// model cannot lose its identity attributes through the layout path. A caller
// can put anything in MarkerSource; a directive that cannot be read describes
// no shape to keep.
func TestEmitFallsBackToOneLineForUnreadableMarkerSource(t *testing.T) {
	output, err := Emit(Document{Atoms: []Atom{{
		ID:           "4P8W2H6K",
		MarkerSource: "not a directive at all",
		Text:         "Paragraph.",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	want := "<!-- <atom id=\"4P8W2H6K\"/> -->\n\nParagraph.\n\n"
	if string(output) != want {
		t.Fatalf("Emit() = %q,\nwant %q", output, want)
	}
}
