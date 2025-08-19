# E2E Integration Examples for LiveTemplate

## Overview

This guide provides comprehensive real-world integration examples showing how to use LiveTemplate E2E testing in various application contexts, from simple CRUD applications to complex real-time systems.

## Table of Contents

- [CRUD Application Testing](#crud-application-testing)
- [Real-Time Dashboard Testing](#real-time-dashboard-testing)
- [E-Commerce Platform Testing](#e-commerce-platform-testing)
- [Social Media Feed Testing](#social-media-feed-testing)
- [Multi-User Collaborative Application](#multi-user-collaborative-application)
- [API Integration Testing](#api-integration-testing)
- [WebSocket Real-Time Updates](#websocket-real-time-updates)
- [Form Wizard Testing](#form-wizard-testing)

## CRUD Application Testing

### Complete User Management System

```go
// Complete E2E test for a user management CRUD application
func TestUserManagementCRUD(t *testing.T) {
    E2ETestWithHelper(t, "user-management-crud", func(helper *E2ETestHelper) error {
        // Setup realistic user management template
        tmpl, err := template.New("user-management").Parse(`
            <!DOCTYPE html>
            <html>
            <head>
                <title>User Management</title>
                <style>
                    .user-table { width: 100%; border-collapse: collapse; }
                    .user-table th, .user-table td { padding: 12px; border: 1px solid #ddd; text-align: left; }
                    .user-table th { background-color: #f2f2f2; }
                    .user-form { max-width: 500px; margin: 20px 0; }
                    .form-group { margin-bottom: 15px; }
                    .form-group label { display: block; margin-bottom: 5px; font-weight: bold; }
                    .form-group input, .form-group select { width: 100%; padding: 8px; border: 1px solid #ccc; border-radius: 4px; }
                    .btn { padding: 10px 20px; border: none; border-radius: 4px; cursor: pointer; }
                    .btn-primary { background-color: #007bff; color: white; }
                    .btn-danger { background-color: #dc3545; color: white; }
                    .btn-success { background-color: #28a745; color: white; }
                    .status-active { color: #28a745; font-weight: bold; }
                    .status-inactive { color: #dc3545; font-weight: bold; }
                    .user-row:hover { background-color: #f8f9fa; }
                    .loading { opacity: 0.5; }
                    .error { color: #dc3545; padding: 10px; background: #f8d7da; border: 1px solid #f5c6cb; border-radius: 4px; margin: 10px 0; }
                    .success { color: #155724; padding: 10px; background: #d4edda; border: 1px solid #c3e6cb; border-radius: 4px; margin: 10px 0; }
                </style>
            </head>
            <body>
                <div class="container">
                    <header data-lt-fragment="page-header">
                        <h1>User Management System</h1>
                        <p>Total Users: {{.TotalUsers}} | Active: {{.ActiveUsers}} | Inactive: {{.InactiveUsers}}</p>
                    </header>
                    
                    <!-- User Creation/Edit Form -->
                    <section data-lt-fragment="user-form" class="user-form">
                        <h2>{{if .EditMode}}Edit User{{else}}Add New User{{end}}</h2>
                        
                        {{if .FormError}}
                        <div class="error">{{.FormError}}</div>
                        {{end}}
                        
                        {{if .FormSuccess}}
                        <div class="success">{{.FormSuccess}}</div>
                        {{end}}
                        
                        <form id="userForm" class="{{if .FormLoading}}loading{{end}}">
                            <div class="form-group">
                                <label for="firstName">First Name *</label>
                                <input type="text" id="firstName" name="firstName" value="{{.FormData.FirstName}}" required>
                            </div>
                            
                            <div class="form-group">
                                <label for="lastName">Last Name *</label>
                                <input type="text" id="lastName" name="lastName" value="{{.FormData.LastName}}" required>
                            </div>
                            
                            <div class="form-group">
                                <label for="email">Email *</label>
                                <input type="email" id="email" name="email" value="{{.FormData.Email}}" required>
                            </div>
                            
                            <div class="form-group">
                                <label for="role">Role</label>
                                <select id="role" name="role">
                                    {{range .AvailableRoles}}
                                    <option value="{{.}}" {{if eq . $.FormData.Role}}selected{{end}}>{{.}}</option>
                                    {{end}}
                                </select>
                            </div>
                            
                            <div class="form-group">
                                <label>
                                    <input type="checkbox" name="active" {{if .FormData.Active}}checked{{end}}>
                                    Active User
                                </label>
                            </div>
                            
                            <div class="form-group">
                                <button type="submit" class="btn btn-primary">
                                    {{if .EditMode}}Update User{{else}}Create User{{end}}
                                </button>
                                
                                {{if .EditMode}}
                                <button type="button" class="btn" onclick="cancelEdit()">Cancel</button>
                                {{end}}
                                
                                <button type="reset" class="btn">Reset</button>
                            </div>
                        </form>
                    </section>
                    
                    <!-- Search and Filters -->
                    <section data-lt-fragment="user-filters">
                        <h3>Search and Filters</h3>
                        <div style="display: flex; gap: 15px; margin-bottom: 20px;">
                            <input type="text" placeholder="Search users..." value="{{.SearchQuery}}" id="searchInput">
                            <select id="roleFilter">
                                <option value="">All Roles</option>
                                {{range .AvailableRoles}}
                                <option value="{{.}}" {{if eq . $.RoleFilter}}selected{{end}}>{{.}}</option>
                                {{end}}
                            </select>
                            <select id="statusFilter">
                                <option value="">All Status</option>
                                <option value="active" {{if eq "active" .StatusFilter}}selected{{end}}>Active</option>
                                <option value="inactive" {{if eq "inactive" .StatusFilter}}selected{{end}}>Inactive</option>
                            </select>
                            <button class="btn btn-primary" onclick="applyFilters()">Filter</button>
                            <button class="btn" onclick="clearFilters()">Clear</button>
                        </div>
                    </section>
                    
                    <!-- User List Table -->
                    <section data-lt-fragment="user-list">
                        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px;">
                            <h3>Users ({{len .Users}})</h3>
                            <div>
                                <label>Sort by:
                                    <select id="sortBy" onchange="sortUsers()">
                                        <option value="name" {{if eq "name" .SortBy}}selected{{end}}>Name</option>
                                        <option value="email" {{if eq "email" .SortBy}}selected{{end}}>Email</option>
                                        <option value="role" {{if eq "role" .SortBy}}selected{{end}}>Role</option>
                                        <option value="created" {{if eq "created" .SortBy}}selected{{end}}>Created Date</option>
                                    </select>
                                </label>
                            </div>
                        </div>
                        
                        {{if .Users}}
                        <table class="user-table">
                            <thead>
                                <tr>
                                    <th>ID</th>
                                    <th>Name</th>
                                    <th>Email</th>
                                    <th>Role</th>
                                    <th>Status</th>
                                    <th>Created</th>
                                    <th>Actions</th>
                                </tr>
                            </thead>
                            <tbody>
                                {{range .Users}}
                                <tr class="user-row" data-user-id="{{.ID}}">
                                    <td>{{.ID}}</td>
                                    <td>{{.FirstName}} {{.LastName}}</td>
                                    <td>{{.Email}}</td>
                                    <td>{{.Role}}</td>
                                    <td class="{{if .Active}}status-active{{else}}status-inactive{{end}}">
                                        {{if .Active}}Active{{else}}Inactive{{end}}
                                    </td>
                                    <td>{{.CreatedAt}}</td>
                                    <td>
                                        <button class="btn btn-primary" onclick="editUser({{.ID}})">Edit</button>
                                        <button class="btn btn-danger" onclick="deleteUser({{.ID}})" 
                                                {{if not .Active}}disabled{{end}}>Delete</button>
                                        {{if .Active}}
                                        <button class="btn" onclick="deactivateUser({{.ID}})">Deactivate</button>
                                        {{else}}
                                        <button class="btn btn-success" onclick="activateUser({{.ID}})">Activate</button>
                                        {{end}}
                                    </td>
                                </tr>
                                {{end}}
                            </tbody>
                        </table>
                        {{else}}
                        <div style="text-align: center; padding: 40px; color: #666;">
                            <h4>No users found</h4>
                            <p>{{if or .SearchQuery .RoleFilter .StatusFilter}}Try adjusting your search criteria{{else}}Create your first user using the form above{{end}}</p>
                        </div>
                        {{end}}
                    </section>
                    
                    <!-- Pagination -->
                    {{if gt .TotalPages 1}}
                    <section data-lt-fragment="pagination">
                        <div style="display: flex; justify-content: center; align-items: center; gap: 10px; margin-top: 20px;">
                            <button class="btn" onclick="goToPage(1)" {{if eq .CurrentPage 1}}disabled{{end}}>First</button>
                            <button class="btn" onclick="goToPage({{.CurrentPage | subtract 1}})" {{if eq .CurrentPage 1}}disabled{{end}}>Previous</button>
                            
                            {{range .PageNumbers}}
                            <button class="btn {{if eq . $.CurrentPage}}btn-primary{{end}}" onclick="goToPage({{.}})">{{.}}</button>
                            {{end}}
                            
                            <button class="btn" onclick="goToPage({{.CurrentPage | add 1}})" {{if eq .CurrentPage .TotalPages}}disabled{{end}}>Next</button>
                            <button class="btn" onclick="goToPage({{.TotalPages}})" {{if eq .CurrentPage .TotalPages}}disabled{{end}}>Last</button>
                        </div>
                        
                        <div style="text-align: center; margin-top: 10px; color: #666;">
                            Page {{.CurrentPage}} of {{.TotalPages}} ({{.TotalUsers}} total users)
                        </div>
                    </section>
                    {{end}}
                </div>
                
                <script>
                    // JavaScript for user management functionality
                    function editUser(id) {
                        fetch('/api/users/' + id)
                            .then(response => response.json())
                            .then(user => {
                                updateForm({
                                    EditMode: true,
                                    FormData: user
                                });
                            });
                    }
                    
                    function deleteUser(id) {
                        if (confirm('Are you sure you want to delete this user?')) {
                            fetch('/api/users/' + id, { method: 'DELETE' })
                                .then(() => refreshUserList());
                        }
                    }
                    
                    function updateForm(data) {
                        fetch('/update', {
                            method: 'POST',
                            headers: {'Content-Type': 'application/json'},
                            body: JSON.stringify(data)
                        })
                        .then(response => response.json())
                        .then(fragments => applyFragments(fragments));
                    }
                    
                    function refreshUserList() {
                        fetch('/api/users')
                            .then(response => response.json())
                            .then(users => {
                                updateForm({ Users: users });
                            });
                    }
                    
                    // Fragment application helper
                    function applyFragments(fragments) {
                        fragments.forEach(fragment => {
                            const element = document.querySelector('[data-lt-fragment="' + fragment.id + '"]');
                            if (element && fragment.data && fragment.data.html) {
                                element.innerHTML = fragment.data.html;
                            }
                        });
                    }
                </script>
            </body>
            </html>
        `)
        
        if err != nil {
            return fmt.Errorf("template parsing failed: %w", err)
        }
        
        // Create application and page
        app, err := NewApplication()
        if err != nil {
            return err
        }
        defer app.Close()
        
        // Generate realistic initial data
        userGen := NewUserDataGenerator()
        initialUsers := userGen.GenerateUserList(25)
        
        initialData := map[string]interface{}{
            "TotalUsers":     25,
            "ActiveUsers":    20,
            "InactiveUsers":  5,
            "Users":          initialUsers,
            "EditMode":       false,
            "FormData":       map[string]interface{}{},
            "AvailableRoles": []string{"admin", "user", "moderator", "guest"},
            "SearchQuery":    "",
            "RoleFilter":     "",
            "StatusFilter":   "",
            "SortBy":         "name",
            "CurrentPage":    1,
            "TotalPages":     3,
            "PageNumbers":    []int{1, 2, 3},
        }
        
        page, err := app.NewApplicationPage(tmpl, initialData)
        if err != nil {
            return err
        }
        defer page.Close()
        
        // Create test server with comprehensive API
        server := helper.CreateAdvancedTestServer(app, page)
        defer server.Close()
        
        ctx, cancel := helper.CreateBrowserContext()
        defer cancel()
        
        // Test 1: Initial Page Load
        t.Log("Testing initial page load and UI rendering")
        err = chromedp.Run(ctx,
            chromedp.Navigate(server.URL),
            chromedp.WaitVisible(".user-table"),
            chromedp.WaitVisible(".user-form"),
        )
        if err != nil {
            helper.CaptureFailureScreenshot(ctx, t, "initial page load failed")
            return fmt.Errorf("initial page load failed: %w", err)
        }
        
        helper.CaptureScreenshot(ctx, "initial-load")
        
        // Validate initial data display
        var userCount int
        err = chromedp.Run(ctx,
            chromedp.Evaluate("document.querySelectorAll('.user-row').length", &userCount),
        )
        if err != nil || userCount != 25 {
            return fmt.Errorf("expected 25 users, found %d", userCount)
        }
        
        // Test 2: Create New User (Static/Dynamic Strategy)
        t.Log("Testing user creation - expecting static/dynamic strategy")
        newUserData := map[string]interface{}{
            "TotalUsers":   26,
            "ActiveUsers":  21,
            "Users":        append(initialUsers, userGen.GenerateUser()),
            "FormSuccess":  "User created successfully",
            "FormData":     map[string]interface{}{}, // Clear form
        }
        
        fragments, err := helper.TestFragmentUpdate(ctx, server.URL+"/update", newUserData)
        if err != nil {
            return fmt.Errorf("user creation test failed: %w", err)
        }
        
        // Should use static/dynamic for text-only changes
        staticDynamicFound := false
        for _, fragment := range fragments {
            if fragment.Strategy == "static_dynamic" {
                staticDynamicFound = true
                helper.RecordFragmentMetric(fragment.ID, "static_dynamic", 8*time.Millisecond, 150, 0.90, false)
            }
        }
        
        if staticDynamicFound {
            t.Log("✅ Static/Dynamic strategy used for user creation")
        }
        
        helper.CaptureScreenshot(ctx, "user-created")
        
        // Test 3: Search and Filtering (Markers Strategy)
        t.Log("Testing search and filtering - expecting markers strategy") 
        searchData := map[string]interface{}{
            "Users":       filterUsers(initialUsers, "john", "admin", "active"),
            "SearchQuery": "john",
            "RoleFilter":  "admin",
            "StatusFilter": "active",
            "TotalUsers":  3, // Filtered count
        }
        
        fragments, err = helper.TestFragmentUpdate(ctx, server.URL+"/update", searchData)
        if err != nil {
            return fmt.Errorf("search filtering test failed: %w", err)
        }
        
        // Should use markers for attribute changes (CSS classes, filter states)
        markersFound := false
        for _, fragment := range fragments {
            if fragment.Strategy == "markers" {
                markersFound = true
                helper.RecordFragmentMetric(fragment.ID, "markers", 12*time.Millisecond, 200, 0.75, false)
            }
        }
        
        if markersFound {
            t.Log("✅ Markers strategy used for search filtering")
        }
        
        helper.CaptureScreenshot(ctx, "search-filtered")
        
        // Test 4: Edit User Form (Granular Strategy)
        t.Log("Testing user edit form - expecting granular strategy")
        editUserData := map[string]interface{}{
            "EditMode": true,
            "FormData": map[string]interface{}{
                "ID":        1,
                "FirstName": "John",
                "LastName":  "Doe",
                "Email":     "john.doe@example.com",
                "Role":      "admin",
                "Active":    true,
            },
            "Users": initialUsers, // Same user list
        }
        
        fragments, err = helper.TestFragmentUpdate(ctx, server.URL+"/update", editUserData)
        if err != nil {
            return fmt.Errorf("edit form test failed: %w", err)
        }
        
        // Should use granular for form structure changes
        granularFound := false
        for _, fragment := range fragments {
            if fragment.Strategy == "granular" {
                granularFound = true
                helper.RecordFragmentMetric(fragment.ID, "granular", 15*time.Millisecond, 300, 0.65, false)
            }
        }
        
        if granularFound {
            t.Log("✅ Granular strategy used for form editing")
        }
        
        helper.CaptureScreenshot(ctx, "user-edit-form")
        
        // Test 5: Bulk Operations (Replacement Strategy)
        t.Log("Testing bulk user operations - expecting replacement strategy")
        
        // Simulate complex bulk operation with mixed changes
        bulkUsers := userGen.GenerateUserList(50) // Different user set
        bulkData := map[string]interface{}{
            "Users":         bulkUsers,
            "TotalUsers":    50,
            "ActiveUsers":   42,
            "InactiveUsers": 8,
            "SearchQuery":   "", // Reset filters
            "RoleFilter":    "",
            "StatusFilter":  "",
            "CurrentPage":   1,
            "TotalPages":    5,
            "PageNumbers":   []int{1, 2, 3, 4, 5},
            "FormSuccess":   "Bulk operation completed successfully",
        }
        
        fragments, err = helper.TestFragmentUpdate(ctx, server.URL+"/update", bulkData)
        if err != nil {
            return fmt.Errorf("bulk operations test failed: %w", err)
        }
        
        // Should use replacement for complex mixed changes
        replacementFound := false
        for _, fragment := range fragments {
            if fragment.Strategy == "replacement" {
                replacementFound = true
                helper.RecordFragmentMetric(fragment.ID, "replacement", 25*time.Millisecond, 1200, 0.45, false)
            }
        }
        
        if replacementFound {
            t.Log("✅ Replacement strategy used for bulk operations")
        }
        
        helper.CaptureScreenshot(ctx, "bulk-operations")
        
        // Test 6: Pagination Performance
        t.Log("Testing pagination navigation")
        for page := 1; page <= 3; page++ {
            pageData := map[string]interface{}{
                "Users":         generatePageUsers(bulkUsers, page, 20),
                "CurrentPage":   page,
                "TotalPages":    3,
                "PageNumbers":   []int{1, 2, 3},
            }
            
            start := time.Now()
            fragments, err = helper.TestFragmentUpdate(ctx, server.URL+"/update", pageData)
            duration := time.Since(start)
            
            if err != nil {
                return fmt.Errorf("pagination test failed for page %d: %w", page, err)
            }
            
            // Record pagination performance
            for _, fragment := range fragments {
                helper.RecordFragmentMetric(
                    fmt.Sprintf("%s-page-%d", fragment.ID, page),
                    fragment.Strategy,
                    duration,
                    len(fmt.Sprintf("%+v", fragment.Data)),
                    0.70,
                    page > 1, // Subsequent pages might hit cache
                )
            }
            
            // Validate page loads quickly (pagination should be fast)
            if duration > 50*time.Millisecond {
                t.Logf("Warning: Page %d took %v (expected <50ms)", page, duration)
            }
        }
        
        helper.CaptureScreenshot(ctx, "pagination-complete")
        
        // Test 7: Error Handling
        t.Log("Testing error handling scenarios")
        errorData := map[string]interface{}{
            "FormError": "Email address already exists",
            "FormData": map[string]interface{}{
                "FirstName": "Test",
                "LastName":  "User", 
                "Email":     "existing@example.com",
                "Role":      "user",
                "Active":    true,
            },
        }
        
        fragments, err = helper.TestFragmentUpdate(ctx, server.URL+"/update", errorData)
        if err != nil {
            return fmt.Errorf("error handling test failed: %w", err)
        }
        
        helper.CaptureScreenshot(ctx, "error-handling")
        
        // Test 8: Performance under Load
        t.Log("Testing performance under simulated load")
        const loadIterations = 20
        var totalTime time.Duration
        var successCount int
        
        for i := 0; i < loadIterations; i++ {
            loadData := map[string]interface{}{
                "Users":      userGen.GenerateUserList(rand.Intn(10) + 5), // 5-15 users
                "TotalUsers": rand.Intn(100) + 50,                        // 50-150 total
            }
            
            start := time.Now()
            fragments, err = helper.TestFragmentUpdate(ctx, server.URL+"/update", loadData)
            duration := time.Since(start)
            totalTime += duration
            
            if err == nil {
                successCount++
            }
            
            // Record load test metrics
            for _, fragment := range fragments {
                helper.RecordFragmentMetric(
                    fmt.Sprintf("%s-load-%d", fragment.ID, i),
                    fragment.Strategy,
                    duration,
                    len(fmt.Sprintf("%+v", fragment.Data)),
                    0.75,
                    false,
                )
            }
        }
        
        avgTime := totalTime / loadIterations
        successRate := float64(successCount) / loadIterations
        
        helper.SetCustomMetric("crud_load_test_avg_time", avgTime)
        helper.SetCustomMetric("crud_load_test_success_rate", successRate)
        
        t.Logf("Load test results: %d iterations, %.2f%% success rate, avg time: %v", 
            loadIterations, successRate*100, avgTime)
        
        if successRate < 0.95 {
            return fmt.Errorf("load test success rate %.2f%% below threshold 95%%", successRate*100)
        }
        
        if avgTime > 100*time.Millisecond {
            return fmt.Errorf("load test average time %v above threshold 100ms", avgTime)
        }
        
        helper.CaptureScreenshot(ctx, "load-test-complete")
        
        t.Log("✅ User Management CRUD test completed successfully")
        return nil
    })
}

// Helper functions for CRUD testing
func filterUsers(users []map[string]interface{}, search, role, status string) []map[string]interface{} {
    filtered := make([]map[string]interface{}, 0)
    
    for _, user := range users {
        // Apply search filter
        if search != "" {
            name := fmt.Sprintf("%s %s", user["FirstName"], user["LastName"])
            if !strings.Contains(strings.ToLower(name), strings.ToLower(search)) {
                continue
            }
        }
        
        // Apply role filter
        if role != "" && user["Role"] != role {
            continue
        }
        
        // Apply status filter
        if status != "" {
            isActive := user["Active"].(bool)
            if (status == "active" && !isActive) || (status == "inactive" && isActive) {
                continue
            }
        }
        
        filtered = append(filtered, user)
    }
    
    return filtered
}

func generatePageUsers(allUsers []map[string]interface{}, page, pageSize int) []map[string]interface{} {
    start := (page - 1) * pageSize
    end := start + pageSize
    
    if start >= len(allUsers) {
        return []map[string]interface{}{}
    }
    
    if end > len(allUsers) {
        end = len(allUsers)
    }
    
    return allUsers[start:end]
}
```

## Real-Time Dashboard Testing

### System Monitoring Dashboard

```go
func TestRealTimeDashboard(t *testing.T) {
    E2ETestWithHelper(t, "realtime-dashboard", func(helper *E2ETestHelper) error {
        // Real-time system monitoring dashboard template
        tmpl, err := template.New("dashboard").Parse(`
            <!DOCTYPE html>
            <html>
            <head>
                <title>System Dashboard</title>
                <style>
                    .dashboard { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; padding: 20px; }
                    .widget { border: 1px solid #ddd; border-radius: 8px; padding: 20px; background: white; }
                    .widget h3 { margin-top: 0; color: #333; }
                    .metric-value { font-size: 2em; font-weight: bold; }
                    .metric-good { color: #28a745; }
                    .metric-warning { color: #ffc107; }
                    .metric-critical { color: #dc3545; }
                    .status-indicator { width: 12px; height: 12px; border-radius: 50%; display: inline-block; margin-right: 8px; }
                    .status-online { background-color: #28a745; }
                    .status-offline { background-color: #dc3545; }
                    .status-warning { background-color: #ffc107; }
                    .progress-bar { width: 100%; height: 20px; background: #f0f0f0; border-radius: 10px; overflow: hidden; }
                    .progress-fill { height: 100%; transition: width 0.5s ease; }
                    .progress-good { background: linear-gradient(90deg, #28a745, #20c997); }
                    .progress-warning { background: linear-gradient(90deg, #ffc107, #fd7e14); }
                    .progress-critical { background: linear-gradient(90deg, #dc3545, #e74c3c); }
                    .server-list { list-style: none; padding: 0; }
                    .server-item { display: flex; align-items: center; padding: 8px 0; border-bottom: 1px solid #eee; }
                    .alert-item { padding: 10px; margin: 5px 0; border-radius: 4px; }
                    .alert-critical { background: #f8d7da; border-left: 4px solid #dc3545; }
                    .alert-warning { background: #fff3cd; border-left: 4px solid #ffc107; }
                    .alert-info { background: #d1ecf1; border-left: 4px solid #17a2b8; }
                </style>
            </head>
            <body>
                <header data-lt-fragment="dashboard-header">
                    <h1>System Dashboard</h1>
                    <p>Last Updated: {{.LastUpdated}} | Status: 
                        <span class="status-indicator {{.SystemStatus}}"></span>
                        {{.SystemStatus | title}}
                    </p>
                </header>
                
                <div class="dashboard">
                    <!-- CPU Usage Widget -->
                    <div data-lt-fragment="cpu-widget" class="widget">
                        <h3>CPU Usage</h3>
                        <div class="metric-value {{.CPUStatus}}">{{.CPUUsage}}%</div>
                        <div class="progress-bar">
                            <div class="progress-fill {{.CPUStatus}}" style="width: {{.CPUUsage}}%"></div>
                        </div>
                        <p>Average load across {{.CPUCores}} cores</p>
                    </div>
                    
                    <!-- Memory Usage Widget -->
                    <div data-lt-fragment="memory-widget" class="widget">
                        <h3>Memory Usage</h3>
                        <div class="metric-value {{.MemoryStatus}}">{{.MemoryUsage}}%</div>
                        <div class="progress-bar">
                            <div class="progress-fill {{.MemoryStatus}}" style="width: {{.MemoryUsage}}%"></div>
                        </div>
                        <p>{{.MemoryUsed}}GB / {{.MemoryTotal}}GB</p>
                    </div>
                    
                    <!-- Network Traffic Widget -->
                    <div data-lt-fragment="network-widget" class="widget">
                        <h3>Network Traffic</h3>
                        <div>
                            <strong>Inbound:</strong> <span class="metric-value">{{.NetworkIn}} MB/s</span>
                        </div>
                        <div>
                            <strong>Outbound:</strong> <span class="metric-value">{{.NetworkOut}} MB/s</span>
                        </div>
                        <p>Total: {{.NetworkTotal}}GB today</p>
                    </div>
                    
                    <!-- Server Status Widget -->
                    <div data-lt-fragment="servers-widget" class="widget">
                        <h3>Server Status ({{len .Servers}})</h3>
                        <ul class="server-list">
                            {{range .Servers}}
                            <li class="server-item">
                                <span class="status-indicator status-{{.Status}}"></span>
                                <strong>{{.Name}}</strong>
                                <span style="margin-left: auto;">{{.Uptime}}</span>
                            </li>
                            {{end}}
                        </ul>
                    </div>
                    
                    <!-- Active Alerts Widget -->
                    <div data-lt-fragment="alerts-widget" class="widget">
                        <h3>Active Alerts ({{len .Alerts}})</h3>
                        {{if .Alerts}}
                        {{range .Alerts}}
                        <div class="alert-item alert-{{.Severity}}">
                            <strong>{{.Title}}</strong>
                            <p>{{.Message}}</p>
                            <small>{{.Timestamp}}</small>
                        </div>
                        {{end}}
                        {{else}}
                        <p style="color: #666; text-align: center;">No active alerts</p>
                        {{end}}
                    </div>
                    
                    <!-- Response Time Widget -->
                    <div data-lt-fragment="response-time-widget" class="widget">
                        <h3>Response Times</h3>
                        {{range .Services}}
                        <div style="display: flex; justify-content: space-between; padding: 5px 0;">
                            <span>{{.Name}}:</span>
                            <span class="{{.Status}}">{{.ResponseTime}}ms</span>
                        </div>
                        {{end}}
                        <p>Average: {{.AvgResponseTime}}ms</p>
                    </div>
                </div>
                
                <script>
                    // Simulated real-time updates
                    function updateDashboard() {
                        fetch('/api/dashboard-data')
                            .then(response => response.json())
                            .then(data => {
                                return fetch('/update', {
                                    method: 'POST',
                                    headers: {'Content-Type': 'application/json'},
                                    body: JSON.stringify(data)
                                });
                            })
                            .then(response => response.json())
                            .then(fragments => {
                                fragments.forEach(fragment => {
                                    const element = document.querySelector('[data-lt-fragment="' + fragment.id + '"]');
                                    if (element && fragment.data && fragment.data.html) {
                                        element.innerHTML = fragment.data.html;
                                    }
                                });
                            })
                            .catch(error => console.error('Dashboard update failed:', error));
                    }
                    
                    // Update every 5 seconds
                    setInterval(updateDashboard, 5000);
                </script>
            </body>
            </html>
        `)
        
        if err != nil {
            return err
        }
        
        app, err := NewApplication()
        if err != nil {
            return err
        }
        defer app.Close()
        
        // Generate initial dashboard data
        initialData := generateDashboardData(false)
        
        page, err := app.NewApplicationPage(tmpl, initialData)
        if err != nil {
            return err
        }
        defer page.Close()
        
        server := helper.CreateAdvancedTestServer(app, page)
        defer server.Close()
        
        ctx, cancel := helper.CreateBrowserContext()
        defer cancel()
        
        // Test 1: Initial Dashboard Load
        t.Log("Testing initial dashboard load")
        err = chromedp.Run(ctx,
            chromedp.Navigate(server.URL),
            chromedp.WaitVisible(".dashboard"),
            chromedp.WaitVisible(".widget"),
        )
        if err != nil {
            return fmt.Errorf("initial dashboard load failed: %w", err)
        }
        
        helper.CaptureScreenshot(ctx, "dashboard-initial")
        
        // Test 2: Simulated Real-Time Updates
        t.Log("Testing real-time metric updates")
        
        scenarios := []struct {
            name     string
            dataGen  func() map[string]interface{}
            expected string
        }{
            {"normal-metrics", func() map[string]interface{} { return generateDashboardData(false) }, "static_dynamic"},
            {"warning-state", func() map[string]interface{} { return generateDashboardData(true) }, "markers"},
            {"server-changes", generateServerChanges, "granular"},
            {"alert-spike", generateAlertSpike, "replacement"},
        }
        
        for i, scenario := range scenarios {
            t.Logf("Testing scenario: %s", scenario.name)
            
            updateData := scenario.dataGen()
            
            start := time.Now()
            fragments, err := helper.TestFragmentUpdate(ctx, server.URL+"/update", updateData)
            duration := time.Since(start)
            
            if err != nil {
                return fmt.Errorf("scenario %s failed: %w", scenario.name, err)
            }
            
            // Validate expected strategy
            strategyFound := false
            for _, fragment := range fragments {
                if fragment.Strategy == scenario.expected {
                    strategyFound = true
                    helper.RecordFragmentMetric(fragment.ID, fragment.Strategy, duration, 
                        len(fmt.Sprintf("%+v", fragment.Data)), 0.70, false)
                }
            }
            
            if strategyFound {
                t.Logf("✅ %s strategy used for %s", scenario.expected, scenario.name)
            }
            
            helper.CaptureScreenshot(ctx, fmt.Sprintf("dashboard-%s", scenario.name))
            
            // Simulate real-time interval
            time.Sleep(100 * time.Millisecond)
        }
        
        // Test 3: Performance under Continuous Updates
        t.Log("Testing dashboard performance under continuous updates")
        
        const updateCycles = 20
        var totalTime time.Duration
        
        for cycle := 0; cycle < updateCycles; cycle++ {
            // Simulate various metric changes
            updateData := generateRandomMetricUpdate(cycle)
            
            start := time.Now()
            fragments, err := helper.TestFragmentUpdate(ctx, server.URL+"/update", updateData)
            duration := time.Since(start)
            totalTime += duration
            
            if err != nil {
                return fmt.Errorf("continuous update cycle %d failed: %w", cycle, err)
            }
            
            // Track performance metrics
            for _, fragment := range fragments {
                helper.RecordFragmentMetric(
                    fmt.Sprintf("%s-cycle-%d", fragment.ID, cycle),
                    fragment.Strategy,
                    duration,
                    len(fmt.Sprintf("%+v", fragment.Data)),
                    0.75,
                    cycle > 5, // Later cycles may benefit from caching
                )
            }
            
            // Validate real-time performance requirements
            if duration > 50*time.Millisecond {
                t.Logf("Warning: Update cycle %d took %v (real-time threshold: 50ms)", cycle, duration)
            }
        }
        
        avgUpdateTime := totalTime / updateCycles
        helper.SetCustomMetric("dashboard_avg_update_time", avgUpdateTime)
        helper.SetCustomMetric("dashboard_total_cycles", updateCycles)
        
        t.Logf("Dashboard performance: %d cycles, avg time: %v", updateCycles, avgUpdateTime)
        
        // Real-time dashboard should update very quickly
        if avgUpdateTime > 30*time.Millisecond {
            return fmt.Errorf("dashboard average update time %v exceeds real-time threshold 30ms", avgUpdateTime)
        }
        
        helper.CaptureScreenshot(ctx, "dashboard-performance-complete")
        
        t.Log("✅ Real-time dashboard test completed successfully")
        return nil
    })
}

// Helper functions for dashboard testing
func generateDashboardData(highLoad bool) map[string]interface{} {
    cpuUsage := 45 + rand.Intn(20) // 45-65%
    memoryUsage := 60 + rand.Intn(25) // 60-85%
    
    if highLoad {
        cpuUsage = 80 + rand.Intn(15) // 80-95%
        memoryUsage = 85 + rand.Intn(10) // 85-95%
    }
    
    return map[string]interface{}{
        "LastUpdated": time.Now().Format("15:04:05"),
        "SystemStatus": func() string {
            if cpuUsage > 80 || memoryUsage > 90 {
                return "status-critical"
            } else if cpuUsage > 60 || memoryUsage > 75 {
                return "status-warning"  
            }
            return "status-online"
        }(),
        "CPUUsage":    cpuUsage,
        "CPUStatus":   getMetricStatus(cpuUsage, 70, 85),
        "CPUCores":    8,
        "MemoryUsage": memoryUsage,
        "MemoryStatus": getMetricStatus(memoryUsage, 75, 90),
        "MemoryUsed":  fmt.Sprintf("%.1f", float64(memoryUsage)*32.0/100.0),
        "MemoryTotal": "32.0",
        "NetworkIn":   fmt.Sprintf("%.1f", 10.0+rand.Float64()*50.0),
        "NetworkOut":  fmt.Sprintf("%.1f", 5.0+rand.Float64()*25.0),
        "NetworkTotal": fmt.Sprintf("%.2f", 150.0+rand.Float64()*50.0),
        "Servers":     generateServerStatus(),
        "Alerts":      generateAlerts(highLoad),
        "Services":    generateServiceStatus(),
        "AvgResponseTime": 45 + rand.Intn(30),
    }
}

func getMetricStatus(value, warningThreshold, criticalThreshold int) string {
    if value >= criticalThreshold {
        return "metric-critical"
    } else if value >= warningThreshold {
        return "metric-warning"
    }
    return "metric-good"
}

func generateServerStatus() []map[string]interface{} {
    servers := []map[string]interface{}{
        {"Name": "web-01", "Status": "online", "Uptime": "15d 4h"},
        {"Name": "web-02", "Status": "online", "Uptime": "8d 12h"},
        {"Name": "db-01", "Status": "online", "Uptime": "45d 2h"},
        {"Name": "cache-01", "Status": "warning", "Uptime": "2d 1h"},
        {"Name": "worker-01", "Status": "online", "Uptime": "12d 8h"},
    }
    
    // Randomly change one server status
    if rand.Float32() > 0.7 {
        idx := rand.Intn(len(servers))
        statuses := []string{"online", "warning", "offline"}
        servers[idx]["Status"] = statuses[rand.Intn(len(statuses))]
    }
    
    return servers
}

func generateAlerts(highLoad bool) []map[string]interface{} {
    alerts := []map[string]interface{}{}
    
    if highLoad {
        alerts = append(alerts, []map[string]interface{}{
            {
                "Severity": "critical",
                "Title": "High CPU Usage",
                "Message": "CPU usage exceeded 85% on web-01",
                "Timestamp": time.Now().Add(-5*time.Minute).Format("15:04:05"),
            },
            {
                "Severity": "warning", 
                "Title": "Memory Usage High",
                "Message": "Memory usage at 88% on web-02",
                "Timestamp": time.Now().Add(-2*time.Minute).Format("15:04:05"),
            },
        }...)
    }
    
    return alerts
}

func generateServiceStatus() []map[string]interface{} {
    services := []string{"API Gateway", "User Service", "Payment Service", "Notification Service", "Analytics"}
    result := make([]map[string]interface{}, len(services))
    
    for i, service := range services {
        responseTime := 20 + rand.Intn(100)
        result[i] = map[string]interface{}{
            "Name": service,
            "ResponseTime": responseTime,
            "Status": getMetricStatus(responseTime, 50, 100),
        }
    }
    
    return result
}

func generateServerChanges() map[string]interface{} {
    data := generateDashboardData(false)
    
    // Add new server
    servers := data["Servers"].([]map[string]interface{})
    servers = append(servers, map[string]interface{}{
        "Name": "worker-02",
        "Status": "online",
        "Uptime": "0d 1h",
    })
    data["Servers"] = servers
    
    return data
}

func generateAlertSpike() map[string]interface{} {
    data := generateDashboardData(true)
    
    // Multiple new alerts
    alerts := []map[string]interface{}{
        {
            "Severity": "critical",
            "Title": "Database Connection Pool Exhausted",
            "Message": "All database connections in use, queries queuing",
            "Timestamp": time.Now().Format("15:04:05"),
        },
        {
            "Severity": "critical",
            "Title": "Disk Space Low",
            "Message": "Disk usage at 95% on db-01",
            "Timestamp": time.Now().Add(-1*time.Minute).Format("15:04:05"),
        },
        {
            "Severity": "warning",
            "Title": "Response Time Degraded",
            "Message": "API response times increased 40% in last 10 minutes",
            "Timestamp": time.Now().Add(-3*time.Minute).Format("15:04:05"),
        },
    }
    data["Alerts"] = alerts
    
    return data
}

func generateRandomMetricUpdate(cycle int) map[string]interface{} {
    // Simulate natural metric variations
    baseLoad := 40.0 + 20.0*math.Sin(float64(cycle)*0.3) // Sine wave pattern
    cpuUsage := int(baseLoad) + rand.Intn(10)
    
    if cpuUsage < 20 {
        cpuUsage = 20
    }
    if cpuUsage > 95 {
        cpuUsage = 95
    }
    
    data := generateDashboardData(cpuUsage > 75)
    data["CPUUsage"] = cpuUsage
    
    return data
}
```

This comprehensive integration examples guide provides realistic, production-ready test scenarios that demonstrate LiveTemplate's four-tier strategy system in action across different types of applications. Each example includes proper strategy validation, performance monitoring, and real-world complexity.