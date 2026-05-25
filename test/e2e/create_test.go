// Package e2e exercises basego end-to-end: it scaffolds a project into a
// temporary directory and runs the host `go` toolchain against the result.
// Tests skip cleanly when no go binary is available.
package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// basegoBin builds the basego binary once per test process and reuses it
// across the driver-matrix subtests.
var (
	basegoBinOnce sync.Once
	basegoBinPath string
	basegoBinErr  error
)

func TestCreate_GeneratedProjectBuilds(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go binary not in PATH; skipping e2e build test")
	}

	tests := []struct {
		name       string
		dbFlag     string
		extraFiles []string // beyond the always-present base set
	}{
		{"default (memory only)", "", nil},
		{"mongo", "--db=mongo", []string{
			"cmd/api/mongo.go",
			"internal/resource/database/mongo/client.go",
		}},
		{"postgres", "--db=postgres", []string{
			"cmd/api/postgres.go",
			"internal/resource/database/postgres/client.go",
		}},
		{"mongo+postgres", "--db=mongo,postgres", []string{
			"cmd/api/mongo.go",
			"cmd/api/postgres.go",
			"internal/resource/database/mongo/client.go",
			"internal/resource/database/postgres/client.go",
		}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			basego := buildBasego(t, goBin)
			root := t.TempDir()
			target := filepath.Join(root, "demo")

			args := []string{"create", "--module=example.com/demo"}
			if tc.dbFlag != "" {
				args = append(args, tc.dbFlag)
			}
			args = append(args, "demo")

			create := exec.Command(basego, args...)
			create.Dir = root
			if out, err := create.CombinedOutput(); err != nil {
				t.Fatalf("basego create %v: %v\n%s", args, err, out)
			}

			// Always-present files
			for _, rel := range []string{
				"go.mod",
				"cmd/api/main.go",
				"cmd/api/memory.go",
				"internal/api/router.go",
				"internal/resource/database/memory/store.go",
			} {
				if _, err := os.Stat(filepath.Join(target, rel)); err != nil {
					t.Errorf("missing required file %s: %v", rel, err)
				}
			}
			for _, rel := range tc.extraFiles {
				if _, err := os.Stat(filepath.Join(target, rel)); err != nil {
					t.Errorf("missing driver file %s: %v", rel, err)
				}
			}

			// Generated project must build and its tests must pass.
			for _, gargs := range [][]string{{"build", "./..."}, {"test", "./..."}} {
				cmd := exec.Command(goBin, gargs...)
				cmd.Dir = target
				cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("go %v in generated project failed: %v\n%s", gargs, err, out)
				}
			}
		})
	}
}

// buildBasego compiles the basego binary once and returns the path to the
// resulting executable. Shared across all subtests in the process.
func buildBasego(t *testing.T, goBin string) string {
	t.Helper()
	basegoBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "basego-bin-*")
		if err != nil {
			basegoBinErr = err
			return
		}
		basegoBinPath = filepath.Join(dir, "basego")
		build := exec.Command(goBin, "build", "-o", basegoBinPath, "./cmd/basego")
		build.Dir = repoRoot(t)
		build.Env = append(os.Environ(), "CGO_ENABLED=0")
		if out, err := build.CombinedOutput(); err != nil {
			basegoBinErr = wrapBuildErr(err, out)
		}
	})
	if basegoBinErr != nil {
		t.Fatalf("build basego: %v", basegoBinErr)
	}
	return basegoBinPath
}

func wrapBuildErr(err error, out []byte) error {
	return &buildErr{err: err, out: strings.TrimSpace(string(out))}
}

type buildErr struct {
	err error
	out string
}

func (e *buildErr) Error() string { return e.err.Error() + "\n" + e.out }

// repoRoot returns the basego repo root by walking up from the test file
// until a go.mod whose module path is the basego module is found.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		modPath := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(modPath); err == nil {
			if firstLine(data) == "module github.com/guilherme-grimm/basego" {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find basego repo root from cwd")
		}
		dir = parent
	}
}

func firstLine(b []byte) string {
	for i, c := range b {
		if c == '\n' {
			return string(b[:i])
		}
	}
	return string(b)
}
