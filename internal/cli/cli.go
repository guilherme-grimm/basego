package cli

import (
	"fmt"
	"io"
)

const usage = `usage:
  basego create <name> [extension ...]
  basego version`

// Run is the CLI entrypoint. args is the full argv (args[0] is the program
// name). stdout receives command output; stderr receives diagnostics and
// usage messages. A non-nil error signals failure; ErrUsage in particular
// indicates the caller invoked basego incorrectly.
func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) < 2 {
		fmt.Fprintln(stderr, usage)
		return ErrUsage
	}
	switch args[1] {
	case "version":
		return runVersion(stdout)
	case "create":
		return runCreate(args[2:], stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, usage)
		return nil
	default:
		fmt.Fprintf(stderr, "basego: unknown command %q\n%s\n", args[1], usage)
		return ErrUsage
	}
}
