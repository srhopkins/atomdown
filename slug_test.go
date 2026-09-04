package atomdown

import (
	"strings"
	"testing"
)

// TestSlugifyShape covers the plain-text slugifier. Markdown syntax is
// stripped one layer up, in atomSlugBase; see
// TestAtomSlugBaseStripsMarkdownSyntax.
func TestSlugifyShape(t *testing.T) {
	for _, testCase := range []struct {
		name string
		text string
		want string
	}{
		{name: "words", text: "Decisions waiting on me", want: "decisions-waiting-on-me"},
		{name: "case folds down", text: "RESEA Tickets", want: "resea-tickets"},
		{name: "punctuation collapses", text: "RESEA tickets - due tonight!", want: "resea-tickets-due-tonight"},
		{name: "digits survive", text: "FFAI-72606 needs a decision", want: "ffai-72606-needs-a-decision"},
		{name: "accents fold to ascii", text: "Café über Ångström", want: "cafe-uber-angstrom"},
		{name: "unmapped runes separate", text: "rollout 東京 status", want: "rollout-status"},
		{name: "word limit", text: "one two three four five six seven eight nine ten", want: "one-two-three-four-five-six-seven-eight"},
		{name: "no words", text: "--- ***", want: ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := Slugify(testCase.text); got != testCase.want {
				t.Fatalf("Slugify(%q) = %q, want %q", testCase.text, got, testCase.want)
			}
		})
	}
}

// TestAtomSlugBaseStripsMarkdownSyntax proves a slug comes from the words a
// reader sees, not from the characters that mark the block's kind.
func TestAtomSlugBaseStripsMarkdownSyntax(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		nodeType string
		text     string
		want     string
	}{
		{name: "heading", nodeType: "Heading", text: "## Decisions waiting on me", want: "decisions-waiting-on-me"},
		{name: "closed heading", nodeType: "Heading", text: "# Running todo #", want: "running-todo"},
		{name: "bullet", nodeType: "List", text: "- Ship the rollout doc", want: "ship-the-rollout-doc"},
		{name: "ordered", nodeType: "List", text: "2. Stale bead", want: "stale-bead"},
		{name: "task list", nodeType: "List", text: "- [ ] Call the vendor", want: "call-the-vendor"},
		{name: "block quote", nodeType: "Blockquote", text: "> Quoted claim", want: "quoted-claim"},
		{name: "emphasis and code", nodeType: "Paragraph", text: "**Stale bead.** `atomdown-8og` is closed", want: "stale-bead-atomdown-8og-is-closed"},
		{name: "link text wins", nodeType: "Paragraph", text: "See [the rollout doc](https://example.com/a/b)", want: "see-the-rollout-doc"},
		{name: "first line with words", nodeType: "Paragraph", text: "\n\nSecond line has the words\n", want: "second-line-has-the-words"},
		{name: "wordless block falls back", nodeType: "ThematicBreak", text: "---", want: "break"},
		{name: "unknown wordless kind", nodeType: "CustomBlock", text: "***", want: "custom-block"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got := atomSlugBase(Atom{Text: testCase.text, NodeType: testCase.nodeType})
			if got != testCase.want {
				t.Fatalf("atomSlugBase(%q) = %q, want %q", testCase.text, got, testCase.want)
			}
		})
	}
}

// TestSlugifyCapsAtAWordBoundary proves the length cap never cuts a word in
// half, because a half word reads as a typo rather than as a short name.
func TestSlugifyCapsAtAWordBoundary(t *testing.T) {
	slug := Slugify("Deliverability Reputation Monitoring Escalation Runbook")
	if len(slug) > SlugMaxLength {
		t.Fatalf("slug %q is %d characters, over the %d cap", slug, len(slug), SlugMaxLength)
	}
	if !IsCanonicalSlug(slug) {
		t.Fatalf("slug %q is not canonical", slug)
	}
	for _, word := range strings.Split(slug, "-") {
		if !strings.Contains(strings.ToLower("Deliverability Reputation Monitoring Escalation Runbook"), word) {
			t.Fatalf("slug %q holds truncated word %q", slug, word)
		}
	}
}

// TestSlugifyCutsAnOverlongSingleWord covers the one case with no boundary
// to cut at.
func TestSlugifyCutsAnOverlongSingleWord(t *testing.T) {
	slug := Slugify(strings.Repeat("a", SlugMaxLength+20))
	if len(slug) != SlugMaxLength {
		t.Fatalf("slug is %d characters, want %d", len(slug), SlugMaxLength)
	}
}

func TestMaterializeSlugsNamesAtomsAndGroups(t *testing.T) {
	source := []byte(`<!-- <atomdown version="1"/> -->

<!-- <atom-group id="3G7K9R5V"> -->

<!-- <atom id="4P8W2H6K"/> -->

## Decisions waiting on me

<!-- <atom id="9R3C7M5D"/> -->

The first regional rollout finished in March.

<!-- </atom-group> -->
`)
	result, marked, slugged, err := MaterializeSlugs(source, SlugOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if marked != 0 {
		t.Fatalf("marked = %d, want 0", marked)
	}
	if slugged != 3 {
		t.Fatalf("slugged = %d, want 3", slugged)
	}
	// The group takes its name from the first heading inside it. A group has
	// no text of its own, and the heading is what a person calls the section.
	for _, want := range []string{
		`<atom-group id="3G7K9R5V" slug="decisions-waiting-on-me">`,
		`<atom id="4P8W2H6K" slug="decisions-waiting-on-me-2"/>`,
		`<atom id="9R3C7M5D" slug="the-first-regional-rollout-finished-in-march"/>`,
	} {
		if !strings.Contains(string(result), want) {
			t.Fatalf("output is missing %q:\n%s", want, result)
		}
	}
}

// TestMaterializeSlugsGroupFallsBackToItsFirstAtom covers a group with no
// heading in it.
func TestMaterializeSlugsGroupFallsBackToItsFirstAtom(t *testing.T) {
	source := []byte(`<!-- <atomdown version="1"/> -->

<!-- <atom-group id="3G7K9R5V"> -->

<!-- <atom id="4P8W2H6K"/> -->

Retries use exponential backoff.

<!-- </atom-group> -->
`)
	result, _, _, err := MaterializeSlugs(source, SlugOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), `<atom-group id="3G7K9R5V" slug="retries-use-exponential-backoff">`) {
		t.Fatalf("group did not slug from its first atom:\n%s", result)
	}
}

// TestMaterializeSlugsKeepsAHandWrittenSlug is the rule the feature is
// built around: an author's own wording for a block outranks anything a
// generator can derive, so nothing overwrites it without --force-slugs.
func TestMaterializeSlugsKeepsAHandWrittenSlug(t *testing.T) {
	source := []byte(`<!-- <atomdown version="1"/> -->

<!-- <atom-group id="3G7K9R5V" slug="resea"> -->

<!-- <atom id="4P8W2H6K" slug="tonight"/> -->

## RESEA tickets - due tonight

<!-- </atom-group> -->
`)
	result, _, slugged, err := MaterializeSlugs(source, SlugOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if slugged != 0 {
		t.Fatalf("slugged = %d, want 0", slugged)
	}
	if string(result) != string(source) {
		t.Fatalf("a hand-written slug was rewritten:\n%s", result)
	}

	forced, _, forcedCount, err := MaterializeSlugs(source, SlugOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if forcedCount != 2 {
		t.Fatalf("forced slug count = %d, want 2", forcedCount)
	}
	if !strings.Contains(string(forced), `slug="resea-tickets-due-tonight"`) {
		t.Fatalf("--force-slugs did not replace the slugs:\n%s", forced)
	}
}

// TestMaterializeSlugsAvoidsAHandWrittenSlug proves an existing slug is
// reserved before anything is minted, so a generated slug never silently
// collides with one the author wrote.
func TestMaterializeSlugsAvoidsAHandWrittenSlug(t *testing.T) {
	source := []byte(`<!-- <atomdown version="1"/> -->

<!-- <atom id="4P8W2H6K" slug="findings"/> -->

Reserved by hand.

<!-- <atom id="9R3C7M5D"/> -->

## Findings

`)
	result, _, _, err := MaterializeSlugs(source, SlugOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), `<atom id="9R3C7M5D" slug="findings-2"/>`) {
		t.Fatalf("generated slug collided with the hand-written one:\n%s", result)
	}
}

// TestMaterializeSlugsLeavesTheDigestAlone is the load-bearing guarantee.
// A digest covers an atom's block bytes and never a byte of a directive
// (SPEC.md "Content digest"), so writing a slug cannot invalidate one. The
// attribute also lands in the specified id, slug, digest order.
func TestMaterializeSlugsLeavesTheDigestAlone(t *testing.T) {
	source := []byte("<!-- <atomdown version=\"1\"/> -->\n\n<!-- <atom id=\"4P8W2H6K\"/> -->\n\nThe regional rollout continued through April.\n")
	digested, _, _, err := MaterializeDigest(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(Drift(digested)) != 0 {
		t.Fatal("digest pass produced drift")
	}

	slugged, _, count, err := MaterializeSlugs(digested, SlugOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("slugged = %d, want 1", count)
	}
	if drifted := Drift(slugged); len(drifted) != 0 {
		t.Fatalf("writing a slug changed the digest: %#v", drifted)
	}

	before := Parse(digested).Atoms[0].Digest
	after := Parse(slugged).Atoms[0]
	if after.Digest != before {
		t.Fatalf("digest = %q, want %q", after.Digest, before)
	}
	if !strings.Contains(string(slugged), `id="4P8W2H6K" slug="the-regional-rollout-continued-through-april" digest=`) {
		t.Fatalf("attributes are not in id, slug, digest order:\n%s", slugged)
	}
}

// TestMaterializeSlugsKeepsAWrappedDirectiveWrapped holds the writer to the
// same layout rule emit follows: the attribute set changed, so the sequence
// is rebuilt, but the shape the author gave the directive survives.
func TestMaterializeSlugsKeepsAWrappedDirectiveWrapped(t *testing.T) {
	source := []byte("<!-- <atomdown version=\"1\"/> -->\n\n<!--\n  <atom\n    id=\"4P8W2H6K\"\n    acme-owner=\"ada\"\n  />\n-->\n\n## Findings\n")
	result, _, _, err := MaterializeSlugs(source, SlugOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := "<!--\n  <atom\n    id=\"4P8W2H6K\"\n    slug=\"findings\"\n    acme-owner=\"ada\"\n  />\n-->"
	if !strings.Contains(string(result), want) {
		t.Fatalf("wrapped directive lost its shape:\n%s", result)
	}
}

// TestMaterializeSlugsMarksAnImplicitAtomFirst proves --slugs composes with
// the ordinary materialize pass: a block with no marker gets one, and then a
// slug in it.
func TestMaterializeSlugsMarksAnImplicitAtomFirst(t *testing.T) {
	source := []byte("## Findings\n\nOne paragraph.\n")
	result, marked, slugged, err := MaterializeSlugs(source, SlugOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if marked != 2 || slugged != 2 {
		t.Fatalf("marked = %d, slugged = %d, want 2 and 2", marked, slugged)
	}
	document := Parse(result)
	if document.HasErrors() {
		t.Fatalf("result has lint errors: %#v", document.Diagnostics)
	}
	if document.Atoms[0].Slug != "findings" || document.Atoms[1].Slug != "one-paragraph" {
		t.Fatalf("slugs = %q, %q", document.Atoms[0].Slug, document.Atoms[1].Slug)
	}
}

func TestMaterializeSlugsIsIdempotentAndPreservesMarkdown(t *testing.T) {
	source := []byte("## Findings\n\nOne paragraph.\n\n---\n\n## Findings\n")
	first, _, _, err := MaterializeSlugs(source, SlugOptions{})
	if err != nil {
		t.Fatal(err)
	}
	second, _, count, err := MaterializeSlugs(first, SlugOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("second run wrote %d slugs, want 0", count)
	}
	if string(second) != string(first) {
		t.Fatalf("second run changed the document:\n%s", second)
	}
	if string(Strip(first)) != string(Strip(source)) {
		t.Fatal("visible Markdown changed")
	}

	document := Parse(first)
	// The thematic break holds no words, so it takes the fallback name, and
	// the repeated heading takes a collision suffix.
	if document.Atoms[2].Slug != "break" {
		t.Fatalf("thematic break slug = %q, want %q", document.Atoms[2].Slug, "break")
	}
	if document.Atoms[3].Slug != "findings-2" {
		t.Fatalf("repeated heading slug = %q, want %q", document.Atoms[3].Slug, "findings-2")
	}
}

func TestMaterializeSlugsAreUniqueAndCanonical(t *testing.T) {
	source := []byte(strings.Repeat("## Findings\n\n", 12))
	result, _, _, err := MaterializeSlugs(source, SlugOptions{})
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, atom := range Parse(result).Atoms {
		if !IsCanonicalSlug(atom.Slug) {
			t.Fatalf("slug %q is not canonical", atom.Slug)
		}
		if seen[atom.Slug] {
			t.Fatalf("slug %q was generated twice", atom.Slug)
		}
		seen[atom.Slug] = true
	}
	if len(seen) != 12 {
		t.Fatalf("generated %d unique slugs, want 12", len(seen))
	}
}

// TestSlugCollisionSuffixFitsTheCap covers the case where the suffix would
// push a capped base slug over the limit: the base is shortened, not the
// suffix.
func TestSlugCollisionSuffixFitsTheCap(t *testing.T) {
	registry := newSlugRegistry()
	base := strings.Repeat("word-", 20) + "end"
	for index := 0; index < 3; index++ {
		slug := registry.mint(Slugify(base))
		if !IsCanonicalSlug(slug) {
			t.Fatalf("slug %q is not canonical", slug)
		}
		if len(slug) > SlugMaxLength {
			t.Fatalf("slug %q is %d characters, over the %d cap", slug, len(slug), SlugMaxLength)
		}
	}
}

func TestLintReportsDuplicateSlugAsAWarning(t *testing.T) {
	source := []byte(`<!-- <atomdown version="1"/> -->

<!-- <atom id="4P8W2H6K" slug="findings"/> -->

First.

<!-- <atom id="9R3C7M5D" slug="findings"/> -->

Second.
`)
	document := Parse(source)
	// A duplicate slug must never be an error: SPEC.md says the slug is not
	// identity, so this document is valid and a reader has to accept it.
	if document.HasErrors() {
		t.Fatalf("duplicate slug produced an error: %#v", document.Diagnostics)
	}
	found := false
	for _, diagnostic := range document.Diagnostics {
		if diagnostic.Code != "duplicate-slug" {
			continue
		}
		found = true
		if diagnostic.Severity != SeverityWarning {
			t.Fatalf("duplicate-slug severity = %q, want warning", diagnostic.Severity)
		}
	}
	if !found {
		t.Fatalf("no duplicate-slug diagnostic: %#v", document.Diagnostics)
	}
}

func TestLintReportsANonCanonicalSlug(t *testing.T) {
	source := []byte("<!-- <atomdown version=\"1\"/> -->\n\n<!-- <atom id=\"4P8W2H6K\" slug=\"Q3 Findings\"/> -->\n\nFirst.\n")
	document := Parse(source)
	if document.HasErrors() {
		t.Fatalf("a loose slug produced an error: %#v", document.Diagnostics)
	}
	for _, diagnostic := range document.Diagnostics {
		if diagnostic.Code == "non-canonical-slug" {
			if diagnostic.Severity != SeverityWarning {
				t.Fatalf("non-canonical-slug severity = %q, want warning", diagnostic.Severity)
			}
			return
		}
	}
	t.Fatalf("no non-canonical-slug diagnostic: %#v", document.Diagnostics)
}

// TestLintSharesOneSlugNamespace proves an atom and a group carrying the
// same slug is reported, because a selector cannot tell them apart either.
func TestLintSharesOneSlugNamespace(t *testing.T) {
	source := []byte(`<!-- <atomdown version="1"/> -->

<!-- <atom-group id="3G7K9R5V" slug="findings"> -->

<!-- <atom id="4P8W2H6K" slug="findings"/> -->

First.

<!-- </atom-group> -->
`)
	for _, diagnostic := range Parse(source).Diagnostics {
		if diagnostic.Code == "duplicate-slug" {
			return
		}
	}
	t.Fatal("an atom and a group sharing a slug was not reported")
}
