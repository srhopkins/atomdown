package atomdown

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// TestSplitCorpusMatchesGolden runs materialize --split list-item over every
// fixture in testdata/split/ and compares the result with the matching
// golden file in testdata/golden/split/. Atom and atom-group IDs are random
// (see NewID), so both the actual output and the golden file replace every
// ID with a sequential placeholder before comparing; this is the same
// randomness problem TestMaterializeSplit* in split_test.go solves with
// regexes instead of a golden file.
//
// Each fixture also carries a structural assertion below
// (TestSplitCorpusStructure) naming the specific behavior it locks in, so a
// golden-file diff is never the only signal when one of these regresses.
func TestSplitCorpusMatchesGolden(t *testing.T) {
	for _, path := range fixtureFiles(t, "testdata/split/*.md") {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			source := readFixture(t, path)
			output, _, err := MaterializeSplit(source, []string{"list-item"})
			if err != nil {
				t.Fatal(err)
			}

			document := Parse(output)
			if document.HasErrors() {
				t.Fatalf("split output has errors: %#v", document.Diagnostics)
			}

			stem := strings.TrimSuffix(name, filepath.Ext(name))
			goldenPath := filepath.Join("testdata/golden/split", stem+".split.md")
			assertGolden(t, goldenPath, normalizeSplitIDs(output))
		})
	}
}

// TestSplitCorpusStructure asserts the specific Atomdown-model shape each
// fixture is meant to demonstrate, in addition to the golden-text comparison
// above. README.md and SPEC.md describe two limits of materialize --split
// list-item that this locks in: a nested list's children are not
// individually addressable (nested-list.md), and a GFM table's rows are not
// addressable at all (table.md, tracked separately as atomdown-w01).
func TestSplitCorpusStructure(t *testing.T) {
	cases := []struct {
		name        string
		fixture     string
		wantGroups  int
		wantPerItem []int // AtomIDs length for each group, in order
		wantAtoms   int   // total explicit atoms (sum across groups plus ungrouped)
	}{
		{
			name:        "GFM task list: every item gets its own atom",
			fixture:     "task-list.md",
			wantGroups:  1,
			wantPerItem: []int{3},
			wantAtoms:   4, // heading + 3 task items
		},
		{
			name:        "ordered list: every item gets its own atom",
			fixture:     "ordered-list.md",
			wantGroups:  1,
			wantPerItem: []int{3},
			wantAtoms:   4, // heading + 3 items
		},
		{
			name:        "ordered list with two-digit markers: every item gets its own atom",
			fixture:     "ordered-list-two-digit.md",
			wantGroups:  1,
			wantPerItem: []int{3},
			wantAtoms:   4, // heading + 3 items
		},
		{
			name:        "nested list: only top-level items get atoms",
			fixture:     "nested-list.md",
			wantGroups:  1,
			wantPerItem: []int{2}, // 2 top-level items; the 2 nested children stay inside the first item's atom
			wantAtoms:   3,        // heading + 2 top-level items
		},
		{
			name:        "double-blank-line list is one loose list, not several",
			fixture:     "loose-list.md",
			wantGroups:  1,
			wantPerItem: []int{3},
			wantAtoms:   4, // heading + 3 items
		},
		{
			name:        "an image paragraph between bullets breaks one list into two",
			fixture:     "list-broken-by-image.md",
			wantGroups:  2,
			wantPerItem: []int{2, 2},
			wantAtoms:   6, // heading + 2 items + image paragraph + 2 items
		},
		{
			name:        "a GFM table gets one atom; rows are not addressable",
			fixture:     "table.md",
			wantGroups:  0,
			wantPerItem: nil,
			wantAtoms:   2, // heading + whole table
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := readFixture(t, filepath.Join("testdata/split", tc.fixture))
			output, _, err := MaterializeSplit(source, []string{"list-item"})
			if err != nil {
				t.Fatal(err)
			}

			document := Parse(output)
			if document.HasErrors() {
				t.Fatalf("split output has errors: %#v", document.Diagnostics)
			}

			if len(document.Groups) != tc.wantGroups {
				t.Fatalf("groups = %d, want %d: %#v", len(document.Groups), tc.wantGroups, document.Groups)
			}
			for index, group := range document.Groups {
				if got := len(group.AtomIDs); got != tc.wantPerItem[index] {
					t.Fatalf("group %d atoms = %d, want %d: %#v", index, got, tc.wantPerItem[index], group)
				}
			}

			explicit := 0
			for _, atom := range document.Atoms {
				if !atom.Implicit {
					explicit++
				}
			}
			if explicit != tc.wantAtoms {
				t.Fatalf("explicit atoms = %d, want %d: %#v", explicit, tc.wantAtoms, document.Atoms)
			}
		})
	}
}

// TestSplitOrderedListRendersWithCorrectNumbering renders the CommonMark
// produced by materialize --split list-item and checks that a split ordered
// list still numbers its items 1, 2, 3 (or 9, 10, 11) and not 1, 1, 1.
// Splitting turns one <ol> into several single-item <ol> elements; CommonMark
// requires goldmark to emit start="N" on any <ol> that does not begin at 1,
// so this is the one behavior in this file that could not be confirmed by
// reading the Markdown output alone.
func TestSplitOrderedListRendersWithCorrectNumbering(t *testing.T) {
	cases := []struct {
		fixture string
		starts  []int // expected first-item number of each split single-item list, in order
	}{
		{fixture: "ordered-list.md", starts: []int{1, 2, 3}},
		{fixture: "ordered-list-two-digit.md", starts: []int{9, 10, 11}},
	}

	markdown := goldmark.New(goldmark.WithExtensions(extension.GFM))
	olPattern := regexp.MustCompile(`<ol(?: start="(\d+)")?>`)

	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			source := readFixture(t, filepath.Join("testdata/split", tc.fixture))
			output, _, err := MaterializeSplit(source, []string{"list-item"})
			if err != nil {
				t.Fatal(err)
			}

			var rendered bytes.Buffer
			if err := markdown.Convert(output, &rendered); err != nil {
				t.Fatal(err)
			}

			matches := olPattern.FindAllStringSubmatch(rendered.String(), -1)
			if len(matches) != len(tc.starts) {
				t.Fatalf("rendered %d <ol> elements, want %d:\n%s", len(matches), len(tc.starts), rendered.String())
			}
			for index, match := range matches {
				start := 1
				if match[1] != "" {
					start, err = strconv.Atoi(match[1])
					if err != nil {
						t.Fatal(err)
					}
				}
				if start != tc.starts[index] {
					t.Fatalf("<ol> #%d start = %d, want %d (source order must survive the split):\n%s",
						index+1, start, tc.starts[index], rendered.String())
				}
			}
		})
	}
}

// splitIDPlaceholderPattern matches the id attribute of an Atomdown atom or
// atom-group directive, so normalizeSplitIDs can replace random IDs with
// stable placeholders before comparing against a golden file.
var splitIDPlaceholderPattern = regexp.MustCompile(`id="[0-9A-HJKMNP-TV-Z]{8}"`)

// normalizeSplitIDs replaces every atom and atom-group ID in output with a
// sequential placeholder, in order of first appearance. materialize --split
// mints new random IDs (NewID, id.go) on every run, so a literal golden file
// can only match after this normalization.
func normalizeSplitIDs(output []byte) []byte {
	counter := 0
	placeholders := make(map[string]string)
	return splitIDPlaceholderPattern.ReplaceAllFunc(output, func(match []byte) []byte {
		id := string(match[4 : len(match)-1])
		placeholder, ok := placeholders[id]
		if !ok {
			counter++
			placeholder = fmt.Sprintf("%08d", counter)
			placeholders[id] = placeholder
		}
		return []byte(fmt.Sprintf(`id="%s"`, placeholder))
	})
}
