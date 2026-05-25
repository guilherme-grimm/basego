// Package oapi parses OpenAPI 3 specs just enough to drive basego's
// per-tag vertical-slice generation. Full schema/codegen work happens
// elsewhere (oapi-codegen, wired in a later deliverable); this package
// only extracts what scaffold-time decisions need: the set of tags and
// which operations belong to each.
package oapi

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// tagRe restricts tags to characters safe as Go package names. basego maps
// one tag = one Go package; anything outside this set would force renames
// later. Fail fast at parse time instead.
var tagRe = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

// Operation is the minimal view of one OpenAPI operation that basego
// cares about at scaffold time.
type Operation struct {
	Method      string // upper-case (GET, POST, ...)
	Path        string // raw OpenAPI path, e.g. /pets/{id}
	OperationID string
}

// Slice groups operations by tag. One slice = one tag = one generated
// vertical slice (entity / gateway / service / handler / per-driver impls).
type Slice struct {
	Tag        string
	Operations []Operation
}

// Spec is the parsed-down view returned to the rest of basego.
type Spec struct {
	Source string  // absolute path the spec was read from
	Slices []Slice // sorted by Tag for determinism
}

// rawSpec is the YAML shape we decode into. Path entries are kept as
// yaml.Node so we can selectively decode only recognized HTTP-method keys
// and skip extensions like `x-internal-id` or sibling fields like
// `parameters`/`summary`.
type rawSpec struct {
	OpenAPI string                          `yaml:"openapi"`
	Paths   map[string]map[string]yaml.Node `yaml:"paths"`
}

type rawOperation struct {
	OperationID string   `yaml:"operationId"`
	Tags        []string `yaml:"tags"`
}

// httpMethods is the set of OpenAPI method keys we recognize under a path.
// Anything else (parameters, summary, description) is silently ignored.
var httpMethods = map[string]bool{
	"get": true, "put": true, "post": true, "delete": true,
	"options": true, "head": true, "patch": true, "trace": true,
}

// ParseFile reads path, parses it, and returns a Spec. Untagged operations
// fail with a specific error (DESIGN §6). Operations within a slice and
// slices themselves are sorted for deterministic downstream output.
func ParseFile(path string) (*Spec, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("oapi: resolve %s: %w", path, err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("oapi: read %s: %w", abs, err)
	}
	spec, err := Parse(data)
	if err != nil {
		return nil, err
	}
	spec.Source = abs
	return spec, nil
}

// Parse decodes spec bytes into a Spec without any filesystem access.
// Exposed for unit tests; production callers use ParseFile.
func Parse(data []byte) (*Spec, error) {
	var raw rawSpec
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("oapi: parse yaml: %w", err)
	}
	if strings.TrimSpace(raw.OpenAPI) == "" {
		return nil, fmt.Errorf("oapi: missing 'openapi' version field")
	}
	byTag := map[string][]Operation{}
	for pth, methods := range raw.Paths {
		for method, node := range methods {
			if !httpMethods[strings.ToLower(method)] {
				continue
			}
			var op rawOperation
			if err := node.Decode(&op); err != nil {
				return nil, fmt.Errorf("oapi: decode %s %s: %w", strings.ToUpper(method), pth, err)
			}
			if len(op.Tags) == 0 {
				return nil, fmt.Errorf("oapi: %s %s has no tag — every operation must declare exactly one tag", strings.ToUpper(method), pth)
			}
			if len(op.Tags) > 1 {
				return nil, fmt.Errorf("oapi: %s %s declares %d tags; basego requires exactly one tag per operation", strings.ToUpper(method), pth, len(op.Tags))
			}
			tag := op.Tags[0]
			if !tagRe.MatchString(tag) {
				return nil, fmt.Errorf("oapi: tag %q is not a valid Go package name; basego requires lowercase letters and digits, starting with a letter", tag)
			}
			byTag[tag] = append(byTag[tag], Operation{
				Method:      strings.ToUpper(method),
				Path:        pth,
				OperationID: op.OperationID,
			})
		}
	}
	slices := make([]Slice, 0, len(byTag))
	for tag, ops := range byTag {
		sort.Slice(ops, func(i, j int) bool {
			if ops[i].Path != ops[j].Path {
				return ops[i].Path < ops[j].Path
			}
			return ops[i].Method < ops[j].Method
		})
		slices = append(slices, Slice{Tag: tag, Operations: ops})
	}
	sort.Slice(slices, func(i, j int) bool { return slices[i].Tag < slices[j].Tag })
	return &Spec{Slices: slices}, nil
}

