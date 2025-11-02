package livetemplate

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// ConnectionLimits enforces connection limits to prevent resource exhaustion.
//
// Limits are enforced at two levels:
// 1. Global limit: Maximum total connections across all users/groups
// 2. Per-group limit: Maximum connections per session group (prevents single-user DOS)
//
// Thread-safe: safe for concurrent access from multiple goroutines.
type ConnectionLimits struct {
	maxConnections         int64 // 0 means no limit
	maxConnectionsPerGroup int64 // 0 means no limit

	activeConnections       atomic.Int64             // Current total connections
	connectionsRejected     atomic.Int64             // Counter for rejected connections
	groupConnectionCounts   map[string]*atomic.Int64 // groupID → count
	groupConnectionCountsMu sync.RWMutex             // Protects groupConnectionCounts map
}

// NewConnectionLimits creates a new connection limits enforcer.
//
// Parameters:
//   - maxConnections: Maximum total connections (0 = unlimited)
//   - maxConnectionsPerGroup: Maximum connections per group (0 = unlimited)
func NewConnectionLimits(maxConnections, maxConnectionsPerGroup int64) *ConnectionLimits {
	return &ConnectionLimits{
		maxConnections:         maxConnections,
		maxConnectionsPerGroup: maxConnectionsPerGroup,
		groupConnectionCounts:  make(map[string]*atomic.Int64),
	}
}

// CanAccept checks if a new connection for the given group can be accepted.
//
// Returns:
//   - true if connection can be accepted
//   - false if at capacity (either global or per-group limit exceeded)
//
// This method does NOT increment counters - call Acquire() to actually accept the connection.
func (cl *ConnectionLimits) CanAccept(groupID string) bool {
	// Check global limit
	if cl.maxConnections > 0 && cl.activeConnections.Load() >= cl.maxConnections {
		return false
	}

	// Check per-group limit
	if cl.maxConnectionsPerGroup > 0 {
		cl.groupConnectionCountsMu.RLock()
		groupCount := cl.groupConnectionCounts[groupID]
		cl.groupConnectionCountsMu.RUnlock()

		if groupCount != nil && groupCount.Load() >= cl.maxConnectionsPerGroup {
			return false
		}
	}

	return true
}

// Acquire increments counters for a new connection.
//
// Should be called after CanAccept() returns true.
// Returns an error if limits are exceeded (defensive check).
func (cl *ConnectionLimits) Acquire(groupID string) error {
	// Double-check global limit
	if cl.maxConnections > 0 {
		current := cl.activeConnections.Add(1)
		if current > cl.maxConnections {
			cl.activeConnections.Add(-1) // Revert
			cl.connectionsRejected.Add(1)
			return fmt.Errorf("global connection limit reached (%d)", cl.maxConnections)
		}
	} else {
		cl.activeConnections.Add(1)
	}

	// Double-check per-group limit
	if cl.maxConnectionsPerGroup > 0 {
		cl.groupConnectionCountsMu.Lock()
		groupCount := cl.groupConnectionCounts[groupID]
		if groupCount == nil {
			groupCount = &atomic.Int64{}
			cl.groupConnectionCounts[groupID] = groupCount
		}
		cl.groupConnectionCountsMu.Unlock()

		current := groupCount.Add(1)
		if current > cl.maxConnectionsPerGroup {
			groupCount.Add(-1)           // Revert group count
			cl.activeConnections.Add(-1) // Revert global count
			cl.connectionsRejected.Add(1)
			return fmt.Errorf("per-group connection limit reached (%d)", cl.maxConnectionsPerGroup)
		}
	} else {
		// Still need to track group count even if no limit
		cl.groupConnectionCountsMu.Lock()
		groupCount := cl.groupConnectionCounts[groupID]
		if groupCount == nil {
			groupCount = &atomic.Int64{}
			cl.groupConnectionCounts[groupID] = groupCount
		}
		cl.groupConnectionCountsMu.Unlock()
		groupCount.Add(1)
	}

	return nil
}

// Release decrements counters when a connection closes.
//
// Should be called when a WebSocket connection is closed.
// Idempotent: safe to call multiple times for the same connection.
func (cl *ConnectionLimits) Release(groupID string) {
	cl.activeConnections.Add(-1)

	cl.groupConnectionCountsMu.RLock()
	groupCount := cl.groupConnectionCounts[groupID]
	cl.groupConnectionCountsMu.RUnlock()

	if groupCount != nil {
		newCount := groupCount.Add(-1)

		// Cleanup empty group counters to prevent memory leaks
		if newCount <= 0 {
			cl.groupConnectionCountsMu.Lock()
			// Double-check before deleting (could have been incremented)
			if groupCount.Load() <= 0 {
				delete(cl.groupConnectionCounts, groupID)
			}
			cl.groupConnectionCountsMu.Unlock()
		}
	}
}

// ActiveConnections returns the current number of active connections.
func (cl *ConnectionLimits) ActiveConnections() int64 {
	return cl.activeConnections.Load()
}

// ConnectionsRejected returns the total number of connections rejected due to limits.
func (cl *ConnectionLimits) ConnectionsRejected() int64 {
	return cl.connectionsRejected.Load()
}

// GroupConnectionCount returns the current number of connections for a group.
func (cl *ConnectionLimits) GroupConnectionCount(groupID string) int64 {
	cl.groupConnectionCountsMu.RLock()
	defer cl.groupConnectionCountsMu.RUnlock()

	groupCount := cl.groupConnectionCounts[groupID]
	if groupCount == nil {
		return 0
	}
	return groupCount.Load()
}

// Stats returns current connection statistics.
func (cl *ConnectionLimits) Stats() ConnectionLimitStats {
	cl.groupConnectionCountsMu.RLock()
	activeGroups := len(cl.groupConnectionCounts)
	cl.groupConnectionCountsMu.RUnlock()

	return ConnectionLimitStats{
		ActiveConnections:   cl.activeConnections.Load(),
		ConnectionsRejected: cl.connectionsRejected.Load(),
		MaxConnections:      cl.maxConnections,
		MaxPerGroup:         cl.maxConnectionsPerGroup,
		ActiveGroups:        int64(activeGroups),
	}
}

// ConnectionLimitStats contains connection limit statistics.
type ConnectionLimitStats struct {
	ActiveConnections   int64 // Current active connections
	ConnectionsRejected int64 // Total rejected connections
	MaxConnections      int64 // Maximum allowed connections (0 = unlimited)
	MaxPerGroup         int64 // Maximum per group (0 = unlimited)
	ActiveGroups        int64 // Number of groups with active connections
}
