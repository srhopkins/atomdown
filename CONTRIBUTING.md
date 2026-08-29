# Contributing to Atomdown

Atomdown aims to remain small, familiar, and compatible with ordinary Markdown and XML tools.

## Before proposing a core feature

Check the design requirements in `SPEC.md`. Features belong in an extension when they do not require every Atomdown reader to agree on their meaning.

Open an issue before making a breaking syntax or schema change. Describe the use case, the smallest required primitive, and the effect on existing Markdown files.

## Development

1. Make the smallest focused change.
2. Add or update a fixture in `testdata/`.
3. Run `go test ./...`.
4. Run `go vet ./...`.
5. Run `CGO_ENABLED=0 go build ./cmd/atomdown`.

Golden files are public compatibility examples. Update them only when the output change is intentional.

## Pull requests

State whether the change affects Atomdown Core, an extension point, the Go implementation, or documentation only. Include any compatibility or migration effect.
