package testing

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

const (
	dockerImage = "chromedp/headless-shell:latest"
)

// GetFreePort asks the kernel for a free open port that is ready to use
func GetFreePort() (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}

// GetChromeTestURL returns the URL for Chrome (in Docker) to access the test server
// On Linux with host networking: use localhost
// On macOS/Windows: use host.docker.internal
func GetChromeTestURL(port int) string {
	portStr := fmt.Sprintf("%d", port)
	if runtime.GOOS == "linux" {
		return "http://localhost:" + portStr
	}
	return "http://host.docker.internal:" + portStr
}

// StartDockerChrome starts the chromedp headless-shell Docker container
func StartDockerChrome(t *testing.T, debugPort int) *exec.Cmd {
	t.Helper()

	// Check if Docker is available
	if err := exec.Command("docker", "version").Run(); err != nil {
		t.Skip("Docker not available, skipping E2E test")
	}

	// Check if image exists, if not try to pull it (with timeout)
	checkCmd := exec.Command("docker", "image", "inspect", dockerImage)
	if err := checkCmd.Run(); err != nil {
		// Image doesn't exist, try to pull with timeout
		t.Log("Pulling chromedp/headless-shell Docker image...")
		pullCmd := exec.Command("docker", "pull", dockerImage)
		if err := pullCmd.Start(); err != nil {
			t.Fatalf("Failed to start docker pull: %v", err)
		}

		// Wait for pull with timeout
		pullDone := make(chan error, 1)
		go func() {
			pullDone <- pullCmd.Wait()
		}()

		select {
		case err := <-pullDone:
			if err != nil {
				t.Fatalf("Failed to pull Docker image: %v", err)
			}
			t.Log("✅ Docker image pulled successfully")
		case <-time.After(60 * time.Second):
			pullCmd.Process.Kill()
			t.Fatal("Docker pull timed out after 60 seconds")
		}
	} else {
		t.Log("✅ Docker image already exists, skipping pull")
	}

	// Start the container
	t.Log("Starting Chrome headless Docker container...")
	var cmd *exec.Cmd
	portMapping := fmt.Sprintf("%d:9222", debugPort)

	if runtime.GOOS == "linux" {
		// On Linux, use host networking so container can access localhost
		cmd = exec.Command("docker", "run", "--rm",
			"--network", "host",
			"--name", "chrome-e2e-test",
			dockerImage,
		)
	} else {
		// On macOS/Windows, map port for remote debugging
		// (container will use host.docker.internal to reach host)
		// Note: Don't pass Chrome flags - the image has a built-in setup
		cmd = exec.Command("docker", "run", "--rm",
			"-p", portMapping,
			"--name", "chrome-e2e-test",
			"--add-host", "host.docker.internal:host-gateway",
			dockerImage,
		)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start Chrome Docker container: %v", err)
	}

	// Wait for Chrome to be ready
	t.Log("Waiting for Chrome to be ready...")
	chromeURL := fmt.Sprintf("http://localhost:%d/json/version", debugPort)
	ready := false
	for i := 0; i < 30; i++ {
		resp, err := http.Get(chromeURL)
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !ready {
		cmd.Process.Kill()
		t.Fatal("Chrome failed to start within 15 seconds")
	}

	t.Log("✅ Chrome headless Docker container ready")
	return cmd
}

// StopDockerChrome stops the Chrome Docker container
func StopDockerChrome(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	t.Log("Stopping Chrome Docker container...")

	// Check if container exists before trying to stop it
	checkCmd := exec.Command("docker", "ps", "-a", "-q", "-f", "name=chrome-e2e-test")
	output, _ := checkCmd.Output()

	if len(output) > 0 {
		// Container exists, stop it gracefully
		stopCmd := exec.Command("docker", "stop", "chrome-e2e-test")
		if err := stopCmd.Run(); err != nil {
			t.Logf("Warning: Failed to stop Docker container: %v", err)
		}
	}

	// Kill the process if still running
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}
}

// StartTestServer starts a Go server on the specified port
// mainPath should be the path to main.go (e.g., "main.go" or "../../examples/counter/main.go")
func StartTestServer(t *testing.T, mainPath string, port int) *exec.Cmd {
	t.Helper()

	portStr := fmt.Sprintf("%d", port)
	serverURL := fmt.Sprintf("http://localhost:%d", port)

	t.Logf("Starting test server on port %s", portStr)
	cmd := exec.Command("go", "run", mainPath)
	cmd.Env = append([]string{"PORT=" + portStr}, cmd.Environ()...)

	// Start the server
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Wait for server to be ready
	ready := false
	for i := 0; i < 50; i++ { // 5 seconds
		resp, err := http.Get(serverURL)
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !ready {
		cmd.Process.Kill()
		t.Fatal("Server failed to start within 5 seconds")
	}

	t.Logf("✅ Test server ready at %s", serverURL)
	return cmd
}
