// Package postgres will host PostgreSQL gateway implementations.
//
// Pool lifecycle is owned by cmd/api/postgres.go (per DESIGN §5). Files in
// this package wrap the pool that cmd/api hands them; they do not dial
// themselves.
package postgres

// Pool is a placeholder for the pgx connection pool that gateway
// implementations will accept. Real driver wiring (pgxpool) lands in a
// follow-up PR — for v1 the type exists so the wiring graph compiles.
type Pool struct{}
