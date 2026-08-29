# Atomdown Core 1

Atomdown is a backward-compatible annotation layer for Markdown. XML-shaped directives inside HTML comments give top-level Markdown blocks persistent identity, grouping, and extensible attributes.

## Design requirements

Atomdown Core follows these requirements in priority order:

1. An Atomdown document is valid CommonMark. A normal Markdown tool can parse it without Atomdown support.
2. Atomdown directives use ordinary XML 1.0 element and attribute syntax inside ordinary HTML comments.
3. The normalized metadata model is ordinary XML validated by an XML Schema Definition (XSD) 1.0 schema.
4. Core stays small. It defines identity, grouping, ordering, and extension preservation.
5. Extensions use XML attributes. Unknown attributes remain valid and must survive a parse-write cycle.
6. Visible Markdown content remains the source of truth. Atomdown does not replace it with an application-owned representation.

These requirements are normative. A feature that violates an earlier requirement does not belong in Atomdown Core.

## Core directives

```markdown
<!-- <atomdown version="1"/> -->
<!-- <atom id="4P8W2H6K" slug="claim"/> -->
<!-- <atom-group id="7K3M9X2D" slug="findings"> -->
<!-- </atom-group> -->
```

`atomdown` and `atom` directives are self-closing. `atom-group` uses paired opening and closing directives.

Each directive must occupy its own source line. Only whitespace may appear before `<!--` or after `-->`. Directives must not appear inside paragraphs, lists, code fences, tables, block quotes, or other Markdown blocks.

An `atom` directive applies to the next top-level CommonMark block. Source order is canonical. An `atom-group` contains the explicit atoms between its opening and closing directives. Atomdown Core 1 does not allow nested groups.

## Identity

Every explicit atom and atom group requires a unique eight-character Crockford Base32 `id`. Moving an item preserves its ID. Copying an item creates a new ID.

`slug` is an optional human-readable alias. It is not identity.

## Extensions

Core defines only `version`, `id`, and `slug`. Any directive may contain additional XML attributes. Conforming tools must preserve unknown attributes, must not assign meaning to them, and must not execute their contents.

Attribute names are case-sensitive. Extension authors should use lowercase kebab-case with a domain prefix, such as `audit-approved-by`.

An extension may define attribute meaning, validation, editor decoration, or agent behavior. It must not change the meaning of Core elements or Core attributes.

Core tools must accept and preserve unknown attributes. They may report extension-specific diagnostics only when the extension is installed or explicitly requested.

## Mixed Markdown

Unmarked top-level Markdown blocks are implicit atoms. Tools must not discard them or silently attach them to the previous explicit atom. A tool may materialize an implicit atom by inserting a marker with a new ID.

## XML validation model

The Markdown source is not an XML document. A conforming parser extracts Atomdown directives and constructs the normalized XML model defined by `schema/atomdown-1.xsd`. XSD validates the core shape and extension attributes. The Atomdown linter validates Markdown association, source order, group balance, and ID rules.

## Conformance

A conforming reader recognizes Core directives and never corrupts them. A conforming writer emits valid CommonMark, valid XML-shaped directives, unique IDs, and balanced groups. A conforming tool preserves unknown attributes.

Tools should provide machine-readable diagnostics. Diagnostic messages should name the defect, source position, and repair.
