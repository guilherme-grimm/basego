// Package memory provides in-process gateway implementations that share a
// single Store. The memory driver always ships — it's a first-class runtime
// option (per DESIGN §5) and the canonical test double for service tests.
package memory

import "sync"

// Store is the shared in-memory state. Per-slice gateway implementations
// (added in later deliverables) take a pointer to Store and operate under
// its mutex.
type Store struct {
	Mu sync.Mutex
}

// NewStore returns a ready-to-use Store. Wired in cmd/api at startup.
func NewStore() *Store { return &Store{} }
