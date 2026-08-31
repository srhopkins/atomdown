package atomdown

import "testing"

func TestLintWarnsOnBareListSplitWithoutGroup(t *testing.T) {
	source := []byte(`<!-- <atomdown version="1"/> -->

<!-- <atom id="AAAAAAA1"/> -->
* one
<!-- <atom id="AAAAAAA2"/> -->
* two
`)
	document := Parse(source)
	if document.HasErrors() {
		t.Fatalf("unexpected errors: %#v", document.Diagnostics)
	}
	found := false
	for _, diagnostic := range document.Diagnostics {
		if diagnostic.Code == "directive-splits-list" {
			found = true
			if diagnostic.Severity != SeverityWarning {
				t.Fatalf("severity = %q, want warning", diagnostic.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("diagnostics = %#v, want directive-splits-list", document.Diagnostics)
	}
}

func TestLintDoesNotWarnOnDeliberateSplitGroup(t *testing.T) {
	source := []byte("# T\n\n* one\n* two\n* three\n")
	split, _, err := MaterializeSplit(source, []string{"list-item"})
	if err != nil {
		t.Fatal(err)
	}
	document := Parse(split)
	if document.HasErrors() {
		t.Fatalf("unexpected errors: %#v", document.Diagnostics)
	}
	for _, diagnostic := range document.Diagnostics {
		if diagnostic.Code == "directive-splits-list" {
			t.Fatalf("deliberate --split output should not warn: %#v", document.Diagnostics)
		}
	}
}

func TestLintDoesNotWarnOnInertDirectives(t *testing.T) {
	source := []byte("<!-- <atomdown version=\"1\"/> -->\n\n<!-- <atom id=\"4P8W2H6K\"/> -->\n\n# T\n\nA paragraph.\n")
	document := Parse(source)
	if document.HasErrors() {
		t.Fatalf("unexpected errors: %#v", document.Diagnostics)
	}
	for _, diagnostic := range document.Diagnostics {
		if diagnostic.Code == "directive-splits-list" {
			t.Fatalf("inert directives should not warn: %#v", document.Diagnostics)
		}
	}
}

func TestLintDoesNotWarnOnAnUnsplitMultiItemList(t *testing.T) {
	source := []byte("<!-- <atomdown version=\"1\"/> -->\n\n<!-- <atom id=\"4P8W2H6K\"/> -->\n\n* one\n* two\n* three\n")
	document := Parse(source)
	if document.HasErrors() {
		t.Fatalf("unexpected errors: %#v", document.Diagnostics)
	}
	for _, diagnostic := range document.Diagnostics {
		if diagnostic.Code == "directive-splits-list" {
			t.Fatalf("a single unsplit list atom should not warn: %#v", document.Diagnostics)
		}
	}
}

func TestLintWarnsOnceForARunOfSplitItems(t *testing.T) {
	source := []byte(`<!-- <atomdown version="1"/> -->

<!-- <atom id="AAAAAAA1"/> -->
* one
<!-- <atom id="AAAAAAA2"/> -->
* two
<!-- <atom id="AAAAAAA3"/> -->
* three
`)
	document := Parse(source)
	count := 0
	for _, diagnostic := range document.Diagnostics {
		if diagnostic.Code == "directive-splits-list" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("directive-splits-list count = %d, want 1: %#v", count, document.Diagnostics)
	}
}
