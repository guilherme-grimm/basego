package scaffold_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/guilherme-grimm/basego/internal/scaffold"
)

func TestRender_ProducesExpectedFiles(t *testing.T) {
	t.Parallel()
	req := &scaffold.CreateRequest{
		Name:    "demo",
		Module:  "example.com/demo",
		Drivers: []string{"memory"},
	}
	plan, err := scaffold.Render(req)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if plan.Target != "demo" {
		t.Errorf("Target = %q, want %q", plan.Target, "demo")
	}
	got := map[string][]byte{}
	for _, f := range plan.Files {
		got[f.Path] = f.Content
	}
	wantPaths := []string{
		".gitattributes",
		".gitignore",
		"README.md",
		"cmd/api/main.go",
		"config/config.yaml",
		"go.mod",
		"internal/api/health.go",
		"internal/api/health_test.go",
		"internal/api/router.go",
	}
	var gotPaths []string
	for p := range got {
		gotPaths = append(gotPaths, p)
	}
	sort.Strings(gotPaths)
	if strings.Join(gotPaths, ",") != strings.Join(wantPaths, ",") {
		t.Errorf("files mismatch:\n got:  %v\n want: %v", gotPaths, wantPaths)
	}
}

func TestRender_SubstitutesModuleAndName(t *testing.T) {
	t.Parallel()
	req := &scaffold.CreateRequest{Name: "demo", Module: "example.com/demo"}
	plan, err := scaffold.Render(req)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, f := range plan.Files {
		switch f.Path {
		case "go.mod":
			if !strings.Contains(string(f.Content), "module example.com/demo") {
				t.Errorf("go.mod missing module line; got %q", f.Content)
			}
		case "README.md":
			if !strings.Contains(string(f.Content), "# demo") {
				t.Errorf("README.md missing project name; got %q", f.Content)
			}
		case "cmd/api/main.go":
			if !strings.Contains(string(f.Content), `"example.com/demo/internal/api"`) {
				t.Errorf("main.go missing module import; got %q", f.Content)
			}
		}
	}
}

func TestRender_NilRequest(t *testing.T) {
	t.Parallel()
	_, err := scaffold.Render(nil)
	if err == nil || !strings.Contains(err.Error(), "nil request") {
		t.Fatalf("Render(nil): err = %v, want 'nil request'", err)
	}
}
