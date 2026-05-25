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
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(p, templatesRoot+"/")
		rel = strings.TrimSuffix(rel, tmplSuffix)

		raw, err := templatesFS.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		content := raw
		if strings.HasSuffix(p, tmplSuffix) {
			t, err := template.New(rel).Option("missingkey=error").Parse(string(raw))
			if err != nil {
				return fmt.Errorf("parse %s: %w", p, err)
			}
			var buf bytes.Buffer
			if err := t.Execute(&buf, req); err != nil {
				return fmt.Errorf("execute %s: %w", p, err)
			}
			content = buf.Bytes()
		}
		files = append(files, File{Path: rel, Content: content})
		return nil
	})
	if err != nil {
		return Plan{}, err
	}
	return Plan{Target: req.Name, Files: files}, nil
}
