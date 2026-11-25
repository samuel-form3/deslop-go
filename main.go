// Command deslop-go is an opinionated linter for sloppy code structure in Go
// modules. It follows A Philosophy of Software Design: deep modules, narrow
// interfaces.
//
// Usage:
//
//	deslop-go [-allow file] [-update-allow file] [-skip lints] [packages]
//
// Packages default to ./... . Every package of their modules is loaded, so
// that all the callers are known, but only findings about the packages given
// are reported. It exits 1 when there are findings, and 2 when it fails.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/samuel-form3/deslop-go/internal/lint"
	"github.com/samuel-form3/deslop-go/internal/model"
)

// errFound reports that the code has findings, rather than that the linter failed.
var errFound = errors.New("found sloppy code")

func main() {
	err := run(os.Args[1:], os.Stdout, os.Stderr)
	switch {
	case err == nil:
	case errors.Is(err, errFound):
		os.Exit(1)
	case errors.Is(err, flag.ErrHelp):
		os.Exit(2)
	default:
		fmt.Fprintln(os.Stderr, "deslop-go:", err)
		os.Exit(2)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("deslop-go", flag.ContinueOnError)
	flags.SetOutput(stderr)
	allowPath := flags.String("allow", "", "mute the findings listed in `file`")
	updatePath := flags.String("update-allow", "", "write every current finding to `file`, to mute them")
	skip := flags.String("skip", "", "comma-separated `lints` not to run, among: "+strings.Join(lint.Lints, ", "))
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: deslop-go [flags] [packages]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	patterns := flags.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	skipped, err := parseSkip(*skip)
	if err != nil {
		return err
	}

	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	m, err := model.Load(dir, patterns)
	if err != nil {
		return err
	}
	findings := slices.DeleteFunc(lint.Run(m), func(f lint.Finding) bool { return skipped[f.Lint] })

	if *updatePath != "" {
		if err := writeAllow(*updatePath, findings); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "deslop-go: muted %s in %s\n", lint.Count(len(findings)), *updatePath)
		return nil
	}
	if *allowPath != "" {
		allowed, err := readAllow(*allowPath)
		if err != nil {
			return err
		}
		var gone int
		findings, gone = allowed.filter(findings)
		if gone > 0 {
			fmt.Fprintf(stderr, "deslop-go: %s muted in %s no longer found; run with -update-allow to forget them\n", lint.Count(gone), *allowPath)
		}
	}

	lint.Print(stdout, findings, dir)
	if len(findings) == 0 {
		fmt.Fprintln(stdout, "deslop-go: no errors")
		return nil
	}
	fmt.Fprintf(stdout, "deslop-go: %s\n", lint.Count(len(findings)))
	return errFound
}

// parseSkip parses the -skip flag into the set of lints not to run.
func parseSkip(value string) (map[string]bool, error) {
	skipped := map[string]bool{}
	for name := range strings.SplitSeq(value, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if !slices.Contains(lint.Lints, name) {
			return nil, fmt.Errorf("-skip: unknown lint %q; lints are: %s", name, strings.Join(lint.Lints, ", "))
		}
		skipped[name] = true
	}
	return skipped, nil
}
