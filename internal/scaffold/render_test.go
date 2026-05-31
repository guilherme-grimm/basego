package scaffold_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/guilherme-grimm/basego/internal/oapi"
	"github.com/guilherme-grimm/basego/internal/scaffold"
)

// baseFiles are emitted regardless of driver selection.
var baseFiles = []string{
	".gitattributes",
	".gitignore",
	"Dockerfile",
	"Makefile",
	"README.md",
	"docker-compose.yml",
	"cmd/api/deps.go",
	"cmd/api/main.go",
	"cmd/api/memory.go",
	"cmd/api/services.go",
	"config/config.yaml",
	"go.mod",
	"internal/api/health.go",
	"internal/api/health_test.go",
	"internal/api/router.go",
	"internal/domain/entity/example/errors.go",
	"internal/domain/entity/example/example.go",
	"internal/domain/entity/problem.go",
	"internal/domain/gateway/example/gateway.go",
	"internal/resource/database/memory/example.go",
	"internal/resource/database/memory/store.go",
	"internal/service/example/service.go",
	"internal/service/example/service_test.go",
}

func TestRender_DriverMatrix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		drivers []string
		extra   []string // files added on top of baseFiles
	}{
		{"memory only", []string{"memory"}, nil},
		{"with mongo", []string{"memory", "mongo"}, []string{
			"cmd/api/mongo.go",
			"internal/resource/database/mongo/client.go",
			"internal/resource/database/mongo/example.go",
		}},
		{"with postgres", []string{"memory", "postgres"}, []string{
			"cmd/api/postgres.go",
			"internal/resource/database/postgres/client.go",
			"internal/resource/database/postgres/example.go",
		}},
		{"with both", []string{"memory", "mongo", "postgres"}, []string{
			"cmd/api/mongo.go",
			"cmd/api/postgres.go",
			"internal/resource/database/mongo/client.go",
			"internal/resource/database/mongo/example.go",
			"internal/resource/database/postgres/client.go",
			"internal/resource/database/postgres/example.go",
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := &scaffold.CreateRequest{
				Name:    "demo",
				Module:  "example.com/demo",
				Drivers: tc.drivers,
			}
			plan, err := scaffold.Render(req)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}

			var got []string
			for _, f := range plan.Files {
				got = append(got, f.Path)
			}
			sort.Strings(got)

			want := append([]string{}, baseFiles...)
			want = append(want, tc.extra...)
			sort.Strings(want)

			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("files mismatch:\n got:  %v\n want: %v", got, want)
			}
		})
	}
}

func TestRender_ConfigYAMLContainsSelectedResources(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		drivers     []string
		mustContain []string
		mustNot     []string
	}{
		{
			"memory only — no resources block",
			[]string{"memory"},
			nil,
			[]string{"resources:", "mongo:", "postgres:"},
		},
		{
			"mongo only",
			[]string{"memory", "mongo"},
			[]string{"resources:", "mongo:", "uri: mongodb://"},
			[]string{"postgres:"},
		},
		{
			"both",
			[]string{"memory", "mongo", "postgres"},
			[]string{"mongo:", "postgres:", "dsn: postgres://"},
			nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := &scaffold.CreateRequest{Name: "demo", Module: "example.com/demo", Drivers: tc.drivers}
			plan, err := scaffold.Render(req)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			var yaml string
			for _, f := range plan.Files {
				if f.Path == "config/config.yaml" {
					yaml = string(f.Content)
					break
				}
			}
			if yaml == "" {
				t.Fatal("config/config.yaml not in plan")
			}
			for _, s := range tc.mustContain {
				if !strings.Contains(yaml, s) {
					t.Errorf("config.yaml missing %q\n---\n%s", s, yaml)
				}
			}
			for _, s := range tc.mustNot {
				if strings.Contains(yaml, s) {
					t.Errorf("config.yaml unexpectedly contains %q\n---\n%s", s, yaml)
				}
			}
		})
	}
}

func TestRender_DockerComposeServices(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		drivers     []string
		mustContain []string
		mustNot     []string
	}{
		{
			"memory only — app service alone",
			[]string{"memory"},
			[]string{"app:", "build: ."},
			[]string{"mongo:", "postgres:", "depends_on:"},
		},
		{
			"mongo adds its service + dependency",
			[]string{"memory", "mongo"},
			[]string{"mongo:", "image: mongo:7", "depends_on:", "RESOURCES_MONGO_URI: mongodb://mongo:27017"},
			[]string{"postgres:"},
		},
		{
			"both DBs present",
			[]string{"memory", "mongo", "postgres"},
			[]string{"mongo:", "postgres:", "image: postgres:16", "RESOURCES_POSTGRES_DSN: postgres://"},
			nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := &scaffold.CreateRequest{Name: "demo", Module: "example.com/demo", Drivers: tc.drivers}
			plan, err := scaffold.Render(req)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			var compose string
			for _, f := range plan.Files {
				if f.Path == "docker-compose.yml" {
					compose = string(f.Content)
					break
				}
			}
			if compose == "" {
				t.Fatal("docker-compose.yml not in plan")
			}
			for _, s := range tc.mustContain {
				if !strings.Contains(compose, s) {
					t.Errorf("compose missing %q\n---\n%s", s, compose)
				}
			}
			for _, s := range tc.mustNot {
				if strings.Contains(compose, s) {
					t.Errorf("compose unexpectedly contains %q\n---\n%s", s, compose)
				}
			}
		})
	}
}

func TestRender_SubstitutesModuleAndName(t *testing.T) {
	t.Parallel()
	req := &scaffold.CreateRequest{Name: "demo", Module: "example.com/demo", Drivers: []string{"memory"}}
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
		case "cmd/api/memory.go":
			if !strings.Contains(string(f.Content), `"example.com/demo/internal/resource/database/memory"`) {
				t.Errorf("memory.go missing module import; got %q", f.Content)
			}
		}
	}
}

func TestRender_EmitsSpecAndPerTagStubs(t *testing.T) {
	t.Parallel()
	spec, err := oapi.Parse([]byte(`
openapi: 3.0.3
paths:
  /pets:
    get:
      tags: [pets]
      operationId: listPets
  /orders:
    get:
      tags: [orders]
      operationId: listOrders
`))
	if err != nil {
		t.Fatalf("oapi.Parse: %v", err)
	}
	req := &scaffold.CreateRequest{
		Name:      "demo",
		Module:    "example.com/demo",
		Drivers:   []string{"memory"},
		Spec:      spec,
		SpecBytes: []byte("openapi: 3.0.3\n"),
	}
	plan, err := scaffold.Render(req)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	wantPaths := map[string][]string{
		"internal/api/openapi/spec.yaml":     {"openapi: 3.0.3\n"},
		"internal/api/openapi/pets/doc.go":   {"package pets", "//go:generate", "-include-tags=pets", "-o=types_gen.go ../spec.yaml"},
		"internal/api/openapi/orders/doc.go": {"package orders", "//go:generate", "-include-tags=orders"},
	}
	got := map[string]string{}
	for _, f := range plan.Files {
		got[f.Path] = string(f.Content)
	}
	for path, mustHaves := range wantPaths {
		content, ok := got[path]
		if !ok {
			t.Errorf("missing file %s", path)
			continue
		}
		for _, mustHave := range mustHaves {
			if !strings.Contains(content, mustHave) {
				t.Errorf("file %s missing %q\n---\n%s", path, mustHave, content)
			}
		}
	}
}

func TestRender_HandlerGoPerSlice(t *testing.T) {
	t.Parallel()
	spec, err := oapi.Parse([]byte(`
openapi: 3.0.3
paths:
  /pets:
    get:
      tags: [pets]
      operationId: listPets
    post:
      tags: [pets]
      operationId: createPet
  /pets/{id}:
    get:
      tags: [pets]
      operationId: getPet
  /pets/search:
    post:
      tags: [pets]
      operationId: searchPets
`))
	if err != nil {
		t.Fatalf("oapi.Parse: %v", err)
	}
	req := &scaffold.CreateRequest{
		Name: "demo", Module: "example.com/demo",
		Drivers: []string{"memory"},
		Spec:    spec, SpecBytes: []byte("openapi: 3.0.3\n"),
	}
	plan, err := scaffold.Render(req)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var handler string
	for _, f := range plan.Files {
		if f.Path == "internal/api/openapi/pets/handler.go" {
			handler = string(f.Content)
		}
	}
	if handler == "" {
		t.Fatal("pets/handler.go not in plan")
	}
	// CRUD ops get 501 placeholder.
	for _, mustHave := range []string{
		"package pets",
		"func NewHandler() *Handler",
		"func (h *Handler) ListPets(",
		"(CRUD: list)",
		"http.StatusNotImplemented",
		"func (h *Handler) GetPet(",
		"(CRUD: get_by_id)",
	} {
		if !strings.Contains(handler, mustHave) {
			t.Errorf("handler missing %q\n---\n%s", mustHave, handler)
		}
	}
	// Non-CRUD POST /pets/search panics.
	for _, mustHave := range []string{
		"func (h *Handler) SearchPets(",
		"(non-CRUD)",
		`panic("pets.Handler.SearchPets: not implemented")`,
	} {
		if !strings.Contains(handler, mustHave) {
			t.Errorf("non-CRUD branch missing %q\n---\n%s", mustHave, handler)
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
