# LiveTemplate Testing Framework Examples

This directory contains examples demonstrating how to use the `cmd/lvt/testing` framework for e2e testing.

## Examples

### 01_basic - Simple Smoke Test

**Location:** `01_basic/`

A minimal example showing:
- Basic Setup() and Cleanup()
- Navigate() to test pages
- Simple assertions (PageContains, WebSocketConnected)

**Run:**
```bash
cd 01_basic
go test -v
```

**Key Features:**
- 10 lines of test code (vs ~100 lines without framework)
- Automatic Chrome management
- Automatic server startup
- WebSocket verification

### 03_debugging - Console & Debugging

**Location:** `03_debugging/`

Demonstrates debugging capabilities:
- Browser console log capture (info, warning, error)
- Server log capture
- WebSocket message monitoring
- Print methods for debugging test failures

**Run:**
```bash
cd 03_debugging
go test -v
```

**Key Features:**
- `test.Console.GetLogs()` - Capture all console logs
- `test.Console.HasErrors()` - Check for console errors
- `test.Server.FindLog()` - Search server logs
- `test.WebSocket.GetMessages()` - Monitor WebSocket traffic
- Print methods for debugging: `Print()`, `PrintLast(n)`, `PrintMatching(pattern)`

### 04_assertions - Comprehensive Assertions

**Location:** `04_assertions/`

Demonstrates all available assertion methods:
- Page content assertions (PageContains, PageNotContains)
- Element existence (ElementExists, ElementNotExists)
- Element count (ElementCount, TableRowCount)
- Text content (TextContent, TextContains)
- Attributes (AttributeValue)
- CSS classes (HasClass, NotHasClass)
- Visibility (ElementVisible, ElementHidden)
- Console errors (NoConsoleErrors)
- WebSocket connection (WebSocketConnected)
- Template validation (NoTemplateErrors)

**Run:**
```bash
cd 04_assertions
go test -v
```

**Key Features:**
- 17 different assertion methods
- All assertions return descriptive errors
- Thread-safe console error checking
- Comprehensive coverage of common test scenarios

### 02_crud - CRUD Operations

**Location:** `02_crud/`

Demonstrates CRUD testing with the CRUDTester helper:
- Create products with multiple field types
- Delete products by ID
- Verify existence/non-existence
- Test with TextField, FloatField, IntField, BoolField

**Run:**
```bash
cd 02_crud
go test -v
```

**Key Features:**
- `CRUDTester` for streamlined CRUD operations
- Polymorphic Field interface for form filling
- Automatic WebSocket synchronization
- ~10 lines per test (vs ~50 without framework)

### 05_modal - Modal Testing

**Location:** `05_modal/`

Demonstrates modal dialog testing:
- Open/close modals with actions
- Verify visibility states
- Fill forms inside modals
- Wait for modal state changes

**Run:**
```bash
cd 05_modal
go test -v
```

**Key Features:**
- `ModalTester` for modal interactions
- `VerifyVisible()` and `VerifyHidden()` assertions
- `FillForm()` with Field interface
- `WaitForOpen()` and `WaitForClose()` helpers
- Support for create/edit modal workflows

## Coming Soon

- 06_interactions - Search, sort, pagination
- 07_database - Database seeding
- 08_resource - One-liner resource testing
- 09_parallel - Parallel testing with shared Chrome

## Requirements

- Docker (for Chrome headless)
- Go 1.21+

## Usage Pattern

```go
package main

import (
    "testing"
    lvttest "github.com/livetemplate/livetemplate/cmd/lvt/testing"
)

func TestMyApp(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping E2E test in short mode")
    }

    // Setup
    test := lvttest.Setup(t, &lvttest.SetupOptions{
        AppPath: "./main.go",
    })
    defer test.Cleanup()

    // Navigate
    test.Navigate("/")

    // Assert
    assert := lvttest.NewAssert(test)
    assert.PageContains("Welcome")
}
```

## Skip E2E Tests

For fast iteration during development:

```bash
# Run only unit tests
go test -short

# Run all tests including e2e
go test -v
```
