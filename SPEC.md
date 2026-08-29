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

## Identity

Each explicit atom and atom group requires an eight-character Crockford Base32 `id`.
Use the uppercase Crockford Base32 alphabet only.
IDs must be unique within a document.

Preserve the ID when you move an item. Generate a new ID when you copy an item.

The optional `slug` attribute is a readable alias. The slug is not identity.

## Extensions

Core defines the `version`, `id`, and `slug` attributes. A directive can contain additional XML attributes.

Core tools must preserve unknown attributes. Core tools must not assign meaning to unknown attributes or run their contents.

Attribute names are case-sensitive. Use lowercase kebab-case with a prefix that the owning application defines, such as `acme-approved-by`. The `acme-` examples in this repository are placeholders, not Core vocabulary.

An extension can define attribute meaning, validation, editor decoration, or agent behavior. An extension must not change Core elements or Core attributes.

A core tool accepts and preserves unknown attributes. The tool reports extension diagnostics only when the applicable extension is active.

## Mixed Markdown

An unmarked top-level Markdown block is an implicit atom. A tool must not discard the block or attach it to the previous atom.

A tool can materialize an implicit atom. The tool inserts an atom marker with a new ID.

## XML validation model

The Markdown source is not an XML document. A parser extracts Atomdown directives and creates the normalized XML model.

The file `schema/atomdown-1.xsd` defines this model. The schema validates the Core shape and extension attributes.

The Atomdown linter validates block association, source order, group balance, and ID rules.

Default lint permits implicit atoms because mixed documents support partial adoption. Use `lint --strict` to report each implicit atom.

When the source has no atomdown document marker, the normalized model assumes version 1 and lists only explicit atoms and groups.

## Conformance

A conforming reader recognizes Core directives and preserves their contents.

A conforming writer emits valid CommonMark and valid XML-shaped directives. The writer also emits unique IDs and balanced groups.

A conforming tool preserves unknown attributes.

Tools should provide machine-readable diagnostics. Each diagnostic should identify the defect, source position, and repair.
