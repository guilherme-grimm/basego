package cli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/guilherme-grimm/basego/internal/cli"
)

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		wantErr     error
		stdoutHas   string
		stderrHas   string
		stdoutEmpty bool
		stderrEmpty bool
	}{
		{
			name:      "no args prints usage to stderr and errors",
			args:      []string{"basego"},
			wantErr:   cli.ErrUsage,
			stderrHas: "usage:",
		},
		{
			name:      "unknown command errors with named diagnostic",
			args:      []string{"basego", "bogus"},
			wantErr:   cli.ErrUsage,
			stderrHas: `unknown command "bogus"`,
		},
		{
			name:      "version writes to stdout",
			args:      []string{"basego", "version"},
			stdoutHas: "basego ",
		},
		{
			name:      "help writes usage to stdout",
			args:      []string{"basego", "help"},
			stdoutHas: "basego create",
		},
		{
			name:      "create with no name errors",
			args:      []string{"basego", "create"},
			wantErr:   cli.ErrUsage,
			stderrHas: "missing project name",
		},
		{
			name:      "create with invalid name errors",
			args:      []string{"basego", "create", "9bad"},
			wantErr:   cli.ErrUsage,
			stderrHas: "invalid project name",
		},
		{
			name:      "create with valid name accepted",
			args:      []string{"basego", "create", "my-app"},
			stdoutHas: `"my-app"`,
		},
		{
			name:      "create with extras rejected (deliverable 2)",
			args:      []string{"basego", "create", "my-app", "file", "spec.yaml"},
			wantErr:   cli.ErrUsage,
			stderrHas: "not yet implemented",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			err := cli.Run(tc.args, &stdout, &stderr)

			if tc.wantErr == nil && err != nil {
				t.Fatalf("Run: unexpected error %v (stderr=%q)", err, stderr.String())
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("Run: err = %v, want errors.Is(%v)", err, tc.wantErr)
			}
			if tc.stdoutHas != "" && !strings.Contains(stdout.String(), tc.stdoutHas) {
				t.Errorf("stdout missing %q; got %q", tc.stdoutHas, stdout.String())
			}
			if tc.stderrHas != "" && !strings.Contains(stderr.String(), tc.stderrHas) {
				t.Errorf("stderr missing %q; got %q", tc.stderrHas, stderr.String())
			}
		})
	}
}
