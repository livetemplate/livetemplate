package e2e

import (
	"os"
	"testing"

	"github.com/livefir/statetemplate"
)

// TestMain runs before all tests and can be used for setup/teardown
func TestMain(m *testing.M) {
	// Run all tests
	exitCode := m.Run()

	// Clean up if needed
	os.Exit(exitCode)
}

// TestAllExamples runs all example tests together for comprehensive validation
func TestAllExamples(t *testing.T) {
	t.Run("Realtime", func(t *testing.T) {
		TestRealtimeExample(t)
	})

	t.Run("RealtimeWebsocket", func(t *testing.T) {
		TestRealtimeRendererWebsocketCompatibility(t)
	})

	t.Run("Comprehensive", func(t *testing.T) {
		TestComprehensiveTemplateActions(t)
	})
}

// BenchmarkRealtimeExample measures performance of real-time rendering
func BenchmarkRealtimeExample(b *testing.B) {
	renderer := statetemplate.NewRenderer(nil)

	template := `<div>Count: {{.Count}}, Message: {{.Message}}</div>`
	err := renderer.AddTemplate("bench", template)
	if err != nil {
		b.Fatalf("Failed to add template: %v", err)
	}

	type TestData struct {
		Count   int
		Message string
	}

	initialData := TestData{Count: 0, Message: "Initial"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testData := TestData{Count: i, Message: "Updated"}
		_, err := renderer.SetInitialData(initialData)
		if err != nil {
			b.Fatalf("Failed to set initial data: %v", err)
		}

		renderer.SendUpdate(testData)
	}
}
