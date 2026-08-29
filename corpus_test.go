package atomdown

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestValidCorpus(t *testing.T) {
	for _, path := range fixtureFiles(t, "testdata/valid/*.md") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			document := Parse(readFixture(t, path))
			if document.HasErrors() {
				t.Fatalf("valid fixture produced errors: %#v", document.Diagnostics)
			}
		})
	}
}

func TestMixedCorpusRemainsLosslessMarkdown(t *testing.T) {
	for _, path := range fixtureFiles(t, "testdata/mixed/*.md") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			source := readFixture(t, path)
			stream := Tokenize(source)
			var reconstructed strings.Builder
			for _, token := range stream.Tokens {
				reconstructed.WriteString(token.Raw)
			}
			if reconstructed.String() != string(source) {
				t.Fatal("token stream did not reconstruct the source")
			}
			if len(Strip(source)) == 0 {
				t.Fatal("Markdown projection is empty")
			}
		})
	}
}

func TestMalformedCorpusDiagnostics(t *testing.T) {
	var expected map[string][]string
	if err := json.Unmarshal(readFixture(t, "testdata/golden/malformed.codes.json"), &expected); err != nil {
		t.Fatal(err)
	}
	for _, path := range fixtureFiles(t, "testdata/malformed/*.md") {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			document := Parse(readFixture(t, path))
			if !document.HasErrors() {
				t.Fatal("malformed fixture produced no error")
			}
			var codes []string
			for _, diagnostic := range document.Diagnostics {
				codes = append(codes, diagnostic.Code)
			}
			if !reflect.DeepEqual(codes, expected[name]) {
				t.Fatalf("diagnostic codes = %v, want %v", codes, expected[name])
			}
		})
	}
}

func TestCompleteGoldenOutputs(t *testing.T) {
	source := readFixture(t, "testdata/valid/complete.md")
	streamJSON, err := json.MarshalIndent(Tokenize(source), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	streamJSON = append(streamJSON, '\n')
	assertGolden(t, "testdata/golden/complete.tokens.json", streamJSON)
	assertGolden(t, "testdata/golden/complete.stripped.md", Strip(source))

	normalized, err := NormalizedXML(Parse(source))
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "testdata/golden/complete.xml", normalized)
}

func fixtureFiles(t *testing.T, pattern string) []string {
	t.Helper()
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no fixtures matched %q", pattern)
	}
	sort.Strings(files)
	return files
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func assertGolden(t *testing.T, path string, actual []byte) {
	t.Helper()
	expected := readFixture(t, path)
	if !bytes.Equal(actual, expected) {
		t.Fatalf("output differs from %s", path)
	}
}
