package serve

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type FileChangeHandler func(path string)

type Watcher struct {
	dir      string
	handler  FileChangeHandler
	debounce time.Duration
	mu       sync.Mutex
	stop     chan struct{}
	running  bool
	ignores  []string
	watchMap map[string]time.Time
}

func NewWatcher(dir string, handler FileChangeHandler) (*Watcher, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		dir:      absDir,
		handler:  handler,
		debounce: 100 * time.Millisecond,
		stop:     make(chan struct{}),
		ignores: []string{
			".git",
			".lvt",
			"node_modules",
			".DS_Store",
			"*.swp",
			"*.tmp",
			"*~",
		},
		watchMap: make(map[string]time.Time),
	}

	return w, nil
}

func (w *Watcher) Start() error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	w.mu.Unlock()

	go w.watch()
	log.Printf("File watcher started for: %s", w.dir)
	return nil
}

func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.running = false
	close(w.stop)
	log.Println("File watcher stopped")
}

func (w *Watcher) watch() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	fileStates := make(map[string]fileState)

	w.scanDirectory(w.dir, fileStates)

	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			w.checkForChanges(fileStates)
		}
	}
}

type fileState struct {
	modTime time.Time
	size    int64
}

func (w *Watcher) scanDirectory(dir string, states map[string]fileState) {
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if w.shouldIgnore(path, info) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !info.IsDir() {
			states[path] = fileState{
				modTime: info.ModTime(),
				size:    info.Size(),
			}
		}

		return nil
	})
}

func (w *Watcher) checkForChanges(fileStates map[string]fileState) {
	currentStates := make(map[string]fileState)
	w.scanDirectory(w.dir, currentStates)

	for path, currentState := range currentStates {
		oldState, exists := fileStates[path]

		if !exists {
			w.handleChange(path)
			fileStates[path] = currentState
		} else if currentState.modTime != oldState.modTime || currentState.size != oldState.size {
			w.handleChange(path)
			fileStates[path] = currentState
		}
	}

	for path := range fileStates {
		if _, exists := currentStates[path]; !exists {
			w.handleChange(path)
			delete(fileStates, path)
		}
	}
}

func (w *Watcher) handleChange(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	if lastChange, exists := w.watchMap[path]; exists {
		if now.Sub(lastChange) < w.debounce {
			return
		}
	}

	w.watchMap[path] = now

	go func() {
		time.Sleep(w.debounce)
		w.handler(path)
	}()
}

func (w *Watcher) shouldIgnore(path string, info os.FileInfo) bool {
	base := filepath.Base(path)

	for _, pattern := range w.ignores {
		if strings.HasPrefix(pattern, "*") {
			suffix := strings.TrimPrefix(pattern, "*")
			if strings.HasSuffix(base, suffix) {
				return true
			}
		} else if base == pattern {
			return true
		}
	}

	rel, err := filepath.Rel(w.dir, path)
	if err != nil {
		return false
	}

	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts {
		for _, pattern := range w.ignores {
			if !strings.HasPrefix(pattern, "*") && part == pattern {
				return true
			}
		}
	}

	return false
}

func (w *Watcher) AddIgnorePattern(pattern string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ignores = append(w.ignores, pattern)
}

func (w *Watcher) SetDebounce(duration time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.debounce = duration
}

func (w *Watcher) GetWatchedFiles() []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	files := make([]string, 0, len(w.watchMap))
	for path := range w.watchMap {
		files = append(files, path)
	}
	return files
}
