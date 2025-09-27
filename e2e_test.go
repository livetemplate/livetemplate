package livetemplate

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// E2E test data structures
type TodoItem struct {
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
	Priority  string `json:"priority,omitempty"`
}

type E2EAppState struct {
	Title          string     `json:"title"`
	Counter        int        `json:"counter"`
	Todos          []TodoItem `json:"todos"`
	TodoCount      int        `json:"todo_count"`
	CompletedCount int        `json:"completed_count"`
	RemainingCount int        `json:"remaining_count"`
	CompletionRate int        `json:"completion_rate"`
	LastUpdated    string     `json:"last_updated"`
	SessionID      string     `json:"session_id"`
}

func TestTemplate_E2E_CompleteRenderingSequence(t *testing.T) {
	// Initial state
	initialState := E2EAppState{
		Title:          "Task Manager",
		Counter:        1,
		Todos:          []TodoItem{},
		TodoCount:      0,
		CompletedCount: 0,
		RemainingCount: 0,
		CompletionRate: 0,
		LastUpdated:    "2023-01-01 10:00:00",
		SessionID:      "session-12345",
	}

	// Update 1: Add some todos and increase counter
	update1State := E2EAppState{
		Title:   "Task Manager",
		Counter: 3,
		Todos: []TodoItem{
			{Text: "Learn Go templates", Completed: false, Priority: "High"},
			{Text: "Build live updates", Completed: true, Priority: "Medium"},
			{Text: "Write documentation", Completed: false, Priority: "Low"},
		},
		TodoCount:      3,
		CompletedCount: 1,
		RemainingCount: 2,
		CompletionRate: 33,
		LastUpdated:    "2023-01-01 10:15:00",
		SessionID:      "session-12345",
	}

	// Update 2: Remove a todo and increase counter significantly
	update2State := E2EAppState{
		Title:   "Task Manager",
		Counter: 8, // Triggers "High Activity" status
		Todos: []TodoItem{
			{Text: "Learn Go templates", Completed: true, Priority: "High"},
			{Text: "Write documentation", Completed: false, Priority: "Low"},
		},
		TodoCount:      2,
		CompletedCount: 1,
		RemainingCount: 1,
		CompletionRate: 50,
		LastUpdated:    "2023-01-01 10:30:00",
		SessionID:      "session-12345",
	}

	// Update 3: Complete remaining todo (tests conditional branching)
	update3State := E2EAppState{
		Title:   "Task Manager",
		Counter: 8, // Same counter value
		Todos: []TodoItem{
			{Text: "Learn Go templates", Completed: true, Priority: "High"},
			{Text: "Write documentation", Completed: true, Priority: "Low"}, // Now completed
		},
		TodoCount:      2,
		CompletedCount: 2,   // Both completed
		RemainingCount: 0,   // None remaining
		CompletionRate: 100, // 100% completion
		LastUpdated:    "2023-01-01 10:45:00",
		SessionID:      "session-12345",
	}

	// Create template
	tmpl := New("e2e-test")
	_, err := tmpl.ParseFiles("testdata/e2e/input.tmpl")
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Step 1: Render initial full HTML page
	t.Run("1_Initial_Full_Render", func(t *testing.T) {
		var buf bytes.Buffer
		err := tmpl.Execute(&buf, initialState)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		renderedHTML := buf.String()

		// Save rendered HTML for review
		err = os.WriteFile("testdata/e2e/rendered_00_initial.html", []byte(renderedHTML), 0644)
		if err != nil {
			t.Logf("Warning: Could not save rendered_00_initial.html: %v", err)
		}

		// Verify key content is present
		expectedContent := []string{
			"<!DOCTYPE html>",
			"Task Manager - LiveTemplate Demo",
			"Count: 1",
			"Status: Low Activity",
			"Total Todos: 0",
			"No todos yet. Add some tasks!",
			"Last updated: 2023-01-01 10:00:00",
			"Session ID: session-12345",
			"data-lvt-id=", // Wrapper injection
		}

		for _, expected := range expectedContent {
			if !strings.Contains(renderedHTML, expected) {
				t.Errorf("Rendered HTML missing expected content: %q", expected)
			}
		}

		// Generate the initial tree structure for TypeScript client (force first render)
		tmplForTree := New("e2e-tree-test")
		_, err = tmplForTree.ParseFiles("testdata/e2e/input.tmpl")
		if err == nil {
			var treeBuf bytes.Buffer
			err = tmplForTree.ExecuteUpdates(&treeBuf, initialState)
			if err == nil {
				initialTreeJSON := treeBuf.Bytes()
				err = os.WriteFile("testdata/e2e/tree_00_initial.json", initialTreeJSON, 0644)
				if err != nil {
					t.Logf("Warning: Could not save tree_00_initial.json: %v", err)
				}
			}
		}

		t.Logf("✅ Initial render complete - HTML length: %d bytes", len(renderedHTML))
	})

	// Step 2: Add todos update - demonstrates adding new items
	t.Run("2_Add_Todos_Update", func(t *testing.T) {
		// Create a fresh template instance for the first update to include statics
		tmplFirstUpdate := New("e2e-first-update")
		_, err := tmplFirstUpdate.ParseFiles("testdata/e2e/input.tmpl")
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		var buf bytes.Buffer
		err = tmplFirstUpdate.ExecuteUpdates(&buf, update1State)
		if err != nil {
			t.Fatalf("ExecuteUpdates failed: %v", err)
		}

		updateJSON := buf.Bytes()

		// Parse and verify update structure
		var updateTree map[string]interface{}
		err = json.Unmarshal(updateJSON, &updateTree)
		if err != nil {
			t.Fatalf("Failed to parse update JSON: %v", err)
		}

		// Save the already-ordered JSON directly (preserves ordering from marshalOrderedJSON)
		err = os.WriteFile("testdata/e2e/update_01_add_todos.json", updateJSON, 0644)
		if err != nil {
			t.Logf("Warning: Could not save update_01_add_todos.json: %v", err)
		}

		// Compare with golden file
		compareWithGoldenFile(t, "update_01_add_todos", updateTree)

		// Render and save the full HTML after this update for reviewability
		var htmlBuf bytes.Buffer
		err = tmpl.Execute(&htmlBuf, update1State)
		if err == nil {
			renderedHTML := htmlBuf.String()
			err = os.WriteFile("testdata/e2e/rendered_01_add_todos.html", []byte(renderedHTML), 0644)
			if err != nil {
				t.Logf("Warning: Could not save rendered_01_add_todos.html: %v", err)
			}
		}

		// SHOULD contain static structure for first update (client needs to cache it)
		if _, hasStatics := updateTree["s"]; !hasStatics {
			t.Errorf("First update should contain static structure ('s' key) for client caching")
		}

		// Verify update contains new content
		updateStr := string(updateJSON)
		expectedUpdates := []string{
			"\"0\":\"Task Manager\"",        // Title in segment 0
			"\"1\":\"3\"",                   // Counter value in segment 1
			"\"4\":\"3\"",                   // Total todos in segment 4
			"\"5\":\"1\"",                   // Completed count in segment 5
			"\"6\":\"2\"",                   // Remaining count in segment 6
			"\"7\":\"33%\"",                 // Completion rate in segment 7
			"\"9\":\"2023-01-01 10:15:00\"", // Last updated in segment 9
			"\"10\":\"session-12345\"",      // Session ID in segment 10
		}

		for _, expected := range expectedUpdates {
			if !strings.Contains(updateStr, expected) {
				t.Errorf("Update 1 missing expected content: %q", expected)
			}
		}

		t.Logf("✅ Add todos update complete - JSON length: %d bytes", len(updateJSON))
		t.Logf("Update keys: %v", getMapKeys(updateTree))
	})

	// Step 3: Remove todo update - demonstrates removing items and status changes
	t.Run("3_Remove_Todo_Update", func(t *testing.T) {
		// Continue with the same template from step 2 (it has cached statics)
		// But we need to use the template from the previous test, so let's create it again
		// and call ExecuteUpdates twice to simulate the sequence
		tmplSequence := New("e2e-sequence")
		_, err := tmplSequence.ParseFiles("testdata/e2e/input.tmpl")
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		// First update to establish cached statics
		var firstBuf bytes.Buffer
		err = tmplSequence.ExecuteUpdates(&firstBuf, update1State)
		if err != nil {
			t.Fatalf("First ExecuteUpdates failed: %v", err)
		}

		// Second update - should not include statics
		var buf bytes.Buffer
		err = tmplSequence.ExecuteUpdates(&buf, update2State)
		if err != nil {
			t.Fatalf("ExecuteUpdates failed: %v", err)
		}

		updateJSON := buf.Bytes()

		// Parse and verify update structure
		var updateTree map[string]interface{}
		err = json.Unmarshal(updateJSON, &updateTree)
		if err != nil {
			t.Fatalf("Failed to parse update JSON: %v", err)
		}

		// Save the already-ordered JSON directly (preserves ordering from marshalOrderedJSON)
		err = os.WriteFile("testdata/e2e/update_02_remove_todo.json", updateJSON, 0644)
		if err != nil {
			t.Logf("Warning: Could not save update_02_remove_todo.json: %v", err)
		}

		// Compare with golden file
		compareWithGoldenFile(t, "update_02_remove_todo", updateTree)

		// Render and save the full HTML after this update for reviewability
		var htmlBuf bytes.Buffer
		err = tmpl.Execute(&htmlBuf, update2State)
		if err == nil {
			renderedHTML := htmlBuf.String()
			err = os.WriteFile("testdata/e2e/rendered_02_remove_todo.html", []byte(renderedHTML), 0644)
			if err != nil {
				t.Logf("Warning: Could not save rendered_02_remove_todo.html: %v", err)
			}
		}

		// Should NOT contain static structure on subsequent updates (cache-aware)
		if _, hasStatics := updateTree["s"]; hasStatics {
			t.Errorf("Subsequent updates should not contain static structure ('s' key) when cached")
		}

		// Verify status change from counter > 5 and todo removal
		updateStr := string(updateJSON)
		expectedUpdates := []string{
			"\"1\":\"Task Manager\"",        // Title in segment 1
			"\"2\":\"8\"",                   // Counter value in segment 2
			"\"5\":\"2\"",                   // Total todos in segment 5 (reduced from 3 to 2)
			"\"6\":\"1\"",                   // Completed count in segment 6
			"\"7\":\"1\"",                   // Remaining count in segment 7
			"\"8\":\"50%\"",                 // Completion rate in segment 8 (changed from 33 to 50)
			"\"10\":\"2023-01-01 10:30:00\"", // Last updated in segment 10
			"\"11\":\"session-12345\"",      // Session ID in segment 11
		}

		for _, expected := range expectedUpdates {
			if !strings.Contains(updateStr, expected) {
				t.Errorf("Update 2 missing expected content: %q", expected)
			}
		}

		t.Logf("✅ Remove todo update complete - JSON length: %d bytes", len(updateJSON))
		t.Logf("Update keys: %v", getMapKeys(updateTree))
	})

	// Step 4: Complete todo update - tests conditional branching fingerprinting
	t.Run("4_Complete_Todo_Update", func(t *testing.T) {
		// Continue with the same template sequence
		tmplSequence2 := New("e2e-sequence-2")
		_, err := tmplSequence2.ParseFiles("testdata/e2e/input.tmpl")
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		// First two updates to establish state
		var firstBuf bytes.Buffer
		err = tmplSequence2.ExecuteUpdates(&firstBuf, update1State)
		if err != nil {
			t.Fatalf("First ExecuteUpdates failed: %v", err)
		}

		var secondBuf bytes.Buffer
		err = tmplSequence2.ExecuteUpdates(&secondBuf, update2State)
		if err != nil {
			t.Fatalf("Second ExecuteUpdates failed: %v", err)
		}

		// Third update - complete the remaining todo
		var buf bytes.Buffer
		err = tmplSequence2.ExecuteUpdates(&buf, update3State)
		if err != nil {
			t.Fatalf("ExecuteUpdates failed: %v", err)
		}

		updateJSON := buf.Bytes()

		// Parse and verify update structure
		var updateTree map[string]interface{}
		err = json.Unmarshal(updateJSON, &updateTree)
		if err != nil {
			t.Fatalf("Failed to parse update JSON: %v", err)
		}

		// Save the update JSON for review
		err = os.WriteFile("testdata/e2e/update_03_complete_todo.json", updateJSON, 0644)
		if err != nil {
			t.Logf("Warning: Could not save update_03_complete_todo.json: %v", err)
		}

		// Compare with golden file
		compareWithGoldenFile(t, "update_03_complete_todo", updateTree)

		// Render and save the full HTML after this update for reviewability
		var htmlBuf bytes.Buffer
		err = tmpl.Execute(&htmlBuf, update3State)
		if err == nil {
			renderedHTML := htmlBuf.String()
			err = os.WriteFile("testdata/e2e/rendered_03_complete_todo.html", []byte(renderedHTML), 0644)
			if err != nil {
				t.Logf("Warning: Could not save rendered_03_complete_todo.html: %v", err)
			}
		}

		// Should NOT contain static structure on subsequent updates
		if _, hasStatics := updateTree["s"]; hasStatics {
			t.Errorf("Subsequent updates should not contain static structure ('s' key) when cached")
		}

		// Verify conditional branching changes - completion changes completed status
		updateStr := string(updateJSON)
		expectedUpdates := []string{
			"\"6\":\"2\"",                   // Completed count changed from 1 to 2
			"\"7\":\"0\"",                   // Remaining count changed from 1 to 0
			"\"8\":\"100%\"",               // Completion rate changed from 50 to 100
			"\"10\":\"2023-01-01 10:45:00\"", // Last updated timestamp
		}

		for _, expected := range expectedUpdates {
			if !strings.Contains(updateStr, expected) {
				t.Errorf("Update 3 missing expected content: %q", expected)
			}
		}

		t.Logf("✅ Complete todo update complete - JSON length: %d bytes", len(updateJSON))
		t.Logf("Update keys: %v", getMapKeys(updateTree))
	})

	// Step 5: Verify caching behavior with identical data
	t.Run("5_No_Change_Update", func(t *testing.T) {
		// Use the same sequence as step 4 to ensure proper fingerprint comparison
		tmplSequence3 := New("e2e-sequence-3")
		_, err := tmplSequence3.ParseFiles("testdata/e2e/input.tmpl")
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		// First three updates to establish state (same as previous tests)
		var firstBuf bytes.Buffer
		err = tmplSequence3.ExecuteUpdates(&firstBuf, update1State)
		if err != nil {
			t.Fatalf("First ExecuteUpdates failed: %v", err)
		}

		var secondBuf bytes.Buffer
		err = tmplSequence3.ExecuteUpdates(&secondBuf, update2State)
		if err != nil {
			t.Fatalf("Second ExecuteUpdates failed: %v", err)
		}

		// Now test with the same data again - should be optimized away
		var buf bytes.Buffer
		err = tmplSequence3.ExecuteUpdates(&buf, update2State) // Same data as update 2
		if err != nil {
			t.Fatalf("ExecuteUpdates failed: %v", err)
		}

		updateJSON := buf.Bytes()

		// Should be minimal/empty when no changes
		if len(updateJSON) > 10 { // Allow for empty JSON object "{}"
			var updateTree map[string]interface{}
			err = json.Unmarshal(updateJSON, &updateTree)
			if err == nil && len(updateTree) > 0 {
				t.Errorf("No-change update should be minimal, got %d bytes: %s", len(updateJSON), updateJSON)
			}
		}

		t.Logf("✅ No-change update verified - %d bytes (should be minimal)", len(updateJSON))
	})

	// Step 6: Performance verification
	t.Run("6_Performance_Check", func(t *testing.T) {
		// Measure update generation time
		start := time.Now()
		for i := 0; i < 100; i++ {
			var buf bytes.Buffer
			_ = tmpl.ExecuteUpdates(&buf, update1State)
		}
		duration := time.Since(start)

		avgDuration := duration / 100
		if avgDuration > 10*time.Millisecond {
			t.Errorf("Average update generation too slow: %v (should be <10ms)", avgDuration)
		}

		t.Logf("✅ Performance check passed - average update time: %v", avgDuration)
	})
}

// Helper function to get map keys for logging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// compareWithGoldenFile compares generated update with expected golden file
func compareWithGoldenFile(t *testing.T, updateName string, generatedUpdate TreeNode) {
	goldenFile := "testdata/e2e/" + updateName + ".golden.json"

	// Read golden file
	goldenData, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Logf("Golden file %s not found, creating reference...", goldenFile)
		return
	}

	// Parse golden file
	var expected map[string]interface{}
	err = json.Unmarshal(goldenData, &expected)
	if err != nil {
		t.Fatalf("Failed to parse golden file %s: %v", goldenFile, err)
	}

	// Convert generated update to map for comparison
	generated := map[string]interface{}(generatedUpdate)

	// Compare structures
	if !deepEqual(expected, generated) {
		t.Errorf("Generated update for %s does not match golden file", updateName)

		// Show detailed differences
		expectedJSON, _ := json.MarshalIndent(expected, "", "  ")
		generatedJSON, _ := json.MarshalIndent(generated, "", "  ")

		t.Logf("Expected (golden):\n%s", string(expectedJSON))
		t.Logf("Generated (actual):\n%s", string(generatedJSON))

		// Show specific differences
		showDifferences(t, expected, generated, "")
	} else {
		t.Logf("✅ %s matches golden file perfectly", updateName)
	}
}

// Note: deepEqual function is defined in template.go

// showDifferences shows detailed differences between expected and actual
func showDifferences(t *testing.T, expected, actual map[string]interface{}, prefix string) {
	// Check for missing keys in actual
	for key, expectedVal := range expected {
		actualVal, exists := actual[key]
		keyPath := prefix + key

		if !exists {
			t.Logf("Missing key: %s", keyPath)
			continue
		}

		if !deepEqual(expectedVal, actualVal) {
			t.Logf("Different value at %s:", keyPath)
			t.Logf("  Expected: %v", expectedVal)
			t.Logf("  Actual: %v", actualVal)
		}
	}

	// Check for extra keys in actual
	for key := range actual {
		if _, exists := expected[key]; !exists {
			t.Logf("Extra key: %s%s", prefix, key)
		}
	}
}
