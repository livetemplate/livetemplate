package main

import (
	"os"
	"testing"

	e2etest "github.com/livetemplate/livetemplate/cmd/lvt/testing"
)

func TestMain(m *testing.M) {
	// Best-effort cleanup in case previous runs leaked containers.
	e2etest.CleanupChromeContainers()

	code := m.Run()

	// Ensure we don't leave Chrome containers behind when the test process exits early.
	e2etest.CleanupChromeContainers()
	os.Exit(code)
}
