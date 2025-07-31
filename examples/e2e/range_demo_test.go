package e2e

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/livefir/statetemplate"
)

// Test data structures for range demo testing
type RangeDemoItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type RangeDemoItemList struct {
	Items []RangeDemoItem `json:"items"`
}

// TestRangeDemoE2E tests the complete range fragment functionality end-to-end
func TestRangeDemoE2E(t *testing.T) {
	// Create real-time renderer
	config := &statetemplate.RealtimeConfig{
		WrapperTag:     "div",
		IDPrefix:       "fragment-",
		PreserveBlocks: true,
	}
	renderer := statetemplate.NewRealtimeRenderer(config)

	// Range template (same as in the demo)
	templateContent := `<div>
	<h2>Item List</h2>
	<ul>
		{{range .Items}}
			<li data-id="{{.ID}}">{{.Name}}</li>
		{{end}}
	</ul>
</div>`

	// Add template
	err := renderer.AddTemplate("range_demo", templateContent)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	// Initial data with 2 items
	initialData := &RangeDemoItemList{
		Items: []RangeDemoItem{
			{ID: "1", Name: "First Item"},
			{ID: "2", Name: "Second Item"},
		},
	}

	// Set initial data and get full HTML
	fullHTML, err := renderer.SetInitialData(initialData)
	if err != nil {
		t.Fatalf("Failed to set initial data: %v", err)
	}

	// Verify initial HTML structure
	if !strings.Contains(fullHTML, `<ul id="`) {
		t.Error("Expected ul element to have an ID attribute for container targeting")
	}
	if !strings.Contains(fullHTML, `<li id="`) {
		t.Error("Expected li elements to have ID attributes for item targeting")
	}
	if !strings.Contains(fullHTML, `data-id="1">First Item`) {
		t.Error("Expected first item with data-id='1' and content 'First Item'")
	}
	if !strings.Contains(fullHTML, `data-id="2">Second Item`) {
		t.Error("Expected second item with data-id='2' and content 'Second Item'")
	}

	t.Logf("✅ Initial HTML structure correct:\n%s", fullHTML)

	// Start the renderer
	renderer.Start()
	defer renderer.Stop()

	// Get update channel
	updateChan := renderer.GetUpdateChannel()

	// Collect updates for validation
	var receivedUpdates []statetemplate.RealtimeUpdate
	updateTimeout := time.After(5 * time.Second)

	// Collector goroutine
	go func() {
		for {
			select {
			case update := <-updateChan:
				receivedUpdates = append(receivedUpdates, update)
			case <-updateTimeout:
				return
			}
		}
	}()

	// Test 1: Add an item (should trigger append)
	t.Log("🧪 Test 1: Adding a new item...")
	newData1 := &RangeDemoItemList{
		Items: []RangeDemoItem{
			{ID: "1", Name: "First Item"},
			{ID: "2", Name: "Second Item"},
			{ID: "3", Name: "Third Item"}, // New item
		},
	}
	renderer.SendUpdate(newData1)
	time.Sleep(500 * time.Millisecond) // Allow update processing

	// Test 2: Remove an item (should trigger remove)
	t.Log("🧪 Test 2: Removing an item...")
	newData2 := &RangeDemoItemList{
		Items: []RangeDemoItem{
			{ID: "1", Name: "First Item"},
			{ID: "3", Name: "Third Item"}, // Removed ID: "2"
		},
	}
	renderer.SendUpdate(newData2)
	time.Sleep(500 * time.Millisecond) // Allow update processing

	// Test 3: Modify an item (should trigger replace)
	t.Log("🧪 Test 3: Modifying an item...")
	newData3 := &RangeDemoItemList{
		Items: []RangeDemoItem{
			{ID: "1", Name: "Modified First Item"}, // Changed name
			{ID: "3", Name: "Third Item"},
		},
	}
	renderer.SendUpdate(newData3)
	time.Sleep(500 * time.Millisecond) // Allow update processing

	// Wait for all updates to be processed
	time.Sleep(1 * time.Second)

	// Validate updates
	if len(receivedUpdates) == 0 {
		t.Fatal("❌ No updates received - range fragment system may not be working")
	}

	t.Logf("📊 Received %d updates total", len(receivedUpdates))

	// Track different types of operations
	var appendUpdates, removeUpdates, replaceUpdates []statetemplate.RealtimeUpdate

	for _, update := range receivedUpdates {
		updateJSON, _ := json.MarshalIndent(update, "  ", "  ")
		t.Logf("📨 Update received: %s", updateJSON)

		// Validate common fields
		if update.FragmentID == "" {
			t.Error("❌ Update missing FragmentID")
		}
		if update.Action == "" {
			t.Error("❌ Update missing Action")
		}

		// Categorize by action
		switch update.Action {
		case "append":
			appendUpdates = append(appendUpdates, update)
		case "remove":
			removeUpdates = append(removeUpdates, update)
		case "replace":
			replaceUpdates = append(replaceUpdates, update)
		}
	}

	// Validate append operations
	if len(appendUpdates) == 0 {
		t.Error("❌ Expected at least one append operation when adding item")
	} else {
		appendUpdate := appendUpdates[0]
		if appendUpdate.ContainerID == "" {
			t.Error("❌ Append update missing ContainerID")
		}
		if !strings.Contains(appendUpdate.HTML, "Third Item") {
			t.Error("❌ Append update should contain 'Third Item'")
		}
		if appendUpdate.ItemIndex != 2 {
			t.Errorf("❌ Expected append ItemIndex to be 2, got %d", appendUpdate.ItemIndex)
		}
		t.Log("✅ Append operation validated successfully")
	}

	// Validate remove operations
	if len(removeUpdates) == 0 {
		t.Error("❌ Expected at least one remove operation when removing item")
	} else {
		removeUpdate := removeUpdates[0]
		if removeUpdate.ContainerID == "" {
			t.Error("❌ Remove update missing ContainerID")
		}
		if removeUpdate.HTML != "" {
			t.Error("❌ Remove update should have empty HTML")
		}
		t.Log("✅ Remove operation validated successfully")
	}

	// Validate replace operations
	if len(replaceUpdates) == 0 {
		t.Error("❌ Expected at least one replace operation when modifying item")
	} else {
		// Find the replace update for the first item (index 0)
		var modificationUpdate *statetemplate.RealtimeUpdate
		for _, update := range replaceUpdates {
			if update.ItemIndex == 0 {
				modificationUpdate = &update
				break
			}
		}

		if modificationUpdate == nil {
			t.Error("❌ Could not find replace update for modified first item")
		} else {
			if modificationUpdate.ContainerID == "" {
				t.Error("❌ Replace update missing ContainerID")
			}
			if !strings.Contains(modificationUpdate.HTML, "Modified First Item") {
				t.Errorf("❌ Replace update should contain 'Modified First Item', got: %s", modificationUpdate.HTML)
			} else {
				t.Log("✅ Replace operation validated successfully")
			}
		}
	}

	// Validate fragment ID patterns
	for _, update := range receivedUpdates {
		if !strings.Contains(update.FragmentID, "-item-") {
			t.Errorf("❌ Fragment ID '%s' should contain '-item-' pattern", update.FragmentID)
		}

		// Fragment ID should be in format: containerID-item-index
		parts := strings.Split(update.FragmentID, "-item-")
		if len(parts) != 2 {
			t.Errorf("❌ Fragment ID '%s' should be in format 'containerID-item-index'", update.FragmentID)
		} else {
			containerID := parts[0]
			if update.ContainerID != "" && update.ContainerID != containerID {
				t.Errorf("❌ Fragment ID container part '%s' should match ContainerID '%s'", containerID, update.ContainerID)
			}
		}
	}

	t.Log("🎉 Range Demo E2E test completed successfully!")
	t.Log("✅ All range fragment operations (append, remove, replace) working correctly")
	t.Log("✅ Fragment IDs properly generated for JavaScript client targeting")
	t.Log("✅ Container and item IDs correctly structured for DOM manipulation")
}

// TestRangeDemoFragmentStructure tests the fragment structure generation
func TestRangeDemoFragmentStructure(t *testing.T) {
	// Create real-time renderer
	config := &statetemplate.RealtimeConfig{
		WrapperTag:     "div",
		IDPrefix:       "fragment-",
		PreserveBlocks: true,
	}
	renderer := statetemplate.NewRealtimeRenderer(config)

	// Range template
	templateContent := `<div>
	<h2>Item List</h2>
	<ul>
		{{range .Items}}
			<li data-id="{{.ID}}">{{.Name}}</li>
		{{end}}
	</ul>
</div>`

	// Add template
	err := renderer.AddTemplate("range_structure", templateContent)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	// Check fragment details
	fragmentDetails := renderer.GetFragmentDetails()
	t.Logf("📋 Fragment details: %+v", fragmentDetails)

	// Check that range fragments were detected
	fragmentIDs := renderer.GetFragmentIDs()
	t.Logf("🔍 Fragment IDs: %+v", fragmentIDs)

	// Verify fragment count
	fragmentCount := renderer.GetFragmentCount()
	if fragmentCount == 0 {
		t.Error("❌ Expected fragments to be detected for range template")
	}
	t.Logf("📊 Total fragments detected: %d", fragmentCount)

	// Test with data to verify HTML structure
	initialData := &RangeDemoItemList{
		Items: []RangeDemoItem{
			{ID: "test1", Name: "Test Item 1"},
			{ID: "test2", Name: "Test Item 2"},
		},
	}

	fullHTML, err := renderer.SetInitialData(initialData)
	if err != nil {
		t.Fatalf("Failed to set initial data: %v", err)
	}

	// Verify HTML structure contains proper IDs
	if !strings.Contains(fullHTML, `id="`) {
		t.Error("❌ Expected HTML to contain element IDs for fragment targeting")
	}

	// Check for proper ul container ID
	if !strings.Contains(fullHTML, `<ul id="`) {
		t.Error("❌ Expected ul element to have container ID")
	}

	// Check for proper li item IDs
	liIDCount := strings.Count(fullHTML, `<li id="`)
	if liIDCount != 2 {
		t.Errorf("❌ Expected 2 li elements with IDs, found %d", liIDCount)
	}

	t.Log("✅ Fragment structure test completed successfully!")
	t.Logf("🏗️  Generated HTML structure:\n%s", fullHTML)
}
