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
//  4. git init + initial commit ("initial scaffold from basego v<version>")
//
// Each network-touching step honors a BASEGO_NO_* env-var escape hatch so
// tests can run hermetically. Production users leave them unset and get
// PostWrite runs deterministic post-scaffolding commands inside the given target directory.
// It formats Go files, optionally runs `go generate` when the request has a Spec and
// BASEGO_NO_GENERATE is unset, optionally runs `go mod tidy` when BASEGO_NO_TIDY is unset,
// and optionally initializes a git repository with an initial commit when BASEGO_NO_GIT is unset.
// Errors from each step are returned wrapped with a short prefix identifying the failing step.
func PostWrite(target string, req *CreateRequest, version string) error {
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
		if err := initGitRepo(target, version); err != nil {
			return fmt.Errorf("scaffold: git init: %w", err)
		}
	}
	return nil
}

// initGitRepo creates a fresh repo on `main` and stages the initial
// commit. user.name/user.email are injected per-invocation so the user's
// initGitRepo initializes a new Git repository in target on branch main, stages all files,
// and creates an initial commit with a fixed author identity.
 // It runs `git init --initial-branch=main -q`, `git add .`, and
// `git -c user.name=basego -c user.email=basego@localhost commit -q -m "initial scaffold from basego v<version>"`.
// Returns any error produced while running these commands.
func initGitRepo(target, version string) error {
	msg := fmt.Sprintf("initial scaffold from basego v%s", version)
	for _, args := range [][]string{
		{"init", "--initial-branch=main", "-q"},
		{"add", "."},
		{
			"-c", "user.name=basego",
			"-c", "user.email=basego@localhost",
			"commit", "-q", "-m", msg,
		},
	} {
		if err := runIn(target, "git", args...); err != nil {
			return err
		}
	}
	return nil
}

// runIn runs the named binary with the provided arguments in dir and returns any execution error.
// It sets the process working directory to dir and captures combined stdout and stderr; on failure
// it returns an error that includes the binary, arguments, the underlying error, and the captured output.
func runIn(dir, bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v failed: %w\n%s", bin, args, err, out)
	}
	return nil
}
