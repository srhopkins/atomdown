package atomdown

import (
	"context"
	"strings"
	"testing"
)

func TestParseAtomsAndGroup(t *testing.T) {
	source := []byte(`<!-- <atomdown version="1" acme-profile="draft"/> -->

<!-- <atom-group id="7K3M9X2D" slug="findings"> -->

<!-- <atom id="4P8W2H6K" acme-approved-by="reviewer"/> -->

First finding.

<!-- <atom id="9R3C7M5D"/> -->

- Second finding.
- Supporting detail.

<!-- </atom-group> -->
`)
	document := Parse(source)
	if document.HasErrors() {
		t.Fatalf("unexpected errors: %#v", document.Diagnostics)
	}
	if document.Version != "1" {
		t.Fatalf("version = %q", document.Version)
	}
	if len(document.Atoms) != 2 {
		t.Fatalf("atoms = %d, want 2", len(document.Atoms))
	}
	if len(document.Groups) != 1 || len(document.Groups[0].AtomIDs) != 2 {
		t.Fatalf("groups = %#v", document.Groups)
	}
	if document.Atoms[0].GroupID != "7K3M9X2D" {
		t.Fatalf("group = %q", document.Atoms[0].GroupID)
	}
	if got := document.Atoms[0].Attributes[0]; got.Name != "acme-approved-by" || got.Value != "reviewer" {
		t.Fatalf("attribute = %#v", got)
	}
}

func TestImplicitAtomWarning(t *testing.T) {
	document := Parse([]byte("Unmarked paragraph.\n"))
	if len(document.Atoms) != 1 || !document.Atoms[0].Implicit {
		t.Fatalf("atoms = %#v", document.Atoms)
	}
	// A source with no document marker also carries missing-version-directive.
	if len(document.Diagnostics) != 2 || document.Diagnostics[0].Code != "missing-version-directive" || document.Diagnostics[1].Code != "implicit-atom" {
		t.Fatalf("diagnostics = %#v", document.Diagnostics)
	}
}

func TestMissingVersionDirectiveWarning(t *testing.T) {
	document := Parse([]byte("<!-- <atom id=\"4P8W2H6K\"/> -->\n\nFirst.\n"))
	found := false
	for _, diagnostic := range document.Diagnostics {
		if diagnostic.Code == "missing-version-directive" {
			found = true
			if diagnostic.Severity != SeverityWarning {
				t.Fatalf("severity = %q, want warning", diagnostic.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("diagnostics = %#v, want missing-version-directive", document.Diagnostics)
	}
	if document.HasErrors() {
		t.Fatalf("missing-version-directive must be a warning, not an error: %#v", document.Diagnostics)
	}
}

func TestDeclaredDocumentHasNoMissingVersionDirectiveWarning(t *testing.T) {
	document := Parse([]byte("<!-- <atomdown version=\"1\"/> -->\n\n<!-- <atom id=\"4P8W2H6K\"/> -->\n\nFirst.\n"))
	for _, diagnostic := range document.Diagnostics {
		if diagnostic.Code == "missing-version-directive" {
			t.Fatalf("declared document should not warn: %#v", document.Diagnostics)
		}
	}
}

func TestDuplicateID(t *testing.T) {
	source := []byte(`<!-- <atom id="4P8W2H6K"/> -->

First.

<!-- <atom id="4P8W2H6K"/> -->

Second.
`)
	document := Parse(source)
	if !document.HasErrors() {
		t.Fatal("expected duplicate ID error")
	}
	found := false
	for _, diagnostic := range document.Diagnostics {
		if diagnostic.Code == "duplicate-id" {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %#v", document.Diagnostics)
	}
}

func TestInlineDirectiveIsRejected(t *testing.T) {
	document := Parse([]byte("Before <!-- <atom id=\"4P8W2H6K\"/> --> after.\n"))
	if !document.HasErrors() {
		t.Fatal("expected inline directive error")
	}
	found := false
	for _, diagnostic := range document.Diagnostics {
		if diagnostic.Code == "inline-directive" {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %#v", document.Diagnostics)
	}
}

func TestNormalizedXML(t *testing.T) {
	source := []byte(`<!-- <atom id="4P8W2H6K" acme-status="approved"/> -->

First.
`)
	document := Parse(source)
	normalized, err := NormalizedXML(document)
	if err != nil {
		t.Fatal(err)
	}
	value := string(normalized)
	for _, expected := range []string{`<atomdown version="1">`, `id="4P8W2H6K"`, `acme-status="approved"`} {
		if !strings.Contains(value, expected) {
			t.Fatalf("normalized XML missing %q:\n%s", expected, value)
		}
	}
}

func TestMisplacedCoreAttributesArePreserved(t *testing.T) {
	source := []byte(`<!-- <atomdown version="1" id="document-id" slug="document-slug"/> -->

<!-- <atom id="4P8W2H6K" version="2"/> -->

First.

<!-- <atom-group id="7K3M9X2D" version="3"> -->
<!-- <atom id="9R3C7M5D"/> -->
Second.
<!-- </atom-group> -->
`)
	document := Parse(source)
	if document.HasErrors() {
		t.Fatalf("unexpected errors: %#v", document.Diagnostics)
	}
	if got := document.Attributes; len(got) != 2 || got[0].Name != "id" || got[1].Name != "slug" {
		t.Fatalf("document attributes = %#v", got)
	}
	if got := document.Atoms[0].Attributes; len(got) != 1 || got[0].Name != "version" || got[0].Value != "2" {
		t.Fatalf("atom attributes = %#v", got)
	}
	if got := document.Groups[0].Attributes; len(got) != 1 || got[0].Name != "version" || got[0].Value != "3" {
		t.Fatalf("group attributes = %#v", got)
	}

	stream := Tokenize(source)
	if got := stream.Tokens[0].Directive.Attributes; len(got) != 2 || got[0].Name != "id" || got[1].Name != "slug" {
		t.Fatalf("document token attributes = %#v", got)
	}
	for _, token := range stream.Tokens {
		if token.Directive != nil && token.Directive.Element == "atom" && token.Directive.ID == "4P8W2H6K" {
			if got := token.Directive.Attributes; len(got) != 1 || got[0].Name != "version" {
				t.Fatalf("atom token attributes = %#v", got)
			}
		}
	}

	normalized, err := NormalizedXML(document)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`id="document-id"`, `slug="document-slug"`, `version="2"`, `version="3"`} {
		if !strings.Contains(string(normalized), expected) {
			t.Fatalf("normalized XML missing %q:\n%s", expected, normalized)
		}
	}
}

func TestNewID(t *testing.T) {
	id, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	if !atomIDPattern.MatchString(id) {
		t.Fatalf("invalid ID %q", id)
	}
}

func TestTokenizeIsLossless(t *testing.T) {
	source := []byte(`<!-- <atom id="4P8W2H6K" acme-status="approved"/> -->

# Heading

Paragraph.
`)
	stream := Tokenize(source)
	var reconstructed strings.Builder
	foundDirective := false
	foundHeading := false
	for _, token := range stream.Tokens {
		reconstructed.WriteString(token.Raw)
		if token.Kind == TokenDirective && token.Directive.Element == "atom" {
			foundDirective = true
		}
		if token.Kind == TokenMarkdown && token.NodeType == "Heading" {
			foundHeading = true
		}
	}
	if reconstructed.String() != string(source) {
		t.Fatalf("token stream is not lossless:\n%s", reconstructed.String())
	}
	if !foundDirective || !foundHeading {
		t.Fatalf("directive=%v heading=%v tokens=%#v", foundDirective, foundHeading, stream.Tokens)
	}
}

func TestAtomContentIncludesMarkdownBlockMarkers(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		nodeType string
	}{
		{name: "heading", markdown: "## Evidence", nodeType: "Heading"},
		{name: "list", markdown: "- First\n- Second", nodeType: "List"},
		{name: "blockquote", markdown: "> Quoted", nodeType: "Blockquote"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := []byte("<!-- <atom id=\"4P8W2H6K\"/> -->\n\n" + test.markdown + "\n")
			document := Parse(source)
			if document.HasErrors() {
				t.Fatalf("unexpected errors: %#v", document.Diagnostics)
			}
			if len(document.Atoms) != 1 {
				t.Fatalf("atoms = %#v", document.Atoms)
			}
			atom := document.Atoms[0]
			if atom.Text != test.markdown {
				t.Fatalf("Text = %q, want %q", atom.Text, test.markdown)
			}
			if atom.NodeType != test.nodeType {
				t.Fatalf("NodeType = %q, want %q", atom.NodeType, test.nodeType)
			}
			if got := string(source[atom.Content.Start.Offset:atom.Content.End.Offset]); got != test.markdown {
				t.Fatalf("Content = %q, want %q", got, test.markdown)
			}

			stream := Tokenize(source)
			var reconstructed strings.Builder
			foundBlock := false
			for _, token := range stream.Tokens {
				reconstructed.WriteString(token.Raw)
				if token.Kind == TokenMarkdown && token.NodeType == test.nodeType {
					foundBlock = true
					if token.Raw != test.markdown {
						t.Fatalf("token Raw = %q, want %q", token.Raw, test.markdown)
					}
				}
			}
			if reconstructed.String() != string(source) {
				t.Fatalf("token stream is not lossless:\n%s", reconstructed.String())
			}
			if !foundBlock {
				t.Fatalf("missing %s token: %#v", test.nodeType, stream.Tokens)
			}
		})
	}
}

func TestStripPreservesMarkdown(t *testing.T) {
	source := []byte("<!-- <atom id=\"4P8W2H6K\"/> -->\n\nParagraph.\n\n<!-- ordinary comment -->\n")
	stripped := string(Strip(source))
	if strings.Contains(stripped, "4P8W2H6K") {
		t.Fatalf("Atomdown marker remains: %s", stripped)
	}
	if !strings.Contains(stripped, "Paragraph.") || !strings.Contains(stripped, "ordinary comment") {
		t.Fatalf("Markdown was not preserved: %s", stripped)
	}
}

func TestStripRemovesWholeDirectiveLine(t *testing.T) {
	// Both directives sit at the top of the document with only blank-line
	// scaffolding between and after them, so no blank line survives: there
	// is no preceding block for one to separate from.
	source := []byte("<!-- <atomdown version=\"1\"/> -->\r\n\r\n<!-- <atom id=\"4P8W2H6K\"/> -->\r\nParagraph.\r\n")
	want := "Paragraph.\r\n"
	if got := string(Strip(source)); got != want {
		t.Fatalf("Strip() = %q, want %q", got, want)
	}
}

func TestDirectiveInsideFencedCodeIsMarkdown(t *testing.T) {
	source := []byte("```markdown\n<!-- <atom id=\"4P8W2H6K\"/> -->\n```\n\n<!-- <atom id=\"9R3C7M5D\"/> -->\n\nParagraph.\n")

	document := Parse(source)
	if document.HasErrors() {
		t.Fatalf("unexpected errors: %#v", document.Diagnostics)
	}
	if len(document.Atoms) != 2 {
		t.Fatalf("atoms = %#v, want fenced code plus marked paragraph", document.Atoms)
	}
	if !document.Atoms[0].Implicit || document.Atoms[1].ID != "9R3C7M5D" {
		t.Fatalf("atoms = %#v", document.Atoms)
	}

	// The two blank lines around the real directive (line 5) are pure
	// scaffolding for a directive that no longer exists, so they collapse to
	// the one blank line CommonMark needs to separate the fenced code block
	// from the paragraph.
	wantStripped := "```markdown\n<!-- <atom id=\"4P8W2H6K\"/> -->\n```\n\nParagraph.\n"
	if got := string(Strip(source)); got != wantStripped {
		t.Fatalf("Strip() = %q, want %q", got, wantStripped)
	}

	stream := Tokenize(source)
	for _, token := range stream.Tokens {
		if token.Kind == TokenDirective && token.Directive.ID == "4P8W2H6K" {
			t.Fatalf("fenced code became a directive token: %#v", token)
		}
	}
}

func TestStripLeavesNoLeadingBlankLine(t *testing.T) {
	source := []byte("<!-- <atomdown version=\"1\"/> -->\n\n<!-- <atom id=\"4P8W2H6K\"/> -->\n\nParagraph.\n")
	want := "Paragraph.\n"
	if got := string(Strip(source)); got != want {
		t.Fatalf("Strip() = %q, want %q", got, want)
	}
}

func TestStripLeavesNoTrailingBlankLine(t *testing.T) {
	source := []byte("<!-- <atom id=\"4P8W2H6K\"/> -->\n\nParagraph.\n\n<!-- </atom-group> -->")
	want := "Paragraph.\n"
	if got := string(Strip(source)); got != want {
		t.Fatalf("Strip() = %q, want %q", got, want)
	}
}

func TestStripCollapsesDirectiveScaffoldingToOneBlankLine(t *testing.T) {
	source := []byte("First.\n\n<!-- <atom id=\"4P8W2H6K\"/> -->\n\nSecond.\n")
	want := "First.\n\nSecond.\n"
	if got := string(Strip(source)); got != want {
		t.Fatalf("Strip() = %q, want %q", got, want)
	}
}

// TestStripWrappedDirectiveMatchesSingleLineDirective is the correctness core
// of the wrapped-directive rule. A directive is never part of the stripped
// Markdown projection, so wrapping one must not change that projection by a
// single byte. Each pair below strips the same document twice: once with a
// one-line directive, once with the same directive wrapped.
func TestStripWrappedDirectiveMatchesSingleLineDirective(t *testing.T) {
	cases := []struct {
		name    string
		single  string
		wrapped string
	}{
		{
			name:    "between two paragraphs",
			single:  "First.\n\n<!-- <atom id=\"4P8W2H6K\"/> -->\n\nSecond.\n",
			wrapped: "First.\n\n<!--\n  <atom\n    id=\"4P8W2H6K\"\n  />\n-->\n\nSecond.\n",
		},
		{
			name:    "at the start of the document",
			single:  "<!-- <atom id=\"4P8W2H6K\"/> -->\n\nParagraph.\n",
			wrapped: "<!--\n  <atom\n    id=\"4P8W2H6K\"\n  />\n-->\n\nParagraph.\n",
		},
		{
			name:    "at the end of the document",
			single:  "Paragraph.\n\n<!-- </atom-group> -->\n",
			wrapped: "Paragraph.\n\n<!--\n  </atom-group>\n-->\n",
		},
		{
			name:    "adjacent to the block it marks",
			single:  "<!-- <atom id=\"4P8W2H6K\"/> -->\nParagraph.\n",
			wrapped: "<!--\n  <atom id=\"4P8W2H6K\"/>\n-->\nParagraph.\n",
		},
		{
			name:    "splitting a list between two items",
			single:  "- one\n<!-- <atom id=\"4P8W2H6K\"/> -->\n- two\n",
			wrapped: "- one\n<!-- <atom\n     id=\"4P8W2H6K\"/> -->\n- two\n",
		},
		{
			name:    "with a blank line inside the directive",
			single:  "First.\n\n<!-- <atom id=\"4P8W2H6K\"/> -->\n\nSecond.\n",
			wrapped: "First.\n\n<!--\n\n  <atom id=\"4P8W2H6K\"/>\n\n-->\n\nSecond.\n",
		},
		{
			name:    "with CRLF line endings",
			single:  "First.\r\n\r\n<!-- <atom id=\"4P8W2H6K\"/> -->\r\n\r\nSecond.\r\n",
			wrapped: "First.\r\n\r\n<!--\r\n  <atom id=\"4P8W2H6K\"/>\r\n-->\r\n\r\nSecond.\r\n",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			want := string(Strip([]byte(testCase.single)))
			got := string(Strip([]byte(testCase.wrapped)))
			if got != want {
				t.Fatalf("Strip(wrapped) = %q, want the single-line result %q", got, want)
			}
			if strings.Contains(got, "<!--") {
				t.Fatalf("wrapped directive was not recognized; scaffolding remains: %q", got)
			}
		})
	}
}

func TestStripPreservesADeliberateDoubleBlankLine(t *testing.T) {
	// The double blank line here is the author's own and sits nowhere near
	// a directive, so it must survive: removing it could turn a loose list
	// tight and change the rendered HTML.
	source := []byte("First.\n\n\nSecond, deliberately separated by two blank lines.\n")
	if got := string(Strip(source)); got != string(source) {
		t.Fatalf("Strip() = %q, want source unchanged %q", got, source)
	}
}

func TestStripRemovesTrailingSpacesOnADirectiveLine(t *testing.T) {
	source := []byte("<!-- <atom id=\"4P8W2H6K\"/> -->   \n\nParagraph.\n")
	want := "Paragraph.\n"
	if got := string(Strip(source)); got != want {
		t.Fatalf("Strip() = %q, want %q", got, want)
	}
}

func TestStripRemovesLeadingSpacesBeforeADirective(t *testing.T) {
	source := []byte("   <!-- <atom id=\"4P8W2H6K\"/> -->\n\nParagraph.\n")
	want := "Paragraph.\n"
	if got := string(Strip(source)); got != want {
		t.Fatalf("Strip() = %q, want %q", got, want)
	}
}

func TestStripAfterMaterializeIsByteExactForCRLFDocument(t *testing.T) {
	original := []byte("# Title\r\n\r\nPara one.\r\n\r\nPara two.\r\n")
	materialized, _, err := Materialize(original)
	if err != nil {
		t.Fatal(err)
	}
	if got := Strip(materialized); string(got) != string(original) {
		t.Fatalf("Strip(Materialize(original)) = %q, want original %q", got, original)
	}
}

func TestStripAfterMaterializeSplitIsByteExact(t *testing.T) {
	original := []byte("* one\n* two\n* three\n")
	split, _, err := MaterializeSplit(original, []string{"list-item"})
	if err != nil {
		t.Fatal(err)
	}
	if got := Strip(split); string(got) != string(original) {
		t.Fatalf("Strip(MaterializeSplit(original)) = %q, want original %q", got, original)
	}
}

func TestDirectiveInsideBlockquoteRemainsInlineError(t *testing.T) {
	document := Parse([]byte("> <!-- <atom id=\"4P8W2H6K\"/> -->\n"))
	for _, diagnostic := range document.Diagnostics {
		if diagnostic.Code == "inline-directive" {
			return
		}
	}
	t.Fatalf("diagnostics = %#v, want inline-directive", document.Diagnostics)
}

func TestProcessorExtension(t *testing.T) {
	extension := ExtensionFunc{
		ExtensionName: "test-extension",
		TransformFunc: func(_ context.Context, _ []byte, document *Document) error {
			document.Diagnostics = append(document.Diagnostics, Diagnostic{
				Code: "extension-test", Severity: SeverityWarning, Message: "extension ran",
			})
			return nil
		},
	}
	document, err := NewProcessor(extension).Process(context.Background(), []byte("Paragraph.\n"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, diagnostic := range document.Diagnostics {
		if diagnostic.Code == "extension-test" {
			found = true
		}
	}
	if !found {
		t.Fatalf("extension diagnostic missing: %#v", document.Diagnostics)
	}
}

// TestThematicBreakHasOwnBlockExtent guards against a thematic break being
// treated as having no source extent. Before the fix, a "---" between two
// paragraphs produced no markdownBlock entry at all, so the paragraph
// before it silently absorbed the break line and everything up to the next
// real block.
func TestThematicBreakHasOwnBlockExtent(t *testing.T) {
	source := []byte("First paragraph.\n\n---\n\nSecond paragraph.\n")
	document := Parse(source)
	if document.HasErrors() {
		t.Fatalf("unexpected errors: %#v", document.Diagnostics)
	}
	if len(document.Atoms) != 3 {
		t.Fatalf("atoms = %#v, want 3 (paragraph, break, paragraph)", document.Atoms)
	}
	if got := document.Atoms[0].Text; got != "First paragraph." {
		t.Fatalf("first atom text = %q, want %q (must not swallow the break)", got, "First paragraph.")
	}
	if got := document.Atoms[1].NodeType; got != "ThematicBreak" {
		t.Fatalf("second atom nodeType = %q, want ThematicBreak", got)
	}
	if got := document.Atoms[1].Text; got != "---" {
		t.Fatalf("second atom text = %q, want %q", got, "---")
	}
	if got := document.Atoms[2].Text; got != "Second paragraph." {
		t.Fatalf("third atom text = %q, want %q", got, "Second paragraph.")
	}
}

// TestAtomCanTargetThematicBreak guards against a break being permanently
// unaddressable. Before the fix, an atom marker placed directly before a
// "---" skipped straight over it and attached to whatever real block came
// next, so a break itself could never carry an atom marker. SPEC.md
// documents the choice: a thematic break is an ordinary top-level block,
// so an atom directive can target it like any other.
func TestAtomCanTargetThematicBreak(t *testing.T) {
	source := []byte("<!-- <atom id=\"4P8W2H6K\"/> -->\n\n---\n\nParagraph after the break.\n")
	document := Parse(source)
	if document.HasErrors() {
		t.Fatalf("unexpected errors: %#v", document.Diagnostics)
	}
	if len(document.Atoms) != 2 {
		t.Fatalf("atoms = %#v, want 2 (break, paragraph)", document.Atoms)
	}
	atom := document.Atoms[0]
	if atom.Implicit {
		t.Fatalf("break atom is implicit, want explicit: %#v", atom)
	}
	if atom.ID != "4P8W2H6K" {
		t.Fatalf("break atom ID = %q, want 4P8W2H6K", atom.ID)
	}
	if atom.NodeType != "ThematicBreak" {
		t.Fatalf("break atom nodeType = %q, want ThematicBreak", atom.NodeType)
	}
	if atom.Text != "---" {
		t.Fatalf("break atom text = %q, want %q", atom.Text, "---")
	}
	if got := document.Atoms[1].Text; got != "Paragraph after the break." {
		t.Fatalf("second atom text = %q, want %q", got, "Paragraph after the break.")
	}
}

// TestStackedAtomMarkersReportAccurateDiagnostic guards against a false
// "not followed by a Markdown block" message when two atom markers stack
// above one paragraph. A block does exist; the second marker claims it
// first, and the first marker's diagnostic must say so accurately.
func TestStackedAtomMarkersReportAccurateDiagnostic(t *testing.T) {
	source := []byte("<!-- <atom id=\"4P8W2H6K\"/> -->\n<!-- <atom id=\"9R3C7M5D\"/> -->\n\nParagraph.\n")
	document := Parse(source)
	var found *Diagnostic
	for index := range document.Diagnostics {
		if document.Diagnostics[index].Code == "shadowed-atom" {
			found = &document.Diagnostics[index]
		}
		if document.Diagnostics[index].Code == "orphan-atom" {
			t.Fatalf("got orphan-atom, want shadowed-atom: a block does exist, the next marker claims it")
		}
	}
	if found == nil {
		t.Fatalf("diagnostics = %#v, want a shadowed-atom diagnostic", document.Diagnostics)
	}
	if len(document.Atoms) != 1 || document.Atoms[0].ID != "9R3C7M5D" {
		t.Fatalf("atoms = %#v, want the paragraph assigned to the second marker only", document.Atoms)
	}
}

// TestWrappedDirectiveIsAccepted proves a directive may span several source
// lines. Wrapping is legal because the digest is block-only: the directive is
// never hashed, so its internal whitespace carries no meaning and a reader
// loses nothing by letting an author break a long attribute list over lines.
func TestWrappedDirectiveIsAccepted(t *testing.T) {
	source := []byte("<!-- <atom\n  id=\"4P8W2H6K\"\n  slug=\"claim\"\n/> -->\n\nParagraph.\n")
	document := Parse(source)
	if document.HasErrors() {
		t.Fatalf("unexpected errors: %#v", document.Diagnostics)
	}
	if len(document.Atoms) != 1 {
		t.Fatalf("atoms = %#v, want exactly one explicit atom", document.Atoms)
	}
	atom := document.Atoms[0]
	if atom.ID != "4P8W2H6K" || atom.Slug != "claim" || atom.Implicit {
		t.Fatalf("atom = %#v, want explicit 4P8W2H6K/claim", atom)
	}
	if atom.Text != "Paragraph." {
		t.Fatalf("atom text = %q, want %q", atom.Text, "Paragraph.")
	}
	if atom.Marker == nil || atom.Marker.Start.Line != 1 || atom.Marker.End.Line != 4 {
		t.Fatalf("marker = %#v, want a span from line 1 to line 4", atom.Marker)
	}
}

// TestDirectiveWhitespaceIsSemanticallyIrrelevant pins decision 2 of the
// wrapped-directive rule: the content digest covers the atom's block only, so
// two documents that differ solely in whitespace inside a directive must parse
// the same way and produce the same digest.
func TestDirectiveWhitespaceIsSemanticallyIrrelevant(t *testing.T) {
	single := []byte("<!-- <atom id=\"4P8W2H6K\" slug=\"claim\"/> -->\n\nParagraph.\n")
	wrapped := []byte("<!--\n  <atom\n    id=\"4P8W2H6K\"\n    slug=\"claim\"\n  />\n-->\n\nParagraph.\n")

	singleDocument := Parse(single)
	wrappedDocument := Parse(wrapped)
	if wrappedDocument.HasErrors() {
		t.Fatalf("unexpected errors: %#v", wrappedDocument.Diagnostics)
	}
	if len(singleDocument.Atoms) != 1 || len(wrappedDocument.Atoms) != 1 {
		t.Fatalf("atom counts = %d and %d, want 1 and 1", len(singleDocument.Atoms), len(wrappedDocument.Atoms))
	}

	singleAtom := singleDocument.Atoms[0]
	wrappedAtom := wrappedDocument.Atoms[0]
	if singleAtom.ID != wrappedAtom.ID || singleAtom.Slug != wrappedAtom.Slug {
		t.Fatalf("identity differs: %#v vs %#v", singleAtom, wrappedAtom)
	}
	if singleAtom.Text != wrappedAtom.Text {
		t.Fatalf("block text differs: %q vs %q", singleAtom.Text, wrappedAtom.Text)
	}
	if ContentDigest(singleAtom.Text) != ContentDigest(wrappedAtom.Text) {
		t.Fatal("directive whitespace changed the content digest; the digest must be block-only")
	}
}

// TestWrappedDirectiveWithStrayContentIsRejected covers the defect that
// replaces the retired "one source line" rule: whitespace inside a directive
// comment is free, but any other content inside it is not.
func TestWrappedDirectiveWithStrayContentIsRejected(t *testing.T) {
	source := []byte("<!--\n  <atom id=\"4P8W2H6K\"/>\n  stray text\n-->\n\nParagraph.\n")
	document := Parse(source)
	var found bool
	for _, diagnostic := range document.Diagnostics {
		if diagnostic.Code == "extra-directive-content" {
			found = true
			if diagnostic.Position.Line != 1 {
				t.Fatalf("diagnostic position line = %d, want 1 (the opening <!--)", diagnostic.Position.Line)
			}
		}
	}
	if !found {
		t.Fatalf("diagnostics = %#v, want an extra-directive-content diagnostic", document.Diagnostics)
	}
	for _, atom := range document.Atoms {
		if atom.ID == "4P8W2H6K" {
			t.Fatalf("comment with stray content was still accepted as an atom marker: %#v", atom)
		}
	}
}

// TestInlineDirectiveStillRejectedWhenWrapped proves the relaxed rule did not
// relax the other half: only whitespace may precede the opening "<!--" and
// follow the closing "-->", on a wrapped directive too.
func TestInlineDirectiveStillRejectedWhenWrapped(t *testing.T) {
	for name, source := range map[string]string{
		"content before the opening token": "Text before <!-- <atom\n  id=\"4P8W2H6K\"\n/> -->\n\nParagraph.\n",
		"content after the closing token":  "<!-- <atom\n  id=\"4P8W2H6K\"\n/> --> text after\n\nParagraph.\n",
	} {
		t.Run(name, func(t *testing.T) {
			document := Parse([]byte(source))
			var found bool
			for _, diagnostic := range document.Diagnostics {
				if diagnostic.Code == "inline-directive" {
					found = true
				}
			}
			if !found {
				t.Fatalf("diagnostics = %#v, want an inline-directive diagnostic", document.Diagnostics)
			}
		})
	}
}
