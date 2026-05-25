package entity

// Code is the closed enum of domain-level error codes. The HTTP error hook
// maps Code to status; an `exhaustive` lint guards the mapping (DESIGN §7).
type Code string

const (
	CodeNotFound         Code = "not_found"
	CodeConflict         Code = "conflict"
	CodeValidationFailed Code = "validation_failed"
	CodeUnauthorized     Code = "unauthorized"
	CodeForbidden        Code = "forbidden"
	CodeInternal         Code = "internal"
)
