package atomdown

import (
	"context"
	"strings"
	"testing"
)

func TestParseAtomsAndGroup(t *testing.T) {
	source := []byte(`<!-- <atomdown version="1" audit-profile="draft"/> -->

<!-- <atom-group id="7K3M9X2D" slug="findings"> -->

<!-- <atom id="4P8W2H6K" audit-approved-by="steve"/> -->

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
	if got := document.Atoms[0].Attributes[0]; got.Name != "audit-approved-by" || got.Value != "steve" {
		t.Fatalf("attribute = %#v", got)
	}
}

func TestImplicitAtomWarning(t *testing.T) {
	document := Parse([]byte("Unmarked paragraph.\n"))
	if len(document.Atoms) != 1 || !document.Atoms[0].Implicit {
		t.Fatalf("atoms = %#v", document.Atoms)
	}
	if len(document.Diagnostics) != 1 || document.Diagnostics[0].Code != "implicit-atom" {
		t.Fatalf("diagnostics = %#v", document.Diagnostics)
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
	source := []byte(`<!-- <atom id="4P8W2H6K" audit-status="approved"/> -->

First.
`)
	document := Parse(source)
	normalized, err := NormalizedXML(document)
	if err != nil {
		t.Fatal(err)
	}
	value := string(normalized)
	for _, expected := range []string{`<atomdown version="1">`, `id="4P8W2H6K"`, `audit-status="approved"`} {
		if !strings.Contains(value, expected) {
			t.Fatalf("normalized XML missing %q:\n%s", expected, value)
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
	source := []byte(`<!-- <atom id="4P8W2H6K" audit-status="approved"/> -->

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

func TestProcessorExtension(t *testing.T) {
	extension := ExtensionFunc{
		ExtensionName: "test-audit",
		TransformFunc: func(_ context.Context, _ []byte, document *Document) error {
			document.Diagnostics = append(document.Diagnostics, Diagnostic{
				Code: "audit-test", Severity: SeverityWarning, Message: "extension ran",
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
		if diagnostic.Code == "audit-test" {
			found = true
		}
	}
	if !found {
		t.Fatalf("extension diagnostic missing: %#v", document.Diagnostics)
	}
}
