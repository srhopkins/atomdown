package atomdown

import (
	"bytes"
	"regexp"
	"testing"
)

func TestMaterializeReportsCountOfNewMarkers(t *testing.T) {
	source := []byte("# One\n\npara two\n\n* a\n* b\n\npara three\n")
	output, marked, err := Materialize(source)
	if err != nil {
		t.Fatal(err)
	}
	if marked != 4 {
		t.Fatalf("expected 4 marked blocks, got %d", marked)
	}
	atomPattern := regexp.MustCompile(`<!-- <atom id="[0-9A-HJKMNP-TV-Z]{8}"/> -->`)
	matches := atomPattern.FindAll(output, -1)
	if len(matches) != marked {
		t.Fatalf("expected %d markers in output, found %d:\n%s", marked, len(matches), output)
	}
}

func TestMaterializeReportsZeroWhenNothingToDo(t *testing.T) {
	source := []byte("<!-- <atom id=\"4P8W2H6K\"/> -->\n\nAlready marked.\n")
	_, marked, err := Materialize(source)
	if err != nil {
		t.Fatal(err)
	}
	if marked != 0 {
		t.Fatalf("expected 0 marked blocks, got %d", marked)
	}
}

func TestMaterializeAddsVersionDirectiveWhenMissing(t *testing.T) {
	source := []byte("# One\n\npara two\n")
	output, _, err := Materialize(source)
	if err != nil {
		t.Fatal(err)
	}
	directivePattern := regexp.MustCompile(`(?m)^<!-- <atomdown version="1"/> -->$`)
	matches := directivePattern.FindAllIndex(output, -1)
	if len(matches) != 1 {
		t.Fatalf("expected exactly one version directive, found %d:\n%s", len(matches), output)
	}
	atomPattern := regexp.MustCompile(`<!-- <atom id="[0-9A-HJKMNP-TV-Z]{8}"/> -->`)
	atomIndex := atomPattern.FindIndex(output)
	if atomIndex == nil {
		t.Fatalf("expected an atom marker in output:\n%s", output)
	}
	if matches[0][0] > atomIndex[0] {
		t.Fatalf("version directive must precede the first atom marker:\n%s", output)
	}
}

func TestMaterializeNeverDuplicatesVersionDirective(t *testing.T) {
	source := []byte("# One\n\npara two\n")
	firstOutput, _, err := Materialize(source)
	if err != nil {
		t.Fatal(err)
	}
	secondOutput, _, err := Materialize(firstOutput)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstOutput, secondOutput) {
		t.Fatalf("second materialize changed the file:\nfirst:\n%s\nsecond:\n%s", firstOutput, secondOutput)
	}
}

func TestMaterializeLeavesExistingVersionDirectiveUntouched(t *testing.T) {
	source := []byte("<!-- <atomdown version=\"1\" acme-profile=\"draft\"/> -->\n\npara.\n")
	output, _, err := Materialize(source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`<!-- <atomdown version="1" acme-profile="draft"/> -->`)) {
		t.Fatalf("existing version directive was altered:\n%s", output)
	}
	if bytes.Count(output, []byte("atomdown version=")) != 1 {
		t.Fatalf("expected exactly one document directive:\n%s", output)
	}
}

func TestMaterializeIsIdempotentOnSecondRun(t *testing.T) {
	source := []byte("# One\n\npara two\n")
	firstOutput, firstMarked, err := Materialize(source)
	if err != nil {
		t.Fatal(err)
	}
	if firstMarked == 0 {
		t.Fatal("expected the first run to mark at least one block")
	}
	_, secondMarked, err := Materialize(firstOutput)
	if err != nil {
		t.Fatal(err)
	}
	if secondMarked != 0 {
		t.Fatalf("expected the second run to mark zero blocks, got %d", secondMarked)
	}
}
