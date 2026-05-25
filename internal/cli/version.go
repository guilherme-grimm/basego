package cli

import (
	"fmt"
	"io"
)

// Populated via -ldflags at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func versionString() string {
	return fmt.Sprintf("basego %s (commit %s, built %s)", version, commit, date)
}

func runVersion(stdout io.Writer) error {
	_, err := fmt.Fprintln(stdout, versionString())
	return err
}
