package e2e

import (
	"fmt"
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
	t.Run("Simple", func(t *testing.T) {
		TestSimpleExample(t)
	})

	t.Run("SimpleRendering", func(t *testing.T) {
		TestSimpleExampleTemplateRendering(t)
	})

	t.Run("Files", func(t *testing.T) {
		TestFilesExample(t)
	})

	t.Run("FilesPathDetection", func(t *testing.T) {
		TestFilesExamplePathDetection(t)
	})

	t.Run("Fragments", func(t *testing.T) {
		TestFragmentsExample(t)
	})

	t.Run("FragmentsGranular", func(t *testing.T) {
		TestFragmentsExampleGranularUpdates(t)
	})

	t.Run("Realtime", func(t *testing.T) {
		TestRealtimeExample(t)
	})

	t.Run("RealtimeWebsocket", func(t *testing.T) {
		TestRealtimeRendererWebsocketCompatibility(t)
	})
}

// BenchmarkExamples provides performance benchmarks for the examples
func BenchmarkSimpleExample(b *testing.B) {
	for i := 0; i < b.N; i++ {
		t := &testing.T{}
		TestSimpleExample(t)
		if t.Failed() {
			b.Fatal("Simple example test failed")
		}
	}
}

func BenchmarkFilesExample(b *testing.B) {
	for i := 0; i < b.N; i++ {
		t := &testing.T{}
		TestFilesExample(t)
		if t.Failed() {
			b.Fatal("Files example test failed")
		}
	}
}

func BenchmarkFragmentsExample(b *testing.B) {
	for i := 0; i < b.N; i++ {
		t := &testing.T{}
		TestFragmentsExample(t)
		if t.Failed() {
			b.Fatal("Fragments example test failed")
		}
	}
}

// BenchmarkRealtimeExample measures performance of real-time rendering
func BenchmarkRealtimeExample(b *testing.B) {
	renderer := statetemplate.NewRealtimeRenderer(nil)

	template := `<div>Count: {{.Count}}, Message: {{.Message}}</div>`
	err := renderer.AddTemplate("bench", template)
	if err != nil {
		b.Fatalf("Failed to add template: %v", err)
	}

	type TestData struct {
		Count   int
		Message string
	}

	initialData := &TestData{Count: 0, Message: "Initial"}
	_, err = renderer.SetInitialData(initialData)
	if err != nil {
		b.Fatalf("Failed to set initial data: %v", err)
	}

	renderer.Start()
	defer renderer.Stop()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		newData := &TestData{
			Count:   i,
			Message: fmt.Sprintf("Update %d", i),
		}
		renderer.SendUpdate(newData)
	}
}
