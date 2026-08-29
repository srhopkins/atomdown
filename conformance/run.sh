#!/usr/bin/env bash
# Language-neutral Atomdown conformance runner.
# Usage: ./run.sh [--regen] /path/to/atomdown
set -eu

usage() {
  echo "Usage: $0 [--regen] /path/to/atomdown" >&2
  exit 2
}

REGEN=0
if [ "${1:-}" = "--regen" ]; then
  REGEN=1
  shift
fi
[ -n "${1:-}" ] || usage

BIN=$1
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

if [ ! -x "$BIN" ]; then
  if command -v "$BIN" >/dev/null 2>&1; then
    BIN=$(command -v "$BIN")
  elif [ -f "$BIN" ]; then
    chmod +x "$BIN" 2>/dev/null || true
  else
    echo "FAIL: binary not found: $BIN" >&2
    exit 2
  fi
fi

# Resolve to an absolute path when the file exists.
case $BIN in
  /*) ;;
  *)
    if [ -f "$BIN" ]; then
      BIN=$(CDPATH= cd -- "$(dirname -- "$BIN")" && pwd)/$(basename -- "$BIN")
    fi
    ;;
esac

export ATOMDOWN_BIN="$BIN"
export ATOMDOWN_ROOT="$ROOT"
export ATOMDOWN_REGEN="$REGEN"

python3 - "$ROOT/cases.json" <<'PY'
import json
import os
import subprocess
import sys

manifest_path = sys.argv[1]
bin_path = os.environ["ATOMDOWN_BIN"]
root = os.environ["ATOMDOWN_ROOT"]
regen = os.environ.get("ATOMDOWN_REGEN") == "1"

with open(manifest_path, encoding="utf-8") as fh:
    data = json.load(fh)

cases = data["cases"] if isinstance(data, dict) else data
failed = 0


def resolve(path):
    if os.path.isabs(path):
        return path
    return os.path.normpath(os.path.join(root, path))


def run_cmd(command, args, input_path, use_stdin):
    argv = [bin_path, command, *args]
    if use_stdin:
        with open(input_path, "rb") as fh:
            source = fh.read()
        argv.append("-")
        proc = subprocess.run(argv, input=source, capture_output=True)
    else:
        argv.append(input_path)
        proc = subprocess.run(argv, capture_output=True)
    return proc.returncode, proc.stdout, proc.stderr


def atom_ids_from_parse(stdout):
    doc = json.loads(stdout.decode("utf-8"))
    ids = []
    for atom in doc.get("atoms") or []:
        value = atom.get("id")
        if isinstance(value, str) and value != "":
            ids.append(value)
    return ids


def codes_from_lint(stdout):
    text = stdout.decode("utf-8").strip()
    if text == "" or text == "null":
        return []
    payload = json.loads(text)
    if payload is None:
        return []
    if not isinstance(payload, list):
        raise ValueError("lint --json must write a JSON array or null")
    return [item.get("code") for item in payload]


def check(name, cond, detail):
    if not cond:
        return f"{name}: {detail}"
    return None


for case in cases:
    name = case["name"]
    expect = case.get("expect") or {}
    input_path = resolve(case["input"])
    use_stdin = bool(case.get("stdin"))
    errors = []

    if not os.path.isfile(input_path):
        errors.append(f"input missing: {input_path}")
        print(f"FAIL  {name}  ({'; '.join(errors)})")
        failed += 1
        continue

    need_parse = any(key in expect for key in ("parse_exit", "atom_ids"))
    if need_parse:
        code, out, err = run_cmd("parse", [], input_path, use_stdin)
        if "parse_exit" in expect:
            errors.append(check("parse_exit", code == expect["parse_exit"], f"got {code} want {expect['parse_exit']}"))
        if "atom_ids" in expect:
            try:
                got_ids = atom_ids_from_parse(out)
            except Exception as exc:
                got_ids = None
                errors.append(f"parse JSON: {exc}")
            if got_ids is not None:
                errors.append(check("atom_ids", got_ids == expect["atom_ids"], f"got {got_ids} want {expect['atom_ids']}"))

    need_lint = any(key in expect for key in ("lint_exit", "diagnostic_codes"))
    if need_lint:
        lint_args = ["--json"]
        if expect.get("lint_strict"):
            lint_args.append("--strict")
        code, out, err = run_cmd("lint", lint_args, input_path, use_stdin)
        if "lint_exit" in expect:
            errors.append(check("lint_exit", code == expect["lint_exit"], f"got {code} want {expect['lint_exit']}"))
        if "diagnostic_codes" in expect:
            try:
                got_codes = codes_from_lint(out)
            except Exception as exc:
                got_codes = None
                errors.append(f"lint JSON: {exc}")
            if got_codes is not None:
                errors.append(check("diagnostic_codes", got_codes == expect["diagnostic_codes"], f"got {got_codes} want {expect['diagnostic_codes']}"))

    if "tokens_exit" in expect:
        code, out, err = run_cmd("tokens", [], input_path, use_stdin)
        errors.append(check("tokens_exit", code == expect["tokens_exit"], f"got {code} want {expect['tokens_exit']}"))

    need_xml = any(key in expect for key in ("xml_exit", "xml_file"))
    if need_xml:
        code, out, err = run_cmd("xml", [], input_path, use_stdin)
        if "xml_exit" in expect:
            errors.append(check("xml_exit", code == expect["xml_exit"], f"got {code} want {expect['xml_exit']}"))
        if "xml_file" in expect:
            dest = resolve(expect["xml_file"])
            if regen and code == 0:
                os.makedirs(os.path.dirname(dest), exist_ok=True)
                with open(dest, "wb") as fh:
                    fh.write(out)
            elif not os.path.isfile(dest):
                errors.append(f"xml_file missing: {dest}")
            else:
                with open(dest, "rb") as fh:
                    wanted = fh.read()
                errors.append(check("xml_file", out == wanted, f"stdout differs from {expect['xml_file']}"))

    need_strip = any(key in expect for key in ("strip_exit", "strip_file"))
    if need_strip:
        code, out, err = run_cmd("strip", [], input_path, use_stdin)
        if "strip_exit" in expect:
            errors.append(check("strip_exit", code == expect["strip_exit"], f"got {code} want {expect['strip_exit']}"))
        if "strip_file" in expect:
            dest = resolve(expect["strip_file"])
            if regen and code == 0:
                os.makedirs(os.path.dirname(dest), exist_ok=True)
                with open(dest, "wb") as fh:
                    fh.write(out)
            elif not os.path.isfile(dest):
                errors.append(f"strip_file missing: {dest}")
            else:
                with open(dest, "rb") as fh:
                    wanted = fh.read()
                errors.append(check("strip_file", out == wanted, f"stdout differs from {expect['strip_file']}"))

    errors = [item for item in errors if item]
    if errors:
        print(f"FAIL  {name}  ({'; '.join(errors)})")
        failed += 1
    else:
        label = "REGEN" if regen else "PASS"
        print(f"{label}  {name}")

total = len(cases)
passed = total - failed
print(f"{passed}/{total} passed")
sys.exit(0 if failed == 0 else 1)
PY
