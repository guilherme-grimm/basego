package scaffold_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	"observability/grafana/datasources.yaml",
	"observability/loki.yaml",
	"observability/otel-collector.yaml",
	"observability/prometheus.yml",
	"observability/tempo.yaml",
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
			require.NoError(t, err)

			var got []string
			for _, f := range plan.Files {
				got = append(got, f.Path)
			}

			want := append([]string{}, baseFiles...)
			want = append(want, tc.extra...)
			assert.ElementsMatch(t, want, got)
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
			require.NoError(t, err)
			var yaml string
			for _, f := range plan.Files {
				if f.Path == "config/config.yaml" {
					yaml = string(f.Content)
					break
				}
			}
			require.NotEmpty(t, yaml, "config/config.yaml not in plan")
			for _, s := range tc.mustContain {
				assert.Contains(t, yaml, s, "config.yaml")
			}
			for _, s := range tc.mustNot {
				assert.NotContains(t, yaml, s, "config.yaml")
			}
		})
	}
}

func TestRender_MetricsRouteAndObservabilityConfig(t *testing.T) {
	t.Parallel()
	req := &scaffold.CreateRequest{Name: "demo", Module: "example.com/demo", Drivers: []string{"memory"}}
	plan, err := scaffold.Render(req)
	require.NoError(t, err)
	var router, cfg string
	for _, f := range plan.Files {
		switch f.Path {
		case "internal/api/router.go":
			router = string(f.Content)
		case "config/config.yaml":
			cfg = string(f.Content)
		}
	}
	for _, s := range []string{"promhttp", `"GET /metrics"`} {
		assert.Contains(t, router, s, "router.go")
	}
	for _, s := range []string{"observability:", "service_name: demo", "otlp_endpoint:"} {
		assert.Contains(t, cfg, s, "config.yaml")
	}
}

func TestRender_ObservabilityStack(t *testing.T) {
	t.Parallel()
	req := &scaffold.CreateRequest{Name: "demo", Module: "example.com/demo", Drivers: []string{"memory"}}
	plan, err := scaffold.Render(req)
	require.NoError(t, err)
	files := make(map[string]string, len(plan.Files))
	for _, f := range plan.Files {
		files[f.Path] = string(f.Content)
	}

	compose := files["docker-compose.yml"]
	for _, s := range []string{
		"otel-collector:", "prometheus:", "loki:", "tempo:", "grafana:",
		"OBSERVABILITY_OTLP_ENDPOINT: otel-collector:4317",
		"--web.enable-remote-write-receiver",
	} {
		assert.Contains(t, compose, s, "docker-compose.yml")
	}

	stack := map[string]string{
		"observability/prometheus.yml":           "app:8080",
		"observability/otel-collector.yaml":      "tempo:4317",
		"observability/loki.yaml":                "http_listen_port: 3100",
		"observability/tempo.yaml":               "http_listen_port: 3200",
		"observability/grafana/datasources.yaml": "http://prometheus:9090",
	}
	for path, want := range stack {
		if !assert.Contains(t, files, path, "plan") {
			continue
		}
		assert.Contains(t, files[path], want, path)
		assert.Contains(t, compose, "./"+path+":", "docker-compose.yml mount")
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
			require.NoError(t, err)
			var compose string
			for _, f := range plan.Files {
				if f.Path == "docker-compose.yml" {
					compose = string(f.Content)
					break
				}
			}
			require.NotEmpty(t, compose, "docker-compose.yml not in plan")
			for _, s := range tc.mustContain {
				assert.Contains(t, compose, s, "compose")
			}
			for _, s := range tc.mustNot {
				assert.NotContains(t, compose, s, "compose")
			}
		})
	}
}

func TestRender_SubstitutesModuleAndName(t *testing.T) {
	t.Parallel()
	req := &scaffold.CreateRequest{Name: "demo", Module: "example.com/demo", Drivers: []string{"memory"}}
	plan, err := scaffold.Render(req)
	require.NoError(t, err)
	for _, f := range plan.Files {
		switch f.Path {
		case "go.mod":
			assert.Contains(t, string(f.Content), "module example.com/demo", "go.mod")
		case "README.md":
			assert.Contains(t, string(f.Content), "# demo", "README.md")
		case "cmd/api/main.go":
			assert.Contains(t, string(f.Content), `"example.com/demo/internal/api"`, "main.go")
		case "cmd/api/memory.go":
			assert.Contains(t, string(f.Content), `"example.com/demo/internal/resource/database/memory"`, "memory.go")
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
	require.NoError(t, err)
	req := &scaffold.CreateRequest{
		Name:      "demo",
		Module:    "example.com/demo",
		Drivers:   []string{"memory"},
		Spec:      spec,
		SpecBytes: []byte("openapi: 3.0.3\n"),
	}
	plan, err := scaffold.Render(req)
	require.NoError(t, err)
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
		if !assert.Contains(t, got, path, "plan") {
			continue
		}
		for _, mustHave := range mustHaves {
			assert.Contains(t, got[path], mustHave, path)
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
	require.NoError(t, err)
	req := &scaffold.CreateRequest{
		Name: "demo", Module: "example.com/demo",
		Drivers: []string{"memory"},
		Spec:    spec, SpecBytes: []byte("openapi: 3.0.3\n"),
	}
	plan, err := scaffold.Render(req)
	require.NoError(t, err)
	var handler string
	for _, f := range plan.Files {
		if f.Path == "internal/api/openapi/pets/handler.go" {
			handler = string(f.Content)
		}
	}
	require.NotEmpty(t, handler, "pets/handler.go not in plan")
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
		assert.Contains(t, handler, mustHave, "handler")
	}
	// Non-CRUD POST /pets/search panics.
	for _, mustHave := range []string{
		"func (h *Handler) SearchPets(",
		"(non-CRUD)",
		`panic("pets.Handler.SearchPets: not implemented")`,
	} {
		assert.Contains(t, handler, mustHave, "non-CRUD branch")
	}
}

func TestRender_NilRequest(t *testing.T) {
	t.Parallel()
	_, err := scaffold.Render(nil)
	require.ErrorContains(t, err, "nil request")
}
