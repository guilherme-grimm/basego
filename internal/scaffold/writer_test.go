package scaffold_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/guilherme-grimm/basego/internal/scaffold"
)

func TestWrite_HappyPath(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "my-app")
	plan := scaffold.Plan{
		Target: target,
		Files: []scaffold.File{
			{Path: "README.md", Content: []byte("hello\n")},
			{Path: "cmd/api/main.go", Content: []byte("package main\n")},
			{Path: "internal/api/router.go", Content: []byte("package api\n")},
		},
	}
	if err := scaffold.Write(plan); err != nil {
		t.Fatalf("Write: %v", err)
	}
	for _, f := range plan.Files {
		got, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(f.Path)))
		if err != nil {
			t.Fatalf("read %s: %v", f.Path, err)
		}
		if string(got) != string(f.Content) {
			t.Errorf("file %s: content %q, want %q", f.Path, got, f.Content)
		}
	}
}

func TestWrite_RejectsExistingTarget(t *testing.T) {
	t.Parallel()
	target := t.TempDir() // already exists
	err := scaffold.Write(scaffold.Plan{
		Target: target,
		Files:  []scaffold.File{{Path: "a.txt", Content: []byte("x")}},
	})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Write: err = %v, want 'already exists'", err)
	}
}

func TestWrite_EmptyTarget(t *testing.T) {
	t.Parallel()
	err := scaffold.Write(scaffold.Plan{Target: "   "})
	if err == nil || !strings.Contains(err.Error(), "target is empty") {
		t.Fatalf("Write: err = %v, want 'target is empty'", err)
	}
}

func TestWrite_InvalidPaths(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		path   string
		errHas string
	}{
		{"empty path", "", "path is empty"},
		{"absolute path", "/etc/passwd", "is absolute"},
		{"traversal explicit", "../escape.txt", "escapes target"},
		{"traversal nested", "foo/../../escape.txt", "escapes target"},
		{"dot path", ".", "escapes target"},
		{"backslash path", `foo\bar.txt`, "backslash"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			target := filepath.Join(t.TempDir(), "out")
			err := scaffold.Write(scaffold.Plan{
				Target: target,
				Files:  []scaffold.File{{Path: tc.path, Content: []byte("x")}},
			})
			if err == nil {
				t.Fatalf("Write: expected error containing %q, got nil", tc.errHas)
			}
			if !strings.Contains(err.Error(), tc.errHas) {
				t.Errorf("err %q missing %q", err, tc.errHas)
			}
			// Target must not be created when validation fails.
			if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
				t.Errorf("target was created despite validation failure: stat err = %v", statErr)
			}
		})
	}
}

func TestWrite_DuplicatePaths(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "out")
	err := scaffold.Write(scaffold.Plan{
		Target: target,
		Files: []scaffold.File{
			{Path: "a.txt", Content: []byte("1")},
			{Path: "a.txt", Content: []byte("2")},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("Write: err = %v, want 'duplicate'", err)
	}
}

func TestWrite_FileMode(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "out")
	err := scaffold.Write(scaffold.Plan{
		Target: target,
		Files: []scaffold.File{
			{Path: "default.txt", Content: []byte("a")},
			{Path: "script.sh", Content: []byte("#!/bin/sh\n"), Mode: 0o755},
		},
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	cases := []struct {
		path string
		want fs.FileMode
	}{
		{"default.txt", 0o644},
		{"script.sh", 0o755},
	}
	for _, c := range cases {
		info, err := os.Stat(filepath.Join(target, c.path))
		if err != nil {
			t.Fatalf("stat %s: %v", c.path, err)
		}
		if got := info.Mode().Perm(); got != c.want {
			t.Errorf("%s: mode = %o, want %o", c.path, got, c.want)
		}
	}
}

// TestWrite_DeterministicOrdering checks that the on-disk creation order
// matches the sorted path order regardless of input order. We assert via
// mtime ordering — same logical contract, same observable result.
func TestWrite_DeterministicOrdering(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "out")
	// Input order intentionally not sorted.
	plan := scaffold.Plan{
		Target: target,
		Files: []scaffold.File{
			{Path: "zeta.txt", Content: []byte("z")},
			{Path: "alpha.txt", Content: []byte("a")},
			{Path: "middle.txt", Content: []byte("m")},
		},
	}
	if err := scaffold.Write(plan); err != nil {
		t.Fatalf("Write: %v", err)
	}
	type stamped struct {
		name  string
		mtime int64
	}
	var got []stamped
	for _, f := range plan.Files {
		info, err := os.Stat(filepath.Join(target, f.Path))
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		got = append(got, stamped{f.Path, info.ModTime().UnixNano()})
	}
	sort.Slice(got, func(i, j int) bool { return got[i].mtime < got[j].mtime })
	wantOrder := []string{"alpha.txt", "middle.txt", "zeta.txt"}
	for i, s := range got {
		if s.name != wantOrder[i] {
			// Same-mtime is OK on coarse filesystems; only fail on
			// clear out-of-order writes.
			if i > 0 && s.mtime == got[i-1].mtime {
				continue
			}
			t.Errorf("position %d: %s, want %s", i, s.name, wantOrder[i])
		}
	}
}
