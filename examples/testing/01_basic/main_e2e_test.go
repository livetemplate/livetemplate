package main

import (
	"testing"

	lvttest "github.com/livetemplate/livetemplate/cmd/lvt/testing"
)

func TestBasicE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Setup test environment
	test := lvttest.Setup(t, &lvttest.SetupOptions{
		AppPath: "./main.go",
	})
	defer test.Cleanup()

	// Navigate to home page
	if err := test.Navigate("/"); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	// Create assertion helper
	assert := lvttest.NewAssert(test)

	// Verify page content
	if err := assert.PageContains("Welcome"); err != nil {
		t.Errorf("Page should contain 'Welcome': %v", err)
	}

	if err := assert.PageContains("Hello from LiveTemplate!"); err != nil {
		t.Errorf("Page should contain message: %v", err)
	}

	if err := assert.PageContains("Count: 42"); err != nil {
		t.Errorf("Page should contain count: %v", err)
	}

	// Verify WebSocket connected
	if err := assert.WebSocketConnected(); err != nil {
		t.Errorf("WebSocket should be connected: %v", err)
	}

	// Verify no template errors
	if err := assert.NoTemplateErrors(); err != nil {
		t.Errorf("Should have no template errors: %v", err)
	}

	t.Log("✅ Basic e2e test passed!")
}
