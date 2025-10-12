package e2e

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	goruntime "runtime"
	"testing"
	"time"
)

// startDockerChrome starts Chrome in Docker for E2E testing
func startDockerChrome(t *testing.T, debugPort int) *exec.Cmd {
	t.Helper()

	// Check if Docker is available
	if err := exec.Command("docker", "version").Run(); err != nil {
		t.Skip("Docker not available, skipping E2E test")
	}

	dockerImage := "chromedp/headless-shell:latest"

	// Pull image if needed
	checkCmd := exec.Command("docker", "image", "inspect", dockerImage)
	if err := checkCmd.Run(); err != nil {
		t.Log("Pulling Chrome Docker image...")
		pullCmd := exec.Command("docker", "pull", dockerImage)
		pullCmd.Stdout = os.Stdout
		pullCmd.Stderr = os.Stderr
		if err := pullCmd.Run(); err != nil {
			t.Fatalf("Failed to pull Docker image: %v", err)
		}
	}

	// Start container
	t.Log("Starting Chrome Docker container...")
	portMapping := fmt.Sprintf("%d:9222", debugPort)
	containerName := fmt.Sprintf("lvt-e2e-chrome-%d", debugPort)

	var cmd *exec.Cmd
	if goruntime.GOOS == "linux" {
		cmd = exec.Command("docker", "run", "--rm",
			"--network", "host",
			"--name", containerName,
			dockerImage,
		)
	} else {
		cmd = exec.Command("docker", "run", "--rm",
			"-p", portMapping,
			"--name", containerName,
			"--add-host", "host.docker.internal:host-gateway",
			dockerImage,
		)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start Chrome: %v", err)
	}

	// Wait for Chrome to be ready
	chromeURL := fmt.Sprintf("http://localhost:%d/json/version", debugPort)
	for i := 0; i < 60; i++ {
		resp, err := http.Get(chromeURL)
		if err == nil {
			resp.Body.Close()
			t.Log("✅ Chrome ready")
			return cmd
		}
		time.Sleep(500 * time.Millisecond)
	}

	cmd.Process.Kill()
	t.Fatal("Chrome failed to start")
	return nil
}

// stopDockerChrome stops the Chrome container
func stopDockerChrome(t *testing.T, cmd *exec.Cmd, debugPort int) {
	t.Helper()
	containerName := fmt.Sprintf("lvt-e2e-chrome-%d", debugPort)

	// Stop the container and wait for it to complete
	stopCmd := exec.Command("docker", "stop", containerName)
	if err := stopCmd.Run(); err != nil {
		// Container might already be stopped, that's okay
		t.Logf("Docker stop returned: %v", err)
	}

	// Wait for the process to fully exit
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
		cmd.Wait() // Wait for process to complete I/O
	}
}

// getTestURL returns the URL for Chrome to access the test server
func getTestURL(port int) string {
	if goruntime.GOOS == "linux" {
		return fmt.Sprintf("http://localhost:%d", port)
	}
	return fmt.Sprintf("http://host.docker.internal:%d", port)
}
