package invariants

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/livetemplate/livetemplate/internal/build"
)

// TypeScriptOracle uses the real TypeScript client to apply diffs.
// It maintains a persistent Node.js process for efficiency.
type TypeScriptOracle struct {
	clientDir string
	process   *exec.Cmd
	stdin     io.WriteCloser
	stdout    *bufio.Reader
	stderr    io.ReadCloser
	mu        sync.Mutex
	closed    bool
}

// TypeScriptOracleRequest is sent to the oracle server.
type TypeScriptOracleRequest struct {
	OldTree any `json:"oldTree"`
	Diff    any `json:"diff"`
}

// TypeScriptOracleResponse is received from the oracle server.
type TypeScriptOracleResponse struct {
	HTML  string `json:"html"`
	Tree  any    `json:"tree"`
	Error string `json:"error"`
}

// ErrOracleClosed is returned when trying to use a closed oracle.
var ErrOracleClosed = errors.New("TypeScript oracle is closed")

// NewTypeScriptOracle starts a persistent Node.js process running oracle-server.js.
// The clientDir should point to the directory containing oracle-server.js.
func NewTypeScriptOracle(clientDir string) (*TypeScriptOracle, error) {
	// Verify oracle-server.js exists
	serverPath := filepath.Join(clientDir, "oracle-server.js")
	if _, err := os.Stat(serverPath); err != nil {
		return nil, fmt.Errorf("oracle-server.js not found at %s: %w", serverPath, err)
	}

	// Verify dist/state/tree-renderer.js exists (oracle-server.js requires it)
	rendererPath := filepath.Join(clientDir, "dist", "state", "tree-renderer.js")
	if _, err := os.Stat(rendererPath); err != nil {
		return nil, fmt.Errorf("tree-renderer.js not found at %s (run 'npm run build' in client directory): %w", rendererPath, err)
	}

	cmd := exec.Command("node", "oracle-server.js")
	cmd.Dir = clientDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting oracle server: %w", err)
	}

	return &TypeScriptOracle{
		clientDir: clientDir,
		process:   cmd,
		stdin:     stdin,
		stdout:    bufio.NewReader(stdout),
		stderr:    stderr,
	}, nil
}

// ApplyDiff sends a request to the persistent Node.js process and returns the result.
// This is equivalent to calling ApplyDiff in the Go oracle but uses the real TypeScript client.
func (o *TypeScriptOracle) ApplyDiff(oldTree, diffTree *build.TreeNode) (*TypeScriptOracleResponse, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.closed {
		return nil, ErrOracleClosed
	}

	// Convert trees to maps for JSON serialization
	request := TypeScriptOracleRequest{
		OldTree: TreeToMap(oldTree),
		Diff:    TreeToMap(diffTree),
	}

	// Send request as JSON line
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	if _, err := o.stdin.Write(append(requestBytes, '\n')); err != nil {
		return nil, fmt.Errorf("writing request: %w", err)
	}

	// Read response line with timeout
	responseChan := make(chan struct {
		line []byte
		err  error
	}, 1)

	go func() {
		line, err := o.stdout.ReadBytes('\n')
		responseChan <- struct {
			line []byte
			err  error
		}{line, err}
	}()

	select {
	case result := <-responseChan:
		if result.err != nil {
			// Try to read stderr for more context
			stderrBytes := make([]byte, 4096)
			n, _ := o.stderr.Read(stderrBytes)
			stderrMsg := ""
			if n > 0 {
				stderrMsg = string(stderrBytes[:n])
			}
			return nil, fmt.Errorf("reading response: %w (stderr: %s)", result.err, stderrMsg)
		}

		var response TypeScriptOracleResponse
		if err := json.Unmarshal(result.line, &response); err != nil {
			return nil, fmt.Errorf("parsing response: %w (raw: %s)", err, string(result.line))
		}

		if response.Error != "" {
			return nil, fmt.Errorf("TypeScript oracle error: %s", response.Error)
		}

		return &response, nil

	case <-time.After(10 * time.Second):
		return nil, errors.New("timeout waiting for oracle response")
	}
}

// ApplyDiffRaw sends raw map data to the oracle (for cases where trees are already converted).
func (o *TypeScriptOracle) ApplyDiffRaw(oldTree, diffTree map[string]any) (*TypeScriptOracleResponse, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.closed {
		return nil, ErrOracleClosed
	}

	request := TypeScriptOracleRequest{
		OldTree: oldTree,
		Diff:    diffTree,
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	if _, err := o.stdin.Write(append(requestBytes, '\n')); err != nil {
		return nil, fmt.Errorf("writing request: %w", err)
	}

	responseChan := make(chan struct {
		line []byte
		err  error
	}, 1)

	go func() {
		line, err := o.stdout.ReadBytes('\n')
		responseChan <- struct {
			line []byte
			err  error
		}{line, err}
	}()

	select {
	case result := <-responseChan:
		if result.err != nil {
			stderrBytes := make([]byte, 4096)
			n, _ := o.stderr.Read(stderrBytes)
			stderrMsg := ""
			if n > 0 {
				stderrMsg = string(stderrBytes[:n])
			}
			return nil, fmt.Errorf("reading response: %w (stderr: %s)", result.err, stderrMsg)
		}

		var response TypeScriptOracleResponse
		if err := json.Unmarshal(result.line, &response); err != nil {
			return nil, fmt.Errorf("parsing response: %w (raw: %s)", err, string(result.line))
		}

		if response.Error != "" {
			return nil, fmt.Errorf("TypeScript oracle error: %s", response.Error)
		}

		return &response, nil

	case <-time.After(10 * time.Second):
		return nil, errors.New("timeout waiting for oracle response")
	}
}

// Close shuts down the oracle server process.
func (o *TypeScriptOracle) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.closed {
		return nil
	}

	o.closed = true

	// Close stdin to signal server to exit; accumulate error but
	// always continue to wait/kill the child process.
	var closeErr error
	if o.stdin != nil {
		if err := o.stdin.Close(); err != nil {
			closeErr = fmt.Errorf("closing stdin: %w", err)
		}
	}

	// Wait for process to exit with timeout
	done := make(chan error, 1)
	go func() {
		done <- o.process.Wait()
	}()

	select {
	case err := <-done:
		return errors.Join(closeErr, err)
	case <-time.After(5 * time.Second):
		// Force kill if it doesn't exit
		if err := o.process.Process.Kill(); err != nil {
			return errors.Join(closeErr, fmt.Errorf("oracle server did not exit cleanly, kill failed: %w", err))
		}
		return errors.Join(closeErr, errors.New("oracle server did not exit cleanly, killed"))
	}
}

// FindClientDir locates the TypeScript client directory containing oracle-server.js.
// Searches common locations including CI paths and environment variable override.
func FindClientDir() (string, error) {
	// Allow override via environment variable for CI/custom setups
	if envPath := os.Getenv("LIVETEMPLATE_CLIENT_DIR"); envPath != "" {
		if _, err := os.Stat(filepath.Join(envPath, "oracle-server.js")); err == nil {
			return envPath, nil
		}
	}

	candidates := []string{
		"./client",           // CI: client checked out as subdirectory
		"../client",          // Local: sibling directory
		"../../client",       // Local: from internal/fuzz/invariants
		"../../../client",    // Local: deeper nesting
		"../../../../client", // Local: even deeper
	}

	for _, candidate := range candidates {
		absPath, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(absPath, "oracle-server.js")); err == nil {
			return absPath, nil
		}
	}

	return "", errors.New("could not find TypeScript client directory with oracle-server.js (set LIVETEMPLATE_CLIENT_DIR to override)")
}
