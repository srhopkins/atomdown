package atomdown

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
)

func TestNormalizedXMLPreservesNamespacedAttributes(t *testing.T) {
	source := []byte(`<!-- <atomdown version="1"/> -->

<!-- <atom id="CCCCCCCC" xml:space="preserve"/> -->

Hello.
`)
	document := Parse(source)
	if document.HasErrors() {
		t.Fatalf("unexpected diagnostics: %+v", document.Diagnostics)
	}
	if len(document.Atoms) != 1 {
		t.Fatalf("got %d atoms, want 1", len(document.Atoms))
	}
	attrs := document.Atoms[0].Attributes
	if len(attrs) != 1 {
		t.Fatalf("got %d attributes, want 1: %+v", len(attrs), attrs)
	}
	if attrs[0].Name != "xml:space" {
		t.Fatalf("attribute name = %q, want %q", attrs[0].Name, "xml:space")
	}
	if attrs[0].Value != "preserve" {
		t.Fatalf("attribute value = %q, want %q", attrs[0].Value, "preserve")
	}
	if strings.Contains(attrs[0].Name, "{") {
		t.Fatalf("attribute name still mangled: %q", attrs[0].Name)
	}

	out, err := NormalizedXML(document)
	if err != nil {
		t.Fatalf("NormalizedXML: %v", err)
	}
	if bytes.Contains(out, []byte("{")) {
		t.Fatalf("XML still contains mangled name:\n%s", out)
	}
	if !bytes.Contains(out, []byte(`xml:space="preserve"`)) && !bytes.Contains(out, []byte(`space="preserve"`)) {
		t.Fatalf("xml:space value missing from XML:\n%s", out)
	}
	if err := xml.Unmarshal(out, new(struct{})); err != nil {
		t.Fatalf("XML not well-formed: %v\n%s", err, out)
	}
}

func TestToXMLAttributesKeepsPrefixedName(t *testing.T) {
	attrs := toXMLAttributes([]Attribute{{Name: "xml:space", Value: "preserve"}})
	if len(attrs) != 1 {
		t.Fatalf("got %d attrs, want 1", len(attrs))
	}
	if attrs[0].Name.Local != "xml:space" {
		t.Fatalf("Local = %q, want %q", attrs[0].Name.Local, "xml:space")
	}
	if attrs[0].Name.Space != "" {
		t.Fatalf("Space = %q, want empty (prefix lives in Local)", attrs[0].Name.Space)
	}
	if attrs[0].Value != "preserve" {
		t.Fatalf("Value = %q, want %q", attrs[0].Value, "preserve")
	}
}

func TestDuplicateNamespacedAttributesStillError(t *testing.T) {
	source := []byte(`<!-- <atomdown version="1"/> -->

<!-- <atom id="CCCCCCCC" xml:space="preserve" xml:space="default"/> -->

Hello.
`)
	document := Parse(source)
	if !document.HasErrors() {
		t.Fatal("expected duplicate-attribute error, got none")
	}
}
