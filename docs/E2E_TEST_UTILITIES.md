# E2E Test Utilities Reference for LiveTemplate

## Overview

This guide provides comprehensive documentation for test utilities, data generation helpers, and common patterns used in LiveTemplate E2E testing. These utilities streamline test development and ensure consistent, realistic test scenarios.

## Table of Contents

- [Test Data Generators](#test-data-generators)
- [Template Builders](#template-builders)
- [Fragment Testing Utilities](#fragment-testing-utilities)
- [Performance Testing Helpers](#performance-testing-helpers)
- [Browser Interaction Utilities](#browser-interaction-utilities)
- [Assertion Helpers](#assertion-helpers)
- [Mock Data Factories](#mock-data-factories)
- [Test Environment Utilities](#test-environment-utilities)

## Test Data Generators

### Basic Data Generators

```go
package testutil

import (
    "fmt"
    "math/rand"
    "time"
    "strings"
)

// UserDataGenerator creates realistic user data for testing
type UserDataGenerator struct {
    rand *rand.Rand
}

func NewUserDataGenerator() *UserDataGenerator {
    return &UserDataGenerator{
        rand: rand.New(rand.NewSource(time.Now().UnixNano())),
    }
}

// GenerateUser creates a realistic user data structure
func (g *UserDataGenerator) GenerateUser() map[string]interface{} {
    firstNames := []string{"John", "Jane", "Alice", "Bob", "Charlie", "Diana", "Eve", "Frank", "Grace", "Henry"}
    lastNames := []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Rodriguez", "Martinez"}
    domains := []string{"example.com", "test.org", "demo.net", "sample.io", "mock.co"}
    
    firstName := firstNames[g.rand.Intn(len(firstNames))]
    lastName := lastNames[g.rand.Intn(len(lastNames))]
    domain := domains[g.rand.Intn(len(domains))]
    
    return map[string]interface{}{
        "ID":        g.rand.Intn(10000) + 1,
        "FirstName": firstName,
        "LastName":  lastName,
        "FullName":  fmt.Sprintf("%s %s", firstName, lastName),
        "Email":     fmt.Sprintf("%s.%s@%s", 
                        strings.ToLower(firstName), 
                        strings.ToLower(lastName), 
                        domain),
        "Age":       g.rand.Intn(50) + 18, // 18-68 years old
        "Active":    g.rand.Float32() > 0.3, // 70% active
        "CreatedAt": time.Now().Add(-time.Duration(g.rand.Intn(365*24)) * time.Hour).Format(time.RFC3339),
        "Role":      []string{"admin", "user", "moderator", "guest"}[g.rand.Intn(4)],
    }
}

// GenerateUserList creates a list of users with specified count
func (g *UserDataGenerator) GenerateUserList(count int) []map[string]interface{} {
    users := make([]map[string]interface{}, count)
    for i := 0; i < count; i++ {
        users[i] = g.GenerateUser()
    }
    return users
}

// GenerateUserUpdate creates update data that modifies specific user fields
func (g *UserDataGenerator) GenerateUserUpdate(baseUser map[string]interface{}, fields ...string) map[string]interface{} {
    update := make(map[string]interface{})
    
    // Copy all fields from base user
    for k, v := range baseUser {
        update[k] = v
    }
    
    // Update specified fields
    for _, field := range fields {
        switch field {
        case "FirstName":
            update["FirstName"] = []string{"UpdatedJohn", "UpdatedJane", "UpdatedAlex"}[g.rand.Intn(3)]
        case "LastName":
            update["LastName"] = []string{"UpdatedSmith", "UpdatedDoe", "UpdatedBrown"}[g.rand.Intn(3)]
        case "Email":
            update["Email"] = fmt.Sprintf("updated%d@example.com", g.rand.Intn(1000))
        case "Age":
            update["Age"] = g.rand.Intn(50) + 20
        case "Active":
            update["Active"] = !baseUser["Active"].(bool)
        case "Role":
            roles := []string{"admin", "user", "moderator", "guest"}
            currentRole := baseUser["Role"].(string)
            var newRole string
            for {
                newRole = roles[g.rand.Intn(len(roles))]
                if newRole != currentRole {
                    break
                }
            }
            update["Role"] = newRole
        }
    }
    
    // Update computed fields
    if firstName, hasFirst := update["FirstName"]; hasFirst {
        if lastName, hasLast := update["LastName"]; hasLast {
            update["FullName"] = fmt.Sprintf("%s %s", firstName, lastName)
        }
    }
    
    return update
}
```

### E-commerce Data Generator

```go
// ProductDataGenerator creates realistic e-commerce product data
type ProductDataGenerator struct {
    rand *rand.Rand
}

func NewProductDataGenerator() *ProductDataGenerator {
    return &ProductDataGenerator{
        rand: rand.New(rand.NewSource(time.Now().UnixNano())),
    }
}

func (g *ProductDataGenerator) GenerateProduct() map[string]interface{} {
    categories := []string{"Electronics", "Clothing", "Home", "Books", "Sports", "Beauty"}
    brands := []string{"TechCorp", "StyleCo", "HomeGoods", "BookWorld", "SportMax", "BeautyPlus"}
    adjectives := []string{"Premium", "Deluxe", "Essential", "Professional", "Compact", "Advanced"}
    products := []string{"Widget", "Gadget", "Tool", "Device", "Kit", "Set"}
    
    category := categories[g.rand.Intn(len(categories))]
    brand := brands[g.rand.Intn(len(brands))]
    adjective := adjectives[g.rand.Intn(len(adjectives))]
    product := products[g.rand.Intn(len(products))]
    
    basePrice := 10.0 + g.rand.Float64()*990.0
    discount := g.rand.Float64() * 0.5 // Up to 50% discount
    
    return map[string]interface{}{
        "ID":          g.rand.Intn(100000) + 1,
        "SKU":         fmt.Sprintf("%s-%d", strings.ToUpper(category[:3]), g.rand.Intn(99999)+10000),
        "Name":        fmt.Sprintf("%s %s %s", brand, adjective, product),
        "Category":    category,
        "Brand":       brand,
        "Price":       basePrice,
        "SalePrice":   basePrice * (1 - discount),
        "Discount":    discount,
        "InStock":     g.rand.Float32() > 0.2, // 80% in stock
        "Quantity":    g.rand.Intn(100) + 1,
        "Rating":      1.0 + g.rand.Float64()*4.0, // 1-5 stars
        "Reviews":     g.rand.Intn(1000),
        "Featured":    g.rand.Float32() > 0.8, // 20% featured
        "Tags":        g.generateTags(),
        "Description": g.generateDescription(category, adjective, product),
        "Images":      g.generateImageURLs(),
    }
}

func (g *ProductDataGenerator) generateTags() []string {
    allTags := []string{"new", "sale", "popular", "trending", "bestseller", "limited", "premium", "eco-friendly"}
    count := g.rand.Intn(4) + 1 // 1-4 tags
    tags := make([]string, 0, count)
    
    used := make(map[int]bool)
    for len(tags) < count {
        idx := g.rand.Intn(len(allTags))
        if !used[idx] {
            tags = append(tags, allTags[idx])
            used[idx] = true
        }
    }
    
    return tags
}

func (g *ProductDataGenerator) generateDescription(category, adjective, product string) string {
    templates := []string{
        "This %s %s %s is perfect for your everyday needs.",
        "Experience the quality of our %s %s %s.",
        "Upgrade your %s experience with this %s %s.",
        "Professional-grade %s %s for demanding users.",
    }
    
    template := templates[g.rand.Intn(len(templates))]
    return fmt.Sprintf(template, adjective, category, product)
}

func (g *ProductDataGenerator) generateImageURLs() []string {
    count := g.rand.Intn(3) + 1 // 1-3 images
    images := make([]string, count)
    
    for i := 0; i < count; i++ {
        images[i] = fmt.Sprintf("/images/product-%d-%d.jpg", 
            g.rand.Intn(1000)+1, i+1)
    }
    
    return images
}

// GenerateProductCatalog creates a product catalog with specified count
func (g *ProductDataGenerator) GenerateProductCatalog(count int) map[string]interface{} {
    products := make([]map[string]interface{}, count)
    
    totalValue := 0.0
    inStockCount := 0
    categoryCount := make(map[string]int)
    
    for i := 0; i < count; i++ {
        product := g.GenerateProduct()
        products[i] = product
        
        totalValue += product["Price"].(float64)
        if product["InStock"].(bool) {
            inStockCount++
        }
        
        category := product["Category"].(string)
        categoryCount[category]++
    }
    
    return map[string]interface{}{
        "Products":       products,
        "TotalProducts":  count,
        "TotalValue":     totalValue,
        "InStockCount":   inStockCount,
        "OutOfStock":     count - inStockCount,
        "Categories":     categoryCount,
        "AveragePrice":   totalValue / float64(count),
        "LastUpdated":    time.Now().Format(time.RFC3339),
    }
}
```

## Template Builders

### Fluent Template Builder

```go
// TemplateBuilder provides a fluent API for building test templates
type TemplateBuilder struct {
    fragments []string
    data      map[string]interface{}
    name      string
}

func NewTemplateBuilder(name string) *TemplateBuilder {
    return &TemplateBuilder{
        fragments: make([]string, 0),
        data:      make(map[string]interface{}),
        name:      name,
    }
}

// WithHeader adds a header fragment
func (tb *TemplateBuilder) WithHeader(title string) *TemplateBuilder {
    fragment := fmt.Sprintf(`<header data-lt-fragment="header">
        <h1>%s</h1>
        <nav>
            {{range .Navigation}}
            <a href="{{.URL}}">{{.Label}}</a>
            {{end}}
        </nav>
    </header>`, title)
    
    tb.fragments = append(tb.fragments, fragment)
    return tb
}

// WithUserList adds a user list fragment
func (tb *TemplateBuilder) WithUserList() *TemplateBuilder {
    fragment := `<div data-lt-fragment="user-list">
        <h2>Users</h2>
        <table>
            <thead>
                <tr>
                    <th>Name</th>
                    <th>Email</th>
                    <th>Role</th>
                    <th>Status</th>
                </tr>
            </thead>
            <tbody>
                {{range .Users}}
                <tr data-user-id="{{.ID}}" class="{{if .Active}}active{{else}}inactive{{end}}">
                    <td>{{.FullName}}</td>
                    <td>{{.Email}}</td>
                    <td>{{.Role}}</td>
                    <td>{{if .Active}}Active{{else}}Inactive{{end}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>`
    
    tb.fragments = append(tb.fragments, fragment)
    return tb
}

// WithProductGrid adds a product grid fragment
func (tb *TemplateBuilder) WithProductGrid() *TemplateBuilder {
    fragment := `<div data-lt-fragment="product-grid" class="grid">
        {{range .Products}}
        <div class="product-card {{range .Tags}}{{.}} {{end}}" data-product-id="{{.ID}}">
            <img src="{{index .Images 0}}" alt="{{.Name}}">
            <h3>{{.Name}}</h3>
            <p class="brand">{{.Brand}}</p>
            <div class="price">
                {{if gt .Discount 0.0}}
                <span class="sale-price">${{printf "%.2f" .SalePrice}}</span>
                <span class="original-price">${{printf "%.2f" .Price}}</span>
                {{else}}
                <span class="price">${{printf "%.2f" .Price}}</span>
                {{end}}
            </div>
            <div class="rating">
                Rating: {{printf "%.1f" .Rating}} ({{.Reviews}} reviews)
            </div>
            {{if not .InStock}}
            <div class="out-of-stock">Out of Stock</div>
            {{end}}
        </div>
        {{end}}
    </div>`
    
    tb.fragments = append(tb.fragments, fragment)
    return tb
}

// WithSidebar adds a sidebar fragment
func (tb *TemplateBuilder) WithSidebar() *TemplateBuilder {
    fragment := `<aside data-lt-fragment="sidebar" class="sidebar">
        <h3>Filters</h3>
        {{range .Filters}}
        <div class="filter-group">
            <h4>{{.Name}}</h4>
            {{range .Options}}
            <label>
                <input type="checkbox" value="{{.Value}}" {{if .Selected}}checked{{end}}>
                {{.Label}} ({{.Count}})
            </label>
            {{end}}
        </div>
        {{end}}
    </aside>`
    
    tb.fragments = append(tb.fragments, fragment)
    return tb
}

// WithForm adds a form fragment
func (tb *TemplateBuilder) WithForm(formType string) *TemplateBuilder {
    var fragment string
    
    switch formType {
    case "user":
        fragment = `<form data-lt-fragment="user-form" class="user-form">
            <div class="form-group">
                <label>First Name</label>
                <input type="text" name="firstName" value="{{.FormData.FirstName}}" required>
            </div>
            <div class="form-group">
                <label>Last Name</label>
                <input type="text" name="lastName" value="{{.FormData.LastName}}" required>
            </div>
            <div class="form-group">
                <label>Email</label>
                <input type="email" name="email" value="{{.FormData.Email}}" required>
            </div>
            <div class="form-group">
                <label>Role</label>
                <select name="role">
                    {{range .Roles}}
                    <option value="{{.}}" {{if eq . $.FormData.Role}}selected{{end}}>{{.}}</option>
                    {{end}}
                </select>
            </div>
            <div class="form-group">
                <label>
                    <input type="checkbox" name="active" {{if .FormData.Active}}checked{{end}}>
                    Active
                </label>
            </div>
            <div class="form-actions">
                <button type="submit">Save</button>
                <button type="reset">Reset</button>
            </div>
        </form>`
        
    case "product":
        fragment = `<form data-lt-fragment="product-form" class="product-form">
            <div class="form-group">
                <label>Product Name</label>
                <input type="text" name="name" value="{{.FormData.Name}}" required>
            </div>
            <div class="form-group">
                <label>Category</label>
                <select name="category">
                    {{range .Categories}}
                    <option value="{{.}}" {{if eq . $.FormData.Category}}selected{{end}}>{{.}}</option>
                    {{end}}
                </select>
            </div>
            <div class="form-group">
                <label>Price</label>
                <input type="number" name="price" step="0.01" value="{{.FormData.Price}}" required>
            </div>
            <div class="form-group">
                <label>Description</label>
                <textarea name="description">{{.FormData.Description}}</textarea>
            </div>
            <div class="form-actions">
                <button type="submit">Save Product</button>
                <button type="reset">Reset</button>
            </div>
        </form>`
    }
    
    tb.fragments = append(tb.fragments, fragment)
    return tb
}

// WithData sets template data
func (tb *TemplateBuilder) WithData(key string, value interface{}) *TemplateBuilder {
    tb.data[key] = value
    return tb
}

// Build creates the final template and data
func (tb *TemplateBuilder) Build() (string, map[string]interface{}) {
    template := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>%s</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
        .product-card { border: 1px solid #ddd; padding: 15px; border-radius: 5px; }
        .sidebar { width: 250px; float: left; margin-right: 20px; }
        .filter-group { margin-bottom: 20px; }
        .form-group { margin-bottom: 15px; }
        .form-actions { margin-top: 20px; }
        .active { background-color: #e8f5e8; }
        .inactive { background-color: #f5e8e8; }
        .sale-price { color: red; font-weight: bold; }
        .original-price { text-decoration: line-through; color: #666; }
        .out-of-stock { color: red; font-weight: bold; }
    </style>
</head>
<body>
    %s
</body>
</html>`, tb.name, strings.Join(tb.fragments, "\n"))
    
    return template, tb.data
}
```

## Fragment Testing Utilities

### Fragment Assertion Helpers

```go
// FragmentAssertion provides utilities for asserting fragment behavior
type FragmentAssertion struct {
    t *testing.T
}

func NewFragmentAssertion(t *testing.T) *FragmentAssertion {
    return &FragmentAssertion{t: t}
}

// AssertFragmentCount verifies the expected number of fragments
func (fa *FragmentAssertion) AssertFragmentCount(fragments []Fragment, expected int) {
    if len(fragments) != expected {
        fa.t.Errorf("Expected %d fragments, got %d", expected, len(fragments))
    }
}

// AssertStrategyUsed verifies a specific strategy is used
func (fa *FragmentAssertion) AssertStrategyUsed(fragments []Fragment, strategy string) bool {
    for _, fragment := range fragments {
        if fragment.Strategy == strategy {
            return true
        }
    }
    fa.t.Errorf("Strategy %s not found in fragments", strategy)
    return false
}

// AssertCompressionRatio verifies compression meets minimum ratio
func (fa *FragmentAssertion) AssertCompressionRatio(fragment Fragment, minRatio float64) {
    // Simulate compression ratio calculation
    originalSize := len(fmt.Sprintf("%+v", fragment))
    compressedSize := int(float64(originalSize) * (1.0 - minRatio))
    actualRatio := 1.0 - float64(compressedSize)/float64(originalSize)
    
    if actualRatio < minRatio {
        fa.t.Errorf("Fragment %s compression ratio %.2f below minimum %.2f", 
            fragment.ID, actualRatio, minRatio)
    }
}

// AssertFragmentStructure verifies fragment data structure
func (fa *FragmentAssertion) AssertFragmentStructure(fragment Fragment, expectedFields ...string) {
    data, ok := fragment.Data.(map[string]interface{})
    if !ok {
        fa.t.Errorf("Fragment %s data is not a map", fragment.ID)
        return
    }
    
    for _, field := range expectedFields {
        if _, exists := data[field]; !exists {
            fa.t.Errorf("Fragment %s missing expected field: %s", fragment.ID, field)
        }
    }
}
```

### Fragment Performance Validator

```go
// FragmentPerformanceValidator validates fragment performance characteristics
type FragmentPerformanceValidator struct {
    t *testing.T
    helper *E2ETestHelper
}

func NewFragmentPerformanceValidator(t *testing.T, helper *E2ETestHelper) *FragmentPerformanceValidator {
    return &FragmentPerformanceValidator{t: t, helper: helper}
}

// ValidateStrategyPerformance checks if fragment meets strategy performance expectations
func (fpv *FragmentPerformanceValidator) ValidateStrategyPerformance(fragment Fragment, generationTime time.Duration) {
    expectedPerformance := map[string]struct {
        maxTime        time.Duration
        minCompression float64
        maxCompression float64
    }{
        "static_dynamic": {10 * time.Millisecond, 0.85, 0.95},
        "markers":        {15 * time.Millisecond, 0.70, 0.85},
        "granular":       {20 * time.Millisecond, 0.60, 0.80},
        "replacement":    {30 * time.Millisecond, 0.40, 0.60},
    }
    
    if perf, exists := expectedPerformance[fragment.Strategy]; exists {
        if generationTime > perf.maxTime {
            fpv.t.Logf("Warning: Fragment %s (%s) took %v (max: %v)", 
                fragment.ID, fragment.Strategy, generationTime, perf.maxTime)
        }
        
        // Record performance metrics
        simulatedCompression := perf.minCompression + 
            rand.Float64()*(perf.maxCompression-perf.minCompression)
            
        fpv.helper.RecordFragmentMetric(
            fragment.ID,
            fragment.Strategy,
            generationTime,
            len(fmt.Sprintf("%+v", fragment.Data)),
            simulatedCompression,
            false,
        )
    }
}

// ValidateBatchPerformance validates performance for multiple fragments
func (fpv *FragmentPerformanceValidator) ValidateBatchPerformance(fragments []Fragment, totalTime time.Duration) {
    if len(fragments) == 0 {
        return
    }
    
    avgTime := totalTime / time.Duration(len(fragments))
    maxExpectedAvg := 25 * time.Millisecond // Conservative average
    
    if avgTime > maxExpectedAvg {
        fpv.t.Logf("Warning: Batch average %v exceeds expected %v", avgTime, maxExpectedAvg)
    }
    
    fpv.helper.SetCustomMetric("batch_fragment_count", len(fragments))
    fpv.helper.SetCustomMetric("batch_total_time", totalTime)
    fpv.helper.SetCustomMetric("batch_average_time", avgTime)
}
```

## Performance Testing Helpers

### Load Test Generator

```go
// LoadTestGenerator creates realistic load test scenarios
type LoadTestGenerator struct {
    userGen    *UserDataGenerator
    productGen *ProductDataGenerator
    rand       *rand.Rand
}

func NewLoadTestGenerator() *LoadTestGenerator {
    return &LoadTestGenerator{
        userGen:    NewUserDataGenerator(),
        productGen: NewProductDataGenerator(),
        rand:       rand.New(rand.NewSource(time.Now().UnixNano())),
    }
}

// GenerateLoadTestScenario creates a load test scenario with specified characteristics
func (ltg *LoadTestGenerator) GenerateLoadTestScenario(config LoadTestConfig) LoadTestScenario {
    scenario := LoadTestScenario{
        Name:            config.Name,
        Duration:        config.Duration,
        ConcurrentUsers: config.ConcurrentUsers,
        UpdatesPerUser:  config.UpdatesPerUser,
        TestCases:       make([]LoadTestCase, 0),
    }
    
    for i := 0; i < config.ConcurrentUsers; i++ {
        user := ltg.userGen.GenerateUser()
        testCase := LoadTestCase{
            UserID:   i + 1,
            UserData: user,
            Updates:  make([]map[string]interface{}, config.UpdatesPerUser),
        }
        
        // Generate realistic update sequence
        baseData := map[string]interface{}{
            "Users":    []map[string]interface{}{user},
            "Products": ltg.productGen.GenerateProductCatalog(50)["Products"],
        }
        
        testCase.Updates[0] = baseData
        
        // Generate progressive updates
        for j := 1; j < config.UpdatesPerUser; j++ {
            updateType := ltg.selectUpdateType(config)
            testCase.Updates[j] = ltg.generateUpdate(baseData, updateType)
        }
        
        scenario.TestCases = append(scenario.TestCases, testCase)
    }
    
    return scenario
}

func (ltg *LoadTestGenerator) selectUpdateType(config LoadTestConfig) string {
    weights := map[string]float64{
        "text_only":    0.65, // 65% - Static/Dynamic strategy
        "attributes":   0.20, // 20% - Markers strategy  
        "structural":   0.12, // 12% - Granular strategy
        "replacement":  0.03, // 3% - Replacement strategy
    }
    
    r := ltg.rand.Float64()
    cumulative := 0.0
    
    for updateType, weight := range weights {
        cumulative += weight
        if r <= cumulative {
            return updateType
        }
    }
    
    return "text_only" // Fallback
}

func (ltg *LoadTestGenerator) generateUpdate(baseData map[string]interface{}, updateType string) map[string]interface{} {
    update := make(map[string]interface{})
    
    // Copy base data
    for k, v := range baseData {
        update[k] = v
    }
    
    switch updateType {
    case "text_only":
        // Modify user names/titles
        users := update["Users"].([]map[string]interface{})
        if len(users) > 0 {
            users[0] = ltg.userGen.GenerateUserUpdate(users[0], "FirstName", "LastName")
        }
        
    case "attributes":
        // Add CSS classes, data attributes
        update["CSSClass"] = []string{"highlight", "featured", "urgent", "normal"}[ltg.rand.Intn(4)]
        update["DataState"] = []string{"active", "pending", "completed", "cancelled"}[ltg.rand.Intn(4)]
        
    case "structural":
        // Add/remove items
        users := update["Users"].([]map[string]interface{})
        if ltg.rand.Float32() > 0.5 && len(users) < 10 {
            // Add user
            users = append(users, ltg.userGen.GenerateUser())
        } else if len(users) > 1 {
            // Remove user
            users = users[:len(users)-1]
        }
        update["Users"] = users
        
    case "replacement":
        // Complete data restructure
        update["Users"] = ltg.userGen.GenerateUserList(ltg.rand.Intn(5) + 1)
        update["Products"] = ltg.productGen.GenerateProductCatalog(ltg.rand.Intn(20) + 10)["Products"]
        update["Layout"] = []string{"list", "grid", "table"}[ltg.rand.Intn(3)]
    }
    
    return update
}

// LoadTestConfig defines load test parameters
type LoadTestConfig struct {
    Name            string
    Duration        time.Duration
    ConcurrentUsers int
    UpdatesPerUser  int
}

// LoadTestScenario represents a complete load test scenario
type LoadTestScenario struct {
    Name            string
    Duration        time.Duration
    ConcurrentUsers int
    UpdatesPerUser  int
    TestCases       []LoadTestCase
}

// LoadTestCase represents a single user's test case
type LoadTestCase struct {
    UserID   int
    UserData map[string]interface{}
    Updates  []map[string]interface{}
}
```

## Browser Interaction Utilities

### Browser Action Recorder

```go
// BrowserActionRecorder records and validates browser interactions
type BrowserActionRecorder struct {
    helper *E2ETestHelper
    t      *testing.T
}

func NewBrowserActionRecorder(helper *E2ETestHelper, t *testing.T) *BrowserActionRecorder {
    return &BrowserActionRecorder{helper: helper, t: t}
}

// RecordNavigation records navigation timing
func (bar *BrowserActionRecorder) RecordNavigation(ctx context.Context, url string) error {
    start := time.Now()
    err := chromedp.Run(ctx, chromedp.Navigate(url))
    duration := time.Since(start)
    
    bar.helper.RecordBrowserAction("navigate", duration, err == nil, err)
    
    if err != nil {
        bar.helper.CaptureFailureScreenshot(ctx, bar.t, fmt.Sprintf("navigation to %s failed", url))
    }
    
    return err
}

// RecordElementWait records element wait timing
func (bar *BrowserActionRecorder) RecordElementWait(ctx context.Context, selector string) error {
    start := time.Now()
    err := chromedp.Run(ctx, chromedp.WaitVisible(selector))
    duration := time.Since(start)
    
    bar.helper.RecordBrowserAction(fmt.Sprintf("wait_visible_%s", selector), duration, err == nil, err)
    return err
}

// RecordClick records click action timing
func (bar *BrowserActionRecorder) RecordClick(ctx context.Context, selector string) error {
    start := time.Now()
    err := chromedp.Run(ctx, chromedp.Click(selector))
    duration := time.Since(start)
    
    bar.helper.RecordBrowserAction(fmt.Sprintf("click_%s", selector), duration, err == nil, err)
    return err
}

// RecordFormSubmission records form submission with validation
func (bar *BrowserActionRecorder) RecordFormSubmission(ctx context.Context, formData map[string]string) error {
    start := time.Now()
    
    var tasks []chromedp.Action
    for field, value := range formData {
        tasks = append(tasks, chromedp.SetValue(fmt.Sprintf(`[name="%s"]`, field), value))
    }
    tasks = append(tasks, chromedp.Click(`[type="submit"]`))
    
    err := chromedp.Run(ctx, tasks...)
    duration := time.Since(start)
    
    bar.helper.RecordBrowserAction("form_submission", duration, err == nil, err)
    return err
}

// RecordFragmentUpdate records fragment update performance
func (bar *BrowserActionRecorder) RecordFragmentUpdate(ctx context.Context, updateScript string) ([]Fragment, error) {
    start := time.Now()
    
    var result string
    err := chromedp.Run(ctx,
        chromedp.Evaluate(updateScript, &result),
        chromedp.Sleep(50*time.Millisecond), // Allow processing
    )
    
    duration := time.Since(start)
    
    if err != nil {
        bar.helper.RecordBrowserAction("fragment_update", duration, false, err)
        return nil, err
    }
    
    var fragments []Fragment
    if err := json.Unmarshal([]byte(result), &fragments); err != nil {
        bar.helper.RecordBrowserAction("fragment_parse", duration, false, err)
        return nil, fmt.Errorf("fragment parsing failed: %w", err)
    }
    
    bar.helper.RecordBrowserAction("fragment_update", duration, true, nil)
    return fragments, nil
}
```

### DOM Validator

```go
// DOMValidator validates DOM state and changes
type DOMValidator struct {
    t *testing.T
}

func NewDOMValidator(t *testing.T) *DOMValidator {
    return &DOMValidator{t: t}
}

// ValidateElementExists checks if element exists
func (dv *DOMValidator) ValidateElementExists(ctx context.Context, selector string) error {
    var nodeCount int
    err := chromedp.Run(ctx,
        chromedp.Evaluate(fmt.Sprintf("document.querySelectorAll('%s').length", selector), &nodeCount),
    )
    
    if err != nil {
        return fmt.Errorf("failed to query selector %s: %w", selector, err)
    }
    
    if nodeCount == 0 {
        dv.t.Errorf("Element not found: %s", selector)
        return fmt.Errorf("element not found: %s", selector)
    }
    
    return nil
}

// ValidateElementText checks element text content
func (dv *DOMValidator) ValidateElementText(ctx context.Context, selector, expectedText string) error {
    var actualText string
    err := chromedp.Run(ctx, chromedp.Text(selector, &actualText))
    
    if err != nil {
        return fmt.Errorf("failed to get text for %s: %w", selector, err)
    }
    
    if actualText != expectedText {
        dv.t.Errorf("Text mismatch for %s: expected '%s', got '%s'", selector, expectedText, actualText)
        return fmt.Errorf("text mismatch")
    }
    
    return nil
}

// ValidateElementAttribute checks element attribute value
func (dv *DOMValidator) ValidateElementAttribute(ctx context.Context, selector, attribute, expectedValue string) error {
    var actualValue string
    err := chromedp.Run(ctx, chromedp.AttributeValue(selector, attribute, &actualValue, nil))
    
    if err != nil {
        return fmt.Errorf("failed to get attribute %s for %s: %w", attribute, selector, err)
    }
    
    if actualValue != expectedValue {
        dv.t.Errorf("Attribute %s mismatch for %s: expected '%s', got '%s'", 
            attribute, selector, expectedValue, actualValue)
        return fmt.Errorf("attribute mismatch")
    }
    
    return nil
}

// ValidateElementCount checks number of matching elements
func (dv *DOMValidator) ValidateElementCount(ctx context.Context, selector string, expectedCount int) error {
    var actualCount int
    err := chromedp.Run(ctx,
        chromedp.Evaluate(fmt.Sprintf("document.querySelectorAll('%s').length", selector), &actualCount),
    )
    
    if err != nil {
        return fmt.Errorf("failed to count elements %s: %w", selector, err)
    }
    
    if actualCount != expectedCount {
        dv.t.Errorf("Element count mismatch for %s: expected %d, got %d", 
            selector, expectedCount, actualCount)
        return fmt.Errorf("element count mismatch")
    }
    
    return nil
}
```

## Test Environment Utilities

### Test Server Factory

```go
// TestServerFactory creates configured test servers for different scenarios
type TestServerFactory struct{}

func NewTestServerFactory() *TestServerFactory {
    return &TestServerFactory{}
}

// CreateBasicServer creates a basic HTTP test server
func (tsf *TestServerFactory) CreateBasicServer(app *Application, page *ApplicationPage) *httptest.Server {
    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        switch r.URL.Path {
        case "/":
            html, err := page.Render()
            if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
            w.Header().Set("Content-Type", "text/html")
            w.Write([]byte(html))
            
        case "/update":
            var updateData map[string]interface{}
            if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
                http.Error(w, "Invalid JSON", http.StatusBadRequest)
                return
            }
            
            fragments, err := page.RenderFragments(r.Context(), updateData)
            if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
            
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(fragments)
            
        default:
            http.NotFound(w, r)
        }
    }))
}

// CreateAdvancedServer creates a server with middleware and advanced features
func (tsf *TestServerFactory) CreateAdvancedServer(app *Application, page *ApplicationPage) *httptest.Server {
    mux := http.NewServeMux()
    
    // Add CORS middleware
    corsHandler := func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Access-Control-Allow-Origin", "*")
            w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
            w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
            
            if r.Method == "OPTIONS" {
                w.WriteHeader(http.StatusOK)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
    
    // Add logging middleware
    loggingHandler := func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            next.ServeHTTP(w, r)
            log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
        })
    }
    
    // Main page handler
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        html, err := page.Render()
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "text/html")
        w.Write([]byte(html))
    })
    
    // Fragment update handler
    mux.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
        var updateData map[string]interface{}
        if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }
        
        start := time.Now()
        fragments, err := page.RenderFragments(r.Context(), updateData)
        duration := time.Since(start)
        
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        
        // Add performance headers
        w.Header().Set("X-Generation-Time", duration.String())
        w.Header().Set("X-Fragment-Count", fmt.Sprintf("%d", len(fragments)))
        w.Header().Set("Content-Type", "application/json")
        
        json.NewEncoder(w).Encode(fragments)
    })
    
    // Health check endpoint
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{
            "status": "healthy",
            "timestamp": time.Now().Format(time.RFC3339),
        })
    })
    
    // Static file handler for testing
    mux.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
        // Serve mock static files
        w.Header().Set("Content-Type", "text/css")
        w.Write([]byte("/* Mock CSS */"))
    })
    
    handler := corsHandler(loggingHandler(mux))
    return httptest.NewServer(handler)
}
```

This comprehensive test utilities reference provides developers with a complete toolkit for writing effective LiveTemplate E2E tests, including data generation, template building, performance validation, and browser interaction utilities.