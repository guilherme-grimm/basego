// Package e2e exercises basego end-to-end: it scaffolds a project into a
// temporary directory and runs the host `go` toolchain against the result.
// Tests skip cleanly when no go binary is available.
package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCreate_GeneratedProjectBuilds(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go binary not in PATH; skipping e2e build test")
	}

	root := t.TempDir()
	target := filepath.Join(root, "demo")

	// Build the basego binary into a known path so we can invoke it from
	// inside the temp workdir without depending on $PATH.
	basego := filepath.Join(root, "basego")
	build := exec.Command(goBin, "build", "-o", basego, "./cmd/basego")
	build.Dir = repoRoot(t)
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build basego: %v\n%s", err, out)
	}

	// Run `basego create demo --module=example.com/demo` from root.
	create := exec.Command(basego, "create", "--module=example.com/demo", "demo")
	create.Dir = root
	if out, err := create.CombinedOutput(); err != nil {
		t.Fatalf("basego create: %v\n%s", err, out)
	}

	for _, rel := range []string{"go.mod", "cmd/api/main.go", "internal/api/router.go"} {
		if _, err := os.Stat(filepath.Join(target, rel)); err != nil {
			t.Errorf("expected %s in generated project: %v", rel, err)
		}
	}

	// Generated project must build and its tests must pass.
	for _, args := range [][]string{
		{"build", "./..."},
		{"test", "./..."},
	} {
		cmd := exec.Command(goBin, args...)
		cmd.Dir = target
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %v in generated project failed: %v\n%s", args, err, out)
		}
	}
}

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
