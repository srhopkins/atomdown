# Atomdown

Atomdown adds persistent block IDs, groups, and extensible metadata to Markdown. Atomdown documents remain valid CommonMark.

Use Atomdown when people and AI agents need to address and review individual Markdown blocks. Visible Markdown remains the source of truth.

The `materialize` command splits a document into addressable blocks, one atom marker per top-level block. Some people call this step chunking a document for agent consumption.

Atomdown uses XML-shaped directives inside HTML comments:

```markdown
<!-- <atomdown version="1"/> -->
<!-- <atom id="4P8W2H6K" slug="launch-claim"/> -->

The product launched in March.
```

Normal Markdown tools treat each directive as an invisible comment. Atomdown tools read the same directive as block metadata. This holds for every directive placement except one: deliberately splitting a list with `--split` (see the tradeoff under "Splitting a list into per-item atoms" below), which does change the rendered HTML on purpose.

## Why Atomdown

Many block editors store documents as application-owned JSON. Atomdown keeps Markdown canonical and adds a small annotation layer.

Atomdown supports:

- Stable Markdown block IDs for links, comments, and review workflows.
- Ordered groups of addressable blocks.
- Application metadata that core tools preserve but do not interpret.
- Lossless token output for editors and embedded applications.

Core defines only `version`, `id`, and `slug`. Applications extend Atomdown with their own XML attributes:

```markdown
<!-- <atom id="4P8W2H6K" acme-owner="research"/> -->

The product launched in March.
```

The `acme-owner` attribute is an application-defined example. It is not part of Atomdown Core. Core tools preserve it and do not interpret it.

A directive with many attributes can wrap across several source lines, so a long attribute list stays readable in an editor and in a diff:

```markdown
<!--
  <atom
    id="4P8W2H6K"
    slug="launch-claim"
    acme-owner="research"
  />
-->

The product launched in March.
```

Only whitespace can share the directive's first line before `<!--` or its last line after `-->`. Whitespace inside a directive carries no meaning, because a content digest covers the atom's block only and never the directive, so wrapping or unwrapping a directive changes nothing a tool reads. See SPEC.md "Core directives" for the rule and the reasoning behind it.

Atomdown Core 1 is an early specification. Use the current syntax and conformance corpus for experiments and review.

## Design

- Documents remain valid CommonMark.
- Directives use XML 1.0 syntax inside HTML comments.
- The normalized metadata model uses XML Schema Definition (XSD) 1.0.
- Unknown XML attributes extend Atomdown without changing Core.
- Core defines identity, grouping, order, and preservation rules.

The implementation is a pure-Go library and command-line interface (CLI). It uses `encoding/xml` and the pure-Go goldmark parser. It does not use CGO.

## Install

```bash
go install github.com/srhopkins/atomdown/cmd/atomdown@latest
```

Prebuilt binaries for macOS and Linux are on the [releases page](https://github.com/srhopkins/atomdown/releases).

## CLI

Run the CLI from the repository:

```bash
go run ./cmd/atomdown lint testdata/example.md
go run ./cmd/atomdown parse testdata/example.md
go run ./cmd/atomdown emit document.json
go run ./cmd/atomdown tokens testdata/example.md
go run ./cmd/atomdown xml testdata/example.md
go run ./cmd/atomdown strip testdata/example.md
go run ./cmd/atomdown materialize testdata/example.md
go run ./cmd/atomdown materialize --split list-item testdata/example.md
go run ./cmd/atomdown materialize --digest testdata/example.md
go run ./cmd/atomdown materialize --slugs testdata/example.md
go run ./cmd/atomdown get launch-claim testdata/example.md
go run ./cmd/atomdown drift testdata/example.md
go run ./cmd/atomdown id
```

Each file command accepts one file. Use `-` or omit the file to read standard input.

- `lint` checks syntax, IDs, block associations, and groups. It also reports a directive that silently changed the document's rendered block structure (see `--split` below).
- `lint --strict` also reports unmarked top-level blocks and a missing document version directive. Default lint permits mixed documents so teams can adopt Atomdown in stages.
- `parse` writes the semantic document model as JSON.
- `emit` writes marked Markdown from `parse` JSON. Agents can edit the model and write it back. Each directive keeps the source layout its author gave it: a directive the author wrapped over several lines comes back wrapped, at the same indentation, byte for byte. See "How `emit` treats directive layout" below.
- `emit --flatten` rewrites every directive to one line in canonical attribute order. Use it to canonicalize a document on purpose.
- `tokens` writes a lossless stream of Markdown, whitespace, and Atomdown directives.
- `xml` writes the normalized XML metadata model.
- `strip` removes Atomdown directives and writes plain Markdown.
- `materialize` splits a document into addressable blocks by adding a new atom marker before each unmarked top-level block. It also adds the document version directive at the top of the file when the source does not already declare one. Use `materialize -w FILE` to update the file in place. It reports how many blocks it marked, on stderr, so a piped stdout run stays clean.
- `materialize --split <node-types>` gives finer granularity than one atom per top-level block. `<node-types>` is a comma-separated list of CommonMark node names; today only `list-item` is accepted. An unknown name exits non-zero and names the accepted values. See "Splitting a list into per-item atoms" below before you use it.
- `materialize --digest` adds a Core content digest to every atom that does not already have one, so a later `drift` run can tell whether the block changed since this run. It never touches an atom that already has a digest. See "Detecting content drift" below.
- `materialize --slugs` adds a generated readable slug to every atom and atom group that does not already have one, so a person can name a block without reading an ID. It never touches a slug that is already there. `materialize --force-slugs` replaces every slug with a generated one. See "Generating readable slugs" below.
- `get <selector>` prints one atom: its resolved ID, its slug, its group, its directive, and its block text. A `<selector>` is an atom ID, an atom or atom-group slug, or `slug:<name>`. It is read-only. See "Naming one atom with a selector" below.
- `drift` (also `verify`) reports which atom IDs have a digest that no longer matches their content, and exits non-zero when it finds any. An atom with no digest is not checked.
- `id` creates an eight-character Crockford Base32 ID.

### How `emit` treats directive layout

A directive can span several source lines, so an agent that edits the model and writes it back has to answer what happens to the author's line breaks. `emit` answers it this way:

- A directive whose attributes are unchanged comes back exactly as the author wrote it. Line breaks, interior whitespace, and indentation all survive, and the attribute order is the author's, not a canonical one.
- A directive with an added, removed, or changed attribute keeps its authored shape and rebuilds only the attribute sequence. A wrapped directive stays wrapped, each attribute keeps the author's indentation, and the closing token keeps its own line. A one-line directive stays on one line. The attribute order becomes canonical, because an attribute that just arrived has no authored position.
- `emit --flatten` writes every directive on one line, whether it changed or not.

`emit` normalizes the blank lines between Markdown blocks, so a whole file is not guaranteed byte-identical across a parse and emit cycle. The directive text is.

### Splitting a list into per-item atoms

By default `materialize` marks one atom per top-level block, so a bullet list of six items gets one atom for the whole list. Review workflows often want to address a single bullet. `materialize --split list-item` does this:

```bash
go run ./cmd/atomdown materialize --split list-item -w criteria.md
```

```markdown
<!-- <atomdown version="1"/> -->
<!-- <atom-group id="KGB6SPB0"> -->
<!-- <atom id="WME9C5F7"/> -->
* A failed charge retries a maximum of three times.
<!-- <atom id="2H55ECG6"/> -->
* Retries use exponential backoff starting at 30 seconds.
<!-- </atom-group> -->
```

**How it works.** Atomdown never puts a directive inside a container block such as a list item; SPEC.md forbids it. Instead, a directive sits on its own line or lines between two adjacent items with no blank line, which is enough to end one CommonMark list and start the next. Each item becomes its own top-level list with its own atom. Visible Markdown text does not change; `strip` still reconstructs the original file byte for byte.

**The tradeoff.** Splitting turns one list into N single-item lists. Rendered HTML changes from one `<ul>` with N `<li>` elements to N separate `<ul>` elements, one per item; spacing and what a screen reader announces both change. Nothing else about the document changes. This is why `--split` is opt-in, never the default.

**The atom-group is load-bearing.** `--split` always wraps the split items in one `atom-group`. The group records that the items belonged to one list, and it is the only way to tell a deliberate split from an accidental one, since the two are otherwise byte-identical to a parser. `lint` warns when it finds split single-item lists that are not wrapped in a shared atom-group (`directive-splits-list`), because that pattern silently changes rendered structure without recording that the change was deliberate. Running `materialize --split list-item` again is a no-op: a list already split to one item per group is left alone.

**Two limits.** A nested list item's children are not individually addressable: `--split list-item` only splits the top-level items of a list, so a parent item's atom still covers every child nested under it. A GFM table's rows are not addressable at all; a table always gets one atom for the whole table (tracked separately, see the "materialize --split table-row" issue).

### Generating readable slugs

An atom ID is eight random Crockford Base32 characters. That is the right
shape for identity and the wrong shape for a person: grouping a page of
sections by ID means looking up every one of them. The `slug` attribute has
always been in Core for exactly this, as a readable alias that is not
identity. `materialize --slugs` fills it in.

```bash
go run ./cmd/atomdown materialize --slugs -w running.md
```

```markdown
<!-- <atomdown version="1"/> -->
<!-- <atom-group id="NS67J8K5" slug="resea-tickets-due-tonight"> -->
<!-- <atom id="QQE8MK3D" slug="resea-tickets-due-tonight-2"/> -->
## RESEA tickets - due tonight
<!-- </atom-group> -->
```

**Where the slug comes from.** From the item's own content, never from
anything outside it. An atom slugs from the first line of its text that has
words in it, with the Markdown that marks the block's kind stripped off:
heading hashes, list bullets and numbers, task-list checkboxes, block-quote
markers, emphasis, code spans, and a link's URL. So a heading slugs from
its heading text, and a paragraph slugs from its opening words.

An atom group slugs from **the first heading inside it**, and from its first
atom when the group has no heading. A group carries no text of its own, so
there is nothing else to name it by, and the heading a section opens with is
what a person calls that section anyway. This is the case the feature exists
for.

A block with no words at all — a thematic break, a fence of symbols — takes
a name for its kind (`break`, `code`, `table`) rather than no slug.

**The shape.** Lowercase ASCII kebab-case: `[a-z0-9]` in groups joined by
single hyphens, no leading or trailing hyphen. At most 8 words and at most
**48 characters**, cut at a hyphen so a truncated slug never ends mid-word.
48 is long enough for a real heading and short enough to stay readable in a
terminal column and typeable on a command line. Accented Latin letters fold
to ASCII (`Café` becomes `cafe`); anything else becomes a word separator.

**Slugs are unique within the document.** A collision takes the lowest free
`-2`, `-3`, ... suffix, and atoms and groups share one slug namespace, so a
selector never has to say which kind it means. Uniqueness is this tool's
stance and not a format rule: SPEC.md says the slug is not identity, so a
document with two identical slugs is valid and every reader must accept
one. The format permits duplicates; the tooling declines to create them and
`lint` warns when it finds one.

**A hand-written slug is never overwritten.** An existing slug is reserved
before any slug is minted, so a generated slug never collides with one you
wrote, and an item that already has a slug is left byte for byte alone. Your
wording for a block beats anything a generator can derive from it. Use
`materialize --force-slugs` when you actually want them replaced.

**No digest changes.** A digest covers an atom's block bytes and never a
byte of a directive, so writing a slug into a directive cannot invalidate
one. Running `--slugs` over a digested file leaves `drift` clean.

Unlike `--digest`, which appends its attribute and leaves every other byte
untouched, a slug write rebuilds the directive's attribute sequence. It has
to: SPEC.md orders the identity attributes `id`, `slug`, `digest`, and an
atom that already carries a digest leaves no gap to splice a slug into. The
authored skeleton still survives — a wrapped directive stays wrapped at your
indentation — which is the same rule `emit` follows for a directive whose
attribute set changed.

`lint` reports two slug problems, and neither is an error, because neither
describes an invalid document:

- `duplicate-slug` is a **default-lint warning**. A duplicate slug names no
  single item, so a selector that hits it cannot resolve. That is a defect
  at any stage of Atomdown adoption, which is why it is not hidden behind
  `--strict`.
- `non-canonical-slug` is a **`--strict`-only warning**: a slug that is not
  lowercase kebab-case within 48 characters. It still resolves, uniquely,
  and Core left room for a loose value on purpose, so reporting it by
  default would nag every author who wrote a slug by hand.

### Naming one atom with a selector

```bash
go run ./cmd/atomdown get resea-tickets-due-tonight running.md
go run ./cmd/atomdown get QQE8MK3D running.md
go run ./cmd/atomdown get slug:4P8W2H6K running.md
go run ./cmd/atomdown get resea-tickets-due-tonight --json running.md
```

`get` prints one atom's resolved ID, slug, group, directive, and block text.
It reads the file and changes nothing.

A `<selector>` is an atom ID, an atom or atom-group slug, or a slug with an
explicit `slug:` prefix. The precedence is fixed:

1. A bare selector is matched against atom IDs **first**. An ID is identity
   and a slug is not, so an ID always wins.
2. A bare selector that matches no ID is matched against slugs: an atom's
   own slug first, then an atom-group's slug, which resolves to the group's
   first atom. An atom's own slug is the closer match, so a group is only
   consulted when no atom carries the name.
3. A `slug:` selector skips step 1 entirely, which is how you reach a slug
   that happens to look like an ID.

**An ambiguous slug is an error, never a silent pick.** A slug naming
several atoms exits non-zero and lists every candidate ID, so you can
re-run with the one you meant. Picking one would defeat the only reason to
name an atom by slug, which is knowing which atom you named.

This is the smallest useful selector surface on purpose. The rules live in
one library function, `atomdown.Resolve`, so a later command that moves or
regroups an atom accepts exactly the same spellings without restating them.

### Detecting content drift

An atom ID answers "is this the same block?"; it stays stable across an edit by design. A content digest answers the opposite question, "did this block's text change?" Together they support a review workflow: approve a block by ID, then later confirm its content still matches what was approved.

```bash
go run ./cmd/atomdown materialize --digest -w reviewed.md
# ... time passes, someone edits reviewed.md ...
go run ./cmd/atomdown drift reviewed.md
```

```markdown
<!-- <atomdown version="1"/> -->
<!-- <atom id="4P8W2H6K" digest="sha256:bf7180cdede996da9d68106d9acfcfd2e5aacc0abe2f7fe3adb3cbecfd27f1be"/> -->
The regional rollout continued through April.
```

`materialize --digest` writes a digest to every atom that does not already have one; it never touches one that does. `drift` recomputes each digested atom's content digest and reports every atom ID whose recorded digest no longer matches, then exits non-zero. It does not show what changed inside the block; a diff already does that well.

**Nothing refreshes a digest automatically, ever.** Not `materialize` without `--digest`, not `lint`, nothing. A digest means "someone reviewed this exact content"; a tool that silently updates it when content changes turns that signal into a value that always matches, which tells a reviewer nothing. To mark a block reviewed again after a real edit, remove its `digest` attribute and run `materialize --digest` again.

The digest covers the atom's block bytes exactly as written, including whitespace: reflowing a paragraph or changing indentation counts as drift, and there is no partial-match score, only "changed" or "unchanged". See SPEC.md "Content digest" for the exact byte range and the one line-ending normalization the algorithm performs.

## Go library

Parse and validate a document:

```go
document := atomdown.Parse(source)
if document.HasErrors() {
    // Reject or repair the source.
}
```

Register an extension in an embedded application:

```go
processor := atomdown.NewProcessor(myExtension)
document, err := processor.Process(ctx, source)
```

Extensions implement `atomdown.Extension`. Extensions run in registration order and can add metadata or diagnostics.

Atomdown does not use Go's runtime `plugin` package. This choice keeps the library portable and compatible with `CGO_ENABLED=0`.

## Standard and tests

- [`SPEC.md`](SPEC.md) defines Atomdown Core 1.
- [`schema/atomdown-1.xsd`](schema/atomdown-1.xsd) defines the normalized XML model.
- [`testdata/`](testdata/) provides valid, mixed, malformed, and exact golden files.
- [`conformance/`](conformance/) provides a language-neutral test suite. Second implementations run it without Go.
- [`llms.txt`](llms.txt) gives AI agents a short guide to the repository.

## Contributing

Read [`CONTRIBUTING.md`](CONTRIBUTING.md), then open a Bug report or Proposal. Coding agents must also read [`AGENTS.md`](AGENTS.md).

## License

Atomdown uses the [MIT License](LICENSE).
