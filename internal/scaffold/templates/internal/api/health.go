package api

import "net/http"

// Healthz is the liveness probe. Always 200 once the process is running.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// Readyz is the readiness probe. Returns 200 once dependencies are wired;
// future deliverables will gate this on resource setup completion.
func Readyz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
