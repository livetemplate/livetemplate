# StateTemplate Public API Design

## Current Public API

The StateTemplate library currently exposes many types publicly. For a cleaner API, we should only expose the essential types that users need to interact with directly.

## Recommended Public API

### Core Public Types (Keep Exported)

1. **`RealtimeRenderer`** - Main entry point for users
2. **`RealtimeUpdate`** - Output type that users receive
3. **`RangeInfo`** - Part of RealtimeUpdate, users need to access its fields
4. **`RealtimeConfig`** - Configuration for creating renderer

### Support Functions (Keep Exported)

1. **`NewRealtimeRenderer(config *RealtimeConfig) *RealtimeRenderer`**
2. **`(r *RealtimeRenderer) AddTemplate(name, content string) error`**
3. **`(r *RealtimeRenderer) SetInitialData(data interface{}) (string, error)`**
4. **`(r *RealtimeRenderer) GetUpdateChannel() <-chan RealtimeUpdate`**
5. **`(r *RealtimeRenderer) SendUpdate(newData interface{})`**
6. **`(r *RealtimeRenderer) Start()`**
7. **`(r *RealtimeRenderer) Stop()`**

## Types That Could Be Made Internal (Future Refactoring)

These types are implementation details that users don't need to interact with directly:

### Fragment Types (Make Unexported)

- `TemplateFragment` → `templateFragment`
- `FragmentExtractor` → `fragmentExtractor`
- `RangeFragment` → `rangeFragment`
- `RangeItem` → `rangeItem`
- `ConditionalFragment` → `conditionalFragment`
- `TemplateIncludeFragment` → `templateIncludeFragment`
- `FragmentInfo` → `fragmentInfo`

### Analysis Types (Make Unexported)

- `AdvancedTemplateAnalyzer` → `advancedTemplateAnalyzer`
- `TemplateTracker` → `templateTracker`
- `DataUpdate` → `dataUpdate`
- `TemplateUpdate` → `templateUpdate`

### Proposal Types (Remove Entirely)

These appear to be experimental/proposal files that can be removed:

- `update_interface_proposal.go`
- `enhanced_update.go`

## Implementation Plan

1. **Phase 1**: Document the intended public API (this document)
2. **Phase 2**: Remove experimental/proposal files
3. **Phase 3**: Make internal types unexported (requires updating all references)
4. **Phase 4**: Move unexported types to `internal/` packages if needed

## Current Status

The library works correctly with the current API. The refactoring to hide internal types would be a breaking change but would result in a much cleaner API surface for users.

## Example Clean Usage

With the minimal API, users would only need to know about:

```go
import "github.com/livefir/statetemplate"

// Create renderer
renderer := statetemplate.NewRealtimeRenderer(&statetemplate.RealtimeConfig{
    WrapperTag: "div",
    IDPrefix:   "app-",
})

// Add template
err := renderer.AddTemplate("main", templateContent)

// Set initial data and get HTML
html, err := renderer.SetInitialData(data)

// Start processing updates
renderer.Start()
defer renderer.Stop()

// Listen for updates
updateChan := renderer.GetUpdateChannel()
go func() {
    for update := range updateChan {
        // update is of type statetemplate.RealtimeUpdate
        // Send to WebSocket clients
        if update.RangeInfo != nil {
            // Handle range operations
            fmt.Printf("Range operation: %s, item: %s\n",
                      update.Action, update.RangeInfo.ItemKey)
        }
    }
}()

// Send data updates
renderer.SendUpdate(newData)
```

Users would never need to interact with internal fragment types, analyzers, or trackers directly.
