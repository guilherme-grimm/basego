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

// collectionRe matches `/<name>` — a flat collection path. itemRe matches
// `/<name>/{param}` — a single-resource path. Both are conservative: they
// don't recognize nested collections, since DESIGN §5's CRUD table only
// covers the single-segment family. Anything outside falls back to
// non-CRUD (panic stub).
var (
	collectionRe = regexp.MustCompile(`^/[A-Za-z][A-Za-z0-9_-]*$`)
	itemRe       = regexp.MustCompile(`^/[A-Za-z][A-Za-z0-9_-]*/\{[A-Za-z][A-Za-z0-9_]*\}$`)
)

// Operation is the minimal view of one OpenAPI operation that basego
// cares about at scaffold time.
type Operation struct {
	Method      string // upper-case (GET, POST, ...)
	Path        string // raw OpenAPI path, e.g. /pets/{id}
	OperationID string
}

// CRUDKind classifies an Operation against the six combos DESIGN §5
// recognizes as CRUD. CRUDNone means non-CRUD — the codegen path emits
// a panic stub for these so unfinished routes surface loudly.
type CRUDKind string

const (
	CRUDNone          CRUDKind = ""
	CRUDList          CRUDKind = "list"
	CRUDCreate        CRUDKind = "create"
	CRUDGetByID       CRUDKind = "get_by_id"
	CRUDUpdate        CRUDKind = "update"
	CRUDPartialUpdate CRUDKind = "partial_update"
	CRUDDelete        CRUDKind = "delete"
)

// CRUD returns the operation's CRUD classification. Pure function of
// Method + Path; no side effects.
func (o Operation) CRUD() CRUDKind {
	isCollection := collectionRe.MatchString(o.Path)
	isItem := itemRe.MatchString(o.Path)
	switch {
	case o.Method == "GET" && isCollection:
		return CRUDList
	case o.Method == "POST" && isCollection:
		return CRUDCreate
	case o.Method == "GET" && isItem:
		return CRUDGetByID
	case o.Method == "PUT" && isItem:
		return CRUDUpdate
	case o.Method == "PATCH" && isItem:
		return CRUDPartialUpdate
	case o.Method == "DELETE" && isItem:
		return CRUDDelete
	}
	return CRUDNone
}

// MethodName returns the Go method name to use for this operation in the
// generated handler. OperationID is required (Parse rejects ops without
// one), so this is a straight exportify of OperationID.
func (o Operation) MethodName() string {
	if o.OperationID == "" {
		return ""
	}
	return strings.ToUpper(o.OperationID[:1]) + o.OperationID[1:]
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
// Spec.Source to the file's absolute path.
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
// Parse decodes an OpenAPI v3 document from the provided YAML bytes and returns a Spec containing operations grouped into sorted slices by tag.
// It validates that the `openapi` field is present, accepts only known HTTP methods, requires exactly one lowercase package-safe tag per operation, and requires a non-empty `operationId` for each operation; parsed operations are recorded with their method, path, and operationId, operations within a tag are sorted by path then method, and tag slices are sorted by tag. Error messages are returned when parsing or validation fails.
func Parse(data []byte) (*Spec, error) {
	var raw rawSpec
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("oapi: parse yaml: %w", err)
	}
	if strings.TrimSpace(raw.OpenAPI) == "" {
		return nil, fmt.Errorf("oapi: missing 'openapi' version field")
	}
	byTag := map[string][]Operation{}
	// seenMethod tracks, per tag, the Go method name each operation maps to,
	// so duplicate or first-letter-colliding operationIds (e.g. "foo" vs
	// "Foo") are rejected here rather than producing duplicate method
	// declarations that only fail when the generated project is compiled.
	seenMethod := map[string]map[string]string{}
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
			if strings.TrimSpace(op.OperationID) == "" {
				return nil, fmt.Errorf("oapi: %s %s missing operationId — basego uses it to name the generated Go handler method", strings.ToUpper(method), pth)
			}
			o := Operation{
				Method:      strings.ToUpper(method),
				Path:        pth,
				OperationID: op.OperationID,
			}
			mn := o.MethodName()
			if seenMethod[tag] == nil {
				seenMethod[tag] = map[string]string{}
			}
			if prev, ok := seenMethod[tag][mn]; ok {
				return nil, fmt.Errorf("oapi: tag %q operations %q and %q both map to Go method %q; operationIds must be unique per tag", tag, prev, op.OperationID, mn)
			}
			seenMethod[tag][mn] = op.OperationID
			byTag[tag] = append(byTag[tag], o)
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

