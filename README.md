# Atomdown

Atomdown is a backward-compatible block identity and metadata standard for Markdown. It adds persistent atoms, ordered groups, and extensible XML attributes through invisible HTML comments.

Use Atomdown when Markdown must remain readable by every normal Markdown tool while editors, linters, and AI agents need stable block IDs, provenance, review status, or other machine-readable metadata.

Atomdown is deliberately familiar:

- The document remains valid CommonMark.
- Directives use XML 1.0 syntax inside standard HTML comments.
- The normalized metadata model uses XSD 1.0.
- Unknown XML attributes provide extensions without changing Core.
- Core defines only document version, atoms, atom groups, IDs, slugs, ordering, and preservation rules.

This repository contains a pure-Go library and a small command-line tool. It uses Go's standard `encoding/xml` package for directives and the pure-Go goldmark parser for CommonMark structure. It does not use CGO.

## Why Atomdown

Most block editors store a private JSON document. Atomdown keeps Markdown canonical and adds only a small annotation layer. This makes the same file useful as prose, source code, an agent-editable document, and a collection of addressable blocks.

Common uses include:

- Auditing AI-generated claims and recording approval or provenance.
- Giving Markdown blocks stable IDs for links, comments, and review workflows.
- Grouping ordered blocks without replacing Markdown with JSON.
- Adding application-specific metadata that core tools preserve but do not interpret.

Atomdown Core 1 is an early specification. The syntax and conformance corpus are ready for experimentation and review.

## Example

```markdown
<!-- <atomdown version="1"/> -->

<!-- <atom id="4P8W2H6K" audit-approved-by="steve"/> -->

The product launched in March.
```

## Commands

```bash
go run ./cmd/atomdown lint testdata/example.md
go run ./cmd/atomdown parse testdata/example.md
go run ./cmd/atomdown tokens testdata/example.md
go run ./cmd/atomdown xml testdata/example.md
go run ./cmd/atomdown strip testdata/example.md
go run ./cmd/atomdown id
```

After the first tagged release, install the CLI with:

```bash
go install github.com/srhopkins/atomdown/cmd/atomdown@latest
```

All file commands accept `-` or no file to read standard input.

The [`testdata`](testdata/) directory is a reusable conformance corpus. It contains valid, mixed, malformed, and exact golden-output documents for this and other Atomdown implementations.

- `parse` returns the semantic Atomdown model as JSON.
- `tokens` returns a lossless ordered stream of Markdown, whitespace, and Atomdown directives.
- `strip` returns pure Markdown by removing only recognized Atomdown directives.
- `xml` returns the normalized XML metadata model.
- `lint --json` returns machine-readable diagnostics.

## Library

```go
document := atomdown.Parse(source)
if document.HasErrors() {
    // Reject or repair the source.
}
```

Embedded applications can register compile-time extensions:

```go
processor := atomdown.NewProcessor(myExtension)
document, err := processor.Process(ctx, source)
```

Extensions implement `atomdown.Extension`. They run in registration order and can decorate the document or add profile-specific diagnostics. Atomdown does not use Go's runtime `plugin` package, so the library remains portable across platforms and works with `CGO_ENABLED=0`.

See `SPEC.md` for Atomdown Core 1 and `schema/atomdown-1.xsd` for the normalized XML validation model.

For agents and automated discovery, start with `llms.txt`. It points to the normative files and states the safe editing rules in one screen.

## Related search terms

Atomdown addresses block-based Markdown, persistent Markdown block IDs, addressable Markdown, Markdown AST metadata, Markdown provenance, AI writing audits, LLM document review, XML in Markdown comments, and extensible Markdown annotations.

## Contributing

Read [`CONTRIBUTING.md`](CONTRIBUTING.md) before changing Atomdown Core. Coding agents should also read [`AGENTS.md`](AGENTS.md).
