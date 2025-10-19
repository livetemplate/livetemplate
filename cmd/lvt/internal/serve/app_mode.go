package serve

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type AppMode struct {
	server     *Server
	appProcess *exec.Cmd
	appPort    int
	proxy      *httputil.ReverseProxy
	mu         sync.Mutex
	stopChan   chan struct{}
	mainGoPath string
	buildPath  string
}

func NewAppMode(s *Server) (*AppMode, error) {
	am := &AppMode{
		server:   s,
		appPort:  8080, // Default app port
		stopChan: make(chan struct{}),
	}

	if err := am.detectApp(); err != nil {
		return nil, err
	}

	targetURL, _ := url.Parse(fmt.Sprintf("http://localhost:%d", am.appPort))
	am.proxy = httputil.NewSingleHostReverseProxy(targetURL)

	am.proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Proxy error: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<title>App Starting...</title>
	<style>
		body {
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
			display: flex;
			align-items: center;
			justify-content: center;
			height: 100vh;
			margin: 0;
			background: #f5f5f5;
		}
		.container {
			text-align: center;
			background: white;
			padding: 3rem;
			border-radius: 8px;
			box-shadow: 0 2px 10px rgba(0,0,0,0.1);
		}
		.spinner {
			border: 4px solid #f3f3f3;
			border-top: 4px solid #3498db;
			border-radius: 50%%;
			width: 40px;
			height: 40px;
			animation: spin 1s linear infinite;
			margin: 0 auto 1rem;
		}
		@keyframes spin {
			0%% { transform: rotate(0deg); }
			100%% { transform: rotate(360deg); }
		}
		h1 { color: #2c3e50; margin: 0 0 0.5rem; }
		p { color: #7f8c8d; margin: 0; }
	</style>
	<script>
		setTimeout(() => window.location.reload(), 2000);
	</script>
</head>
<body>
	<div class="container">
		<div class="spinner"></div>
		<h1>Starting App...</h1>
		<p>Your Go application is building and starting up.</p>
		<p style="margin-top: 1rem; font-size: 0.9rem;">This page will refresh automatically.</p>
	</div>
</body>
</html>`)
	}

	if err := am.startApp(); err != nil {
		return nil, fmt.Errorf("failed to start app: %w", err)
	}

	return am, nil
}

func (am *AppMode) detectApp() error {
	goModPath := filepath.Join(am.server.config.Dir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return fmt.Errorf("go.mod not found in directory: %s", am.server.config.Dir)
	}

	mainGo := am.findMainGo()
	if mainGo == "" {
		return fmt.Errorf("main.go not found in project")
	}

	am.mainGoPath = mainGo
	am.buildPath = filepath.Join(os.TempDir(), "lvt-app-"+filepath.Base(am.server.config.Dir))

	log.Printf("App detected: %s", am.mainGoPath)
	return nil
}

func (am *AppMode) findMainGo() string {
	candidates := []string{
		filepath.Join(am.server.config.Dir, "main.go"),
		filepath.Join(am.server.config.Dir, "cmd", "server", "main.go"),
		filepath.Join(am.server.config.Dir, "cmd", "app", "main.go"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	cmdDir := filepath.Join(am.server.config.Dir, "cmd")
	if entries, err := os.ReadDir(cmdDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				mainPath := filepath.Join(cmdDir, entry.Name(), "main.go")
				if _, err := os.Stat(mainPath); err == nil {
					return mainPath
				}
			}
		}
	}

	return ""
}

func (am *AppMode) startApp() error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if am.appProcess != nil {
		am.stopAppLocked()
	}

	log.Printf("Building app from %s...", am.mainGoPath)

	buildCmd := exec.Command("go", "build", "-o", am.buildPath, filepath.Dir(am.mainGoPath))
	buildCmd.Dir = am.server.config.Dir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	log.Printf("Starting app on port %d...", am.appPort)

	am.appProcess = exec.Command(am.buildPath)
	am.appProcess.Dir = am.server.config.Dir
	am.appProcess.Stdout = os.Stdout
	am.appProcess.Stderr = os.Stderr
	am.appProcess.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", am.appPort))

	if err := am.appProcess.Start(); err != nil {
		return fmt.Errorf("failed to start app: %w", err)
	}

	log.Printf("App started (PID: %d)", am.appProcess.Process.Pid)

	time.Sleep(500 * time.Millisecond)

	go func() {
		if err := am.appProcess.Wait(); err != nil {
			log.Printf("App process exited: %v", err)
		}
	}()

	return nil
}

func (am *AppMode) stopAppLocked() {
	if am.appProcess == nil || am.appProcess.Process == nil {
		return
	}

	log.Printf("Stopping app (PID: %d)...", am.appProcess.Process.Pid)

	_ = am.appProcess.Process.Kill()

	done := make(chan error, 1)
	go func() {
		done <- am.appProcess.Wait()
	}()

	select {
	case <-done:
		log.Println("App stopped")
	case <-time.After(5 * time.Second):
		log.Println("App failed to stop gracefully, force killing")
		_ = am.appProcess.Process.Kill()
	}

	am.appProcess = nil
}

func (am *AppMode) Stop() {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.stopAppLocked()

	if am.buildPath != "" {
		os.Remove(am.buildPath)
	}
}

func (am *AppMode) Restart() error {
	log.Println("Restarting app due to file changes...")
	return am.startApp()
}

func (am *AppMode) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	am.proxy.ServeHTTP(w, r)
}

func (am *AppMode) HandleFileChange(path string) {
	ext := filepath.Ext(path)
	if ext == ".go" || ext == ".tmpl" || ext == ".sql" {
		log.Printf("Detected change in %s, restarting app...", path)

		go func() {
			time.Sleep(100 * time.Millisecond)
			if err := am.Restart(); err != nil {
				log.Printf("Failed to restart app: %v", err)
			}
		}()
	}
}

func (am *AppMode) WaitForReady(ctx context.Context) error {
	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, err := client.Get(fmt.Sprintf("http://localhost:%d", am.appPort))
		if err == nil {
			resp.Body.Close()
			return nil
		}

		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("app did not become ready within timeout")
}
