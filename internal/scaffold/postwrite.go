package scaffold

import (
	"fmt"
	"os"
	"os/exec"
)

// PostWrite runs the deterministic side effects from DESIGN §15 against
// the just-scaffolded project at target, in fixed order:
//
//  1. gofmt -w .                (canonical formatting, helps determinism)
//  2. go generate ./...         (only when req.Spec is set — file ext used)
//  3. go mod tidy               (resolve deps; near-no-op on stdlib-only)
//  4. git init + initial commit ("initial scaffold from basego")
//
// Each network-touching step honors a BASEGO_NO_* env-var escape hatch so
// tests can run hermetically. Production users leave them unset and get
// the full sequence.
func PostWrite(target string, req *CreateRequest) error {
	if err := runIn(target, "gofmt", "-w", "."); err != nil {
		return fmt.Errorf("scaffold: gofmt: %w", err)
	}
	if req.Spec != nil && os.Getenv("BASEGO_NO_GENERATE") == "" {
		if err := runIn(target, "go", "generate", "./..."); err != nil {
			return fmt.Errorf("scaffold: go generate: %w", err)
		}
	}
	if os.Getenv("BASEGO_NO_TIDY") == "" {
		if err := runIn(target, "go", "mod", "tidy"); err != nil {
			return fmt.Errorf("scaffold: go mod tidy: %w", err)
		}
	}
	if os.Getenv("BASEGO_NO_GIT") == "" {
		if err := initGitRepo(target); err != nil {
			return fmt.Errorf("scaffold: git init: %w", err)
		}
	}
	return nil
}

// initGitRepo creates a fresh repo on `main` and stages the initial
// commit. user.name/user.email are injected per-invocation so the user's
// global git identity is not required (matters in CI containers).
func initGitRepo(target string) error {
	for _, args := range [][]string{
		{"init", "--initial-branch=main", "-q"},
		{"add", "."},
		{
			"-c", "user.name=basego",
			"-c", "user.email=basego@localhost",
			"commit", "-q", "-m", "initial scaffold from basego",
		},
	} {
		if err := runIn(target, "git", args...); err != nil {
			return err
		}
	}
	return nil
}

func runIn(dir, bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v failed: %w\n%s", bin, args, err, out)
	}
	return nil
}
