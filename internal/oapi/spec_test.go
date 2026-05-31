package oapi_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guilherme-grimm/basego/internal/oapi"
)

func TestParse_GroupsByTag(t *testing.T) {
	t.Parallel()
	spec, err := oapi.Parse([]byte(`
openapi: 3.0.3
paths:
  /pets:
    get:
      operationId: listPets
      tags: [pets]
    post:
      operationId: createPet
      tags: [pets]
  /orders:
    get:
      operationId: listOrders
      tags: [orders]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(spec.Slices) != 2 {
		t.Fatalf("Slices: got %d, want 2", len(spec.Slices))
	}
	// Sorted alphabetically: orders, pets
	if spec.Slices[0].Tag != "orders" || spec.Slices[1].Tag != "pets" {
		t.Errorf("tag order: got [%s, %s], want [orders, pets]", spec.Slices[0].Tag, spec.Slices[1].Tag)
	}
	pets := spec.Slices[1].Operations
	if len(pets) != 2 {
		t.Fatalf("pets ops: got %d, want 2", len(pets))
	}
	// Within slice: sorted by Path then Method — both ops on /pets, so GET < POST.
	if pets[0].Method != "GET" || pets[1].Method != "POST" {
		t.Errorf("pets methods: got %s/%s, want GET/POST", pets[0].Method, pets[1].Method)
	}
}

func TestParse_RejectsUntagged(t *testing.T) {
	t.Parallel()
	_, err := oapi.Parse([]byte(`
openapi: 3.0.3
paths:
  /naked:
    get:
      operationId: getNaked
`))
	if err == nil || !strings.Contains(err.Error(), "no tag") {
		t.Fatalf("Parse: err = %v, want 'no tag'", err)
	}
}

func TestParse_RejectsMultipleTags(t *testing.T) {
	t.Parallel()
	_, err := oapi.Parse([]byte(`
openapi: 3.0.3
paths:
  /multi:
    get:
      tags: [a, b]
      operationId: getMulti
`))
	if err == nil || !strings.Contains(err.Error(), "exactly one tag") {
		t.Fatalf("Parse: err = %v, want 'exactly one tag'", err)
	}
}

func TestParse_RejectsMissingOpenAPIField(t *testing.T) {
	t.Parallel()
	_, err := oapi.Parse([]byte(`paths: {}`))
	if err == nil || !strings.Contains(err.Error(), "openapi") {
		t.Fatalf("Parse: err = %v, want 'openapi' missing", err)
	}
}

func TestParse_RejectsMalformedYAML(t *testing.T) {
	t.Parallel()
	_, err := oapi.Parse([]byte("openapi: 3.0\npaths: [this is not a map"))
	if err == nil || !strings.Contains(err.Error(), "parse yaml") {
		t.Fatalf("Parse: err = %v, want 'parse yaml'", err)
	}
}

func TestParse_IgnoresNonMethodKeys(t *testing.T) {
	t.Parallel()
	// `parameters` and `summary` siblings to methods must not trip up
	// the parser, but only via methods declared above them. We test by
	// having an unrelated key that maps to non-Operation YAML — the
	// parser must skip it cleanly.
	_, err := oapi.Parse([]byte(`
openapi: 3.0.3
paths:
  /pets:
    get:
      operationId: listPets
      tags: [pets]
    x-internal-id: noise
`))
	if err != nil {
		t.Fatalf("Parse: unexpected error %v", err)
	}
}

func TestParseFile_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "spec.yaml")
	const body = `
openapi: 3.0.3
paths:
  /pets:
    get:
      operationId: listPets
      tags: [pets]
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	spec, err := oapi.ParseFile(p)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if spec.Source == "" || !filepath.IsAbs(spec.Source) {
		t.Errorf("Source = %q, want absolute path", spec.Source)
	}
	if len(spec.Slices) != 1 || spec.Slices[0].Tag != "pets" {
		t.Errorf("expected single 'pets' slice; got %+v", spec.Slices)
	}
}

func TestParse_RejectsMissingOperationID(t *testing.T) {
	t.Parallel()
	_, err := oapi.Parse([]byte(`
openapi: 3.0.3
paths:
  /pets:
    get:
      tags: [pets]
`))
	if err == nil || !strings.Contains(err.Error(), "missing operationId") {
		t.Fatalf("Parse: err = %v, want 'missing operationId'", err)
	}
}

func TestDetectCRUD(t *testing.T) {
	t.Parallel()
	tests := []struct {
		method, path string
		want         oapi.CRUDKind
	}{
		{"GET", "/pets", oapi.CRUDList},
		{"POST", "/pets", oapi.CRUDCreate},
		{"GET", "/pets/{id}", oapi.CRUDGetByID},
		{"PUT", "/pets/{id}", oapi.CRUDUpdate},
		{"PATCH", "/pets/{id}", oapi.CRUDPartialUpdate},
		{"DELETE", "/pets/{id}", oapi.CRUDDelete},
		// non-CRUD shapes
		{"POST", "/pets/{id}", oapi.CRUDNone},
		{"GET", "/pets/{id}/photos", oapi.CRUDNone},
		{"POST", "/pets/search", oapi.CRUDNone},
		{"OPTIONS", "/pets", oapi.CRUDNone},
		{"GET", "/", oapi.CRUDNone},
	}
	for _, tc := range tests {
		op := oapi.Operation{Method: tc.method, Path: tc.path}
		if got := op.CRUD(); got != tc.want {
			t.Errorf("CRUD(%s %s) = %q, want %q", tc.method, tc.path, got, tc.want)
		}
	}
}

func TestOperation_MethodName(t *testing.T) {
	t.Parallel()
	op := oapi.Operation{OperationID: "listPets"}
	if got := op.MethodName(); got != "ListPets" {
		t.Errorf("MethodName = %q, want ListPets", got)
	}
}

func TestParseFile_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := oapi.ParseFile(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil || !strings.Contains(err.Error(), "read") {
		t.Fatalf("ParseFile: err = %v, want read error", err)
	}
}
