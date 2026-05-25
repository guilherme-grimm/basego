package api

import "net/http"

// NewRouter returns the HTTP handler for the API. Routes for service slices
// are registered here in later deliverables; for now only health endpoints
// are exposed.
func NewRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", Healthz)
	mux.HandleFunc("GET /readyz", Readyz)
	return mux
}
