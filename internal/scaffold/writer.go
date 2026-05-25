package scaffold

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// File is one file to materialize under Plan.Target. Path is relative and
// uses forward slashes regardless of OS — plans are portable, not platform
// specific. Mode of zero defaults to 0o644.
type File struct {
	Path    string
	Content []byte
	Mode    fs.FileMode
}

// Plan is the deterministic list of files that make up a scaffolded
// project. The writer sorts by path before writing so output ordering is
// stable regardless of how the plan was assembled — see PLAN.md §5.
type Plan struct {
	Target string
	Files  []File
}

// Write materializes p under p.Target. It hard-errors if Target already
// exists (PLAN.md §16) and rejects any file whose path is absolute,
// contains backslashes, or escapes Target via "..".
func Write(p Plan) error {
	if strings.TrimSpace(p.Target) == "" {
		return errors.New("scaffold: target is empty")
	}
	switch _, err := os.Stat(p.Target); {
	case err == nil:
		return fmt.Errorf("scaffold: target %q already exists", p.Target)
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("scaffold: stat target: %w", err)
	}

	files, err := validatePlan(p.Files)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(p.Target, 0o755); err != nil {
		return fmt.Errorf("scaffold: create target: %w", err)
	}
	for _, f := range files {
		abs := filepath.Join(p.Target, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return fmt.Errorf("scaffold: mkdir for %q: %w", f.Path, err)
		}
		mode := f.Mode
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(abs, f.Content, mode); err != nil {
			return fmt.Errorf("scaffold: write %q: %w", f.Path, err)
		}
	}
	return nil
}

// validatePlan validates every entry up front and returns a sorted copy of
// the file list. The whole plan is rejected on the first invalid entry so
// the target directory stays untouched on failure.
func validatePlan(in []File) ([]File, error) {
	seen := make(map[string]struct{}, len(in))
	out := make([]File, len(in))
	for i, f := range in {
		if err := validateRelPath(f.Path); err != nil {
			return nil, fmt.Errorf("scaffold: %w", err)
		}
		if _, dup := seen[f.Path]; dup {
			return nil, fmt.Errorf("scaffold: duplicate path %q", f.Path)
		}
		seen[f.Path] = struct{}{}
		out[i] = f
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// validateRelPath enforces portable, traversal-safe relative paths.
func validateRelPath(p string) error {
	if p == "" {
		return errors.New("path is empty")
	}
	if strings.Contains(p, `\`) {
		return fmt.Errorf("path %q contains backslash; use forward slashes", p)
	}
	if path.IsAbs(p) || filepath.IsAbs(p) {
		return fmt.Errorf("path %q is absolute", p)
	}
	clean := path.Clean(p)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("path %q escapes target", p)
	}
	return nil
}
