// Package mongo will host MongoDB gateway implementations.
//
// Pool lifecycle is owned by cmd/api/mongo.go (per DESIGN §5). Files in
// this package wrap the connection that cmd/api hands them; they do not
// dial themselves.
package mongo

// Client is a placeholder for the mongo connection that gateway
// implementations will accept. Real driver wiring (mongo-go-driver) lands
// in a follow-up PR — for v1 the type exists so the wiring graph compiles.
type Client struct{}
