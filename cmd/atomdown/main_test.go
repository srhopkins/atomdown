package main

import (
	"bytes"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestRunUnknownCommandReportsVersionAndCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"materialize-typo"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error for an unknown command")
	}
	var status exitError
	if !errors.As(err, &status) || status.code != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
	output := stderr.String()
	if !strings.Contains(output, `unknown command "materialize-typo"`) {
		t.Fatalf("missing unknown-command message:\n%s", output)
	}
	if !strings.Contains(output, cliVersion) {
		t.Fatalf("missing version %q in output:\n%s", cliVersion, output)
	}
	for _, name := range commandNames {
		if !strings.Contains(output, name) {
			t.Fatalf("missing command %q in output:\n%s", name, output)
		}
	}
}

func TestRunMissingCommandReportsVersionAndCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run(nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error for a missing command")
	}
	var status exitError
	if !errors.As(err, &status) || status.code != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
	output := stderr.String()
	if !strings.Contains(output, cliVersion) {
		t.Fatalf("missing version %q in output:\n%s", cliVersion, output)
	}
	for _, name := range commandNames {
		if !strings.Contains(output, name) {
			t.Fatalf("missing command %q in output:\n%s", name, output)
		}
	}
}

func TestRunMaterializeStdoutStaysCleanAndReportsCountOnStderr(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/doc.md"
	source := "# One\n\npara two\n"
	writeFile(t, path, source)

	var stdout, stderr bytes.Buffer
	if err := run([]string{"materialize", path}, &stdout, &stderr); err != nil {
		t.Fatalf("materialize failed: %v", err)
	}
	if strings.Contains(stdout.String(), "marked") || strings.Contains(stdout.String(), "ok -") {
		t.Fatalf("stdout mode leaked a status line:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "<atom id=") {
		t.Fatalf("expected marked Markdown on stdout, got:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "marked 2 blocks") {
		t.Fatalf("expected a count on stderr, got:\n%s", stderr.String())
	}
}

func TestRunMaterializeWriteReportsZeroOnSecondRun(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/doc.md"
	writeFile(t, path, "# One\n\npara two\n")

	var stdout1, stderr1 bytes.Buffer
	if err := run([]string{"materialize", "-w", path}, &stdout1, &stderr1); err != nil {
		t.Fatalf("first materialize -w failed: %v", err)
	}
	if !strings.Contains(stderr1.String(), "marked 2 blocks") {
		t.Fatalf("expected a count on the first run, got:\n%s", stderr1.String())
	}
	if stdout1.Len() != 0 {
		t.Fatalf("materialize -w should not write to stdout, got:\n%s", stdout1.String())
	}

	var stdout2, stderr2 bytes.Buffer
	if err := run([]string{"materialize", "-w", path}, &stdout2, &stderr2); err != nil {
		t.Fatalf("second materialize -w failed: %v", err)
	}
	if !strings.Contains(stderr2.String(), "no unmarked blocks") {
		t.Fatalf("expected zero-count message on the second run, got:\n%s", stderr2.String())
	}
}

func TestRunLintDefaultPassesWithNoVersionDirective(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/doc.md"
	writeFile(t, path, "<!-- <atom id=\"4P8W2H6K\"/> -->\n\nFirst.\n")

	var stdout, stderr bytes.Buffer
	if err := run([]string{"lint", path}, &stdout, &stderr); err != nil {
		t.Fatalf("default lint must pass a document with no version directive: %v\n%s", err, stderr.String())
	}
	if strings.Contains(stdout.String(), "version") {
		t.Fatalf("default lint must not mention the missing version directive:\n%s", stdout.String())
	}
}

func TestRunLintStrictWarnsOnMissingVersionDirective(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/doc.md"
	writeFile(t, path, "<!-- <atom id=\"4P8W2H6K\"/> -->\n\nFirst.\n")

	var stdout, stderr bytes.Buffer
	if err := run([]string{"lint", "--strict", path}, &stdout, &stderr); err != nil {
		t.Fatalf("lint --strict warnings must not fail the command: %v\n%s", err, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "missing-version-directive") {
		t.Fatalf("expected a missing-version-directive warning:\n%s", output)
	}
	if !strings.Contains(output, path+":1:1:") {
		t.Fatalf("expected file:line:col like other diagnostics:\n%s", output)
	}
}

func TestRunLintStrictStaysQuietWhenVersionDirectiveIsPresent(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/doc.md"
	writeFile(t, path, "<!-- <atomdown version=\"1\"/> -->\n\n<!-- <atom id=\"4P8W2H6K\"/> -->\n\nFirst.\n")

	var stdout, stderr bytes.Buffer
	if err := run([]string{"lint", "--strict", path}, &stdout, &stderr); err != nil {
		t.Fatalf("lint --strict failed: %v\n%s", err, stderr.String())
	}
	if strings.Contains(stdout.String(), "missing-version-directive") {
		t.Fatalf("declared document should not warn:\n%s", stdout.String())
	}
}

func TestRunMaterializeSplitListItemWrapsGroup(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/doc.md"
	writeFile(t, path, "# T\n\n* one\n* two\n* three\n")

	var stdout, stderr bytes.Buffer
	if err := run([]string{"materialize", "--split", "list-item", "-w", path}, &stdout, &stderr); err != nil {
		t.Fatalf("materialize --split failed: %v\n%s", err, stderr.String())
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "<!-- <atom-group id=") {
		t.Fatalf("expected an atom-group marker, got:\n%s", written)
	}
	if !strings.Contains(string(written), "<!-- </atom-group> -->") {
		t.Fatalf("expected a closing atom-group marker, got:\n%s", written)
	}
}

func TestRunMaterializeSplitUnknownNodeTypeNamesAcceptedValues(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/doc.md"
	writeFile(t, path, "* a\n")

	var stdout, stderr bytes.Buffer
	err := run([]string{"materialize", "--split", "bogus-node", "-w", path}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error for an unknown --split node type")
	}
	if !strings.Contains(err.Error(), "list-item") {
		t.Fatalf("expected accepted values in the error, got: %v", err)
	}
	written, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(written) != "* a\n" {
		t.Fatalf("file was modified despite the error: %q", written)
	}
}

func TestRunMaterializeDigestWritesDigestsAndDriftFindsAnEditedAtom(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/doc.md"
	writeFile(t, path, "# T\n\nPara one.\n\nPara two.\n")

	var stdout, stderr bytes.Buffer
	if err := run([]string{"materialize", "--digest", "-w", path}, &stdout, &stderr); err != nil {
		t.Fatalf("materialize --digest failed: %v\n%s", err, stderr.String())
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), `digest="sha256:`) {
		t.Fatalf("expected a digest attribute after materialize --digest:\n%s", written)
	}

	// A clean document must report no drift and exit 0.
	var driftOut, driftErr bytes.Buffer
	if err := run([]string{"drift", path}, &driftOut, &driftErr); err != nil {
		t.Fatalf("drift on an unchanged document must exit 0: %v\n%s", err, driftOut.String())
	}

	// Editing the last paragraph must make drift find it and exit non-zero.
	edited := strings.Replace(string(written), "Para two.", "Para two, edited.", 1)
	writeFile(t, path, edited)

	var driftOut2, driftErr2 bytes.Buffer
	err = run([]string{"drift", path}, &driftOut2, &driftErr2)
	var status exitError
	if !errors.As(err, &status) || status.code != 1 {
		t.Fatalf("expected drift to exit 1 on a changed document, got %v", err)
	}
	idPattern := regexp.MustCompile(`id="([0-9A-HJKMNP-TV-Z]{8})"`)
	matches := idPattern.FindAllStringSubmatch(edited, -1)
	if len(matches) == 0 {
		t.Fatal("no atom IDs found in the edited document")
	}
	lastID := matches[len(matches)-1][1]
	if !strings.Contains(driftOut2.String(), lastID) {
		t.Fatalf("expected drift output to name the edited atom %q, got:\n%s", lastID, driftOut2.String())
	}

	// verify is an accepted alias for drift.
	var verifyOut bytes.Buffer
	err = run([]string{"verify", path}, &verifyOut, &bytes.Buffer{})
	if !errors.As(err, &status) || status.code != 1 {
		t.Fatalf("expected verify to behave like drift and exit 1, got %v", err)
	}
}

func TestRunMaterializeDigestNeverOverwritesAnExistingDigest(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/doc.md"
	writeFile(t, path, "# T\n\nPara one.\n")

	var stdout1, stderr1 bytes.Buffer
	if err := run([]string{"materialize", "--digest", "-w", path}, &stdout1, &stderr1); err != nil {
		t.Fatalf("first materialize --digest failed: %v\n%s", err, stderr1.String())
	}
	firstWrite, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var stdout2, stderr2 bytes.Buffer
	if err := run([]string{"materialize", "--digest", "-w", path}, &stdout2, &stderr2); err != nil {
		t.Fatalf("second materialize --digest failed: %v\n%s", err, stderr2.String())
	}
	secondWrite, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstWrite) != string(secondWrite) {
		t.Fatalf("a second materialize --digest run changed the file:\nfirst:\n%s\nsecond:\n%s", firstWrite, secondWrite)
	}
	if !strings.Contains(stderr2.String(), "already has a digest") {
		t.Fatalf("expected the second run to report nothing new to digest, got:\n%s", stderr2.String())
	}
}

func TestRunMaterializePlainNeverWritesADigest(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/doc.md"
	writeFile(t, path, "# T\n\nPara one.\n")

	var stdout, stderr bytes.Buffer
	if err := run([]string{"materialize", "-w", path}, &stdout, &stderr); err != nil {
		t.Fatalf("materialize failed: %v\n%s", err, stderr.String())
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(written), "digest=") {
		t.Fatalf("default materialize must never write a digest attribute:\n%s", written)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRunEmitKeepsAuthoredLayoutAndFlattenCanonicalizes drives the whole CLI
// pipeline an author actually runs: parse to JSON, then emit that JSON back to
// Markdown. Default emit must return the wrapped directive byte for byte;
// --flatten must return the canonical one-line form.
func TestRunEmitKeepsAuthoredLayoutAndFlattenCanonicalizes(t *testing.T) {
	directory := t.TempDir()
	path := directory + "/wrapped.md"
	source := "<!-- <atomdown version=\"1\"/> -->\n\n<!--\n  <atom\n    id=\"4P8W2H6K\"\n    slug=\"claim\"\n  />\n-->\n\nParagraph.\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	var parsed, stderr bytes.Buffer
	if err := run([]string{"parse", path}, &parsed, &stderr); err != nil {
		t.Fatal(err)
	}
	model := directory + "/model.json"
	if err := os.WriteFile(model, parsed.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	var emitted bytes.Buffer
	if err := run([]string{"emit", model}, &emitted, &stderr); err != nil {
		t.Fatal(err)
	}
	// emit separates blocks with one blank line and ends the document with
	// one, so compare with the trailing newlines trimmed. The directive text
	// itself must match byte for byte.
	if strings.TrimRight(emitted.String(), "\n") != strings.TrimRight(source, "\n") {
		t.Fatalf("emit did not return the authored bytes:\n%q", emitted.String())
	}

	var flattened bytes.Buffer
	if err := run([]string{"emit", "--flatten", model}, &flattened, &stderr); err != nil {
		t.Fatal(err)
	}
	want := "<!-- <atomdown version=\"1\"/> -->\n\n<!-- <atom id=\"4P8W2H6K\" slug=\"claim\"/> -->\n\nParagraph.\n\n"
	if flattened.String() != want {
		t.Fatalf("emit --flatten = %q,\nwant %q", flattened.String(), want)
	}
}

// TestRunEmitStillRejectsTwoFiles proves adding a flag did not lose the
// one-file rule; flag parsing now decides what counts as a file argument.
func TestRunEmitStillRejectsTwoFiles(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"emit", "one.json", "two.json"}, &stdout, &stderr); err == nil {
		t.Fatal("expected an error for two files")
	}
}

// TestPrintUsageDocumentsTheFlattenFlag keeps the usage text and the flag
// from drifting apart.
func TestPrintUsageDocumentsTheFlattenFlag(t *testing.T) {
	var output bytes.Buffer
	printUsage(&output)
	if !strings.Contains(output.String(), "--flatten") {
		t.Fatalf("usage does not mention --flatten:\n%s", output.String())
	}
}
