package livetemplate

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestConnectionLimits_NoLimits(t *testing.T) {
	limits := NewConnectionLimits(0, 0)

	// Should accept unlimited connections
	for i := 0; i < 1000; i++ {
		groupID := "group-1"
		if !limits.CanAccept(groupID) {
			t.Fatal("CanAccept should return true when no limits are set")
		}
		if err := limits.Acquire(groupID); err != nil {
			t.Fatalf("Acquire failed: %v", err)
		}
	}

	if limits.ActiveConnections() != 1000 {
		t.Errorf("Expected 1000 active connections, got %d", limits.ActiveConnections())
	}
}

func TestConnectionLimits_GlobalLimit(t *testing.T) {
	limits := NewConnectionLimits(10, 0)

	// Accept first 10 connections
	for i := 0; i < 10; i++ {
		groupID := "group-1"
		if !limits.CanAccept(groupID) {
			t.Fatalf("CanAccept should return true for connection %d", i)
		}
		if err := limits.Acquire(groupID); err != nil {
			t.Fatalf("Acquire failed for connection %d: %v", i, err)
		}
	}

	// 11th connection should be rejected
	if limits.CanAccept("group-1") {
		t.Error("CanAccept should return false when at global limit")
	}

	err := limits.Acquire("group-1")
	if err == nil {
		t.Error("Acquire should fail when at global limit")
	}

	stats := limits.Stats()
	if stats.ActiveConnections != 10 {
		t.Errorf("Expected 10 active connections, got %d", stats.ActiveConnections)
	}
	if stats.ConnectionsRejected != 1 {
		t.Errorf("Expected 1 rejected connection, got %d", stats.ConnectionsRejected)
	}
}

func TestConnectionLimits_PerGroupLimit(t *testing.T) {
	limits := NewConnectionLimits(0, 5)

	// Accept 5 connections for group-1
	for i := 0; i < 5; i++ {
		if !limits.CanAccept("group-1") {
			t.Fatalf("CanAccept should return true for connection %d", i)
		}
		if err := limits.Acquire("group-1"); err != nil {
			t.Fatalf("Acquire failed for connection %d: %v", i, err)
		}
	}

	// 6th connection for group-1 should be rejected
	if limits.CanAccept("group-1") {
		t.Error("CanAccept should return false when at per-group limit")
	}

	// But group-2 should still be able to connect
	if !limits.CanAccept("group-2") {
		t.Error("CanAccept should return true for different group")
	}

	if err := limits.Acquire("group-2"); err != nil {
		t.Fatalf("Acquire failed for group-2: %v", err)
	}

	if limits.ActiveConnections() != 6 {
		t.Errorf("Expected 6 active connections, got %d", limits.ActiveConnections())
	}

	if limits.GroupConnectionCount("group-1") != 5 {
		t.Errorf("Expected 5 connections for group-1, got %d", limits.GroupConnectionCount("group-1"))
	}

	if limits.GroupConnectionCount("group-2") != 1 {
		t.Errorf("Expected 1 connection for group-2, got %d", limits.GroupConnectionCount("group-2"))
	}
}

func TestConnectionLimits_Release(t *testing.T) {
	limits := NewConnectionLimits(10, 5)

	// Acquire 5 connections for group-1
	for i := 0; i < 5; i++ {
		if err := limits.Acquire("group-1"); err != nil {
			t.Fatalf("Acquire failed: %v", err)
		}
	}

	// Release 2 connections
	limits.Release("group-1")
	limits.Release("group-1")

	if limits.ActiveConnections() != 3 {
		t.Errorf("Expected 3 active connections after releases, got %d", limits.ActiveConnections())
	}

	if limits.GroupConnectionCount("group-1") != 3 {
		t.Errorf("Expected 3 connections for group-1 after releases, got %d", limits.GroupConnectionCount("group-1"))
	}

	// Should be able to acquire more connections now
	if !limits.CanAccept("group-1") {
		t.Error("CanAccept should return true after releasing connections")
	}
}

func TestConnectionLimits_CleanupEmptyGroups(t *testing.T) {
	limits := NewConnectionLimits(0, 0)

	// Acquire and release a connection
	if err := limits.Acquire("group-1"); err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	limits.Release("group-1")

	stats := limits.Stats()
	if stats.ActiveGroups != 0 {
		t.Errorf("Expected 0 active groups after cleanup, got %d", stats.ActiveGroups)
	}
}

func TestConnectionLimits_ConcurrentAccess(t *testing.T) {
	limits := NewConnectionLimits(100, 10)

	var wg sync.WaitGroup
	successCount := int32(0)

	// Try to acquire 200 connections concurrently (should only succeed 100 or fewer due to limits)
	for i := 0; i < 200; i++ {
		wg.Add(1)
		groupID := "group-1"
		if i >= 100 {
			groupID = "group-2"
		}

		go func(gid string) {
			defer wg.Done()
			// CanAccept is advisory only - there can be a race between check and acquire
			// Just try to acquire and count successes
			if err := limits.Acquire(gid); err == nil {
				atomic.AddInt32(&successCount, 1)
			}
		}(groupID)
	}

	wg.Wait()

	// Should have at most 100 connections (global limit)
	active := limits.ActiveConnections()
	if active > 100 {
		t.Errorf("Expected at most 100 active connections, got %d", active)
	}

	// Should have at most 10 connections per group
	group1Count := limits.GroupConnectionCount("group-1")
	if group1Count > 10 {
		t.Errorf("Expected at most 10 connections for group-1, got %d", group1Count)
	}

	// Verify that limits actually prevented some connections
	if successCount == 200 {
		t.Errorf("Expected limits to prevent some connections, but all 200 succeeded")
	}
}

func TestConnectionLimits_BothLimits(t *testing.T) {
	limits := NewConnectionLimits(15, 5)

	// Fill group-1 to per-group limit (5 connections)
	for i := 0; i < 5; i++ {
		if err := limits.Acquire("group-1"); err != nil {
			t.Fatalf("Acquire failed for group-1: %v", err)
		}
	}

	// Fill group-2 to per-group limit (5 connections)
	for i := 0; i < 5; i++ {
		if err := limits.Acquire("group-2"); err != nil {
			t.Fatalf("Acquire failed for group-2: %v", err)
		}
	}

	// Fill group-3 to per-group limit (5 connections)
	for i := 0; i < 5; i++ {
		if err := limits.Acquire("group-3"); err != nil {
			t.Fatalf("Acquire failed for group-3: %v", err)
		}
	}

	// Now at global limit (15 connections)
	if limits.CanAccept("group-4") {
		t.Error("CanAccept should return false when at global limit")
	}

	// Release a connection from group-1
	limits.Release("group-1")

	// group-4 should now be able to connect (global capacity available)
	if !limits.CanAccept("group-4") {
		t.Error("CanAccept should return true after releasing connection")
	}

	// But group-1 should also be able to add more (under per-group limit)
	if !limits.CanAccept("group-1") {
		t.Error("CanAccept should return true for group-1 after release")
	}
}

func TestConnectionLimits_Stats(t *testing.T) {
	limits := NewConnectionLimits(100, 10)

	if err := limits.Acquire("group-1"); err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if err := limits.Acquire("group-1"); err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if err := limits.Acquire("group-2"); err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	stats := limits.Stats()

	if stats.ActiveConnections != 3 {
		t.Errorf("Expected 3 active connections, got %d", stats.ActiveConnections)
	}

	if stats.MaxConnections != 100 {
		t.Errorf("Expected max 100 connections, got %d", stats.MaxConnections)
	}

	if stats.MaxPerGroup != 10 {
		t.Errorf("Expected max 10 per group, got %d", stats.MaxPerGroup)
	}

	if stats.ActiveGroups != 2 {
		t.Errorf("Expected 2 active groups, got %d", stats.ActiveGroups)
	}

	if stats.ConnectionsRejected != 0 {
		t.Errorf("Expected 0 rejected connections, got %d", stats.ConnectionsRejected)
	}
}
