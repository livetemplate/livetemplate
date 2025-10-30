package e2e

import (
	"time"

	"github.com/chromedp/chromedp"
	e2etest "github.com/livefir/livetemplate/cmd/lvt/testing"
)

// Wrapper functions to use cmd/lvt/testing utilities with shorter names in tests

// Unused: Kept for potential future use
// func startDockerChrome(t *testing.T, debugPort int) *exec.Cmd {
// 	return e2etest.StartDockerChrome(t, debugPort)
// }

// Unused: Kept for potential future use
// func stopDockerChrome(t *testing.T, cmd *exec.Cmd, debugPort int) {
// 	e2etest.StopDockerChrome(t, cmd, debugPort)
// }

func getTestURL(port int) string {
	return e2etest.GetChromeTestURL(port)
}

func waitFor(condition string, timeout time.Duration) chromedp.Action {
	return e2etest.WaitFor(condition, timeout)
}

func waitForWebSocketReady(timeout time.Duration) chromedp.Action {
	return e2etest.WaitForWebSocketReady(timeout)
}

func validateNoTemplateExpressions(selector string) chromedp.Action {
	return e2etest.ValidateNoTemplateExpressions(selector)
}
