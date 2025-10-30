package livetemplate

import (
	"sync"
)

// ClientStructureRegistry tracks exactly what template structures the client has seen.
// It provides the single source of truth for determining whether statics should be
// included in updates, ensuring specification compliance.
//
// The registry maps field paths to structure signatures, enabling definitive answers to:
// "Has the client seen THIS EXACT structure at THIS EXACT path?"
//
// Thread-safe for concurrent WebSocket sessions.
type ClientStructureRegistry struct {
	// structures maps field path → structure signature
	// Examples:
	//   "0" → "scalar"
	//   "11" → "conditional"
	//   "11.0" → "range:items:abc123de"
	structures map[string]StructureSignature

	// mu protects structures map for concurrent access
	mu sync.RWMutex
}

// NewClientStructureRegistry creates a new empty registry.
func NewClientStructureRegistry() *ClientStructureRegistry {
	return &ClientStructureRegistry{
		structures: make(map[string]StructureSignature),
	}
}

// HasSeen returns true if the client has seen this EXACT structure at this path.
// It calculates the signature of the value and compares it to what's stored.
//
// Returns false if:
//   - The path has never been seen
//   - The structure at this path has changed (different signature)
//
// Thread-safe for concurrent reads.
func (r *ClientStructureRegistry) HasSeen(fieldPath string, value interface{}) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Calculate signature of current value
	currentSig := CalculateSignature(value)

	// Look up what client has seen at this path
	seenSig, exists := r.structures[fieldPath]

	// Client has seen it if:
	// 1. Path exists in registry
	// 2. Signatures match exactly
	return exists && seenSig == currentSig
}

// MarkSeen records that the client has seen this structure at this path.
// This should be called whenever a structure is sent to the client.
//
// The signature is calculated from the value and stored at the field path.
//
// Thread-safe for concurrent writes.
func (r *ClientStructureRegistry) MarkSeen(fieldPath string, value interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Calculate and store signature
	sig := CalculateSignature(value)
	r.structures[fieldPath] = sig
}

// Clear resets the registry, removing all tracked structures.
// Useful for testing or session reset scenarios.
//
// Thread-safe for concurrent writes.
func (r *ClientStructureRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Create new empty map
	r.structures = make(map[string]StructureSignature)
}

// GetSignature returns the signature the client has seen at this path.
// Returns (signature, true) if path exists, ("", false) otherwise.
//
// Thread-safe for concurrent reads.
func (r *ClientStructureRegistry) GetSignature(fieldPath string) (StructureSignature, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sig, exists := r.structures[fieldPath]
	return sig, exists
}

// Size returns the number of structures tracked in the registry.
// Useful for debugging and testing.
//
// Thread-safe for concurrent reads.
func (r *ClientStructureRegistry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.structures)
}

// HasPath returns true if the registry has any structure recorded at this path.
// This is different from HasSeen which also checks signature match.
//
// Thread-safe for concurrent reads.
func (r *ClientStructureRegistry) HasPath(fieldPath string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.structures[fieldPath]
	return exists
}
