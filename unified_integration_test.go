package livetemplate

import (
	"context"
	"html/template"
	"strings"
	"testing"

	"github.com/livefir/livetemplate/internal/diff"
)

// TestUnifiedTreeDiffIntegration demonstrates the unified approach with counter template
func TestUnifiedTreeDiffIntegration(t *testing.T) {
	// Counter template from examples/counter/templates/index.html
	templateSource := `<!DOCTYPE html>
<html>
  <head>
    <title>Counter App</title>
  </head>
  <body>
    <div style="color: {{.Color}}">Hello {{.Counter}} World</div>
    <button data-lvt-action="increment">+</button>
    <button data-lvt-action="decrement">-</button>

    <!-- Load LiveTemplate client library - auto-initializes with embedded token -->
    <script src="/client/livetemplate-client.js"></script>
  </body>
</html>`

	// Create unified tree differ
	differ := diff.NewTree()

	// Initial data (like counter example)
	oldData := map[string]interface{}{
		"Counter": 0,
		"Color":   "#ff6b6b",
	}

	// Updated data (after increment)
	newData := map[string]interface{}{
		"Counter": 1,
		"Color":   "#4ecdc4",
	}

	// First render (includes statics)
	firstUpdate, err := differ.Generate(templateSource, nil, oldData)
	if err != nil {
		t.Fatalf("Failed to generate first update: %v", err)
	}

	// Second render (only dynamics should change)
	secondUpdate, err := differ.Generate(templateSource, oldData, newData)
	if err != nil {
		t.Fatalf("Failed to generate second update: %v", err)
	}

	// Log results
	t.Logf("First render (with statics): %s", firstUpdate.String())
	t.Logf("Second render (dynamics only): %s", secondUpdate.String())

	// Verify first render includes statics
	if !firstUpdate.HasStatics() {
		t.Error("First render should include static segments")
	}

	// Verify second render has only dynamics
	if secondUpdate.HasStatics() {
		t.Error("Second render should not include static segments (cached client-side)")
	}

	// Verify dynamics changed
	if !secondUpdate.HasDynamics() {
		t.Error("Second render should have dynamic updates")
	}

	// Calculate bandwidth savings
	firstSize := firstUpdate.GetSize()
	secondSize := secondUpdate.GetSize()

	if secondSize >= firstSize {
		t.Errorf("Expected second update to be smaller than first, got %d >= %d", secondSize, firstSize)
	}

	savings := float64(firstSize-secondSize) / float64(firstSize) * 100
	t.Logf("Bandwidth savings: %.1f%% (%d bytes vs %d bytes)", savings, secondSize, firstSize)

	// The unified approach should achieve significant bandwidth savings
	if savings < 50 {
		t.Errorf("Expected at least 50%% bandwidth savings, got %.1f%%", savings)
	}
}

// TestUnifiedTreeDiffWithPageAPI demonstrates integration with Page API
func TestUnifiedTreeDiffWithPageAPI(t *testing.T) {
	// Template source
	templateSource := `<div style="color: {{.Color}}">Hello {{.Counter}} World</div>`

	// Create a page using the new unified approach
	tmpl, err := template.New("counter").Parse(templateSource)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Initial data
	initialData := map[string]interface{}{
		"Counter": 0,
		"Color":   "#ff6b6b",
	}

	// Create page with template source for unified diff
	page, err := NewPage(tmpl, initialData, WithTemplateSource(templateSource))
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer page.Close()

	// Test initial render
	html, err := page.Render()
	if err != nil {
		t.Fatalf("Failed to render: %v", err)
	}
	t.Logf("Initial HTML: %s", html)

	// Test fragment generation with unified approach
	newData := map[string]interface{}{
		"Counter": 1,
		"Color":   "#4ecdc4",
	}

	fragments, err := page.RenderFragments(context.Background(), newData, WithMetadata())
	if err != nil {
		t.Fatalf("Failed to generate fragments: %v", err)
	}

	if len(fragments) != 1 {
		t.Fatalf("Expected 1 fragment, got %d", len(fragments))
	}

	fragment := fragments[0]
	t.Logf("Generated fragment: %+v", fragment)

	// Verify unified tree diff was used (strategy 2)
	if fragment.Metadata == nil {
		t.Fatal("Expected metadata to be present")
	}

	if fragment.Metadata.Strategy != 2 {
		t.Errorf("Expected strategy 2 (unified tree diff), got %d", fragment.Metadata.Strategy)
	}

	// The fragment data should be UnifiedTreeUpdate
	if update, ok := fragment.Data.(*diff.Update); ok {
		t.Logf("Update structure: %s", update.String())

		// Should have dynamics for the updated values
		if !update.HasDynamics() {
			t.Error("Expected fragment to have dynamic updates")
		}
	} else {
		t.Errorf("Expected diff.Update, got %T", fragment.Data)
	}
}

// BenchmarkUnifiedTreeDiffVsBasic compares unified diff vs basic approach
func BenchmarkUnifiedTreeDiffVsBasic(b *testing.B) {
	templateSource := `<div style="color: {{.Color}}">Hello {{.Counter}} World</div>`

	// Test data
	oldData := map[string]interface{}{"Counter": 0, "Color": "#ff6b6b"}
	newData := map[string]interface{}{"Counter": 1, "Color": "#4ecdc4"}

	b.Run("UnifiedTreeDiff", func(b *testing.B) {
		differ := diff.NewTree()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			update, err := differ.Generate(templateSource, oldData, newData)
			if err != nil {
				b.Fatalf("Generation failed: %v", err)
			}
			_ = update
		}
	})

	b.Run("BasicApproach", func(b *testing.B) {
		tmpl, err := template.New("test").Parse(templateSource)
		if err != nil {
			b.Fatalf("Template parse failed: %v", err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var buf strings.Builder
			err := tmpl.Execute(&buf, newData)
			if err != nil {
				b.Fatalf("Template execution failed: %v", err)
			}
			_ = buf.String()
		}
	})
}
