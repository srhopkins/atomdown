package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/srhopkins/atomdown"
)

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
		printUsage(stderr)
		return errors.New("missing command")
	}
	switch arguments[0] {
	case "parse":
		return runParse(arguments[1:], stdout)
	case "tokens":
		return runTokens(arguments[1:], stdout)
	case "lint":
		return runLint(arguments[1:], stdout)
	case "xml":
		return runXML(arguments[1:], stdout)
	case "strip":
		return runStrip(arguments[1:], stdout)
	case "id":
		return runID(arguments[1:], stdout)
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q", arguments[0])
	}
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
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	source, err := readInput(flags.Args())
	if err != nil {
		return err
	}
	document := atomdown.Parse(source)
	if *jsonOutput {
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(document.Diagnostics); err != nil {
			return err
		}
	} else if len(document.Diagnostics) == 0 {
		fmt.Fprintln(output, "ok")
	} else {
		for _, diagnostic := range document.Diagnostics {
			fmt.Fprintf(output, "%s:%d:%d: %s %s: %s\n", inputName(flags.Args()), diagnostic.Position.Line, diagnostic.Position.Column, diagnostic.Severity, diagnostic.Code, diagnostic.Message)
		}
	}
	if document.HasErrors() {
		return exitError{code: 1}
	}
	return nil
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
	fmt.Fprintln(output, "Usage: atomdown <parse|tokens|lint|xml|strip|id> [options] [file|-]")
}

type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("lint failed with exit code %d", e.code) }
