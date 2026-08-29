# Atomdown conformance fixtures

This directory is a small, human-readable conformance corpus.

- `valid/` contains documents that must parse without errors.
- `mixed/` contains ordinary Markdown combined with partial Atomdown markup.
- `malformed/` contains one intentional defect per document.
- `golden/` contains exact expected projections for `valid/complete.md` and exact diagnostics for the malformed corpus.

The Go tests compare generated output with the golden files. Update a golden file only when the corresponding behavior change is intentional and reviewed.
