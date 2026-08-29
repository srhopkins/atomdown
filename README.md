# Atomdown

Atomdown adds persistent block IDs, groups, and extensible metadata to Markdown. Atomdown documents remain valid CommonMark.

Use Atomdown when people and AI agents need to address and review individual Markdown blocks. Visible Markdown remains the source of truth.

Atomdown uses XML-shaped directives inside HTML comments:

```markdown
<!-- <atomdown version="1"/> -->

<!-- <atom id="4P8W2H6K" slug="launch-claim"/> -->

The product launched in March.
```

Normal Markdown tools treat each directive as an invisible comment. Atomdown tools read the same directive as block metadata.

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

## CLI

Run the CLI from the repository:

```bash
go run ./cmd/atomdown lint testdata/example.md
go run ./cmd/atomdown parse testdata/example.md
go run ./cmd/atomdown tokens testdata/example.md
go run ./cmd/atomdown xml testdata/example.md
go run ./cmd/atomdown strip testdata/example.md
go run ./cmd/atomdown id
```

Each file command accepts one file. Use `-` or omit the file to read standard input.

- `lint` checks syntax, IDs, block associations, and groups.
- `parse` writes the semantic document model as JSON.
- `tokens` writes a lossless stream of Markdown, whitespace, and Atomdown directives.
- `xml` writes the normalized XML metadata model.
- `strip` removes Atomdown directives and writes plain Markdown.
- `id` creates an eight-character Crockford Base32 ID.

After the first tagged release, install the CLI with:

```bash
go install github.com/srhopkins/atomdown/cmd/atomdown@latest
```

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
- [`llms.txt`](llms.txt) gives AI agents a short guide to the repository.

## Contributing

Read [`CONTRIBUTING.md`](CONTRIBUTING.md), then open a Bug report or Proposal. Coding agents must also read [`AGENTS.md`](AGENTS.md).
