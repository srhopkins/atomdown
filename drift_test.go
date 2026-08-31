package atomdown

import (
	"strings"
	"testing"
)

func TestDriftFindsAnAtomWhoseContentChangedSinceItsDigestWasWritten(t *testing.T) {
	source := []byte("# T\n\nPara one.\n\nPara two.\n")
	digested, _, _, err := MaterializeDigest(source)
	if err != nil {
		t.Fatal(err)
	}
	edited := []byte(strings.Replace(string(digested), "Para two.", "Para two, edited.", 1))

	drifted := Drift(edited)
	if len(drifted) != 1 {
		t.Fatalf("expected exactly 1 drifted atom, got %d: %#v", len(drifted), drifted)
	}

	document := Parse(edited)
	var editedID string
	for _, atom := range document.Atoms {
		if atom.Text == "Para two, edited." {
			editedID = atom.ID
		}
	}
	if editedID == "" {
		t.Fatal("could not find the edited atom in the re-parsed document")
	}
	if drifted[0].ID != editedID {
		t.Fatalf("drift reported atom %q, want the edited atom %q", drifted[0].ID, editedID)
	}
}

func TestDriftReportsNothingOnAnUnchangedDocument(t *testing.T) {
	source := []byte("# T\n\nPara one.\n")
	digested, _, _, err := MaterializeDigest(source)
	if err != nil {
		t.Fatal(err)
	}
	if drifted := Drift(digested); len(drifted) != 0 {
		t.Fatalf("expected no drift on an unchanged document, got %#v", drifted)
	}
}

func TestDriftSkipsAtomsWithNoDigest(t *testing.T) {
	source := []byte("<!-- <atomdown version=\"1\"/> -->\n\n<!-- <atom id=\"4P8W2H6K\"/> -->\n\nNever digested, then edited.\n")
	if drifted := Drift(source); len(drifted) != 0 {
		t.Fatalf("an atom with no digest has no baseline and must not be reported as drift, got %#v", drifted)
	}
}
