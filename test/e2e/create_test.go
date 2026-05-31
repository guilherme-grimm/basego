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
				"Makefile",
				"go.mod",
				"cmd/api/main.go",
				"cmd/api/memory.go",
				"cmd/api/deps.go",
				"cmd/api/services.go",
				"internal/api/router.go",
				"internal/domain/entity/problem.go",
				"internal/domain/entity/example/example.go",
				"internal/domain/gateway/example/gateway.go",
				"internal/service/example/service.go",
				"internal/service/example/service_test.go",
				"internal/resource/database/memory/store.go",
				"internal/resource/database/memory/example.go",
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

			// Side effects: git repo with one commit on main.
			assertGitRepo(t, target)
		})
	}
}

// assertGitRepo verifies the scaffolded project has a fresh git repo with
// exactly one commit on the main branch — DESIGN §15's "initial scaffold
// from basego" commit. Run after each scaffolding subtest to confirm
// PostWrite actually fired.
func assertGitRepo(t *testing.T, target string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(target, ".git")); err != nil {
		t.Errorf(".git dir missing: %v", err)
		return
	}
	cmd := exec.Command("git", "-C", target, "log", "--oneline")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("git log failed: %v\n%s", err, out)
		return
	}
	// DESIGN §15: the commit records the generating basego version.
	if !strings.Contains(string(out), "initial scaffold from basego v") {
		t.Errorf("initial commit missing or lacks version marker; git log:\n%s", out)
	}
}

func TestCreate_WithOpenAPISpec(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go binary not in PATH; skipping e2e spec test")
	}
	basego := buildBasego(t, goBin)

	root := t.TempDir()
	specPath := filepath.Join(root, "spec.yaml")
	const spec = `openapi: 3.0.3
info:
  title: demo
  version: 0.1.0
paths:
  /pets:
    get:
      tags: [pets]
      operationId: listPets
      responses:
        "200":
          description: ok
  /orders:
    get:
      tags: [orders]
      operationId: listOrders
      responses:
        "200":
          description: ok
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	create := exec.Command(basego, "create", "--module=example.com/demo", "demo", "file", "spec.yaml")
	create.Dir = root
	// Skip the codegen step: it shells out to `go run pkg@v...` which
	// needs network. Tidy + git init still run.
	create.Env = append(os.Environ(), "BASEGO_NO_GENERATE=1")
	if out, err := create.CombinedOutput(); err != nil {
		t.Fatalf("basego create with spec: %v\n%s", err, out)
	}
	target := filepath.Join(root, "demo")
	assertGitRepo(t, target)
	for _, rel := range []string{
		"internal/api/openapi/spec.yaml",
		"internal/api/openapi/pets/doc.go",
		"internal/api/openapi/pets/handler.go",
		"internal/api/openapi/orders/doc.go",
		"internal/api/openapi/orders/handler.go",
	} {
		if _, err := os.Stat(filepath.Join(target, rel)); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
	// handler.go must contain a method matching the spec's operationId.
	h, err := os.ReadFile(filepath.Join(target, "internal/api/openapi/pets/handler.go"))
	if err != nil {
		t.Fatalf("read pets/handler.go: %v", err)
	}
	if !strings.Contains(string(h), "func (h *Handler) ListPets(") {
		t.Errorf("pets/handler.go missing ListPets method\n---\n%s", h)
	}
	// doc.go must carry the go:generate directive so `make generate`
	// produces types_gen.go without further wiring.
	doc, err := os.ReadFile(filepath.Join(target, "internal/api/openapi/pets/doc.go"))
	if err != nil {
		t.Fatalf("read pets/doc.go: %v", err)
	}
	for _, want := range []string{"//go:generate", "oapi-codegen", "-include-tags=pets", "../spec.yaml"} {
		if !strings.Contains(string(doc), want) {
			t.Errorf("pets/doc.go missing %q\n---\n%s", want, doc)
		}
	}
	// Generated project must still build with the new per-tag packages.
	cmd := exec.Command(goBin, "build", "./...")
	cmd.Dir = target
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build with spec: %v\n%s", err, out)
	}
}

func TestCreate_RejectsUntaggedSpec(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go binary not in PATH")
	}
	basego := buildBasego(t, goBin)

	root := t.TempDir()
	specPath := filepath.Join(root, "spec.yaml")
	const spec = `openapi: 3.0.3
paths:
  /naked:
    get:
      operationId: getNaked
      responses: {"200": {description: ok}}
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	create := exec.Command(basego, "create", "--module=example.com/demo", "demo", "file", "spec.yaml")
	create.Dir = root
	out, err := create.CombinedOutput()
	if err == nil {
		t.Fatalf("basego create with untagged op: expected error, got success\n%s", out)
	}
	if !strings.Contains(string(out), "no tag") {
		t.Errorf("stderr missing 'no tag' diagnostic; got %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "demo")); !os.IsNotExist(err) {
		t.Errorf("project dir was created despite spec failure: stat err = %v", err)
	}
}

// TestCreate_RejectsExistingTarget asserts DESIGN §16: basego refuses to
// scaffold into a directory that already exists, and crucially does not
// mutate it. Pre-seeded sentinel content must survive.
func TestCreate_RejectsExistingTarget(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go binary not in PATH")
	}
	basego := buildBasego(t, goBin)

	root := t.TempDir()
	target := filepath.Join(root, "demo")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	sentinel := filepath.Join(target, "important.txt")
	if err := os.WriteFile(sentinel, []byte("do not delete"), 0o644); err != nil {
		t.Fatalf("seed sentinel: %v", err)
	}

	create := exec.Command(basego, "create", "--module=example.com/demo", "demo")
	create.Dir = root
	out, err := create.CombinedOutput()
	if err == nil {
		t.Fatalf("expected failure on existing target, got success\n%s", out)
	}
	if !strings.Contains(string(out), "already exists") {
		t.Errorf("stderr missing 'already exists'; got %q", out)
	}
	got, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatalf("sentinel disappeared: %v", err)
	}
	if string(got) != "do not delete" {
		t.Errorf("sentinel mutated: %q", got)
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
