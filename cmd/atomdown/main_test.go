package main

import (
	"bytes"
	"errors"
	"os"
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
