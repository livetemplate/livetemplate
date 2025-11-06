package signature

import (
	"container/list"
	"sync"
)

// DefaultMaxRegistrySize is the default maximum number of structure signatures to track.
// Prevents unbounded memory growth in long-lived templates with many structure variations.
const DefaultMaxRegistrySize = 1000

// ClientStructureRegistry tracks exactly what template structures the client has seen.
// It provides the single source of truth for determining whether statics should be
// included in updates, ensuring specification compliance.
//
// The registry maps field paths to structure signatures, enabling definitive answers to:
// "Has the client seen THIS EXACT structure at THIS EXACT path?"
//
// Implements LRU eviction to prevent unbounded memory growth:
//   - When maxSize is reached, least recently used entries are evicted
//   - maxSize of 0 means unlimited (not recommended for production)
//
// Thread-safe for concurrent WebSocket sessions.
type ClientStructureRegistry struct {
	// structures maps field path → structure signature
	// Examples:
	//   "0" → "scalar"
	//   "11" → "conditional"
	//   "11.0" → "range:items:abc123de"
	structures map[string]*list.Element

	// lruList tracks access order (most recent at front)
	lruList *list.List

	// maxSize limits the number of tracked structures (0 = unlimited)
	maxSize int

	// mu protects structures map and LRU list for concurrent access
	mu sync.RWMutex
}

// lruEntry represents an entry in the LRU cache.
type lruEntry struct {
	path      string
	signature StructureSignature
}

// NewClientStructureRegistry creates a new empty registry with default max size.
func NewClientStructureRegistry() *ClientStructureRegistry {
	return NewClientStructureRegistryWithSize(DefaultMaxRegistrySize)
}

// NewClientStructureRegistryWithSize creates a new registry with custom max size.
// Use maxSize=0 for unlimited size (not recommended for long-lived templates).
func NewClientStructureRegistryWithSize(maxSize int) *ClientStructureRegistry {
	return &ClientStructureRegistry{
		structures: make(map[string]*list.Element),
		lruList:    list.New(),
		maxSize:    maxSize,
	}
}

// HasSeen returns true if the client has seen this EXACT structure at this path.
// It calculates the signature of the value and compares it to what's stored.
//
// Returns false if:
//   - The path has never been seen
//   - The structure at this path has changed (different signature)
//
// Thread-safe for concurrent reads. Promotes accessed entries to front of LRU list.
func (r *ClientStructureRegistry) HasSeen(fieldPath string, value interface{}) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Calculate signature of current value
	currentSig := CalculateSignature(value)

	// Look up what client has seen at this path
	elem, exists := r.structures[fieldPath]
	if !exists {
		return false
	}

	entry := elem.Value.(*lruEntry)

	// Check if signatures match
	matches := entry.signature == currentSig

	// Promote to front of LRU list on access
	if matches {
		r.lruList.MoveToFront(elem)
	}

	return matches
}

// MarkSeen records that the client has seen this structure at this path.
// This should be called whenever a structure is sent to the client.
//
// The signature is calculated from the value and stored at the field path.
// If maxSize is reached, evicts the least recently used entry.
//
// Thread-safe for concurrent writes.
func (r *ClientStructureRegistry) MarkSeen(fieldPath string, value interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Calculate signature
	sig := CalculateSignature(value)

	// Check if entry already exists
	if elem, exists := r.structures[fieldPath]; exists {
		// Update existing entry and move to front
		entry := elem.Value.(*lruEntry)
		entry.signature = sig
		r.lruList.MoveToFront(elem)
		return
	}

	// Create new entry
	entry := &lruEntry{
		path:      fieldPath,
		signature: sig,
	}

	// Add to front of LRU list
	elem := r.lruList.PushFront(entry)
	r.structures[fieldPath] = elem

	// Evict LRU entry if size limit exceeded
	if r.maxSize > 0 && r.lruList.Len() > r.maxSize {
		// Remove least recently used (back of list)
		oldest := r.lruList.Back()
		if oldest != nil {
			r.lruList.Remove(oldest)
			oldEntry := oldest.Value.(*lruEntry)
			delete(r.structures, oldEntry.path)
		}
	}
}

// Clear resets the registry, removing all tracked structures.
// Useful for testing or session reset scenarios.
//
// Thread-safe for concurrent writes.
func (r *ClientStructureRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Create new empty map and list
	r.structures = make(map[string]*list.Element)
	r.lruList = list.New()
}

// GetSignature returns the signature the client has seen at this path.
// Returns (signature, true) if path exists, ("", false) otherwise.
//
// Thread-safe for concurrent reads. Promotes accessed entries to front of LRU list.
func (r *ClientStructureRegistry) GetSignature(fieldPath string) (StructureSignature, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	elem, exists := r.structures[fieldPath]
	if !exists {
		return "", false
	}

	entry := elem.Value.(*lruEntry)
	// Promote to front on access
	r.lruList.MoveToFront(elem)

	return entry.signature, true
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
