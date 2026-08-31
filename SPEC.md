# Atomdown Core 1

Atomdown is a backward-compatible annotation layer for Markdown. XML-shaped directives add persistent identity, groups, and extensible attributes to top-level Markdown blocks.

## Design requirements

The requirements are in priority order:

1. Each Atomdown document is valid CommonMark.
2. Atomdown directives use XML 1.0 syntax inside HTML comments.
3. The normalized metadata model uses an XML Schema Definition (XSD) 1.0 schema.
4. Core defines only identity, grouping, order, and extension preservation.
5. Unknown XML attributes remain valid and survive a parse and write cycle.
6. Visible Markdown remains the source of truth.

A feature does not belong in Core when the feature violates an earlier requirement.

## Core directives

```markdown
<!-- <atomdown version="1"/> -->
<!-- <atom id="4P8W2H6K" slug="claim"/> -->
<!-- <atom-group id="7K3M9X2D" slug="findings"> -->
<!-- </atom-group> -->
```

The `atomdown` and `atom` directives are self-closing. The `atom-group` directive has an opening marker and a closing marker.

Each directive must occupy one source line. Only whitespace can occur before `<!--` or after `-->`.

Do not put a directive inside another Markdown block. Examples include paragraphs, lists, code fences, tables, and block quotes.

An `atom` directive applies to the next top-level CommonMark block. Source order is canonical.

An `atom-group` contains the explicit atoms between its markers. Atomdown Core 1 does not permit nested groups.

A thematic break (`---`) is a top-level CommonMark block like any other. It has its own source extent: one atom marker before it must not swallow it, and one atom marker before it must not swallow whatever follows it either. Because it is an ordinary top-level block, an atom directive can target it directly; the atom's content is then that one break line, and nothing else.

## Identity

Each explicit atom and atom group requires an eight-character Crockford Base32 `id`.
Use the uppercase Crockford Base32 alphabet only.
IDs must be unique within a document.

Preserve the ID when you move an item. Generate a new ID when you copy an item.

The optional `slug` attribute is a readable alias. The slug is not identity.

## Content digest

An `atom` directive can carry an optional `digest` attribute. It answers a
different question than `id`: `id` answers "is this the same block?" and
must stay stable across an edit; `digest` answers "did this block's text
change since something recorded this value?" and must change when the
content changes. A review workflow needs both.

A `digest` value has the form `sha256:` followed by the 64-character
lowercase hexadecimal SHA-256 sum of the atom's normalized block bytes. The
algorithm name is part of the value so a future second algorithm does not
require a schema change.

**What is hashed.** The digest covers exactly the atom's block content: the
source bytes from the first byte of the block's first line through the
last byte of the block's last line, with two exclusions:

- The atom's own directive line is excluded. The digest is computed before
  the directive is written, and writing the digest into the directive must
  not change the bytes that produced it.
- The block's trailing line-ending characters, and the blank line that
  separates the block from whatever follows, are excluded. A digest never
  covers bytes outside the block itself.

Leading and interior whitespace inside the block is included and is
significant: indentation sets list nesting, four leading spaces make a code
block, two trailing spaces on a line are a hard line break, and inside a
code fence every space is content. A tool must not collapse, trim, or
reflow any of it before hashing. Two documents whose rendered HTML differs
must not produce the same digest for the same atom.

**The one normalization.** Before hashing, a tool normalizes CRLF and lone
CR line endings inside the block to LF. This is the only transformation
Core performs. Without it, the same document produces a different digest on
a Windows checkout than on a Unix checkout, and a cross-platform team sees
permanent phantom drift. A tool must not perform any other normalization:
not whitespace collapsing, not Unicode normalization, not trimming. Two
implementations that each hash the identical, LF-normalized block bytes
with SHA-256 produce the identical digest; anything more clever creates a
point where implementations can disagree.

**Nothing refreshes a digest automatically.** A digest is a claim that
someone reviewed this exact content. A tool must update or add a digest
only in response to an explicit action that means "I have reviewed this
block now" (for example, running `materialize --digest`, or a comparable
extension command). Default `materialize`, `lint`, and any other command
that runs as a side effect of editing or formatting a document must never
write or change a `digest` attribute. A tool that silently refreshes a
digest when content changes produces a digest that always matches,
which carries no information.

**Detecting drift.** A tool can compare each atom's recorded digest against
a freshly computed digest of its current content. An atom whose digest no
longer matches has drifted: its content changed since the digest was
written. This is a binary result, not a similarity score; a small edit to
text can invert its meaning while changing only a few bytes, so a percent
match would misrepresent significance rather than measure it. Reporting
which atom IDs drifted is the useful signal; showing what changed inside
a block is not this feature's job because a version control diff already
does that well.

`digest` is Core-defined so two independent implementations validate the
same bytes the same way; see the conformance corpus. Writing a digest is
opt-in: a tool must not add one unless the operator asked for it.

## Extensions

Core defines the `version`, `id`, `slug`, and `digest` attributes. A directive can contain additional XML attributes.

Core tools must preserve unknown attributes. Core tools must not assign meaning to unknown attributes or run their contents.

Attribute names are case-sensitive. Use lowercase kebab-case with a prefix that the owning application defines, such as `acme-approved-by`. The `acme-` examples in this repository are placeholders, not Core vocabulary.

An extension can define attribute meaning, validation, editor decoration, or agent behavior. An extension must not change Core elements or Core attributes.

A core tool accepts and preserves unknown attributes. The tool reports extension diagnostics only when the applicable extension is active.

## Mixed Markdown

An unmarked top-level Markdown block is an implicit atom. A tool must not discard the block or attach it to the previous atom.

A tool can materialize an implicit atom. The tool inserts an atom marker with a new ID.

### Splitting a container block into per-item atoms

A tool can give each item of a container block, such as a list, its own atom without putting a directive inside the container. It places a directive on its own source line between two adjacent items instead. Nothing in this Core requires a blank line around that directive; a directive there ends the current top-level CommonMark list and starts a new one, so each remaining item becomes its own top-level list with its own atom. This does not relax the rule above: the directive still sits between top-level blocks, never inside one.

Splitting this way changes the document's structure at the CommonMark level, even though the visible Markdown text is unchanged: one list becomes several single-item lists. A tool that offers this must wrap the resulting atoms in one atom-group. The group is the only record that the split was deliberate; without it, a deliberate split and an accidental one (for example, an unrelated directive that happens to land between two list items) are byte-identical. A linter should report a directive that splits a container block this way when the resulting atoms are not wrapped in a shared atom-group.

This does not reach into a nested list. Splitting a list's top-level items leaves each item's own nested children inside that item's atom; a nested child is never individually addressable this way. A GFM table cannot be split by this method at all: the header row and delimiter row are structural, so a table always gets one atom and its rows are not addressable.

## XML validation model

The Markdown source is not an XML document. A parser extracts Atomdown directives and creates the normalized XML model.

The file `schema/atomdown-1.xsd` defines this model. The schema validates the Core shape and extension attributes.

The Atomdown linter validates block association, source order, group balance, and ID rules.

Default lint permits implicit atoms because mixed documents support partial adoption. Use `lint --strict` to report each implicit atom.

When the source has no atomdown document marker, the normalized model assumes version 1 and lists only explicit atoms and groups.

A tool that writes an Atomdown document emits the version directive; a tool that reads a document must not require the version directive. `lint --strict` reports its absence as a warning.

## Conformance

A conforming reader recognizes Core directives and preserves their contents.

A conforming writer emits valid CommonMark and valid XML-shaped directives. The writer also emits unique IDs and balanced groups.

A conforming tool preserves unknown attributes.

Tools should provide machine-readable diagnostics. Each diagnostic should identify the defect, source position, and repair.
