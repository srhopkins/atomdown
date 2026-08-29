# Agent instructions

## Project purpose

Atomdown adds persistent block IDs, groups, and extensible metadata to Markdown. Visible Markdown remains the source of truth.

Read `SPEC.md` before you change parser behavior. Read `testdata/README.md` before you change fixtures or golden files.

## Build and test

- Run `go test ./...`.
- Run `go vet ./...`.
- Run `CGO_ENABLED=0 go build ./cmd/atomdown`.
- Validate schema changes with `xmllint --noout --schema schema/atomdown-1.xsd FILE`.

## Core rules

- Keep each Atomdown source document valid CommonMark.
- Keep visible Markdown as the source of truth.
- Preserve unknown XML attributes. Do not interpret or run their contents.
- Preserve an atom ID when you move the atom.
- Generate a new atom ID when you copy the atom.
- Put application behavior in extensions.
- Do not parse the complete Markdown document as XML.
- Do not add CGO or Go's runtime `plugin` package.

## Change requirements

- Add one malformed fixture for each new diagnostic.
- Change golden output only when the format change is intentional.
- Treat changes to `SPEC.md` and `schema/atomdown-1.xsd` as compatibility changes.
- Describe compatibility effects in the pull request.
