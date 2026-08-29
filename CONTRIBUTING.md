# Contributing to Atomdown

Atomdown accepts focused bug fixes, documentation changes, tests, and proposals.

## Start with an issue

Choose one GitHub issue form:

- **Bug report** for incorrect parsing, output, validation, or documentation.
- **Proposal** for new behavior, syntax, or extension support.

Search existing issues before you open a new issue. Include a small example that another person or agent can reproduce.

You can submit a small documentation correction without an issue.

## Keep Core small

Read `SPEC.md` before you propose a Core change.

Put a feature in an extension unless every Atomdown reader needs the feature. A Core proposal must explain:

- The problem.
- The smallest required primitive.
- Why an extension cannot solve the problem.
- The effect on existing documents.

## Make a change

1. Make one focused change.
2. Add or update a fixture in `testdata/`.
3. Run `go test ./...`.
4. Run `go vet ./...`.
5. Run `CGO_ENABLED=0 go build ./cmd/atomdown`.

Change a golden file only when the output change is intentional.

## Open a pull request

Link the issue when one exists. Describe the change and its compatibility effect.

State whether the pull request changes Core, an extension point, the Go implementation, tests, or documentation.

AI agents must follow `AGENTS.md` and the same contribution process.
