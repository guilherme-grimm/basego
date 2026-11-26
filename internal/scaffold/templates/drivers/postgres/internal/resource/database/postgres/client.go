// Package postgres hosts the PostgreSQL gateway implementations.
//
// The pool lifecycle is owned by cmd/api/postgres.go (DESIGN §5); repos here
// wrap the *sqlx.DB that cmd/api hands them and never dial themselves.
// Queries are written in queries/*.sql against schema/*.sql, and sqlc turns
// them into typed code in sqlcgen/.
package postgres

//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0 generate
