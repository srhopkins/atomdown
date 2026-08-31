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
go run ./cmd/atomdown drift testdata/example.md
go run ./cmd/atomdown id
```

Each file command accepts one file. Use `-` or omit the file to read standard input.

- `lint` checks syntax, IDs, block associations, and groups. It also reports a directive that silently changed the document's rendered block structure (see `--split` below).
- `lint --strict` also reports unmarked top-level blocks and a missing document version directive. Default lint permits mixed documents so teams can adopt Atomdown in stages.
- `parse` writes the semantic document model as JSON.
- `emit` writes marked Markdown from `parse` JSON. Agents can edit the model and write it back.
- `tokens` writes a lossless stream of Markdown, whitespace, and Atomdown directives.
- `xml` writes the normalized XML metadata model.
- `strip` removes Atomdown directives and writes plain Markdown.
- `materialize` splits a document into addressable blocks by adding a new atom marker before each unmarked top-level block. It also adds the document version directive at the top of the file when the source does not already declare one. Use `materialize -w FILE` to update the file in place. It reports how many blocks it marked, on stderr, so a piped stdout run stays clean.
- `materialize --split <node-types>` gives finer granularity than one atom per top-level block. `<node-types>` is a comma-separated list of CommonMark node names; today only `list-item` is accepted. An unknown name exits non-zero and names the accepted values. See "Splitting a list into per-item atoms" below before you use it.
- `materialize --digest` adds a Core content digest to every atom that does not already have one, so a later `drift` run can tell whether the block changed since this run. It never touches an atom that already has a digest. See "Detecting content drift" below.
- `drift` (also `verify`) reports which atom IDs have a digest that no longer matches their content, and exits non-zero when it finds any. An atom with no digest is not checked.
- `id` creates an eight-character Crockford Base32 ID.

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

**How it works.** Atomdown never puts a directive inside a container block such as a list item; SPEC.md forbids it. Instead, a directive sits on its own line between two adjacent items with no blank line, which is enough to end one CommonMark list and start the next. Each item becomes its own top-level list with its own atom. Visible Markdown text does not change; `strip` still reconstructs the original file byte for byte.

**The tradeoff.** Splitting turns one list into N single-item lists. Rendered HTML changes from one `<ul>` with N `<li>` elements to N separate `<ul>` elements, one per item; spacing and what a screen reader announces both change. Nothing else about the document changes. This is why `--split` is opt-in, never the default.

**The atom-group is load-bearing.** `--split` always wraps the split items in one `atom-group`. The group records that the items belonged to one list, and it is the only way to tell a deliberate split from an accidental one, since the two are otherwise byte-identical to a parser. `lint` warns when it finds split single-item lists that are not wrapped in a shared atom-group (`directive-splits-list`), because that pattern silently changes rendered structure without recording that the change was deliberate. Running `materialize --split list-item` again is a no-op: a list already split to one item per group is left alone.

**Two limits.** A nested list item's children are not individually addressable: `--split list-item` only splits the top-level items of a list, so a parent item's atom still covers every child nested under it. A GFM table's rows are not addressable at all; a table always gets one atom for the whole table (tracked separately, see the "materialize --split table-row" issue).

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
