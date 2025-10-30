/*
Package testing provides a comprehensive framework for end-to-end testing
of LiveTemplate applications.

# Quick Start

	import lvttest "github.com/livefir/livetemplate/cmd/lvt/testing"

	func TestHomePage(t *testing.T) {
		test := lvttest.Setup(t, &lvttest.SetupOptions{
			AppPath: "./main.go",
		})
		defer test.Cleanup()

		test.Navigate("/")

		assert := lvttest.NewAssert(test)
		assert.PageContains("Welcome")
	}

# Features

  - Automatic Chrome/Chromium management (Docker or local)
  - Automatic server startup and shutdown
  - WebSocket connection handling
  - Console log capture (browser, server, WebSocket)
  - CRUD operation helpers
  - Modal testing
  - Search, sort, and pagination testing
  - Database seeding and cleanup
  - Resource testing for lvt-generated apps

# Chrome Modes

The framework supports three Chrome modes:

 1. Docker (default) - Uses chromedp/headless-shell container
 2. Local - Uses locally installed Chrome/Chromium
 3. Shared - Uses shared Chrome instance from TestMain

See Setup() and SetupOptions for configuration.

# Code Reduction

This framework dramatically reduces e2e test boilerplate:

  - Before: ~100-150 lines per CRUD test
  - After: ~10-20 lines per CRUD test
  - Reduction: 85-90%

# Examples

See examples/testing/ directory for complete examples:

  - 01_basic - Simple smoke test
  - 02_crud - Full CRUD workflow
  - 03_debugging - Console log capture
  - 04_assertions - All assertion types
  - 05_modal - Modal interactions
  - 06_interactions - Search, sort, pagination
  - 07_database - Database seeding
  - 08_resource - One-liner resource testing
  - 09_parallel - Parallel testing with shared Chrome
*/
package testing
