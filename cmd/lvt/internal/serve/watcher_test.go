package serve

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWatcher_StartStop(t *testing.T) {
	tmpDir := t.TempDir()

	changes := make(chan string, 10)
	handler := func(path string) {
		changes <- path
	}

	watcher, err := NewWatcher(tmpDir, handler)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}

	if err := watcher.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := watcher.Start(); err != nil {
		t.Fatal("Expected Start to be idempotent")
	}

	watcher.Stop()
	watcher.Stop()
}

func TestWatcher_DetectsChanges(t *testing.T) {
	tmpDir := t.TempDir()

	var mu sync.Mutex
	changes := make(map[string]int)

	handler := func(path string) {
		mu.Lock()
		changes[path]++
		mu.Unlock()
	}

	watcher, err := NewWatcher(tmpDir, handler)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}

	if err := watcher.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer watcher.Stop()

	testFile := filepath.Join(tmpDir, "test.txt")

	_ = os.WriteFile(testFile, []byte("initial"), 0644)
	time.Sleep(800 * time.Millisecond)

	_ = os.WriteFile(testFile, []byte("modified"), 0644)
	time.Sleep(800 * time.Millisecond)

	mu.Lock()
	count := changes[testFile]
	mu.Unlock()

	if count < 1 {
		t.Errorf("Expected at least 1 change event, got %d", count)
	}
}

func TestWatcher_IgnoresPatterns(t *testing.T) {
	tmpDir := t.TempDir()

	var mu sync.Mutex
	changes := make(map[string]int)

	handler := func(path string) {
		mu.Lock()
		changes[path]++
		mu.Unlock()
	}

	watcher, err := NewWatcher(tmpDir, handler)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}

	if err := watcher.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer watcher.Stop()

	gitDir := filepath.Join(tmpDir, ".git")
	_ = os.MkdirAll(gitDir, 0755)
	gitFile := filepath.Join(gitDir, "config")
	_ = os.WriteFile(gitFile, []byte("test"), 0644)

	swpFile := filepath.Join(tmpDir, "test.swp")
	_ = os.WriteFile(swpFile, []byte("test"), 0644)

	time.Sleep(800 * time.Millisecond)

	mu.Lock()
	gitCount := changes[gitFile]
	swpCount := changes[swpFile]
	mu.Unlock()

	if gitCount > 0 {
		t.Errorf("Expected .git files to be ignored, got %d changes", gitCount)
	}
	if swpCount > 0 {
		t.Errorf("Expected .swp files to be ignored, got %d changes", swpCount)
	}
}

func TestWatcher_Debounce(t *testing.T) {
	tmpDir := t.TempDir()

	var mu sync.Mutex
	changes := make(map[string]int)

	handler := func(path string) {
		mu.Lock()
		changes[path]++
		mu.Unlock()
	}

	watcher, err := NewWatcher(tmpDir, handler)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}

	watcher.SetDebounce(200 * time.Millisecond)

	if err := watcher.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer watcher.Stop()

	testFile := filepath.Join(tmpDir, "test.txt")

	for i := 0; i < 5; i++ {
		_ = os.WriteFile(testFile, []byte("change"), 0644)
		time.Sleep(50 * time.Millisecond)
	}

	time.Sleep(1 * time.Second)

	mu.Lock()
	count := changes[testFile]
	mu.Unlock()

	if count > 2 {
		t.Errorf("Expected debouncing to limit events to ~2, got %d", count)
	}
}

func TestWatcher_AddIgnorePattern(t *testing.T) {
	tmpDir := t.TempDir()

	changes := make(chan string, 10)
	handler := func(path string) {
		changes <- path
	}

	watcher, err := NewWatcher(tmpDir, handler)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}

	watcher.AddIgnorePattern("*.test")

	if err := watcher.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer watcher.Stop()

	testFile := filepath.Join(tmpDir, "file.test")
	_ = os.WriteFile(testFile, []byte("test"), 0644)

	time.Sleep(800 * time.Millisecond)

	select {
	case path := <-changes:
		if filepath.Base(path) == "file.test" {
			t.Error("Expected .test files to be ignored")
		}
	default:
	}
}
