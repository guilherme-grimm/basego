package scaffold

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"text/template"
)

//go:embed all:templates
var templatesFS embed.FS

const (
	templatesRoot = "templates"
	driversPrefix = "drivers/"
	tmplSuffix    = ".tmpl"
)

// Render expands the embedded skeleton templates against req and returns a
// Plan ready for Write. The Plan's Target is req.Name; callers (or tests)
// may override it before passing to Write.
//
// Templates ending in ".tmpl" are run through text/template with req as the
// dot value. Other files are copied verbatim. The result is deterministic:
// the Plan's file order is whatever validatePlan sorts it into in Write.
func Render(req *CreateRequest) (Plan, error) {
	if req == nil {
		return Plan{}, fmt.Errorf("scaffold: nil request")
	}
	var files []File
	err := fs.WalkDir(templatesFS, templatesRoot, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if p == templatesRoot {
			return nil
		}
		rel := strings.TrimPrefix(p, templatesRoot+"/")

		outRel, action := mapTemplatePath(req, rel, d.IsDir())
		switch action {
		case actionSkipDir:
			return fs.SkipDir
		case actionSkip:
			return nil
		}
		raw, err := templatesFS.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		content := raw
		if strings.HasSuffix(p, tmplSuffix) {
			t, err := template.New(outRel).Option("missingkey=error").Parse(string(raw))
			if err != nil {
				return fmt.Errorf("parse %s: %w", p, err)
			}
			var buf bytes.Buffer
			if err := t.Execute(&buf, req); err != nil {
				return fmt.Errorf("execute %s: %w", p, err)
			}
			content = buf.Bytes()
		}
		files = append(files, File{Path: outRel, Content: content})
		return nil
	})
	if err != nil {
		return Plan{}, err
	}
	return Plan{Target: req.Name, Files: files}, nil
}

type pathAction int

const (
	actionEmit    pathAction = iota // emit a file with the returned outRel
	actionSkip                      // skip this entry (e.g. directory, unselected driver file)
	actionSkipDir                   // skip the whole subtree (unselected driver dir)
)

// mapTemplatePath resolves an embed-relative path to its output path and
// decides whether to render it. Files under drivers/<name>/ are only
// rendered when req.HasDriver(name); the drivers/<name>/ prefix is then
// stripped from the output.
func mapTemplatePath(req *CreateRequest, rel string, isDir bool) (string, pathAction) {
	if rel == "drivers" {
		return "", actionSkip
	}
	if strings.HasPrefix(rel, driversPrefix) {
		sub := strings.TrimPrefix(rel, driversPrefix)
		driver, inner, _ := splitOnce(sub, "/")
		if !req.HasDriver(driver) {
			if isDir {
				return "", actionSkipDir
			}
			return "", actionSkip
		}
		if isDir || inner == "" {
			return "", actionSkip
		}
		return strings.TrimSuffix(inner, tmplSuffix), actionEmit
	}
	if isDir {
		return "", actionSkip
	}
	return strings.TrimSuffix(rel, tmplSuffix), actionEmit
}

func splitOnce(s, sep string) (head, tail string, ok bool) {
	i := strings.Index(s, sep)
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+len(sep):], true
}
