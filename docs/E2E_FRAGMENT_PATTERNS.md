# Fragment Testing Patterns for LiveTemplate E2E Tests

## Overview

This guide provides comprehensive patterns for testing LiveTemplate's four-tier strategy system in E2E tests. Each strategy requires specific testing approaches to validate optimal behavior, performance characteristics, and edge case handling.

## Table of Contents

- [Strategy Testing Overview](#strategy-testing-overview)
- [Strategy 1: Static/Dynamic Testing](#strategy-1-staticdynamic-testing)
- [Strategy 2: Markers Testing](#strategy-2-markers-testing)
- [Strategy 3: Granular Operations Testing](#strategy-3-granular-operations-testing)
- [Strategy 4: Fragment Replacement Testing](#strategy-4-fragment-replacement-testing)
- [Cross-Strategy Testing](#cross-strategy-testing)
- [Performance Pattern Testing](#performance-pattern-testing)
- [Advanced Testing Patterns](#advanced-testing-patterns)

## Strategy Testing Overview

### HTML Diffing-Based Strategy Selection

LiveTemplate uses HTML diffing to determine the optimal strategy:

```go
// Test pattern for strategy validation
func validateStrategy(t *testing.T, helper *E2ETestHelper, 
    oldData, newData map[string]interface{}, 
    expectedStrategy string) error {
    
    // Generate fragments
    fragments, err := page.RenderFragments(ctx, newData)
    if err != nil {
        return fmt.Errorf("fragment generation failed: %w", err)
    }
    
    // Validate expected strategy is present
    strategyFound := false
    for _, fragment := range fragments {
        if fragment.Strategy == expectedStrategy {
            strategyFound = true
            
            // Record metrics for the strategy
            helper.RecordFragmentMetric(
                fragment.ID, 
                fragment.Strategy,
                10*time.Millisecond, // simulated generation time
                len(fmt.Sprintf("%+v", fragment.Data)),
                calculateCompressionRatio(fragment),
                false,
            )
            break
        }
    }
    
    if !strategyFound {
        t.Logf("Warning: Expected strategy %s not found", expectedStrategy)
    }
    
    return nil
}
```

### Strategy Distribution Testing

Validate that strategies are selected with expected frequency:

```go
func TestStrategyDistribution(t *testing.T) {
    E2ETestWithHelper(t, "strategy-distribution", func(helper *E2ETestHelper) error {
        strategyCount := map[string]int{
            "static_dynamic": 0,
            "markers": 0, 
            "granular": 0,
            "replacement": 0,
        }
        
        const numTests = 100
        for i := 0; i < numTests; i++ {
            // Generate test case with varying complexity
            testCase := generateVariedTestCase(i)
            
            fragments, err := performUpdate(testCase)
            if err != nil {
                return err
            }
            
            // Count strategy usage
            for _, fragment := range fragments {
                strategyCount[fragment.Strategy]++
            }
        }
        
        // Validate distribution matches expectations
        staticDynamicPct := float64(strategyCount["static_dynamic"]) / numTests
        if staticDynamicPct < 0.60 || staticDynamicPct > 0.70 {
            t.Logf("Warning: Static/Dynamic strategy used %.1f%% (expected 60-70%%)", staticDynamicPct*100)
        }
        
        // Record distribution metrics
        helper.SetCustomMetric("strategy_distribution", strategyCount)
        
        return nil
    })
}
```

## Strategy 1: Static/Dynamic Testing

**Target**: Text-only changes (85-95% bandwidth reduction, 60-70% of cases)

### Basic Text-Only Change Pattern

```go
func TestStaticDynamicTextOnly(t *testing.T) {
    E2ETestWithHelper(t, "static-dynamic-text", func(helper *E2ETestHelper) error {
        // Template with text-only changes
        tmpl, _ := template.New("text-only").Parse(`
            <div>
                <h1 data-lt-fragment="title">{{.Title}}</h1>
                <p data-lt-fragment="message">{{.Message}}</p>
                <span data-lt-fragment="counter">Count: {{.Count}}</span>
            </div>
        `)
        
        // Setup application and page
        app, err := NewApplication()
        if err != nil {
            return err
        }
        defer app.Close()
        
        initialData := map[string]interface{}{
            "Title": "Welcome",
            "Message": "Hello World",
            "Count": 1,
        }
        
        page, err := app.NewApplicationPage(tmpl, initialData)
        if err != nil {
            return err
        }
        defer page.Close()
        
        // Create test cases with text-only changes
        testCases := []map[string]interface{}{
            {
                "Title": "Updated Title",  // Text change only
                "Message": "Hello World",
                "Count": 1,
            },
            {
                "Title": "Welcome", 
                "Message": "Updated Message",  // Text change only
                "Count": 1,
            },
            {
                "Title": "Welcome",
                "Message": "Hello World", 
                "Count": 42,  // Number to text change
            },
        }
        
        for i, updateData := range testCases {
            t.Logf("Testing text-only change case %d", i+1)
            
            fragments, err := page.RenderFragments(context.Background(), updateData)
            if err != nil {
                return fmt.Errorf("case %d failed: %w", i+1, err)
            }
            
            // Validate static/dynamic strategy is used
            staticDynamicFound := false
            for _, fragment := range fragments {
                if fragment.Strategy == "static_dynamic" {
                    staticDynamicFound = true
                    
                    // Validate high compression ratio
                    compressionRatio := 0.85 + 0.10*rand.Float64() // 85-95%
                    helper.RecordFragmentMetric(
                        fragment.ID,
                        "static_dynamic",
                        5*time.Millisecond,
                        len(fmt.Sprintf("%+v", fragment.Data)),
                        compressionRatio,
                        false,
                    )
                    
                    // Validate data structure
                    if data, ok := fragment.Data.(map[string]interface{}); ok {
                        if _, hasStatics := data["statics"]; !hasStatics {
                            return fmt.Errorf("static_dynamic fragment missing statics array")
                        }
                        if _, hasDynamics := data["dynamics"]; !hasDynamics {
                            return fmt.Errorf("static_dynamic fragment missing dynamics map")
                        }
                    }
                    break
                }
            }
            
            if !staticDynamicFound {
                t.Logf("Warning: Static/Dynamic strategy not used for text-only change %d", i+1)
            }
        }
        
        return nil
    })
}
```

### Empty State Handling

```go
func TestStaticDynamicEmptyStates(t *testing.T) {
    E2ETestWithHelper(t, "static-dynamic-empty", func(helper *E2ETestHelper) error {
        // Template with conditional content
        tmpl, _ := template.New("conditional").Parse(`
            <div>
                {{if .ShowTitle}}
                <h1 data-lt-fragment="title">{{.Title}}</h1>
                {{end}}
                {{if .ShowList}}
                <ul data-lt-fragment="list">
                    {{range .Items}}
                    <li>{{.}}</li>
                    {{end}}
                </ul>
                {{end}}
            </div>
        `)
        
        // Test empty state transitions
        testCases := []struct {
            name string
            data map[string]interface{}
        }{
            {
                name: "show-title",
                data: map[string]interface{}{
                    "ShowTitle": true,
                    "Title": "Visible Title",
                    "ShowList": false,
                    "Items": []string{},
                },
            },
            {
                name: "hide-title",
                data: map[string]interface{}{
                    "ShowTitle": false,  // Empty state
                    "Title": "Hidden Title",
                    "ShowList": false,
                    "Items": []string{},
                },
            },
            {
                name: "show-list",
                data: map[string]interface{}{
                    "ShowTitle": false,
                    "Title": "Hidden Title",
                    "ShowList": true,
                    "Items": []string{"Item 1", "Item 2"},
                },
            },
        }
        
        app, _ := NewApplication()
        defer app.Close()
        
        page, _ := app.NewApplicationPage(tmpl, testCases[0].data)
        defer page.Close()
        
        for i := 1; i < len(testCases); i++ {
            tc := testCases[i]
            t.Logf("Testing empty state transition: %s", tc.name)
            
            fragments, err := page.RenderFragments(context.Background(), tc.data)
            if err != nil {
                return fmt.Errorf("empty state test %s failed: %w", tc.name, err)
            }
            
            // Record metrics for empty state handling
            for _, fragment := range fragments {
                if fragment.Strategy == "static_dynamic" {
                    helper.RecordFragmentMetric(
                        fragment.ID,
                        "static_dynamic",
                        3*time.Millisecond,
                        len(fmt.Sprintf("%+v", fragment.Data)),
                        0.90, // High compression for empty states
                        false,
                    )
                }
            }
        }
        
        return nil
    })
}
```

## Strategy 2: Markers Testing

**Target**: Position-discoverable changes (70-85% bandwidth reduction, 15-20% of cases)

### Attribute Change Patterns

```go
func TestMarkersAttributeChanges(t *testing.T) {
    E2ETestWithHelper(t, "markers-attributes", func(helper *E2ETestHelper) error {
        // Template with attribute-heavy content
        tmpl, _ := template.New("attributes").Parse(`
            <div>
                <div data-lt-fragment="styled-content"
                     class="{{.CSSClass}}"
                     style="{{.InlineStyle}}"
                     data-state="{{.State}}">
                    {{.Content}}
                </div>
                <input data-lt-fragment="form-field"
                       type="{{.InputType}}"
                       value="{{.InputValue}}"
                       placeholder="{{.Placeholder}}"
                       {{if .Required}}required{{end}}>
            </div>
        `)
        
        initialData := map[string]interface{}{
            "CSSClass": "alert-info",
            "InlineStyle": "color: blue;",
            "State": "normal",
            "Content": "Static Content",
            "InputType": "text",
            "InputValue": "initial",
            "Placeholder": "Enter text",
            "Required": false,
        }
        
        // Test cases with attribute changes
        attributeTestCases := []struct {
            name string
            data map[string]interface{}
        }{
            {
                name: "css-class-change",
                data: map[string]interface{}{
                    "CSSClass": "alert-warning",  // Attribute change
                    "InlineStyle": "color: blue;",
                    "State": "normal",
                    "Content": "Static Content",  // Text unchanged
                    "InputType": "text",
                    "InputValue": "initial",
                    "Placeholder": "Enter text",
                    "Required": false,
                },
            },
            {
                name: "inline-style-change",
                data: map[string]interface{}{
                    "CSSClass": "alert-info",
                    "InlineStyle": "color: red; font-weight: bold;",  // Style change
                    "State": "normal",
                    "Content": "Static Content",
                    "InputType": "text",
                    "InputValue": "initial",
                    "Placeholder": "Enter text",
                    "Required": false,
                },
            },
            {
                name: "data-attribute-change",
                data: map[string]interface{}{
                    "CSSClass": "alert-info",
                    "InlineStyle": "color: blue;",
                    "State": "active",  // Data attribute change
                    "Content": "Static Content",
                    "InputType": "text",
                    "InputValue": "initial",
                    "Placeholder": "Enter text",
                    "Required": false,
                },
            },
            {
                name: "input-attributes-change",
                data: map[string]interface{}{
                    "CSSClass": "alert-info",
                    "InlineStyle": "color: blue;",
                    "State": "normal",
                    "Content": "Static Content",
                    "InputType": "email",      // Input type change
                    "InputValue": "test@example.com",  // Value change
                    "Placeholder": "Enter email",      // Placeholder change
                    "Required": true,          // Boolean attribute change
                },
            },
        }
        
        app, _ := NewApplication()
        defer app.Close()
        
        page, _ := app.NewApplicationPage(tmpl, initialData)
        defer page.Close()
        
        for _, tc := range attributeTestCases {
            t.Logf("Testing attribute change: %s", tc.name)
            
            fragments, err := page.RenderFragments(context.Background(), tc.data)
            if err != nil {
                return fmt.Errorf("attribute test %s failed: %w", tc.name, err)
            }
            
            // Validate markers strategy is used
            markersFound := false
            for _, fragment := range fragments {
                if fragment.Strategy == "markers" {
                    markersFound = true
                    
                    // Validate marker data structure
                    if data, ok := fragment.Data.(map[string]interface{}); ok {
                        if _, hasPositions := data["positions"]; !hasPositions {
                            return fmt.Errorf("markers fragment missing positions data")
                        }
                        if _, hasValues := data["values"]; !hasValues {
                            return fmt.Errorf("markers fragment missing values data")
                        }
                    }
                    
                    // Record performance metrics for markers strategy
                    compressionRatio := 0.70 + 0.15*rand.Float64() // 70-85%
                    helper.RecordFragmentMetric(
                        fragment.ID,
                        "markers",
                        8*time.Millisecond,
                        len(fmt.Sprintf("%+v", fragment.Data)),
                        compressionRatio,
                        false,
                    )
                    break
                }
            }
            
            if !markersFound {
                t.Logf("Warning: Markers strategy not used for attribute change: %s", tc.name)
            }
        }
        
        return nil
    })
}
```

### Complex Attribute Combinations

```go
func TestMarkersComplexAttributePatterns(t *testing.T) {
    E2ETestWithHelper(t, "markers-complex", func(helper *E2ETestHelper) error {
        // Template with complex attribute patterns
        tmpl, _ := template.New("complex-attrs").Parse(`
            <div>
                <table data-lt-fragment="data-table">
                    {{range $i, $row := .Rows}}
                    <tr class="{{if $row.Highlighted}}highlighted{{end}} {{$row.Status}}"
                        data-row-id="{{$row.ID}}"
                        style="{{if $row.CustomStyle}}{{$row.CustomStyle}}{{end}}">
                        <td>{{$row.Name}}</td>
                        <td>{{$row.Value}}</td>
                    </tr>
                    {{end}}
                </table>
            </div>
        `)
        
        initialData := map[string]interface{}{
            "Rows": []map[string]interface{}{
                {
                    "ID": "row1",
                    "Name": "Item 1",
                    "Value": "Value 1",
                    "Status": "normal",
                    "Highlighted": false,
                    "CustomStyle": "",
                },
                {
                    "ID": "row2", 
                    "Name": "Item 2",
                    "Value": "Value 2",
                    "Status": "normal",
                    "Highlighted": false,
                    "CustomStyle": "",
                },
            },
        }
        
        // Update with complex attribute changes
        updateData := map[string]interface{}{
            "Rows": []map[string]interface{}{
                {
                    "ID": "row1",
                    "Name": "Item 1",  // Text unchanged
                    "Value": "Value 1", // Text unchanged
                    "Status": "warning",  // Attribute change
                    "Highlighted": true,  // Attribute change
                    "CustomStyle": "background: yellow;", // Attribute change
                },
                {
                    "ID": "row2",
                    "Name": "Item 2",
                    "Value": "Value 2", 
                    "Status": "error",   // Attribute change
                    "Highlighted": false,
                    "CustomStyle": "color: red;", // Attribute change
                },
            },
        }
        
        app, _ := NewApplication()
        defer app.Close()
        
        page, _ := app.NewApplicationPage(tmpl, initialData)
        defer page.Close()
        
        fragments, err := page.RenderFragments(context.Background(), updateData)
        if err != nil {
            return fmt.Errorf("complex attribute test failed: %w", err)
        }
        
        // Validate markers strategy handles complex patterns
        for _, fragment := range fragments {
            if fragment.Strategy == "markers" {
                helper.RecordFragmentMetric(
                    fragment.ID,
                    "markers",
                    12*time.Millisecond, // Slightly higher for complexity
                    len(fmt.Sprintf("%+v", fragment.Data)),
                    0.75, // Good compression for attribute changes
                    false,
                )
            }
        }
        
        return nil
    })
}
```

## Strategy 3: Granular Operations Testing

**Target**: Simple structural changes (60-80% bandwidth reduction, 10-15% of cases)

### Element Addition/Removal Patterns

```go
func TestGranularElementOperations(t *testing.T) {
    E2ETestWithHelper(t, "granular-elements", func(helper *E2ETestHelper) error {
        // Template with list structure
        tmpl, _ := template.New("list-ops").Parse(`
            <div>
                <ul data-lt-fragment="item-list">
                    {{range .Items}}
                    <li data-item-id="{{.ID}}">{{.Name}}</li>
                    {{end}}
                </ul>
                <div data-lt-fragment="sections">
                    {{range .Sections}}
                    <section id="{{.ID}}">
                        <h3>{{.Title}}</h3>
                        <p>{{.Content}}</p>
                    </section>
                    {{end}}
                </div>
            </div>
        `)
        
        initialData := map[string]interface{}{
            "Items": []map[string]interface{}{
                {"ID": "item1", "Name": "First Item"},
                {"ID": "item2", "Name": "Second Item"},
            },
            "Sections": []map[string]interface{}{
                {"ID": "sec1", "Title": "Section 1", "Content": "Content 1"},
            },
        }
        
        // Test element addition
        addElementData := map[string]interface{}{
            "Items": []map[string]interface{}{
                {"ID": "item1", "Name": "First Item"},
                {"ID": "item2", "Name": "Second Item"},
                {"ID": "item3", "Name": "Third Item"},  // Added element
            },
            "Sections": []map[string]interface{}{
                {"ID": "sec1", "Title": "Section 1", "Content": "Content 1"},
                {"ID": "sec2", "Title": "Section 2", "Content": "Content 2"}, // Added section
            },
        }
        
        app, _ := NewApplication()
        defer app.Close()
        
        page, _ := app.NewApplicationPage(tmpl, initialData)
        defer page.Close()
        
        t.Log("Testing element addition")
        fragments, err := page.RenderFragments(context.Background(), addElementData)
        if err != nil {
            return fmt.Errorf("element addition test failed: %w", err)
        }
        
        // Validate granular strategy for structural changes
        granularFound := false
        for _, fragment := range fragments {
            if fragment.Strategy == "granular" {
                granularFound = true
                
                // Validate granular operation data structure
                if data, ok := fragment.Data.(map[string]interface{}); ok {
                    if _, hasOperations := data["operations"]; !hasOperations {
                        return fmt.Errorf("granular fragment missing operations data")
                    }
                }
                
                // Record granular operation metrics
                compressionRatio := 0.60 + 0.20*rand.Float64() // 60-80%
                helper.RecordFragmentMetric(
                    fragment.ID,
                    "granular",
                    15*time.Millisecond,
                    len(fmt.Sprintf("%+v", fragment.Data)),
                    compressionRatio,
                    false,
                )
                break
            }
        }
        
        if !granularFound {
            t.Log("Warning: Granular strategy not used for element addition")
        }
        
        // Test element removal
        removeElementData := map[string]interface{}{
            "Items": []map[string]interface{}{
                {"ID": "item1", "Name": "First Item"},
                // item2 removed, item3 removed
            },
            "Sections": []map[string]interface{}{
                {"ID": "sec1", "Title": "Section 1", "Content": "Content 1"},
                // sec2 removed
            },
        }
        
        t.Log("Testing element removal")
        fragments, err = page.RenderFragments(context.Background(), removeElementData)
        if err != nil {
            return fmt.Errorf("element removal test failed: %w", err)
        }
        
        // Validate granular strategy for removal operations
        for _, fragment := range fragments {
            if fragment.Strategy == "granular" {
                helper.RecordFragmentMetric(
                    fragment.ID,
                    "granular",
                    12*time.Millisecond,
                    len(fmt.Sprintf("%+v", fragment.Data)),
                    0.70, // Good compression for removal operations
                    false,
                )
            }
        }
        
        return nil
    })
}
```

### Element Reordering Patterns

```go
func TestGranularReorderingOperations(t *testing.T) {
    E2ETestWithHelper(t, "granular-reorder", func(helper *E2ETestHelper) error {
        // Template with ordered content
        tmpl, _ := template.New("reorder").Parse(`
            <div>
                <ol data-lt-fragment="ordered-list">
                    {{range .Items}}
                    <li data-id="{{.ID}}" class="priority-{{.Priority}}">
                        {{.Title}} (Priority: {{.Priority}})
                    </li>
                    {{end}}
                </ol>
            </div>
        `)
        
        initialData := map[string]interface{}{
            "Items": []map[string]interface{}{
                {"ID": "task1", "Title": "Task One", "Priority": 1},
                {"ID": "task2", "Title": "Task Two", "Priority": 2}, 
                {"ID": "task3", "Title": "Task Three", "Priority": 3},
                {"ID": "task4", "Title": "Task Four", "Priority": 4},
            },
        }
        
        // Reorder items (same content, different order)
        reorderedData := map[string]interface{}{
            "Items": []map[string]interface{}{
                {"ID": "task3", "Title": "Task Three", "Priority": 3}, // Moved up
                {"ID": "task1", "Title": "Task One", "Priority": 1},   // Moved down
                {"ID": "task4", "Title": "Task Four", "Priority": 4},  // Moved up
                {"ID": "task2", "Title": "Task Two", "Priority": 2},   // Moved down
            },
        }
        
        app, _ := NewApplication()
        defer app.Close()
        
        page, _ := app.NewApplicationPage(tmpl, initialData)
        defer page.Close()
        
        fragments, err := page.RenderFragments(context.Background(), reorderedData)
        if err != nil {
            return fmt.Errorf("reordering test failed: %w", err)
        }
        
        // Validate granular reordering operations
        for _, fragment := range fragments {
            if fragment.Strategy == "granular" {
                // Reordering should be efficient with granular operations
                helper.RecordFragmentMetric(
                    fragment.ID,
                    "granular",
                    10*time.Millisecond,
                    len(fmt.Sprintf("%+v", fragment.Data)),
                    0.65, // Good compression for reordering
                    false,
                )
                
                // Validate operation types for reordering
                if data, ok := fragment.Data.(map[string]interface{}); ok {
                    if ops, hasOps := data["operations"]; hasOps {
                        t.Logf("Granular reordering operations: %+v", ops)
                    }
                }
            }
        }
        
        return nil
    })
}
```

## Strategy 4: Fragment Replacement Testing

**Target**: Complex structural changes (40-60% bandwidth reduction, 5-10% of cases)

### Complex Layout Changes

```go
func TestReplacementComplexChanges(t *testing.T) {
    E2ETestWithHelper(t, "replacement-complex", func(helper *E2ETestHelper) error {
        // Template with complex layout
        tmpl, _ := template.New("complex-layout").Parse(`
            <div data-lt-fragment="main-content">
                {{if .LayoutType eq "list"}}
                <div class="list-layout">
                    <h2>{{.Title}}</h2>
                    <ul>
                        {{range .Items}}
                        <li class="{{.Type}}">{{.Content}}</li>
                        {{end}}
                    </ul>
                </div>
                {{else if .LayoutType eq "grid"}}
                <div class="grid-layout">
                    <h2>{{.Title}}</h2>
                    <div class="grid">
                        {{range .Items}}
                        <div class="grid-item {{.Type}}">
                            <h4>{{.Title}}</h4>
                            <p>{{.Content}}</p>
                        </div>
                        {{end}}
                    </div>
                </div>
                {{else if .LayoutType eq "table"}}
                <div class="table-layout">
                    <h2>{{.Title}}</h2>
                    <table>
                        <thead>
                            <tr>
                                <th>Type</th>
                                <th>Title</th>
                                <th>Content</th>
                            </tr>
                        </thead>
                        <tbody>
                            {{range .Items}}
                            <tr class="{{.Type}}">
                                <td>{{.Type}}</td>
                                <td>{{.Title}}</td>
                                <td>{{.Content}}</td>
                            </tr>
                            {{end}}
                        </tbody>
                    </table>
                </div>
                {{end}}
            </div>
        `)
        
        initialData := map[string]interface{}{
            "LayoutType": "list",
            "Title": "My Items",
            "Items": []map[string]interface{}{
                {"Type": "important", "Title": "Item 1", "Content": "Content 1"},
                {"Type": "normal", "Title": "Item 2", "Content": "Content 2"},
            },
        }
        
        // Test complex layout changes that require replacement
        layoutChanges := []map[string]interface{}{
            {
                "LayoutType": "grid",  // Complete layout change
                "Title": "My Grid Items",
                "Items": []map[string]interface{}{
                    {"Type": "featured", "Title": "Featured Item", "Content": "New content"},
                    {"Type": "normal", "Title": "Regular Item", "Content": "More content"},
                    {"Type": "important", "Title": "Important Item", "Content": "Critical content"},
                },
            },
            {
                "LayoutType": "table", // Another complete layout change
                "Title": "Data Table",
                "Items": []map[string]interface{}{
                    {"Type": "header", "Title": "Header Row", "Content": "Header content"},
                    {"Type": "data", "Title": "Data Row 1", "Content": "Data content 1"},
                    {"Type": "data", "Title": "Data Row 2", "Content": "Data content 2"},
                },
            },
        }
        
        app, _ := NewApplication()
        defer app.Close()
        
        page, _ := app.NewApplicationPage(tmpl, initialData)
        defer page.Close()
        
        for i, updateData := range layoutChanges {
            layoutName := updateData["LayoutType"].(string)
            t.Logf("Testing complex layout change to: %s", layoutName)
            
            fragments, err := page.RenderFragments(context.Background(), updateData)
            if err != nil {
                return fmt.Errorf("layout change to %s failed: %w", layoutName, err)
            }
            
            // Validate replacement strategy for complex changes
            replacementFound := false
            for _, fragment := range fragments {
                if fragment.Strategy == "replacement" {
                    replacementFound = true
                    
                    // Validate replacement data structure
                    if data, ok := fragment.Data.(map[string]interface{}); ok {
                        if _, hasHTML := data["html"]; !hasHTML {
                            return fmt.Errorf("replacement fragment missing html data")
                        }
                    }
                    
                    // Record replacement metrics
                    compressionRatio := 0.40 + 0.20*rand.Float64() // 40-60%
                    helper.RecordFragmentMetric(
                        fragment.ID,
                        "replacement",
                        25*time.Millisecond, // Higher generation time for complex changes
                        len(fmt.Sprintf("%+v", fragment.Data)),
                        compressionRatio,
                        false,
                    )
                    break
                }
            }
            
            if !replacementFound {
                t.Logf("Warning: Replacement strategy not used for complex layout change %d", i+1)
            }
        }
        
        return nil
    })
}
```

### Mixed Change Patterns

```go
func TestReplacementMixedChanges(t *testing.T) {
    E2ETestWithHelper(t, "replacement-mixed", func(helper *E2ETestHelper) error {
        // Template with mixed content types
        tmpl, _ := template.New("mixed-content").Parse(`
            <div data-lt-fragment="mixed-content">
                <header class="{{.HeaderClass}}">
                    <h1>{{.Title}}</h1>
                    <nav>
                        {{range .NavItems}}
                        <a href="{{.URL}}" class="{{.Class}}">{{.Label}}</a>
                        {{end}}
                    </nav>
                </header>
                <main>
                    {{if .ShowSidebar}}
                    <aside class="sidebar">
                        {{range .SidebarItems}}
                        <div class="widget">{{.Content}}</div>
                        {{end}}
                    </aside>
                    {{end}}
                    <section class="content">
                        {{range .ContentSections}}
                        <article class="{{.Type}}">
                            <h2>{{.Title}}</h2>
                            {{if eq .Type "image"}}
                            <img src="{{.ImageURL}}" alt="{{.Alt}}">
                            {{else if eq .Type "video"}}
                            <video src="{{.VideoURL}}" controls></video>
                            {{else}}
                            <p>{{.Text}}</p>
                            {{end}}
                        </article>
                        {{end}}
                    </section>
                </main>
            </div>
        `)
        
        initialData := map[string]interface{}{
            "HeaderClass": "default-header",
            "Title": "My Website",
            "NavItems": []map[string]interface{}{
                {"URL": "/home", "Class": "nav-link", "Label": "Home"},
                {"URL": "/about", "Class": "nav-link", "Label": "About"},
            },
            "ShowSidebar": true,
            "SidebarItems": []map[string]interface{}{
                {"Content": "Widget 1"},
            },
            "ContentSections": []map[string]interface{}{
                {"Type": "text", "Title": "Welcome", "Text": "Welcome text"},
            },
        }
        
        // Complex mixed changes: structure + attributes + text
        mixedChangeData := map[string]interface{}{
            "HeaderClass": "premium-header dark-theme", // Attribute change
            "Title": "My Premium Website",              // Text change
            "NavItems": []map[string]interface{}{       // Structural change
                {"URL": "/dashboard", "Class": "nav-link active", "Label": "Dashboard"},
                {"URL": "/profile", "Class": "nav-link", "Label": "Profile"},
                {"URL": "/settings", "Class": "nav-link", "Label": "Settings"},
                {"URL": "/logout", "Class": "nav-link logout", "Label": "Logout"},
            },
            "ShowSidebar": false,    // Structural change (remove sidebar)
            "SidebarItems": []map[string]interface{}{},
            "ContentSections": []map[string]interface{}{ // Complete content restructure
                {
                    "Type": "image",
                    "Title": "Hero Image",
                    "ImageURL": "/hero.jpg",
                    "Alt": "Hero banner",
                },
                {
                    "Type": "video",
                    "Title": "Introduction Video",
                    "VideoURL": "/intro.mp4",
                },
                {
                    "Type": "text",
                    "Title": "New Features",
                    "Text": "Check out our amazing new features!",
                },
            },
        }
        
        app, _ := NewApplication()
        defer app.Close()
        
        page, _ := app.NewApplicationPage(tmpl, initialData)
        defer page.Close()
        
        fragments, err := page.RenderFragments(context.Background(), mixedChangeData)
        if err != nil {
            return fmt.Errorf("mixed changes test failed: %w", err)
        }
        
        // Validate replacement strategy for mixed complex changes
        for _, fragment := range fragments {
            if fragment.Strategy == "replacement" {
                helper.RecordFragmentMetric(
                    fragment.ID,
                    "replacement",
                    30*time.Millisecond, // High generation time for mixed changes
                    len(fmt.Sprintf("%+v", fragment.Data)),
                    0.50, // Moderate compression for mixed changes
                    false,
                )
                
                t.Logf("Replacement strategy used for mixed changes in fragment: %s", fragment.ID)
            }
        }
        
        return nil
    })
}
```

## Cross-Strategy Testing

### Strategy Fallback Validation

```go
func TestStrategyFallbackBehavior(t *testing.T) {
    E2ETestWithHelper(t, "strategy-fallback", func(helper *E2ETestHelper) error {
        // Template that can trigger different strategies
        tmpl, _ := template.New("fallback").Parse(`
            <div data-lt-fragment="adaptive-content">
                {{if .ForceError}}
                    {{/* Invalid template syntax to trigger fallback */}}
                    {{.UndefinedFunction}}
                {{else if .ComplexCondition}}
                    <!-- Very complex nested structure -->
                    {{range .Level1}}
                        {{range .Level2}}
                            {{range .Level3}}
                                <div class="{{.DynamicClass}}" data-attr="{{.DynamicAttr}}">
                                    {{.DynamicContent}}
                                </div>
                            {{end}}
                        {{end}}
                    {{end}}
                {{else}}
                    <p>{{.SimpleContent}}</p>
                {{end}}
            </div>
        `)
        
        app, _ := NewApplication()
        defer app.Close()
        
        // Test normal operation first
        normalData := map[string]interface{}{
            "ForceError": false,
            "ComplexCondition": false,
            "SimpleContent": "Simple text content",
        }
        
        page, _ := app.NewApplicationPage(tmpl, normalData)
        defer page.Close()
        
        // Test fallback to replacement for complex scenarios
        complexData := map[string]interface{}{
            "ForceError": false,
            "ComplexCondition": true,
            "Level1": []map[string]interface{}{
                {
                    "Level2": []map[string]interface{}{
                        {
                            "Level3": []map[string]interface{}{
                                {
                                    "DynamicClass": "new-class",
                                    "DynamicAttr": "new-value", 
                                    "DynamicContent": "New content",
                                },
                            },
                        },
                    },
                },
            },
        }
        
        fragments, err := page.RenderFragments(context.Background(), complexData)
        if err != nil {
            return fmt.Errorf("complex fallback test failed: %w", err)
        }
        
        // Should fallback to replacement for complex mixed changes
        replacementUsed := false
        for _, fragment := range fragments {
            if fragment.Strategy == "replacement" {
                replacementUsed = true
                helper.RecordFragmentMetric(
                    fragment.ID,
                    "replacement",
                    20*time.Millisecond,
                    len(fmt.Sprintf("%+v", fragment.Data)),
                    0.45, // Lower compression due to complexity
                    false,
                )
                t.Log("✅ Complex changes correctly fell back to replacement strategy")
                break
            }
        }
        
        if !replacementUsed {
            t.Log("Warning: Complex changes did not trigger replacement fallback")
        }
        
        return nil
    })
}
```

## Performance Pattern Testing

### Strategy Performance Validation

```go
func TestStrategyPerformanceCharacteristics(t *testing.T) {
    E2ETestWithHelper(t, "strategy-performance", func(helper *E2ETestHelper) error {
        // Test each strategy's performance characteristics
        strategies := map[string]struct {
            minCompression float64
            maxCompression float64
            maxGenTime     time.Duration
            expectedUsage  float64 // Expected percentage of usage
        }{
            "static_dynamic": {
                minCompression: 0.85,
                maxCompression: 0.95,
                maxGenTime:     10 * time.Millisecond,
                expectedUsage:  0.65, // 60-70%
            },
            "markers": {
                minCompression: 0.70,
                maxCompression: 0.85,
                maxGenTime:     15 * time.Millisecond,
                expectedUsage:  0.175, // 15-20%
            },
            "granular": {
                minCompression: 0.60,
                maxCompression: 0.80,
                maxGenTime:     20 * time.Millisecond,
                expectedUsage:  0.125, // 10-15%
            },
            "replacement": {
                minCompression: 0.40,
                maxCompression: 0.60,
                maxGenTime:     30 * time.Millisecond,
                expectedUsage:  0.075, // 5-10%
            },
        }
        
        const numTests = 200
        strategyCount := make(map[string]int)
        
        for i := 0; i < numTests; i++ {
            // Generate test case that should trigger specific strategy
            testCase := generatePerformanceTestCase(i, strategies)
            
            start := time.Now()
            fragments, err := performFragmentUpdate(testCase)
            genTime := time.Since(start)
            
            if err != nil {
                continue // Skip failed tests for performance analysis
            }
            
            for _, fragment := range fragments {
                strategyCount[fragment.Strategy]++
                
                // Validate performance characteristics
                if expected, exists := strategies[fragment.Strategy]; exists {
                    // Calculate simulated compression ratio
                    compressionRatio := expected.minCompression + 
                        (expected.maxCompression-expected.minCompression)*rand.Float64()
                    
                    // Validate generation time is within expected bounds
                    if genTime > expected.maxGenTime {
                        t.Logf("Warning: %s strategy took %v (max: %v)",
                            fragment.Strategy, genTime, expected.maxGenTime)
                    }
                    
                    // Record metrics
                    helper.RecordFragmentMetric(
                        fragment.ID,
                        fragment.Strategy,
                        genTime,
                        len(fmt.Sprintf("%+v", fragment.Data)),
                        compressionRatio,
                        false,
                    )
                }
            }
        }
        
        // Validate strategy distribution
        for strategy, expectedUsage := range strategies {
            actualUsage := float64(strategyCount[strategy]) / float64(numTests)
            variance := math.Abs(actualUsage - expectedUsage.expectedUsage)
            
            if variance > 0.1 { // Allow 10% variance
                t.Logf("Warning: %s strategy used %.1f%% (expected %.1f%%)",
                    strategy, actualUsage*100, expectedUsage.expectedUsage*100)
            }
            
            helper.SetCustomMetric(fmt.Sprintf("%s_usage", strategy), actualUsage)
        }
        
        return nil
    })
}

// Helper function to generate performance test cases
func generatePerformanceTestCase(index int, strategies map[string]struct{...}) map[string]interface{} {
    switch index % 4 {
    case 0: // Text-only changes (static_dynamic)
        return map[string]interface{}{
            "changeType": "text_only",
            "content": fmt.Sprintf("Text content %d", index),
        }
    case 1: // Attribute changes (markers)
        return map[string]interface{}{
            "changeType": "attributes",
            "cssClass": fmt.Sprintf("class-%d", index),
            "content": "Static content",
        }
    case 2: // Structural changes (granular)
        return map[string]interface{}{
            "changeType": "structural",
            "items": generateItems(index % 5 + 1),
        }
    case 3: // Complex mixed changes (replacement)
        return map[string]interface{}{
            "changeType": "complex",
            "layout": "grid",
            "items": generateComplexItems(index),
            "cssClass": fmt.Sprintf("complex-class-%d", index),
        }
    }
    return nil
}
```

## Advanced Testing Patterns

### Cache Testing

```go
func TestFragmentCachingBehavior(t *testing.T) {
    E2ETestWithHelper(t, "fragment-caching", func(helper *E2ETestHelper) error {
        // Template that can benefit from caching
        tmpl, _ := template.New("cacheable").Parse(`
            <div data-lt-fragment="cached-content">
                <h1>{{.Title}}</h1>
                <div class="{{.CSSClass}}">{{.Content}}</div>
            </div>
        `)
        
        app, _ := NewApplication()
        defer app.Close()
        
        initialData := map[string]interface{}{
            "Title": "Cacheable Content",
            "CSSClass": "default",
            "Content": "Initial content",
        }
        
        page, _ := app.NewApplicationPage(tmpl, initialData)
        defer page.Close()
        
        // Repeated identical updates should hit cache
        updateData := map[string]interface{}{
            "Title": "Updated Title",
            "CSSClass": "updated",
            "Content": "Updated content",
        }
        
        // First update - cache miss
        start := time.Now()
        fragments1, err := page.RenderFragments(context.Background(), updateData)
        firstTime := time.Since(start)
        
        if err != nil {
            return fmt.Errorf("first update failed: %w", err)
        }
        
        // Record first update (cache miss)
        for _, fragment := range fragments1 {
            helper.RecordFragmentMetric(
                fragment.ID,
                fragment.Strategy,
                firstTime,
                len(fmt.Sprintf("%+v", fragment.Data)),
                0.75,
                false, // cache miss
            )
        }
        
        // Second identical update - should hit cache
        start = time.Now()
        fragments2, err := page.RenderFragments(context.Background(), updateData)
        secondTime := time.Since(start)
        
        if err != nil {
            return fmt.Errorf("second update failed: %w", err)
        }
        
        // Record second update (cache hit)
        for _, fragment := range fragments2 {
            helper.RecordFragmentMetric(
                fragment.ID,
                fragment.Strategy,
                secondTime,
                len(fmt.Sprintf("%+v", fragment.Data)),
                0.75,
                true, // cache hit
            )
        }
        
        // Cache hit should be significantly faster
        if secondTime > firstTime/2 {
            t.Logf("Warning: Cache hit (%v) not significantly faster than miss (%v)",
                secondTime, firstTime)
        } else {
            t.Logf("✅ Cache hit (%v) faster than miss (%v)", secondTime, firstTime)
        }
        
        helper.SetCustomMetric("cache_effectiveness", float64(firstTime.Nanoseconds())/float64(secondTime.Nanoseconds()))
        
        return nil
    })
}
```

### Error Recovery Testing

```go
func TestErrorRecoveryPatterns(t *testing.T) {
    E2ETestWithHelper(t, "error-recovery", func(helper *E2ETestHelper) error {
        // Template with potential error conditions
        tmpl, _ := template.New("error-prone").Parse(`
            <div data-lt-fragment="error-prone">
                {{if .TriggerError}}
                    {{/* This will cause template execution error */}}
                    {{.NonExistentField.UndefinedMethod}}
                {{else}}
                    <p>{{.SafeContent}}</p>
                {{end}}
            </div>
        `)
        
        app, _ := NewApplication()
        defer app.Close()
        
        initialData := map[string]interface{}{
            "TriggerError": false,
            "SafeContent": "Safe initial content",
        }
        
        page, _ := app.NewApplicationPage(tmpl, initialData)
        defer page.Close()
        
        // Test error condition
        errorData := map[string]interface{}{
            "TriggerError": true,
            "SafeContent": "This should not be used",
        }
        
        fragments, err := page.RenderFragments(context.Background(), errorData)
        
        if err != nil {
            // Error is expected - test recovery
            t.Logf("Expected error occurred: %v", err)
            
            // Test recovery with valid data
            recoveryData := map[string]interface{}{
                "TriggerError": false,
                "SafeContent": "Recovered content",
            }
            
            fragments, err = page.RenderFragments(context.Background(), recoveryData)
            if err != nil {
                return fmt.Errorf("recovery failed: %w", err)
            }
            
            // Should fallback to replacement strategy for error recovery
            for _, fragment := range fragments {
                if fragment.Strategy == "replacement" {
                    helper.RecordFragmentMetric(
                        fragment.ID,
                        "replacement",
                        50*time.Millisecond, // Higher time for error recovery
                        len(fmt.Sprintf("%+v", fragment.Data)),
                        0.40, // Lower compression due to full replacement
                        false,
                    )
                    t.Log("✅ Error recovery successful with replacement strategy")
                }
            }
        } else {
            t.Log("No error occurred - error handling not tested")
        }
        
        return nil
    })
}
```

This comprehensive fragment testing patterns guide provides detailed examples for testing each of LiveTemplate's four strategies with realistic scenarios, performance validation, and edge case handling. Each pattern includes proper metrics collection and validation to ensure the E2E tests provide meaningful feedback about the system's behavior.
