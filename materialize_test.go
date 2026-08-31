package atomdown

import (
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
