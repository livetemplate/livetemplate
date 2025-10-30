package e2e

import (
	"strings"
	"testing"
)

// TestResource_List tests listing all available resources
func TestResource_List(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate multiple resources
	resources := []string{"users", "posts", "comments"}
	for _, resource := range resources {
		t.Logf("Generating %s resource...", resource)
		if err := runLvtCommand(t, appDir, "gen", "resource", resource, "name"); err != nil {
			t.Fatalf("Failed to generate %s: %v", resource, err)
		}
	}

	// List resources
	t.Log("Listing resources...")
	output, err := runLvtCommandWithOutput(t, appDir, "resource", "list")
	if err != nil {
		t.Fatalf("Failed to list resources: %v\nOutput: %s", err, output)
	}

	t.Logf("Resource list output:\n%s", output)

	// Verify all resources appear in output
	for _, resource := range resources {
		if !strings.Contains(output, resource) {
			t.Errorf("Resource %s not found in list output", resource)
		}
	}

	t.Log("✅ Resource list test passed")
}

// TestResource_Describe tests describing a specific resource schema
func TestResource_Describe(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate a resource with multiple field types
	t.Log("Generating products resource...")
	if err := runLvtCommand(t, appDir, "gen", "resource", "products",
		"name:string", "price:float", "quantity:int", "active:bool", "released_at:time"); err != nil {
		t.Fatalf("Failed to generate products: %v", err)
	}

	// Describe the resource
	t.Log("Describing products resource...")
	output, err := runLvtCommandWithOutput(t, appDir, "resource", "describe", "products")
	if err != nil {
		t.Fatalf("Failed to describe resource: %v\nOutput: %s", err, output)
	}

	t.Logf("Resource description output:\n%s", output)

	// Verify field information appears
	expectedFields := map[string]string{
		"name":        "TEXT",
		"price":       "REAL",
		"quantity":    "INTEGER",
		"active":      "BOOLEAN",
		"released_at": "DATETIME",
	}

	for field, sqlType := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("Field %s not found in description", field)
		}
		if !strings.Contains(output, sqlType) {
			t.Errorf("SQL type %s not found in description", sqlType)
		}
	}

	// Verify standard fields are present
	standardFields := []string{"id", "created_at"}
	for _, field := range standardFields {
		if !strings.Contains(output, field) {
			t.Errorf("Standard field %s not found in description", field)
		}
	}

	t.Log("✅ Resource describe test passed")
}
