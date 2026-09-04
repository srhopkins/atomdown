package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/srhopkins/atomdown"
)

// cliVersion is the atomdown command-line tool's own version. It is
// independent of the Atomdown Core document version ("1") that atoms declare.
const cliVersion = "0.1.0"

// commandNames lists every command the CLI accepts, in the order printUsage
// and the unknown-command message present them.
var commandNames = []string{"parse", "emit", "tokens", "lint", "xml", "strip", "materialize", "drift", "verify", "id"}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		var status exitError
		if errors.As(err, &status) {
			os.Exit(status.code)
		}
		fmt.Fprintln(os.Stderr, "atomdown:", err)
		os.Exit(2)
	}
}

func run(arguments []string, stdout, stderr io.Writer) error {
	if len(arguments) == 0 {
		fmt.Fprintln(stderr, "atomdown: missing command")
		printVersionAndCommands(stderr)
		return exitError{code: 2}
	}
	switch arguments[0] {
	case "parse":
		return runParse(arguments[1:], stdout)
	case "emit":
		return runEmit(arguments[1:], stdout)
	case "tokens":
		return runTokens(arguments[1:], stdout)
	case "lint":
		return runLint(arguments[1:], stdout)
	case "xml":
		return runXML(arguments[1:], stdout)
	case "strip":
		return runStrip(arguments[1:], stdout)
	case "materialize":
		return runMaterialize(arguments[1:], stdout, stderr)
	case "drift", "verify":
		return runDrift(arguments[1:], stdout)
	case "id":
		return runID(arguments[1:], stdout)
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		fmt.Fprintf(stderr, "atomdown: unknown command %q\n", arguments[0])
		printVersionAndCommands(stderr)
		return exitError{code: 2}
	}
}

// printVersionAndCommands reports the CLI version and every known command.
// It runs after a missing or unknown command, so an operator can tell
// whether they are holding a stale binary without reading the source.
func printVersionAndCommands(output io.Writer) {
	fmt.Fprintf(output, "atomdown version %s\n", cliVersion)
	fmt.Fprintf(output, "commands: %s\n", strings.Join(commandNames, ", "))
}

func runEmit(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("emit", flag.ContinueOnError)
	// emit preserves each directive's authored source layout, so a wrapped
	// directive stays wrapped. --flatten is the opt-in canonicalizing
	// writer: it rewrites every directive to one line.
	flatten := flags.Bool("flatten", false, "rewrite every directive to one line")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if len(flags.Args()) > 1 {
		return errors.New("emit accepts at most one file")
	}
	source, err := readInput(flags.Args())
	if err != nil {
		return err
	}

	var shape map[string]json.RawMessage
	if err := json.Unmarshal(source, &shape); err != nil {
		return fmt.Errorf("invalid document JSON: %w", err)
	}
	if shape == nil {
		return errors.New("invalid document JSON: expected an object")
	}
	atoms, exists := shape["atoms"]
	if !exists {
		return errors.New(`invalid document JSON: missing "atoms"`)
	}
	if string(atoms) == "null" {
		return errors.New(`invalid document JSON: "atoms" must be an array`)
	}

	var document atomdown.Document
	if err := json.Unmarshal(source, &document); err != nil {
		return fmt.Errorf("invalid document JSON: %w", err)
	}
	result, err := atomdown.EmitWithOptions(document, atomdown.EmitOptions{Flatten: *flatten})
	if err != nil {
		return err
	}
	_, err = output.Write(result)
	return err
}

func runTokens(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("tokens", flag.ContinueOnError)
	compact := flags.Bool("compact", false, "emit compact JSON")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	source, err := readInput(flags.Args())
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	if !*compact {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(atomdown.Tokenize(source))
}

func runParse(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("parse", flag.ContinueOnError)
	compact := flags.Bool("compact", false, "emit compact JSON")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	source, err := readInput(flags.Args())
	if err != nil {
		return err
	}
	document := atomdown.Parse(source)
	encoder := json.NewEncoder(output)
	if !*compact {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(document)
}

func runLint(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("lint", flag.ContinueOnError)
	jsonOutput := flags.Bool("json", false, "emit JSON diagnostics")
	strict := flags.Bool("strict", false, "report implicit atoms")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	source, err := readInput(flags.Args())
	if err != nil {
		return err
	}
	document := atomdown.Parse(source)
	diagnostics := document.Diagnostics
	if !*strict {
		diagnostics = withoutStrictOnlyWarnings(diagnostics)
	}
	if *jsonOutput {
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(diagnostics); err != nil {
			return err
		}
	} else if len(diagnostics) == 0 {
		fmt.Fprintln(output, "ok")
	} else {
		for _, diagnostic := range diagnostics {
			fmt.Fprintf(output, "%s:%d:%d: %s %s: %s\n", inputName(flags.Args()), diagnostic.Position.Line, diagnostic.Position.Column, diagnostic.Severity, diagnostic.Code, diagnostic.Message)
		}
	}
	if document.HasErrors() {
		return exitError{code: 1}
	}
	return nil
}

// withoutStrictOnlyWarnings drops the warnings that lint reports only under
// --strict: an implicit atom (no persistent marker) and a missing document
// version directive. Default lint stays permissive so mixed and unversioned
// documents still pass.
func withoutStrictOnlyWarnings(diagnostics []atomdown.Diagnostic) []atomdown.Diagnostic {
	filtered := make([]atomdown.Diagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		if diagnostic.Code != "implicit-atom" && diagnostic.Code != "missing-version-directive" {
			filtered = append(filtered, diagnostic)
		}
	}
	return filtered
}

func runXML(arguments []string, output io.Writer) error {
	if len(arguments) > 1 {
		return errors.New("xml accepts at most one file")
	}
	source, err := readInput(arguments)
	if err != nil {
		return err
	}
	document := atomdown.Parse(source)
	if document.HasErrors() {
		return errors.New("cannot normalize a document with lint errors")
	}
	normalized, err := atomdown.NormalizedXML(document)
	if err != nil {
		return err
	}
	_, err = output.Write(normalized)
	return err
}

func runStrip(arguments []string, output io.Writer) error {
	if len(arguments) > 1 {
		return errors.New("strip accepts at most one file")
	}
	source, err := readInput(arguments)
	if err != nil {
		return err
	}
	_, err = output.Write(atomdown.Strip(source))
	return err
}

func runMaterialize(arguments []string, output, statusOutput io.Writer) error {
	flags := flag.NewFlagSet("materialize", flag.ContinueOnError)
	write := flags.Bool("w", false, "write result in place")
	split := flags.String("split", "", "comma-separated CommonMark node names to split into their own atoms (for example: list-item)")
	digest := flags.Bool("digest", false, "write a Core content digest to every atom that does not already have one")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *write && (len(flags.Args()) != 1 || flags.Args()[0] == "-") {
		return errors.New("materialize -w requires one file")
	}
	if *digest && *split != "" {
		return errors.New("materialize --digest and --split cannot run together")
	}
	source, err := readInput(flags.Args())
	if err != nil {
		return err
	}

	var result []byte
	var marked, digested int
	switch {
	case *digest:
		result, marked, digested, err = atomdown.MaterializeDigest(source)
	case *split != "":
		var nodeTypes []string
		nodeTypes, err = atomdown.ParseSplitNodeTypes(*split)
		if err == nil {
			result, marked, err = atomdown.MaterializeSplit(source, nodeTypes)
		}
	default:
		result, marked, err = atomdown.Materialize(source)
	}
	if err != nil {
		return err
	}
	summary := materializeSummary(marked)
	if *digest {
		summary = materializeDigestSummary(marked, digested)
	}
	if !*write {
		// stdout must carry only the marked Markdown, so the status line
		// goes to stderr here and is never interleaved with piped output.
		_, err = output.Write(result)
		if err != nil {
			return err
		}
		fmt.Fprintln(statusOutput, summary)
		return nil
	}
	info, err := os.Stat(flags.Args()[0])
	if err != nil {
		return err
	}
	if err := os.WriteFile(flags.Args()[0], result, info.Mode().Perm()); err != nil {
		return err
	}
	fmt.Fprintln(statusOutput, summary)
	return nil
}

func materializeSummary(marked int) string {
	switch marked {
	case 0:
		return "ok - no unmarked blocks"
	case 1:
		return "ok - marked 1 block"
	default:
		return fmt.Sprintf("ok - marked %d blocks", marked)
	}
}

func materializeDigestSummary(marked, digested int) string {
	switch digested {
	case 0:
		return "ok - every atom already has a digest"
	case 1:
		return fmt.Sprintf("ok - marked %d block(s), wrote 1 digest", marked)
	default:
		return fmt.Sprintf("ok - marked %d block(s), wrote %d digests", marked, digested)
	}
}

func runDrift(arguments []string, output io.Writer) error {
	if len(arguments) > 1 {
		return errors.New("drift accepts at most one file")
	}
	source, err := readInput(arguments)
	if err != nil {
		return err
	}
	drifted := atomdown.Drift(source)
	if len(drifted) == 0 {
		fmt.Fprintln(output, "ok - no drift")
		return nil
	}
	for _, atom := range drifted {
		fmt.Fprintf(output, "%s: content changed since the digest was recorded\n", atom.ID)
	}
	return exitError{code: 1}
}

func runID(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("id", flag.ContinueOnError)
	count := flags.Int("n", 1, "number of IDs")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *count < 1 {
		return errors.New("ID count must be positive")
	}
	for index := 0; index < *count; index++ {
		id, err := atomdown.NewID()
		if err != nil {
			return err
		}
		fmt.Fprintln(output, id)
	}
	return nil
}

func readInput(arguments []string) ([]byte, error) {
	if len(arguments) > 1 {
		return nil, errors.New("expected zero or one file")
	}
	if len(arguments) == 0 || arguments[0] == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(arguments[0])
}

func inputName(arguments []string) string {
	if len(arguments) == 0 || arguments[0] == "-" {
		return "stdin"
	}
	return arguments[0]
}

func printUsage(output io.Writer) {
	fmt.Fprintln(output, "Usage: atomdown <parse|emit|tokens|lint|xml|strip|materialize|drift|verify|id> [options] [file|-]")
	fmt.Fprintln(output, "  emit [--flatten]   write Markdown from a parse JSON document.")
	fmt.Fprintln(output, "                     Each directive keeps the source layout the author")
	fmt.Fprintln(output, "                     gave it. --flatten rewrites every directive to one line.")
}

type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("lint failed with exit code %d", e.code) }
