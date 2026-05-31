// Package scaffold owns the contract between CLI parsing and the scaffold
// writer.
package scaffold

import "github.com/guilherme-grimm/basego/internal/oapi"

// Extension is a `create`-time add-on like `file spec.yaml`. Name is the
// extension keyword; Args carries its positional arguments in order.
type Extension struct {
	Name string
	Args []string
}

// CreateRequest is the normalized result of parsing `basego create ...`.
// Drivers is always sorted and always contains "memory". Module defaults
// to Name when --module is not provided. Spec is set iff the `file`
// extension was used; SpecBytes is the raw spec content to copy verbatim
// into the generated project.
type CreateRequest struct {
	Name       string
	Module     string
	Drivers    []string
	Extensions []Extension
	Spec       *oapi.Spec
	SpecBytes  []byte
}

// HasExtension reports whether the request includes the named extension.
// Used by the scaffold writer to gate optional side effects (e.g. running
// `go generate` only when `file` is present).
func (r *CreateRequest) HasExtension(name string) bool {
	for _, e := range r.Extensions {
		if e.Name == name {
			return true
		}
	}
	return false
}

// HasDriver reports whether the request selected the named DB driver. Used
// to gate driver-specific template overlays and config sections. Callable
// from text/template via {{.HasDriver "mongo"}}.
func (r *CreateRequest) HasDriver(name string) bool {
	for _, d := range r.Drivers {
		if d == name {
			return true
		}
	}
	return false
}
