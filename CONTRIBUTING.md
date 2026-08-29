# Contributing to Atomdown

Atomdown must remain small and compatible with standard Markdown and XML tools.

## Core proposals

Read the design requirements in `SPEC.md` before you propose a core feature.

Put a feature in an extension unless every Atomdown reader needs the feature. Open an issue before you change syntax or the schema.

In the issue, include:

- The use case.
- The smallest required primitive.
- The effect on existing Markdown documents.

## Development procedure

1. Make one focused change.
2. Add or update a fixture in `testdata/`.
3. Run `go test ./...`.
4. Run `go vet ./...`.
5. Run `CGO_ENABLED=0 go build ./cmd/atomdown`.

Golden files are public compatibility examples. Change a golden file only when the output change is intentional.

## Pull requests

Identify the type of change: Core, extension point, Go implementation, or documentation. Describe all compatibility and migration effects.
