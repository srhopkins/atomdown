# Atomdown portable conformance suite

This directory lets a second implementer test an Atomdown CLI without Go.

The runner uses bash and python3 only.

## Run the suite

Build or install a candidate binary. Then run:

```bash
./run.sh /path/to/atomdown
```

The script prints one pass or fail line per case. It exits 0 only when every case passes.

Paths in `cases.json` are relative to this directory. Input files point at `../testdata/` fixtures.

## Candidate binary contract

The binary name does not matter. The command interface must match this contract.

### Invocation

```
<bin> <command> [options] [file|-]
```

`command` is one of: `parse`, `tokens`, `lint`, `xml`, `strip`, `materialize --digest`, `id`. The Go CLI also has `emit`, plain `materialize`, and `drift`/`verify`; this suite does not cover them because they write output back or depend on a prior digest baseline, which is out of scope for a read-only conformance check. `materialize --digest` is the one exception: run on a document whose atoms already have explicit IDs and no digest, it is fully deterministic (it mints no new ID), so its stdout is a byte-exact oracle for the content digest algorithm — see `digest_file` below.

File commands accept at most one file. Use `-` or omit the file to read standard input.

Commands write the result to standard output. Usage and I/O errors go to standard error.

### Commands

- `parse` writes the document model as JSON. Use `--compact` for one-line JSON.
- `tokens` writes the lossless token stream as JSON. Use `--compact` for one-line JSON.
- `lint` checks the document. Use `--json` to write a JSON array of diagnostics. Use `--strict` to include `implicit-atom` and `missing-version-directive` warnings. Default `lint` hides both. Default `lint` still reports `directive-splits-list`; that warning is not gated by `--strict`.
- `xml` writes the normalized XML metadata model. It refuses a document that has error-severity diagnostics.
- `strip` removes Atomdown directives and writes visible Markdown.
- `id` writes one new eight-character Crockford Base32 ID. Use `-n N` to write N IDs, one per line.

`id` is not in the case list. IDs are random. Check the format in your own tests.

### Exit codes

- `0` means success. For `lint`, success means no error-severity diagnostic.
- `1` means `lint` found at least one error-severity diagnostic. Warnings do not cause exit `1`.
- `2` means a usage error, an I/O error, or `xml` refused a document with errors.

`parse`, `tokens`, and `strip` still exit `0` on a malformed document. They report defects in the JSON model or they strip what they can.

### `parse` JSON

The object has these fields:

- `declared` (boolean): the file has an `atomdown` document marker.
- `version` (string, optional): the document version.
- `atoms` (array): every top-level Markdown unit, in source order.
- `groups` (array, optional): atom groups.
- `diagnostics` (array, optional): parse and lint findings.
- `attributes` (array, optional): extra attributes on the document marker.

Each atom object may have `id` (string) and `digest` (string). Omit `id` when the atom has no persistent ID. Omit `digest` when the atom has no content digest; a digest is opt-in and Core never adds one on its own.

Each diagnostic object has `code` (string) and `severity` (`error` or `warning`).

### `lint --json` output

The command writes a JSON array of diagnostic objects. An empty result may be `[]` or `null`. Treat both as no diagnostics.

Default `lint --json` drops `implicit-atom` and `missing-version-directive` warnings. Pass `--strict` to keep them. `directive-splits-list` is not affected by `--strict`; it appears in default output too.

## Manifest format (`cases.json`)

The file is one JSON object.

### Top-level fields

- `manifest_version` (number): schema version. This suite uses `1`.
- `suite` (string): human label. This suite uses `atomdown-core-1`.
- `description` (string): short purpose text.
- `cases` (array): the case list. Order is the run order.

### Case fields

Each case is one object.

- `name` (string, required): unique case id. Use it in reports.
- `category` (string, required): `valid`, `mixed`, or `malformed`.
- `input` (string, required): Markdown file path, relative to this directory.
- `stdin` (boolean, optional): if `true`, the runner pipes the file to standard input and passes `-` as the file argument. Default is `false`.
- `expect` (object, required): checks for this case. Omit a key to skip that check.

### `expect` fields

Every field is optional. The runner runs a command only when a related key is present.

- `parse_exit` (integer): expected `parse` exit code.
- `lint_exit` (integer): expected `lint` exit code.
- `xml_exit` (integer): expected `xml` exit code.
- `strip_exit` (integer): expected `strip` exit code.
- `tokens_exit` (integer): expected `tokens` exit code.
- `atom_ids` (array of strings): explicit atom IDs from `parse` JSON, in document order. The runner keeps an atom only when `id` is a non-empty string.
- `diagnostic_codes` (array of strings): `code` values from `lint --json`, in order. `null` or `[]` means no codes.
- `lint_strict` (boolean): if `true`, the runner adds `--strict` to `lint`. Default is `false`.
- `xml_file` (string): path to the expected `xml` bytes, relative to this directory. Compare exact bytes.
- `strip_file` (string): path to the expected `strip` bytes, relative to this directory. Compare exact bytes.
- `materialize_digest_exit` (integer): expected exit code of `materialize --digest`.
- `digest_file` (string): path to the expected `materialize --digest` bytes, relative to this directory. Compare exact bytes. Use this only on an input whose atoms already carry explicit `id` attributes and no `digest`, so the run mints no random ID and stdout is fully determined by the content digest algorithm (SHA-256 over the atom's block bytes after normalizing line endings to LF; see `SPEC.md` "Content digest").

A case may use any subset of these keys.

### Categories

- `valid`: the document must parse with no error diagnostics. `lint` exits `0`. `xml` exits `0`.
- `mixed`: ordinary Markdown or partial Atomdown markup. `lint` exits `0` unless the case asks for `--strict` and you treat warnings as errors in your tool. This suite still expects `lint` exit `0` for warnings.
- `malformed`: the document has at least one error diagnostic. `lint` exits `1`. `xml` exits `2`.

## How a TypeScript implementer uses this suite

1. Implement a CLI that matches the contract above.
2. Point `run.sh` at that binary.
3. Keep `cases.json` and `expected/` unchanged. They are the oracle.
4. Fix the binary until every case prints `PASS`.

Do not regenerate `expected/` against a candidate binary. Those files come from the Go reference CLI.

## Regenerate expected files (reference CLI only)

If the reference parser changes on purpose, rebuild it and rewrite the files:

```bash
CGO_ENABLED=0 go build -o /tmp/ad ./cmd/atomdown
./run.sh --regen /tmp/ad
```

`--regen` rewrites every `xml_file`, `strip_file`, and `digest_file` from the given binary. Then run without `--regen` to confirm the suite passes.
