# Atomdown conformance fixtures

This directory contains the Atomdown conformance corpus.

- `valid/` contains documents that must parse without errors.
- `mixed/` contains ordinary Markdown and partial Atomdown markup.
- `malformed/` contains one intentional defect in each document.
- `golden/` contains exact expected output and diagnostic codes.

The Go tests compare generated output with the golden files. Change a golden file only when the behavior change is intentional.
