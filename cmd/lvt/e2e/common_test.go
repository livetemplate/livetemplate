package e2e

import (
	"os/exec"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	e2etest "github.com/livefir/livetemplate/internal/testing"
)

// Wrapper functions to use internal/testing utilities with shorter names in tests

func startDockerChrome(t *testing.T, debugPort int) *exec.Cmd {
	return e2etest.StartDockerChrome(t, debugPort)
}

func stopDockerChrome(t *testing.T, cmd *exec.Cmd, debugPort int) {
	e2etest.StopDockerChrome(t, cmd, debugPort)
}

func getTestURL(port int) string {
	return e2etest.GetChromeTestURL(port)
}

func waitForWebSocketReady(timeout time.Duration) chromedp.Action {
	return e2etest.WaitForWebSocketReady(timeout)
}

func validateNoTemplateExpressions(selector string) chromedp.Action {
	return e2etest.ValidateNoTemplateExpressions(selector)
}
