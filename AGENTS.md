# Agent instructions

## Project purpose

Atomdown is a backward-compatible block identity and metadata standard for Markdown. The visible Markdown is canonical. XML-shaped directives inside HTML comments add identity, grouping, and extensible attributes.

Read `SPEC.md` before changing parser behavior. Read `testdata/README.md` before changing fixtures or golden outputs.

## Build and test

- Run `go test ./...` for all unit and conformance tests.
- Run `go vet ./...` for static checks.
- Run `CGO_ENABLED=0 go build ./cmd/atomdown` to verify the portability requirement.
- Validate normalized XML with `xmllint --noout --schema schema/atomdown-1.xsd FILE` when the schema changes.

## Core safety rules

- Keep every Atomdown source document valid CommonMark.
- Keep visible Markdown as the source of truth.
- Preserve unknown XML attributes without interpreting or executing them.
- Preserve existing IDs when moving atoms. Generate new IDs when copying atoms.
- Keep Atomdown Core small. Put application behavior in extensions.
- Do not parse the entire Markdown document as XML.
- Do not add CGO or Go's runtime `plugin` package.

## Changes

- Add a focused malformed fixture for every new diagnostic.
- Add or update golden output only for intentional format changes.
- Treat `SPEC.md` and `schema/atomdown-1.xsd` changes as compatibility changes.
- Explain compatibility effects in the pull request.
