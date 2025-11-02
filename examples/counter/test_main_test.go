package main

import (
	"os"
	"testing"

	e2etest "github.com/livetemplate/livetemplate/cmd/lvt/testing"
)

func TestMain(m *testing.M) {
	e2etest.CleanupChromeContainers()

	code := m.Run()

	e2etest.CleanupChromeContainers()
	os.Exit(code)
}
