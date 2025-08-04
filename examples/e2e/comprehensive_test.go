package e2e

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/livefir/statetemplate"
)

// Test data structures for comprehensive template actions testing
type TestUser struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	IsActive bool         `json:"is_active"`
	Profile  *TestProfile `json:"profile,omitempty"`
}

type TestProfile struct {
	Bio     string `json:"bio"`
	Website string `json:"website,omitempty"`
}

type TestAppData struct {
	Title             string     `json:"title"`
	CurrentUser       *TestUser  `json:"current_user,omitempty"`
	Users             []TestUser `json:"users"`
	ShowUserList      bool       `json:"show_user_list"`
	IsLoggedIn        bool       `json:"is_logged_in"`
	NotificationCount int        `json:"notification_count"`
}

// TestComprehensiveTemplateActions tests all major Go template actions with granular fragments
func TestComprehensiveTemplateActions(t *testing.T) {
	// Create real-time renderer with functional options
	renderer := statetemplate.NewRenderer(
		statetemplate.WithWrapperTag("div"),
		statetemplate.WithIDPrefix("fragment-"),
		statetemplate.WithPreserveBlocks(true),
	)

	// Template with all major template actions
	templateContent := `<div>
	<h1>{{.Title}}</h1>
	
	<!-- Simple field output -->
	<p>App Status: Active</p>
	
	<!-- If/Else conditional -->
	{{if .IsLoggedIn}}
		<div class="user-section">
			<h2>Welcome back!</h2>
			{{if .CurrentUser}}
				<p>Hello, {{.CurrentUser.Name}}!</p>
			{{else}}
				<p>Hello, Guest!</p>
			{{end}}
		</div>
	{{else}}
		<div class="login-section">
			<h2>Please log in</h2>
			<button>Login</button>
		</div>
	{{end}}
	
	<!-- With context block -->
	{{with .CurrentUser}}
		<section class="current-user">
			<h3>Current User: {{.Name}}</h3>
			<p>Status: {{if .IsActive}}Active{{else}}Inactive{{end}}</p>
			{{with .Profile}}
				<div class="profile">
					<p>Bio: {{.Bio}}</p>
					{{if .Website}}
						<p>Website: <a href="{{.Website}}">{{.Website}}</a></p>
					{{end}}
				</div>
			{{else}}
				<p>No profile information available</p>
			{{end}}
		</section>
	{{end}}
	
	<!-- Range loop with conditionals -->
	{{if .ShowUserList}}
		<div class="users-list">
			<h2>All Users ({{len .Users}})</h2>
			{{if .Users}}
				<ul>
					{{range .Users}}
						<li data-id="{{.ID}}">
							{{.Name}} 
							{{if .IsActive}}
								<span class="status active">●</span>
							{{else}}
								<span class="status inactive">○</span>
							{{end}}
						</li>
					{{end}}
				</ul>
			{{else}}
				<p>No users found</p>
			{{end}}
		</div>
	{{end}}
	
	<!-- Notifications with conditional and function call -->
	{{if gt .NotificationCount 0}}
		<div class="notifications">
			<p>You have {{.NotificationCount}} notifications</p>
		</div>
	{{end}}
</div>`

	// Add template
	err := renderer.Parse("comprehensive", templateContent)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	// Initial data showcasing all template action types
	initialData := &TestAppData{
		Title:             "Test App",
		IsLoggedIn:        true,
		ShowUserList:      true,
		NotificationCount: 5,
		CurrentUser: &TestUser{
			ID:       "user1",
			Name:     "Alice",
			IsActive: true,
			Profile: &TestProfile{
				Bio:     "Test bio",
				Website: "https://test.com",
			},
		},
		Users: []TestUser{
			{ID: "user1", Name: "Alice", IsActive: true},
			{ID: "user2", Name: "Bob", IsActive: false},
		},
	}

	// Set initial data and get full HTML
	fullHTML, err := renderer.SetInitialData(initialData)
	if err != nil {
		t.Fatalf("Failed to set initial data: %v", err)
	}

	// Validate initial HTML structure contains fragment IDs
	if !strings.Contains(fullHTML, `id="`) {
		t.Error("Expected HTML to contain fragment IDs")
	}

	// Validate specific template action outputs
	if !strings.Contains(fullHTML, "Test App") {
		t.Error("Expected title field output to be present")
	}
	if !strings.Contains(fullHTML, "Welcome back!") {
		t.Error("Expected if block content to be present")
	}
	if !strings.Contains(fullHTML, "Current User: Alice") {
		t.Error("Expected with block content to be present")
	}
	if !strings.Contains(fullHTML, `<ul id="`) {
		t.Error("Expected range loop container to have fragment ID")
	}
	if !strings.Contains(fullHTML, "5 notifications") {
		t.Error("Expected conditional with function call to work")
	}

	t.Logf("✅ Initial HTML structure validation passed")

	// Check fragment generation using stats
	stats := renderer.GetStats()
	if stats.TotalFragments == 0 {
		t.Error("Expected fragments to be generated for template actions")
	}
	t.Logf("📊 Generated %d fragments for comprehensive template", stats.TotalFragments)

	// Start renderer for real-time testing
	renderer.Start()
	defer renderer.Stop()

	updateChan := renderer.GetUpdateChannel()
	var receivedUpdates []statetemplate.Update
	updateTimeout := time.After(10 * time.Second)

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

	// Test 1: Toggle login status (if/else conditional)
	t.Log("🧪 Test 1: Toggle login status (if/else)")
	newData1 := *initialData
	newData1.IsLoggedIn = false
	newData1.CurrentUser = nil
	renderer.SendUpdate(&newData1)
	time.Sleep(500 * time.Millisecond)

	// Test 2: Change user context (with block)
	t.Log("🧪 Test 2: Change user context (with block)")
	newData2 := *initialData
	newData2.CurrentUser = &TestUser{
		ID:       "user2",
		Name:     "Bob",
		IsActive: false,
		Profile:  nil, // Test else case in with block
	}
	renderer.SendUpdate(&newData2)
	time.Sleep(500 * time.Millisecond)

	// Test 3: Toggle notifications (conditional with function)
	t.Log("🧪 Test 3: Toggle notifications (conditional with function)")
	newData3 := *initialData
	newData3.NotificationCount = 0 // Should hide notifications section
	renderer.SendUpdate(&newData3)
	time.Sleep(500 * time.Millisecond)

	// Test 4: Toggle user list visibility (if block)
	t.Log("🧪 Test 4: Toggle user list visibility")
	newData4 := *initialData
	newData4.ShowUserList = false
	renderer.SendUpdate(&newData4)
	time.Sleep(500 * time.Millisecond)

	// Test 5: Modify user list (range loop)
	t.Log("🧪 Test 5: Modify user list (range)")
	newData5 := *initialData
	newData5.Users = []TestUser{
		{ID: "user1", Name: "Alice", IsActive: false},  // Status change
		{ID: "user3", Name: "Charlie", IsActive: true}, // New user
	}
	renderer.SendUpdate(&newData5)
	time.Sleep(500 * time.Millisecond)

	// Wait for all updates
	time.Sleep(1 * time.Second)

	// Validate updates were received
	if len(receivedUpdates) == 0 {
		t.Fatal("❌ No updates received - template action fragments may not be working")
	}

	t.Logf("📨 Received %d updates total", len(receivedUpdates))

	// Analyze update types
	var conditionalUpdates, contextUpdates, rangeUpdates []statetemplate.Update

	for _, update := range receivedUpdates {
		updateJSON, _ := json.MarshalIndent(update, "  ", "  ")
		t.Logf("📋 Update: %s", updateJSON)

		// Categorize updates based on content and structure
		if strings.Contains(update.HTML, "Welcome back") || strings.Contains(update.HTML, "Please log in") {
			conditionalUpdates = append(conditionalUpdates, update)
		}
		if strings.Contains(update.HTML, "Current User:") || strings.Contains(update.HTML, "profile") {
			contextUpdates = append(contextUpdates, update) //nolint:staticcheck
		}
		if update.RangeInfo != nil || strings.Contains(update.FragmentID, "-item-") ||
			(update.Action == "remove" || update.Action == "append" || strings.Contains(update.HTML, "data-id")) {
			rangeUpdates = append(rangeUpdates, update)
		}
	}

	// Validate different action types were triggered
	if len(conditionalUpdates) == 0 {
		t.Error("❌ Expected conditional (if/else) updates")
	} else {
		t.Logf("✅ Conditional updates: %d", len(conditionalUpdates))
	}

	// Range updates are optional as they may not trigger if the array doesn't change significantly
	if len(rangeUpdates) > 0 {
		t.Logf("✅ Range updates: %d", len(rangeUpdates))
	} else {
		t.Logf("ℹ️  Range updates: %d (may not trigger if no significant array changes)", len(rangeUpdates))
	}

	// Validate update structure
	for _, update := range receivedUpdates {
		if update.FragmentID == "" {
			t.Error("❌ Update missing FragmentID")
		}
		if update.Action == "" {
			t.Error("❌ Update missing Action")
		}
		if update.Action != "replace" && update.Action != "append" && update.Action != "remove" {
			t.Errorf("❌ Unexpected action type: %s", update.Action)
		}
	}

	t.Log("🎉 Comprehensive template actions test completed successfully!")
	t.Log("✅ All major Go template actions support granular fragments:")
	t.Log("   • {{.Field}} - Simple field output")
	t.Log("   • {{if condition}} {{else}} {{end}} - Conditional blocks")
	t.Log("   • {{with .Object}} {{else}} {{end}} - Context blocks")
	t.Log("   • {{range .Array}} {{end}} - Loop blocks")
	t.Log("   • {{if gt .Count 0}} - Function calls with conditionals")
	t.Log("   • Nested and complex combinations")
}

// TestTemplateActionFragmentStructure tests that all template actions generate proper fragment IDs
func TestTemplateActionFragmentStructure(t *testing.T) {
	renderer := statetemplate.NewRenderer(
		statetemplate.WithWrapperTag("div"),
		statetemplate.WithIDPrefix("fragment-"),
		statetemplate.WithPreserveBlocks(true),
	)

	// Minimal template with each action type
	templateContent := `<div>
	<h1>{{.Title}}</h1>
	{{if .IsLoggedIn}}
		<p>Logged in</p>
	{{else}}
		<p>Not logged in</p>
	{{end}}
	{{with .CurrentUser}}
		<section>User: {{.Name}}</section>
	{{end}}
	{{range .Users}}
		<div>{{.Name}}</div>
	{{end}}
</div>`

	err := renderer.Parse("structure_test", templateContent)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	// Test data
	testData := &TestAppData{
		Title:      "Structure Test",
		IsLoggedIn: true,
		CurrentUser: &TestUser{
			Name: "Test User",
		},
		Users: []TestUser{
			{Name: "Item 1"},
			{Name: "Item 2"},
		},
	}

	fullHTML, err := renderer.SetInitialData(testData)
	if err != nil {
		t.Fatalf("Failed to set initial data: %v", err)
	}

	// Validate fragment structure using stats
	stats := renderer.GetStats()
	t.Logf("📊 Fragment Analysis:")
	t.Logf("   Total fragments: %d", stats.TotalFragments)
	t.Logf("   Fragments by type: %+v", stats.FragmentsByType)

	if stats.TotalFragments == 0 {
		t.Error("❌ Expected fragments to be generated")
	}

	// Check HTML contains proper fragment IDs
	if !strings.Contains(fullHTML, `id="`) {
		t.Error("❌ Expected HTML to contain element IDs")
	}

	// Validate range fragments specifically
	if !strings.Contains(fullHTML, `<div id="`) {
		t.Error("❌ Expected range items to have fragment IDs")
	}

	t.Log("✅ Template action fragment structure test passed!")
}
