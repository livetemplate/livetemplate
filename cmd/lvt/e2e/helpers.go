package e2e

import (
	"fmt"
	"os/exec"
)

// runLVTCommand runs the lvt CLI command with the given arguments
func runLVTCommand(dir string, args ...string) error {
	// Get the lvt binary path - assume it's in the parent directory
	lvtBinary := "../../../lvt"

	cmd := exec.Command(lvtBinary, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %v\nOutput: %s", err, string(output))
	}
	return nil
}
