package cli

import (
	"fmt"
	"io"
	"regexp"
)

// nameRe matches a conservative project-name shape: starts with a letter,
// then letters/digits/dash/underscore. Keeps room for directory and
// module-path safety without locking in module-path rules yet.
var nameRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

func runCreate(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "basego create: missing project name")
		fmt.Fprintln(stderr, "usage: basego create <name> [extension ...]")
		return ErrUsage
	}
	name := args[0]
	if !nameRe.MatchString(name) {
		fmt.Fprintf(stderr, "basego create: invalid project name %q\n", name)
		fmt.Fprintln(stderr, "names must start with a letter and contain only letters, digits, '-' or '_'")
		return ErrUsage
	}
	// Extensions and flags land in Deliverable 2. Reject anything extra
	// for now so we don't silently accept input we won't honor.
	if len(args) > 1 {
		fmt.Fprintf(stderr, "basego create: extensions and flags are not yet implemented (got %v)\n", args[1:])
		return ErrUsage
	}
	fmt.Fprintf(stdout, "basego create: would scaffold %q (scaffolding lands in a later deliverable)\n", name)
	return nil
}
