package cli_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/guilherme-grimm/basego/internal/cli"
)

// versionLine documents the contract from DESIGN.md §2:
//
//	basego <semver> (commit <short>, built <date>)
var versionLine = regexp.MustCompile(`^basego \S+ \(commit \S+, built \S+\)$`)

func TestVersionOutputShape(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	if err := cli.Run([]string{"basego", "version"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run version: %v (stderr=%q)", err, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("version wrote to stderr: %q", stderr.String())
	}
	line := strings.TrimRight(stdout.String(), "\n")
	if !versionLine.MatchString(line) {
		t.Errorf("version line %q does not match %q", line, versionLine)
	}
}
