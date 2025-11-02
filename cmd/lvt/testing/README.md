# LiveTemplate Testing Framework

A comprehensive e2e testing framework for LiveTemplate applications that reduces boilerplate by 85-90%.

## Installation

```bash
go get github.com/livetemplate/livetemplate/cmd/lvt/testing
```

## Quick Start

```go
package main

import (
    "testing"
    lvttest "github.com/livetemplate/livetemplate/cmd/lvt/testing"
)

func TestMyApp(t *testing.T) {
    // Setup test environment (Chrome + Server)
    test := lvttest.Setup(t, &lvttest.SetupOptions{
        AppPath: "./main.go",
    })
    defer test.Cleanup()

    // Navigate to page
    test.Navigate("/")

    // Create assertion helper
    assert := lvttest.NewAssert(test)

    // Run assertions
    assert.PageContains("Welcome")
    assert.WebSocketConnected()
    assert.NoConsoleErrors()
}
```

## Features

### Automatic Setup
- **Chrome Management**: Automatically starts/stops Docker Chrome or local Chrome
- **Server Management**: Starts your Go server on free port
- **WebSocket Ready**: Waits for LiveTemplate WebSocket connection
- **Cleanup**: Automatic teardown of all resources

### Comprehensive Loggers
```go
// Browser console logs
test.Console.GetLogs()
test.Console.GetErrors()
test.Console.PrintErrors()

// Server logs
test.Server.FindLog("pattern")
test.Server.PrintLast(10)

// WebSocket messages
test.WebSocket.GetMessages()
test.WebSocket.CountByDirection("sent")
test.WebSocket.Print()
```

### 17 Built-in Assertions
```go
assert := lvttest.NewAssert(test)

// Content
assert.PageContains("text")
assert.PageNotContains("text")

// Elements
assert.ElementExists("selector")
assert.ElementNotExists("selector")
assert.ElementVisible("selector")
assert.ElementHidden("selector")
assert.ElementCount("selector", 5)

// Text
assert.TextContent("selector", "exact text")
assert.TextContains("selector", "substring")

// Attributes & Classes
assert.AttributeValue("selector", "data-id", "123")
assert.HasClass("selector", "active")
assert.NotHasClass("selector", "disabled")

// Tables
assert.TableRowCount(10)

// Forms
assert.FormFieldValue("input[name='email']", "test@example.com")

// Validation
assert.WebSocketConnected()
assert.NoTemplateErrors()
assert.NoConsoleErrors()
```

### CRUD Testing
```go
crud := lvttest.NewCRUDTester(test, "/products")

// Create with typed fields
crud.Create(
    lvttest.TextField("name", "Widget"),
    lvttest.FloatField("price", 29.99),
    lvttest.IntField("quantity", 100),
    lvttest.BoolField("enabled", true),
)

// Verify existence
crud.VerifyExists("Widget")

// Delete
crud.Delete("record-id")
```

### Modal Testing
```go
modal := lvttest.NewModalTester(test).
    WithModalSelector("[data-test-id='create-modal']")

// Open by action
modal.OpenByAction("open_create")

// Verify visibility
modal.VerifyVisible()

// Fill form
modal.FillForm(
    lvttest.TextField("name", "New Item"),
)

// Click button
modal.ClickButton("Create")

// Wait for close
modal.WaitForClose(2 * time.Second)
```

## Chrome Modes

### Docker (Default)
```go
test := lvttest.Setup(t, &lvttest.SetupOptions{
    AppPath:    "./main.go",
    ChromeMode: lvttest.ChromeDocker, // default
})
```

### Local Chrome
```go
test := lvttest.Setup(t, &lvttest.SetupOptions{
    AppPath:    "./main.go",
    ChromeMode: lvttest.ChromeLocal,
    ChromePath: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
})
```

## Field Types

```go
// All field types for form filling
lvttest.TextField("name", "value")
lvttest.TextAreaField("description", "long text")
lvttest.IntField("quantity", 42)
lvttest.FloatField("price", 19.99)
lvttest.BoolField("enabled", true)
lvttest.SelectField("category", "Electronics")
```

## Examples

See `examples/testing/` for complete examples:
- `01_basic/` - Simple smoke test
- `02_crud/` - Full CRUD operations
- `03_debugging/` - Console & debugging
- `04_assertions/` - All assertion types
- `05_modal/` - Modal interactions

## Code Reduction

**Before (Manual Setup):**
```go
// ~100 lines of boilerplate
func TestManual(t *testing.T) {
    serverPort, _ := e2etest.GetFreePort()
    chromePort, _ := e2etest.GetFreePort()

    serverCmd := exec.Command("go", "run", "main.go")
    serverCmd.Env = append([]string{"PORT=" + fmt.Sprintf("%d", serverPort)}, ...)
    serverCmd.Start()
    defer serverCmd.Process.Kill()

    time.Sleep(2 * time.Second)

    chromeCmd := e2etest.StartDockerChrome(t, chromePort)
    defer e2etest.StopDockerChrome(t, chromeCmd, chromePort)

    allocCtx, allocCancel := chromedp.NewRemoteAllocator(...)
    defer allocCancel()

    ctx, cancel := chromedp.NewContext(allocCtx, ...)
    defer cancel()

    ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
    defer cancel()

    var html string
    chromedp.Run(ctx,
        chromedp.Navigate(fmt.Sprintf("http://localhost:%d", serverPort)),
        chromedp.WaitVisible("h1", chromedp.ByQuery),
        chromedp.OuterHTML("body", &html, chromedp.ByQuery),
    )

    if !strings.Contains(html, "Welcome") {
        t.Error("Page title not found")
    }
}
```

**After (With Framework):**
```go
// ~10 lines - 90% reduction!
func TestFramework(t *testing.T) {
    test := lvttest.Setup(t, &lvttest.SetupOptions{
        AppPath: "./main.go",
    })
    defer test.Cleanup()

    test.Navigate("/")

    assert := lvttest.NewAssert(test)
    assert.PageContains("Welcome")
}
```

## Generated Tests

When using `lvt gen`, tests are automatically generated with the framework:

```bash
lvt gen products name price:float quantity:int
```

Generates:
```go
func TestProductsE2E(t *testing.T) {
    test := lvttest.Setup(t, &lvttest.SetupOptions{
        AppPath: "../../cmd/myapp/main.go",
    })
    defer test.Cleanup()

    test.Navigate("/products")

    assert := lvttest.NewAssert(test)
    crud := lvttest.NewCRUDTester(test, "/products")

    // Automatic CRUD testing
    crud.Create(
        lvttest.TextField("name", "Test Product"),
        lvttest.FloatField("price", 29.99),
        lvttest.IntField("quantity", 100),
    )

    crud.VerifyExists("Test Product")
}
```

## Requirements

- **Docker** (for ChromeDocker mode) OR **Local Chrome** (for ChromeLocal mode)
- **Go 1.21+**

## Best Practices

1. **Skip in short mode**:
   ```go
   if testing.Short() {
       t.Skip("Skipping E2E test in short mode")
   }
   ```

2. **Use subtests**: Group related tests with `t.Run()`

3. **Print debug info on failure**:
   ```go
   if err := assert.NoConsoleErrors(); err != nil {
       test.Console.PrintErrors()
       test.WebSocket.PrintLast(10)
       t.Error(err)
   }
   ```

4. **Clean up test data**: Use `defer` for cleanup operations

5. **Use descriptive test names**: Make failures easy to understand

## Troubleshooting

### Chrome Container Issues
```bash
# Restart Docker
docker restart <chrome-container>

# Check logs
docker logs <chrome-container>
```

### WebSocket Connection Timeout
- Increase timeout in SetupOptions
- Check server logs with `test.Server.Print()`
- Verify WebSocket initialization in browser console

### Test Flakiness
- Use `WaitFor*` methods instead of `Sleep`
- Check console errors: `test.Console.PrintErrors()`
- Monitor WebSocket messages: `test.WebSocket.Print()`

## API Reference

### Core Types

**E2ETest**
- `Context` - chromedp context
- `ServerPort` - allocated server port
- `ChromePort` - allocated Chrome debug port
- `Console` - ConsoleLogger
- `Server` - ServerLogger
- `WebSocket` - WSMessageLogger

**SetupOptions**
- `AppPath` (required) - Path to main.go
- `Port` - Server port (auto if 0)
- `Timeout` - Test timeout (default 60s)
- `ChromeMode` - Docker/Local/Shared
- `ChromePath` - Path to Chrome binary

**Assert**
- 17 assertion methods
- All return `error` (nil on success)
- All use `T.Helper()` for proper error reporting

**CRUDTester**
- `Create(fields ...Field)` - Create record
- `Edit(id, fields ...Field)` - Update record
- `Delete(id)` - Delete record
- `VerifyExists(text)` - Check presence
- `VerifyNotExists(text)` - Check absence

**ModalTester**
- `OpenByAction(action)` - Open modal
- `CloseByAction(action)` - Close modal
- `VerifyVisible()` - Check visibility
- `VerifyHidden()` - Check hidden
- `FillForm(fields ...Field)` - Fill modal form
- `WaitForOpen(timeout)` - Wait for modal
- `WaitForClose(timeout)` - Wait for close

**Wait Utilities**
- `WaitFor(condition, timeout)` - Wait for JavaScript condition to be true
- `WaitForText(selector, text, timeout)` - Wait for element text to contain substring
- `WaitForCount(selector, count, timeout)` - Wait for specific element count
- `WaitForWebSocketReady(timeout)` - Wait for WebSocket connection and initial sync

## License

Same as LiveTemplate project.

## Contributing

Contributions welcome! Please ensure:
- All tests pass
- Code is formatted with `go fmt`
- Examples work correctly
- Documentation is updated
