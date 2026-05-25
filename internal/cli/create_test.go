package cli_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/guilherme-grimm/basego/internal/cli"
	"github.com/guilherme-grimm/basego/internal/scaffold"
)

func TestParseCreate_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want *scaffold.CreateRequest
	}{
		{
			name: "minimal: name only",
			args: []string{"my-app"},
			want: &scaffold.CreateRequest{
				Name: "my-app", Module: "my-app",
				Drivers: []string{"memory"},
			},
		},
		{
			name: "module overrides default",
			args: []string{"--module=github.com/foo/my-app", "my-app"},
			want: &scaffold.CreateRequest{
				Name: "my-app", Module: "github.com/foo/my-app",
				Drivers: []string{"memory"},
			},
		},
		{
			name: "db list adds drivers; memory always included",
			args: []string{"--db=mongo,postgres", "my-app"},
			want: &scaffold.CreateRequest{
				Name: "my-app", Module: "my-app",
				Drivers: []string{"memory", "mongo", "postgres"},
			},
		},
		{
			name: "db list with explicit memory is fine",
			args: []string{"--db=memory,mongo", "my-app"},
			want: &scaffold.CreateRequest{
				Name: "my-app", Module: "my-app",
				Drivers: []string{"memory", "mongo"},
			},
		},
		{
			name: "db list is normalized: case + whitespace",
			args: []string{"--db= Mongo , POSTGRES ", "my-app"},
			want: &scaffold.CreateRequest{
				Name: "my-app", Module: "my-app",
				Drivers: []string{"memory", "mongo", "postgres"},
			},
		},
		{
			name: "drivers are sorted regardless of input order",
			args: []string{"--db=postgres,mongo", "my-app"},
			want: &scaffold.CreateRequest{
				Name: "my-app", Module: "my-app",
				Drivers: []string{"memory", "mongo", "postgres"},
			},
		},
		{
			name: "file extension recognized",
			args: []string{"my-app", "file", "spec.yaml"},
			want: &scaffold.CreateRequest{
				Name: "my-app", Module: "my-app",
				Drivers:    []string{"memory"},
				Extensions: []scaffold.Extension{{Name: "file", Args: []string{"spec.yaml"}}},
			},
		},
		{
			name: ".yml suffix also accepted",
			args: []string{"my-app", "file", "spec.yml"},
			want: &scaffold.CreateRequest{
				Name: "my-app", Module: "my-app",
				Drivers:    []string{"memory"},
				Extensions: []scaffold.Extension{{Name: "file", Args: []string{"spec.yml"}}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := cli.ParseCreate(tc.args)
			if err != nil {
				t.Fatalf("ParseCreate: unexpected error %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseCreate = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseCreate_Invalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		errHas  string
	}{
		{"missing name", []string{}, "missing project name"},
		{"invalid name (leading digit)", []string{"9app"}, "invalid project name"},
		{"invalid name (slash)", []string{"my/app"}, "invalid project name"},
		{"module with whitespace", []string{"--module=foo bar", "my-app"}, "whitespace"},
		{"unknown driver", []string{"--db=redis", "my-app"}, `unknown driver "redis"`},
		{"duplicate driver", []string{"--db=mongo,mongo", "my-app"}, `duplicate driver "mongo"`},
		{"empty driver entry", []string{"--db=mongo,,postgres", "my-app"}, "empty driver"},
		{"unsupported extension", []string{"my-app", "bogus"}, `unsupported extension "bogus"`},
		{"file extension missing arg", []string{"my-app", "file"}, `requires 1 argument`},
		{"file extension bad suffix", []string{"my-app", "file", "spec.json"}, ".yaml/.yml"},
		{"unknown flag", []string{"--nope", "my-app"}, "flag parse"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := cli.ParseCreate(tc.args)
			if err == nil {
				t.Fatalf("ParseCreate: expected error containing %q, got nil", tc.errHas)
			}
			if !strings.Contains(err.Error(), tc.errHas) {
				t.Errorf("error %q missing substring %q", err.Error(), tc.errHas)
			}
		})
	}
}

// TestParseCreate_RunIntegration verifies the cli.Run wrapper maps parser
// errors to ErrUsage and writes diagnostics to stderr.
func TestParseCreate_RunIntegration(t *testing.T) {
	t.Parallel()
	var stdout, stderr strings.Builder
	err := cli.Run([]string{"basego", "create", "--db=redis", "my-app"}, &stdout, &stderr)
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("Run: err = %v, want ErrUsage", err)
	}
	if !strings.Contains(stderr.String(), "unknown driver") {
		t.Errorf("stderr missing diagnostic; got %q", stderr.String())
	}
}
