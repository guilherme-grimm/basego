// Package scaffold owns the contract between CLI parsing and the (future)
// scaffold writer. Deliverable 2 ships only the request types; the writer
// lands in Deliverable 3.
package scaffold

// Extension is a `create`-time add-on like `file spec.yaml`. Name is the
// extension keyword; Args carries its positional arguments in order.
type Extension struct {
	Name string
	Args []string
}

// CreateRequest is the normalized result of parsing `basego create ...`.
// Drivers is always sorted and always contains "memory". Module defaults
// to Name when --module is not provided.
type CreateRequest struct {
	Name       string
	Module     string
	Drivers    []string
	Extensions []Extension
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
