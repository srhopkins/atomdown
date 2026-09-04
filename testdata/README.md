# Atomdown conformance fixtures

This directory contains the Atomdown conformance corpus.

- `valid/` contains documents that must parse without errors.
- `mixed/` contains ordinary Markdown and partial Atomdown markup.
- `malformed/` contains one intentional defect in each document. Every file here must produce at least one **error** diagnostic; the Go test asserts that.
- A diagnostic whose severity is **warning** therefore belongs in `mixed/`, not in `malformed/`, and gets a case in `conformance/cases.json` that asserts its code. `mixed/accidental-list-split.md` and `mixed/duplicate-slug.md` are the examples.
- `split/` contains documents used to test `materialize --split list-item`.
- `golden/` contains exact expected output and diagnostic codes, including `golden/split/` for the `split/` fixtures.

The Go tests compare generated output with the golden files. Change a golden file only when the behavior change is intentional.
