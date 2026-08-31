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
	source := []byte("<!-- <atomdown version=\"1\"/> -->\r\n\r\n<!-- <atom id=\"4P8W2H6K\"/> -->\r\nParagraph.\r\n")
	want := "\r\nParagraph.\r\n"
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

	wantStripped := "```markdown\n<!-- <atom id=\"4P8W2H6K\"/> -->\n```\n\n\nParagraph.\n"
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
