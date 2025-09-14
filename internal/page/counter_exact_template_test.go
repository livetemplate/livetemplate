package page

import (
	"context"
	"html/template"
	"testing"

	"github.com/livefir/livetemplate/internal/diff"
)

// TestCounterExactTemplate tests with the exact counter template
func TestCounterExactTemplate(t *testing.T) {
	// Exact template from counter example
	tmplText := `<!DOCTYPE html>
<html>
  <head>
    <title>Counter App</title>
    <style>
      .color-red { color: #FF6B6B; }
      .color-teal { color: #4ECDC4; }
      .color-blue { color: #45B7D1; }
      .color-green { color: #96CEB4; }
      .color-yellow { color: #FECA57; }
      .color-pink { color: #FF6FA6; }
      .color-purple { color: #9B59B6; }
      .color-lightblue { color: #3498DB; }
      .color-orange { color: #E67E22; }
      .color-turquoise { color: #1ABC9C; }
      .color-darkred { color: #E74C3C; }
      .color-emerald { color: #2ECC71; }
    </style>
  </head>
  <body>
    <h1><span class="{{.Color}}">Simple Counter</span></h1>
    <div>Hello {{.Counter}} World</div>
    <button onclick="sendAction('increment')">+</button>
    <button onclick="sendAction('decrement')">-</button>

    <script src="/static/js/livetemplate-client.js"></script>
    <script>
      // Initialize LiveTemplate client
      const client = new LiveTemplateClient();
      
      // Connect to WebSocket
      const port = window.location.port || "8080";
      client.connect("ws://localhost:" + port + "/ws");
      
      // Helper function for button actions
      function sendAction(action) {
        client.sendAction(action);
      }
    </script>
  </body>
</html>`

	tmpl, err := template.New("test").Parse(tmplText)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	page := &Page{
		ID:            "test-page",
		template:      tmpl,
		data:          map[string]interface{}{"Counter": 0, "Color": "color-red"},
		treeGenerator: diff.NewGenerator(),
	}

	// Detect regions
	regions, err := page.detectTemplateRegions()
	if err != nil {
		t.Fatalf("Failed to detect regions: %v", err)
	}

	t.Logf("Detected %d regions", len(regions))
	for i, region := range regions {
		t.Logf("Region %d: ID=%s, Source=%q", i, region.ID, region.TemplateSource)
	}

	// Store the detected regions in the page for fragment generation
	page.regions = regions

	// New data with both changes
	newData := map[string]interface{}{"Counter": 1, "Color": "color-blue"}

	// Generate fragments using the full page method
	fragments, err := page.renderFragmentsWithConfig(context.TODO(), newData, &FragmentConfig{IncludeMetadata: false})
	if err != nil {
		t.Fatalf("Fragment generation failed: %v", err)
	}

	t.Logf("Generated %d fragments", len(fragments))
	for i, fragment := range fragments {
		t.Logf("Fragment %d: ID=%s, Data=%+v",
			i, fragment.ID, fragment.Data)
	}

	// We expect 2 fragments and check what the actual IDs are
	if len(fragments) != 2 {
		t.Errorf("Expected 2 fragments, got %d", len(fragments))
	}
}
