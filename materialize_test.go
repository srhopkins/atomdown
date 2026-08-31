package atomdown

import (
	"bytes"
	"regexp"
	"strings"
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

// TestMaterializeRegeneratesIDOnCollision forces a duplicate by overriding
// generateID with a fixed sequence: 40 real random bits never collide
// naturally, so this is the only way to prove materialize notices a
// collision and mints a fresh ID instead of reusing one. The sequence
// collides once against a pre-existing ID, then again against itself,
// before finally returning a free ID, proving the retry loop keeps going
// past a single failed attempt.
func TestMaterializeRegeneratesIDOnCollision(t *testing.T) {
	sequence := []string{"AAAAAAA1", "AAAAAAA1", "BBBBBBB2"}
	calls := 0
	original := generateID
	generateID = func() (string, error) {
		id := sequence[calls]
		calls++
		return id, nil
	}
	defer func() { generateID = original }()

	source := []byte("<!-- <atomdown version=\"1\"/> -->\n\n<!-- <atom id=\"AAAAAAA1\"/> -->\n\nFirst.\n\nSecond needs a new ID.\n")
	output, marked, err := Materialize(source)
	if err != nil {
		t.Fatal(err)
	}
	if marked != 1 {
		t.Fatalf("expected 1 marked block, got %d", marked)
	}
	if calls != 3 {
		t.Fatalf("expected the generator to be called 3 times (pre-seeded collision, self collision, success), got %d", calls)
	}
	idPattern := regexp.MustCompile(`id="([0-9A-HJKMNP-TV-Z]{8})"`)
	matches := idPattern.FindAllStringSubmatch(string(output), -1)
	if len(matches) != 2 {
		t.Fatalf("expected 2 IDs in output, found %d:\n%s", len(matches), output)
	}
	seen := make(map[string]bool)
	for _, match := range matches {
		if seen[match[1]] {
			t.Fatalf("duplicate ID %q survived materialize:\n%s", match[1], output)
		}
		seen[match[1]] = true
	}
	if !seen["AAAAAAA1"] || !seen["BBBBBBB2"] {
		t.Fatalf("expected the pre-existing ID and the regenerated ID, got %v", seen)
	}
}

// TestMaterializeErrorsClearlyWhenCollisionRetriesAreExhausted proves the
// bounded retry in newUniqueID surfaces a clear error rather than looping
// forever when every candidate ID collides.
func TestMaterializeErrorsClearlyWhenCollisionRetriesAreExhausted(t *testing.T) {
	original := generateID
	generateID = func() (string, error) { return "AAAAAAA1", nil }
	defer func() { generateID = original }()

	source := []byte("<!-- <atomdown version=\"1\"/> -->\n\n<!-- <atom id=\"AAAAAAA1\"/> -->\n\nFirst.\n\nSecond needs a new ID.\n")
	_, _, err := Materialize(source)
	if err == nil {
		t.Fatal("expected an error when every generated ID collides")
	}
	if !strings.Contains(err.Error(), "exhausted") {
		t.Fatalf("expected a clear exhausted-retries error, got: %v", err)
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
