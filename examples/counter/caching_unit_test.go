package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/livefir/livetemplate"
	"github.com/livefir/livetemplate/internal/diff"
)

// TestDirectActionCaching tests the caching behavior by calling HandleAction directly
func TestDirectActionCaching(t *testing.T) {
	// Create server instance
	server := NewServer()
	
	// Test 1: First action call (no cache info)
	firstMessage := &livetemplate.ActionMessage{
		Action: "increment",
		Data:   make(map[string]interface{}),
		Cache: []string{}, // No cache on first call
	}
	
	fmt.Printf("=== FIRST ACTION (no cache info) ===\n")
	firstFragments, err := server.templatePage.HandleAction(context.Background(), firstMessage)
	if err != nil {
		t.Fatalf("First HandleAction failed: %v", err)
	}
	
	if len(firstFragments) == 0 {
		t.Fatalf("Expected at least one fragment from first action")
	}
	
	firstFragment := firstFragments[0]
	firstJSON, _ := json.MarshalIndent(firstFragment, "", "  ")
	fmt.Printf("First fragment:\n%s\n", string(firstJSON))
	
	// Check if first fragment has statics
	firstUpdate, ok := firstFragment.Data.(*diff.Update)
	if !ok {
		t.Fatalf("Expected fragment data to be *diff.Update, got %T", firstFragment.Data)
	}
	
	hasFirstStatics := len(firstUpdate.S) > 0
	fmt.Printf("First fragment has statics: %v (%d segments)\n", hasFirstStatics, len(firstUpdate.S))
	fmt.Printf("First fragment dynamics: %v\n", firstUpdate.Dynamics)
	
	if !hasFirstStatics {
		t.Errorf("Expected first fragment to have statics")
	}
	
	// Test 2: Second action call WITH cache info
	secondMessage := &livetemplate.ActionMessage{
		Action: "increment",
		Data:   make(map[string]interface{}),
		Cache: []string{firstFragment.ID}, // Client claims to have cached this fragment
	}
	
	fmt.Printf("\n=== SECOND ACTION (with cache info) ===\n")
	fmt.Printf("Cache info: cached_fragments=%v\n", secondMessage.Cache)
	
	secondFragments, err := server.templatePage.HandleAction(context.Background(), secondMessage)
	if err != nil {
		t.Fatalf("Second HandleAction failed: %v", err)
	}
	
	if len(secondFragments) == 0 {
		t.Fatalf("Expected at least one fragment from second action")
	}
	
	secondFragment := secondFragments[0]
	secondJSON, _ := json.MarshalIndent(secondFragment, "", "  ")
	fmt.Printf("Second fragment:\n%s\n", string(secondJSON))
	
	// Check if second fragment has statics (should NOT if caching works)
	secondUpdate, ok := secondFragment.Data.(*diff.Update)
	if !ok {
		t.Fatalf("Expected fragment data to be *diff.Update, got %T", secondFragment.Data)
	}
	
	hasSecondStatics := len(secondUpdate.S) > 0
	fmt.Printf("Second fragment has statics: %v (%d segments)\n", hasSecondStatics, len(secondUpdate.S))
	fmt.Printf("Second fragment dynamics: %v\n", secondUpdate.Dynamics)
	
	// Calculate sizes for bandwidth analysis
	firstSize := len(firstJSON)
	secondSize := len(secondJSON)
	savings := float64(firstSize - secondSize) / float64(firstSize) * 100
	
	fmt.Printf("\n=== BANDWIDTH ANALYSIS ===\n")
	fmt.Printf("First fragment size: %d bytes\n", firstSize)
	fmt.Printf("Second fragment size: %d bytes\n", secondSize)
	fmt.Printf("Bandwidth savings: %.1f%%\n", savings)
	
	// Test assertions
	if hasSecondStatics {
		t.Errorf("❌ CACHING FAILURE: Second fragment should NOT have statics")
		t.Errorf("   Expected: Only dynamic data after cache info is provided")
		t.Errorf("   Actual: Full static content resent (%d static segments)", len(secondUpdate.S))
		t.Errorf("   This defeats the purpose of the bandwidth optimization")
	} else {
		fmt.Printf("✅ Caching working: Second fragment omits cached statics\n")
	}
	
	if savings < 50 {
		t.Errorf("❌ Expected >50%% bandwidth savings, got %.1f%%", savings)
	} else {
		fmt.Printf("✅ Good bandwidth savings: %.1f%%\n", savings)
	}
	
	// Test 3: Third action call with same cache info (should also work)
	fmt.Printf("\n=== THIRD ACTION (same cache info) ===\n")
	thirdFragments, err := server.templatePage.HandleAction(context.Background(), secondMessage)
	if err != nil {
		t.Fatalf("Third HandleAction failed: %v", err)
	}
	
	if len(thirdFragments) > 0 {
		thirdFragment := thirdFragments[0]
		thirdUpdate, ok := thirdFragment.Data.(*diff.Update)
		if !ok {
			t.Fatalf("Expected fragment data to be *diff.Update, got %T", thirdFragment.Data)
		}
		
		hasThirdStatics := len(thirdUpdate.S) > 0
		fmt.Printf("Third fragment has statics: %v (%d segments)\n", hasThirdStatics, len(thirdUpdate.S))
		
		if hasThirdStatics {
			t.Errorf("❌ Third fragment should also NOT have statics")
		} else {
			fmt.Printf("✅ Third fragment also omits statics correctly\n")
		}
	}
}