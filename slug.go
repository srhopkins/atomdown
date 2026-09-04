package atomdown

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// SlugMaxLength is the character cap on a generated slug.
//
// The cap exists for the reader, not the parser: Core puts no limit on a
// slug's length, and a hand-written slug of any length stays valid. 48 is
// long enough to hold a real heading ("shauns-link-not-found",
// "email-ses-pull-requests") and short enough to stay readable in a
// terminal column, a URL fragment, and a command line a person types. A
// generated slug is truncated at a word boundary, so the cap never cuts a
// word in half.
const SlugMaxLength = 48

// slugWordLimit bounds how many words a generated slug draws from an atom's
// text. A heading is usually shorter than this; the limit matters for a
// paragraph, where the point is a recognizable prefix rather than a summary.
const slugWordLimit = 8

// slugPattern is the documented shape of a generated slug: lowercase ASCII
// letters and digits in groups separated by single hyphens, with no leading
// or trailing hyphen.
//
// Core does not require this shape — SPEC.md says the slug is a readable
// alias and not identity — so lint reports a slug outside it only under
// --strict. Everything this package generates matches it.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// IsCanonicalSlug reports whether value has the shape atomdown generates:
// lowercase kebab-case ASCII within SlugMaxLength characters.
func IsCanonicalSlug(value string) bool {
	return len([]rune(value)) <= SlugMaxLength && slugPattern.MatchString(value)
}

// slugFoldings maps the accented Latin letters that appear in ordinary
// English-language prose to their unaligned ASCII equivalents. A slug is
// ASCII by design, so a heading with a name or a loan word in it still
// produces a slug a person can type. Anything not listed here is dropped
// and becomes a word separator, which is the same outcome a symbol gets.
var slugFoldings = map[rune]string{
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'æ': "ae",
	'ç': "c", 'è': "e", 'é': "e", 'ê': "e", 'ë': "e",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i",
	'ñ': "n", 'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o",
	'ß': "ss", 'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ý': "y", 'ÿ': "y",
	'œ': "oe", 'đ': "d", 'ł': "l", 'š': "s", 'ž': "z", 'č': "c",
}

// Slugify turns arbitrary text into the canonical slug shape: lowercase
// ASCII kebab-case, at most slugWordLimit words, at most SlugMaxLength
// characters, cut at a word boundary. It returns an empty string when the
// text holds no letters or digits at all.
func Slugify(text string) string {
	var builder strings.Builder
	for _, symbol := range strings.ToLower(text) {
		switch {
		case symbol < 128 && (unicode.IsLetter(symbol) || unicode.IsDigit(symbol)):
			builder.WriteRune(symbol)
		default:
			if folded, exists := slugFoldings[symbol]; exists {
				builder.WriteString(folded)
				continue
			}
			builder.WriteByte(' ')
		}
	}

	words := strings.Fields(builder.String())
	if len(words) > slugWordLimit {
		words = words[:slugWordLimit]
	}
	return capSlug(strings.Join(words, "-"))
}

// capSlug trims a slug to SlugMaxLength at a hyphen, so a truncated slug
// never ends mid-word. A first word longer than the cap on its own is cut
// hard, because there is no boundary to cut at.
func capSlug(slug string) string {
	if len(slug) <= SlugMaxLength {
		return slug
	}
	cut := slug[:SlugMaxLength]
	if boundary := strings.LastIndexByte(cut, '-'); boundary > 0 {
		return cut[:boundary]
	}
	return cut
}

// slugSourceText picks the text a generated slug derives from: the atom's
// own content, with the Markdown syntax that carries no words stripped off.
// For a heading that leaves the heading text, which is what a person reads
// as the block's name. For any other block it leaves the first line's
// words, so the slug is a recognizable prefix.
func slugSourceText(atom Atom) string {
	for _, line := range strings.Split(atom.Text, "\n") {
		stripped := stripMarkdownSyntax(line)
		if strings.TrimSpace(stripped) != "" {
			return stripped
		}
	}
	return ""
}

// markdownLinkPattern matches an inline link or image and captures its
// visible text, so a heading that is a link slugs from the words a reader
// sees rather than from the URL.
var markdownLinkPattern = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)

// markdownLeaderPattern matches the block leader of a heading, list item,
// task-list checkbox, or block quote: the characters that mark the block's
// kind and are not part of its words.
var markdownLeaderPattern = regexp.MustCompile(`^[ \t]*(?:>[ \t]*)*(?:#{1,6}[ \t]+|(?:[*+-]|[0-9]{1,9}[.)])[ \t]+(?:\[[ xX]\][ \t]+)?)?`)

func stripMarkdownSyntax(line string) string {
	line = markdownLinkPattern.ReplaceAllString(line, "$1")
	line = markdownLeaderPattern.ReplaceAllString(line, "")
	// Emphasis, code spans, and heading closing sequences wrap words without
	// being words. Dropping the characters joins the words they hugged.
	return strings.NewReplacer("*", " ", "_", " ", "`", " ", "~", " ", "#", " ").Replace(line)
}

// slugFallbacks names a block that holds no words at all — a thematic
// break, a fence of symbols — so it still gets a readable slug rather than
// none. A number suffix from the uniqueness pass distinguishes several of
// the same kind.
var slugFallbacks = map[string]string{
	"ThematicBreak":   "break",
	"FencedCodeBlock": "code",
	"CodeBlock":       "code",
	"HTMLBlock":       "html",
	"Table":           "table",
}

func slugFallback(nodeType string) string {
	if name, exists := slugFallbacks[nodeType]; exists {
		return name
	}
	if slug := Slugify(nodeType); slug != "" {
		return slug
	}
	return "atom"
}

// slugRegistry mints slugs that are unique inside one document.
//
// Uniqueness is a tooling stance, not a format rule: SPEC.md says the slug
// is not identity, so a document with two identical slugs is valid and a
// reader must accept it. Generating one is still a mistake, because the
// whole reason to have a slug is that a person can name one atom with it.
// So the generator refuses to mint a duplicate and lint warns about one it
// finds.
type slugRegistry struct{ used map[string]struct{} }

func newSlugRegistry() *slugRegistry {
	return &slugRegistry{used: make(map[string]struct{})}
}

// reserve records a slug the generator must not mint, which is every slug
// already written in the document.
func (registry *slugRegistry) reserve(slug string) {
	if slug != "" {
		registry.used[slug] = struct{}{}
	}
}

// mint returns base, or base with the lowest free "-2", "-3", ... suffix
// when base is taken. The base is shortened first when the suffix would
// push the result past the cap, so the result always fits.
func (registry *slugRegistry) mint(base string) string {
	if base == "" {
		base = "atom"
	}
	base = capSlug(base)
	if _, taken := registry.used[base]; !taken {
		registry.used[base] = struct{}{}
		return base
	}
	for counter := 2; ; counter++ {
		suffix := "-" + strconv.Itoa(counter)
		trimmed := base
		if len(trimmed)+len(suffix) > SlugMaxLength {
			trimmed = capSlug(base[:SlugMaxLength-len(suffix)])
			if trimmed == "" {
				trimmed = base[:SlugMaxLength-len(suffix)]
			}
		}
		candidate := trimmed + suffix
		if _, taken := registry.used[candidate]; !taken {
			registry.used[candidate] = struct{}{}
			return candidate
		}
	}
}

// SlugOptions selects how MaterializeSlugs treats the slugs already in the
// document.
type SlugOptions struct {
	// Force replaces every existing slug with a generated one. Without it,
	// an existing slug is never touched: a hand-written slug is the author's
	// own wording for the block, and a generator has no better name for it.
	Force bool
}

// MaterializeSlugs runs the same base pass as Materialize — it adds the
// document version directive when the source lacks one, and a marker with a
// fresh ID before every implicit atom — and additionally writes a generated
// slug to every atom and atom group that does not already carry one.
//
// A slug is derived from the item's own content. An atom slugs from its
// first line of words, with Markdown syntax stripped, so a heading slugs
// from its heading text. An atom group has no text of its own, so it slugs
// from the first heading inside it, and from its first atom's text when the
// group holds no heading. That is the case a person hits: a group is the
// unit worth naming and the hardest one to name by hand.
//
// A slug is lowercase ASCII kebab-case, at most SlugMaxLength characters,
// cut at a word boundary. Slugs are unique within the document: a collision
// takes the lowest free "-2", "-3", ... suffix. Atoms and groups share one
// slug namespace, so a selector never has to say which kind it means.
//
// An existing slug is never rewritten unless SlugOptions.Force is set, and
// an existing slug is reserved before any slug is minted, so a generated
// slug never collides with a hand-written one.
//
// No digest changes. A digest covers an atom's block bytes and never a
// byte of a directive (SPEC.md "Content digest"), so writing a slug into a
// directive cannot alter the value that directive records.
//
// It returns the resulting Markdown, the number of atom markers it added
// (the same count Materialize reports), and the number of slugs it wrote.
func MaterializeSlugs(source []byte, options SlugOptions) ([]byte, int, int, error) {
	// The implicit atoms have to be marked before slugs are written, because
	// a slug goes in a directive and an implicit atom has none. One
	// Materialize pass first, then one slug pass over the result, keeps both
	// steps simple and makes the combined command idempotent.
	marked, markedCount, err := Materialize(source)
	if err != nil {
		return nil, 0, 0, err
	}
	slugged, slugCount, err := writeSlugs(marked, options)
	if err != nil {
		return nil, 0, 0, err
	}
	return slugged, markedCount, slugCount, nil
}

// slugTarget is one directive a slug pass may rewrite, in source order.
type slugTarget struct {
	marker     Range
	name       string
	attributes []Attribute
	base       string
	existing   string
}

func writeSlugs(source []byte, options SlugOptions) ([]byte, int, error) {
	document := Parse(source)
	registry := newSlugRegistry()
	if !options.Force {
		for _, atom := range document.Atoms {
			registry.reserve(atom.Slug)
		}
		for _, group := range document.Groups {
			registry.reserve(group.Slug)
		}
	}

	targets := collectSlugTargets(document)
	var output bytes.Buffer
	cursor := 0
	written := 0

	for _, target := range targets {
		if target.existing != "" && !options.Force {
			continue
		}
		slug := registry.mint(target.base)
		if slug == target.existing {
			continue
		}
		if target.marker.Start.Offset < cursor {
			return nil, 0, fmt.Errorf("materialize --slugs: overlapping directive at offset %d", target.marker.Start.Offset)
		}
		rewritten, err := rewriteDirectiveWithSlug(source, target, slug)
		if err != nil {
			return nil, 0, err
		}
		output.Write(source[cursor:target.marker.Start.Offset])
		output.WriteString(rewritten)
		cursor = target.marker.End.Offset
		written++
	}
	output.Write(source[cursor:])
	return output.Bytes(), written, nil
}

// collectSlugTargets lists every atom and group directive that can carry a
// slug, in source order, each with the base slug its content produces.
// Source order matters: a collision suffix is assigned to whichever item
// comes second in the file, so the same document always produces the same
// slugs.
func collectSlugTargets(document Document) []slugTarget {
	groupBase := groupSlugBases(document)

	targets := make([]slugTarget, 0, len(document.Atoms)+len(document.Groups))
	for _, group := range document.Groups {
		targets = append(targets, slugTarget{
			marker: group.Marker, name: "atom-group", attributes: groupAttributes(group),
			base: groupBase[group.ID], existing: group.Slug,
		})
	}
	for _, atom := range document.Atoms {
		if atom.Implicit || atom.Marker == nil {
			continue
		}
		targets = append(targets, slugTarget{
			marker: *atom.Marker, name: "atom", attributes: atomAttributes(atom),
			base: atomSlugBase(atom), existing: atom.Slug,
		})
	}
	sortSlugTargetsBySource(targets)
	return targets
}

func sortSlugTargetsBySource(targets []slugTarget) {
	for index := 1; index < len(targets); index++ {
		for back := index; back > 0 && targets[back-1].marker.Start.Offset > targets[back].marker.Start.Offset; back-- {
			targets[back-1], targets[back] = targets[back], targets[back-1]
		}
	}
}

func atomSlugBase(atom Atom) string {
	if slug := Slugify(slugSourceText(atom)); slug != "" {
		return slug
	}
	return slugFallback(atom.NodeType)
}

// groupSlugBases derives each group's base slug from the first heading atom
// inside it, falling back to the group's first atom when it holds no
// heading. A group carries no text of its own, so its own content gives
// nothing to name it by; the heading it opens with is what a person calls
// that section.
func groupSlugBases(document Document) map[string]string {
	bases := make(map[string]string, len(document.Groups))
	firstAtom := make(map[string]Atom, len(document.Groups))
	for _, atom := range document.Atoms {
		if atom.GroupID == "" || atom.Implicit {
			continue
		}
		if _, chosen := bases[atom.GroupID]; !chosen && atom.NodeType == "Heading" {
			if slug := Slugify(slugSourceText(atom)); slug != "" {
				bases[atom.GroupID] = slug
			}
		}
		if _, seen := firstAtom[atom.GroupID]; !seen {
			firstAtom[atom.GroupID] = atom
		}
	}
	for _, group := range document.Groups {
		if bases[group.ID] != "" {
			continue
		}
		if atom, exists := firstAtom[group.ID]; exists {
			bases[group.ID] = atomSlugBase(atom)
			continue
		}
		bases[group.ID] = "group"
	}
	return bases
}

func atomAttributes(atom Atom) []Attribute {
	attributes := []Attribute{{Name: "id", Value: atom.ID}}
	attributes = append(attributes, Attribute{Name: "slug"})
	if atom.Digest != "" {
		attributes = append(attributes, Attribute{Name: "digest", Value: atom.Digest})
	}
	return append(attributes, atom.Attributes...)
}

func groupAttributes(group AtomGroup) []Attribute {
	attributes := []Attribute{{Name: "id", Value: group.ID}, {Name: "slug"}}
	return append(attributes, group.Attributes...)
}

// rewriteDirectiveWithSlug rebuilds one directive with the new slug value in
// the canonical attribute position.
//
// Unlike materialize --digest, which appends its attribute and leaves every
// other byte alone, a slug write rebuilds the attribute sequence. It has to:
// SPEC.md puts the identity attributes in the order id, slug, digest, and an
// atom that already carries a digest has no gap to splice a slug into. The
// authored skeleton still survives — a wrapped directive stays wrapped at
// the author's indentation — which is exactly the rule SPEC.md states for a
// directive whose attribute set changed.
func rewriteDirectiveWithSlug(source []byte, target slugTarget, slug string) (string, error) {
	raw := source[target.marker.Start.Offset:target.marker.End.Offset]
	layout, err := parseDirectiveLayout(raw)
	if err != nil {
		return "", fmt.Errorf("materialize --slugs: %w", err)
	}

	attributes := make([]Attribute, 0, len(target.attributes))
	for _, attribute := range target.attributes {
		if attribute.Name == "slug" {
			attribute.Value = slug
		}
		attributes = append(attributes, attribute)
	}
	marker, err := layout.render(target.name, attributes)
	if err != nil {
		return "", fmt.Errorf("materialize --slugs: %w", err)
	}
	return marker, nil
}
