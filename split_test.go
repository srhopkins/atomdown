package atomdown

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func TestParseSplitNodeTypesAcceptsListItem(t *testing.T) {
	names, err := ParseSplitNodeTypes("list-item")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "list-item" {
		t.Fatalf("names = %#v", names)
	}
}

func TestParseSplitNodeTypesDeduplicatesAndTrims(t *testing.T) {
	names, err := ParseSplitNodeTypes(" list-item , list-item,list-item ")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "list-item" {
		t.Fatalf("names = %#v", names)
	}
}

func TestParseSplitNodeTypesRejectsUnknownName(t *testing.T) {
	_, err := ParseSplitNodeTypes("bogus-node")
	if err == nil {
		t.Fatal("expected an error for an unknown node type")
	}
	if !strings.Contains(err.Error(), "list-item") {
		t.Fatalf("error does not name accepted values: %v", err)
	}
}

func TestParseSplitNodeTypesRejectsEmptyValue(t *testing.T) {
	_, err := ParseSplitNodeTypes("")
	if err == nil {
		t.Fatal("expected an error for an empty --split value")
	}
	if !strings.Contains(err.Error(), "list-item") {
		t.Fatalf("error does not name accepted values: %v", err)
	}
}

func TestMaterializeSplitListItemMarksEveryItemAndWrapsGroup(t *testing.T) {
	source := []byte("# T\n\n* one\n* two\n* three\n")
	output, marked, err := MaterializeSplit(source, []string{"list-item"})
	if err != nil {
		t.Fatal(err)
	}
	if marked != 4 { // heading + 3 items
		t.Fatalf("marked = %d, want 4", marked)
	}
	atomPattern := regexp.MustCompile(`<!-- <atom id="[0-9A-HJKMNP-TV-Z]{8}"/> -->`)
	if got := len(atomPattern.FindAll(output, -1)); got != 4 {
		t.Fatalf("atom markers = %d, want 4:\n%s", got, output)
	}
	if !bytes.Contains(output, []byte(`<!-- <atom-group id="`)) {
		t.Fatalf("missing atom-group open marker:\n%s", output)
	}
	if !bytes.Contains(output, []byte("<!-- </atom-group> -->")) {
		t.Fatalf("missing atom-group close marker:\n%s", output)
	}

	document := Parse(output)
	if document.HasErrors() {
		t.Fatalf("split output has errors: %#v", document.Diagnostics)
	}
	if len(document.Groups) != 1 || len(document.Groups[0].AtomIDs) != 3 {
		t.Fatalf("groups = %#v", document.Groups)
	}
}

func TestMaterializeSplitIsIdempotent(t *testing.T) {
	source := []byte("# T\n\n* one\n* two\n* three\n")
	first, _, err := MaterializeSplit(source, []string{"list-item"})
	if err != nil {
		t.Fatal(err)
	}
	second, secondMarked, err := MaterializeSplit(first, []string{"list-item"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("second --split changed the file:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if secondMarked != 0 {
		t.Fatalf("second --split marked = %d, want 0", secondMarked)
	}
}

func TestPlainMaterializeLeavesSplitOutputAlone(t *testing.T) {
	source := []byte("# T\n\n* one\n* two\n* three\n")
	split, _, err := MaterializeSplit(source, []string{"list-item"})
	if err != nil {
		t.Fatal(err)
	}
	plain, marked, err := Materialize(split)
	if err != nil {
		t.Fatal(err)
	}
	if marked != 0 {
		t.Fatalf("plain materialize marked = %d, want 0", marked)
	}
	if !bytes.Equal(split, plain) {
		t.Fatalf("plain materialize disturbed split output:\nsplit:\n%s\nplain:\n%s", split, plain)
	}
}

func TestMaterializeSplitUnknownNodeType(t *testing.T) {
	_, _, err := MaterializeSplit([]byte("* a\n"), []string{"bogus-node"})
	if err == nil {
		t.Fatal("expected an error for an unknown node type")
	}
	if !strings.Contains(err.Error(), "list-item") {
		t.Fatalf("error does not name accepted values: %v", err)
	}
}

func TestMaterializeSplitStripRestoresOriginal(t *testing.T) {
	original := []byte("# T\n\n* one\n* two\n* three\n")
	split, _, err := MaterializeSplit(original, []string{"list-item"})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(Strip(split)); got != string(original) {
		t.Fatalf("Strip() = %q, want %q", got, string(original))
	}
}
