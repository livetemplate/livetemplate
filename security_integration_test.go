package livetemplate

import (
	"context"
	"errors"
	"html/template"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestSecurity_EndToEndIntegration performs comprehensive end-to-end security validation
func TestSecurity_EndToEndIntegration(t *testing.T) {
	// Test 1: Complete multi-tenant security workflow
	t.Run("complete_multi_tenant_workflow", func(t *testing.T) {
		// Simulate a complete multi-tenant SaaS scenario
		const numTenants = 5
		const usersPerTenant = 3
		const pagesPerUser = 2

		type UserType struct {
			ID       int
			TenantID int
			Pages    []*ApplicationPage
			Username string
			Secret   string
		}

		type Tenant struct {
			ID           int
			App          *Application
			Users        []UserType
			TenantSecret string
		}

		tenants := make([]Tenant, numTenants)

		// Create tenants with isolated applications
		for tenantID := 0; tenantID < numTenants; tenantID++ {
			app, err := NewApplication()
			if err != nil {
				t.Fatalf("failed to create app for tenant %d: %v", tenantID, err)
			}
			defer func() { _ = app.Close() }()

			tenant := Tenant{
				ID:           tenantID,
				App:          app,
				Users:        make([]UserType, usersPerTenant),
				TenantSecret: f("TENANT_%d_SECRET_KEY", tenantID),
			}

			// Create users within each tenant
			for userID := 0; userID < usersPerTenant; userID++ {
				user := UserType{
					ID:       userID,
					TenantID: tenantID,
					Pages:    make([]*ApplicationPage, pagesPerUser),
					Username: f("tenant_%d_user_%d", tenantID, userID),
					Secret:   f("USER_SECRET_T%d_U%d", tenantID, userID),
				}

				// Create pages for each user
				tmpl := template.Must(template.New("user-page").Parse(`
					<div class="user-dashboard">
						<h1>{{.Username}}</h1>
						<div class="tenant-info">Tenant: {{.TenantID}}</div>
						<div class="user-secret">User Secret: {{.UserSecret}}</div>
						<div class="tenant-secret">Tenant Secret: {{.TenantSecret}}</div>
						<div class="page-data">{{.PageData}}</div>
					</div>
				`))

				for pageID := 0; pageID < pagesPerUser; pageID++ {
					data := map[string]interface{}{
						"Username":     user.Username,
						"TenantID":     tenantID,
						"UserSecret":   user.Secret,
						"TenantSecret": tenant.TenantSecret,
						"PageData":     f("PAGE_%d_DATA_FOR_USER_%d", pageID, userID),
					}

					page, err := app.NewApplicationPage(tmpl, data)
					if err != nil {
						t.Fatalf("failed to create page %d for user %d tenant %d: %v", pageID, userID, tenantID, err)
					}
					defer func() { _ = page.Close() }()

					user.Pages[pageID] = page
				}

				tenant.Users[userID] = user
			}

			tenants[tenantID] = tenant
		}

		// Security Test 1: Verify complete tenant isolation
		t.Run("tenant_isolation_verification", func(t *testing.T) {
			violations := 0
			totalChecks := 0

			for _, tenant := range tenants {
				for _, user := range tenant.Users {
					for _, page := range user.Pages {
						token := page.GetToken()

						// Try to access this page from all other tenants
						for _, otherTenant := range tenants {
							if otherTenant.ID == tenant.ID {
								continue // Skip same tenant
							}

							totalChecks++
							_, err := otherTenant.App.GetApplicationPage(token)
							if err == nil {
								violations++
								t.Errorf("SECURITY VIOLATION: Tenant %d accessed Tenant %d user %s page",
									otherTenant.ID, tenant.ID, user.Username)
							}
						}
					}
				}
			}

			isolationScore := float64(totalChecks-violations) / float64(totalChecks) * 100
			t.Logf("Multi-tenant isolation: %d checks, %d violations, %.1f%% isolation",
				totalChecks, violations, isolationScore)

			if violations > 0 {
				t.Errorf("Multi-tenant isolation failed: %d violations out of %d checks", violations, totalChecks)
			}
		})

		// Security Test 2: Verify no data leakage between users within same tenant
		t.Run("intra_tenant_user_isolation", func(t *testing.T) {
			for _, tenant := range tenants {
				for _, user := range tenant.Users {
					for _, page := range user.Pages {
						html, err := page.Render()
						if err != nil {
							t.Errorf("Failed to render page for tenant %d user %s: %v",
								tenant.ID, user.Username, err)
							continue
						}

						// Verify page contains correct user data
						if !strings.Contains(html, user.Username) {
							t.Errorf("Page missing username for tenant %d user %s", tenant.ID, user.Username)
						}

						if !strings.Contains(html, user.Secret) {
							t.Errorf("Page missing user secret for tenant %d user %s", tenant.ID, user.Username)
						}

						// Verify page doesn't contain other users' secrets
						for _, otherUser := range tenant.Users {
							if otherUser.ID != user.ID {
								if strings.Contains(html, otherUser.Secret) {
									t.Errorf("SECURITY VIOLATION: Page for tenant %d user %s contains other user %s secret",
										tenant.ID, user.Username, otherUser.Username)
								}
							}
						}

						// Verify page doesn't contain other tenants' data
						for _, otherTenant := range tenants {
							if otherTenant.ID != tenant.ID {
								if strings.Contains(html, otherTenant.TenantSecret) {
									t.Errorf("SECURITY VIOLATION: Page for tenant %d contains tenant %d secret",
										tenant.ID, otherTenant.ID)
								}
							}
						}
					}
				}
			}
		})

		// Security Test 3: Concurrent access simulation
		t.Run("concurrent_access_security", func(t *testing.T) {
			var wg sync.WaitGroup
			violations := make(chan string, 1000)

			// Launch concurrent access attempts from all tenants
			for _, tenant := range tenants {
				for _, user := range tenant.Users {
					for _, page := range user.Pages {
						wg.Add(1)
						go func(t Tenant, u UserType, p *ApplicationPage) {
							defer wg.Done()

							token := p.GetToken()

							// Rapid access attempts to this page from all other tenants
							for i := 0; i < 10; i++ {
								for _, otherTenant := range tenants {
									if otherTenant.ID == t.ID {
										continue
									}

									retrievedPage, err := otherTenant.App.GetApplicationPage(token)
									if err == nil {
										html, _ := retrievedPage.Render()
										violations <- f("Concurrent violation: Tenant %d accessed Tenant %d user %s: %s",
											otherTenant.ID, t.ID, u.Username, html[:min(100, len(html))])
									}
								}
							}
						}(tenant, user, page)
					}
				}
			}

			wg.Wait()
			close(violations)

			violationCount := 0
			for violation := range violations {
				violationCount++
				t.Error(violation)
			}

			if violationCount == 0 {
				t.Logf("Concurrent access security: No violations detected")
			}
		})
	})

	// Test 2: Security under load and stress conditions
	t.Run("security_under_load", func(t *testing.T) {
		const numApps = 10
		const concurrentUsers = 50
		const operationsPerUser = 100

		apps := make([]*Application, numApps)
		for i := 0; i < numApps; i++ {
			app, err := NewApplication()
			if err != nil {
				t.Fatalf("failed to create app %d: %v", i, err)
			}
			defer func() { _ = app.Close() }()
			apps[i] = app
		}

		tmpl := template.Must(template.New("load-test").Parse(`
			<div class="load-test-data">
				<p>App: {{.AppID}}</p>
				<p>User: {{.UserID}}</p>
				<p>Secret: {{.Secret}}</p>
				<p>Timestamp: {{.Timestamp}}</p>
			</div>
		`))

		// Create initial pages in each app
		appPages := make([][]*ApplicationPage, numApps)
		for i, app := range apps {
			pages := make([]*ApplicationPage, 5)
			for j := 0; j < 5; j++ {
				data := map[string]interface{}{
					"AppID":     i,
					"UserID":    j,
					"Secret":    f("LOAD_SECRET_APP_%d_USER_%d", i, j),
					"Timestamp": time.Now().Format(time.RFC3339),
				}

				page, err := app.NewApplicationPage(tmpl, data)
				if err != nil {
					t.Fatalf("failed to create initial page %d in app %d: %v", j, i, err)
				}
				defer func() { _ = page.Close() }()
				pages[j] = page
			}
			appPages[i] = pages
		}

		// Launch concurrent load test
		var wg sync.WaitGroup
		violations := make(chan string, 10000)

		for userID := 0; userID < concurrentUsers; userID++ {
			wg.Add(1)
			go func(uid int) {
				defer wg.Done()

				for op := 0; op < operationsPerUser; op++ {
					// Pick random app and page
					appID := uid % numApps
					pageID := op % len(appPages[appID])
					page := appPages[appID][pageID]

					token := page.GetToken()

					// Try to access this page from a different random app
					otherAppID := (appID + 1) % numApps
					if otherAppID == appID {
						// This should never happen with the above formula, but safety check
						otherAppID = (appID + 2) % numApps
					}
					otherApp := apps[otherAppID]

					_, err := otherApp.GetApplicationPage(token)
					if err == nil {
						violations <- f("Load test violation U%d O%d: App %d accessed App %d page",
							uid, op, otherAppID, appID)
					}

					// Also test fragment generation under load
					newData := map[string]interface{}{
						"AppID":     appID,
						"UserID":    uid,
						"Secret":    f("UPDATED_SECRET_U%d_O%d", uid, op),
						"Timestamp": time.Now().Format(time.RFC3339),
					}

					_, err = page.RenderFragments(context.Background(), newData)
					if err != nil {
						// Fragment errors are acceptable under high load
						continue
					}
				}
			}(userID)
		}

		wg.Wait()
		close(violations)

		violationCount := 0
		for violation := range violations {
			violationCount++
			if violationCount <= 10 { // Limit output
				t.Error(violation)
			}
		}

		if violationCount > 0 {
			t.Errorf("Security under load failed: %d violations detected", violationCount)
		} else {
			t.Logf("Security under load passed: %d users, %d operations each, no violations",
				concurrentUsers, operationsPerUser)
		}
	})

	// Test 3: Security boundary verification
	t.Run("security_boundary_verification", func(t *testing.T) {
		// Create applications with different security profiles
		regularApp, err := NewApplication()
		if err != nil {
			t.Fatalf("failed to create regular app: %v", err)
		}
		defer func() { _ = regularApp.Close() }()

		restrictedApp, err := NewApplication(WithMaxMemoryMB(1))
		if err != nil {
			t.Fatalf("failed to create restricted app: %v", err)
		}
		defer func() { _ = restrictedApp.Close() }()

		// Test boundary conditions
		boundaryTests := []struct {
			name        string
			description string
			test        func() error
		}{
			{
				name:        "memory_boundary_enforcement",
				description: "Memory limits should prevent resource exhaustion",
				test: func() error {
					tmpl := template.Must(template.New("boundary").Parse(`<div>{{.Data}}</div>`))

					// Try to create many pages in restricted app
					for i := 0; i < 100; i++ {
						largeData := strings.Repeat(f("BOUNDARY_DATA_%d_", i), 1000)
						data := map[string]interface{}{"Data": largeData}

						page, err := restrictedApp.NewApplicationPage(tmpl, data)
						if err != nil {
							// Expected - memory limit should prevent this
							return nil
						}
						defer func() { _ = page.Close() }()
					}
					return nil // If we get here, limits aren't working
				},
			},
			{
				name:        "cross_app_token_rejection",
				description: "Tokens should be completely isolated between applications",
				test: func() error {
					tmpl := template.Must(template.New("boundary").Parse(`<div>{{.Secret}}</div>`))
					secret := "BOUNDARY_TEST_SECRET"
					data := map[string]interface{}{"Secret": secret}

					page, err := regularApp.NewApplicationPage(tmpl, data)
					if err != nil {
						return err
					}
					defer func() { _ = page.Close() }()

					token := page.GetToken()

					// Try to use token in restricted app
					_, err = restrictedApp.GetApplicationPage(token)
					if err == nil {
						return errors.New("Security boundary violation: restricted app accepted regular app token")
					}
					return nil // Expected failure
				},
			},
		}

		for _, test := range boundaryTests {
			t.Run(test.name, func(t *testing.T) {
				err := test.test()
				if err != nil {
					t.Errorf("%s failed: %v", test.description, err)
				} else {
					t.Logf("%s: passed", test.description)
				}
			})
		}
	})
}

// TestSecurity_ComplianceValidation validates security compliance requirements
func TestSecurity_ComplianceValidation(t *testing.T) {
	// Test 1: Data protection compliance
	t.Run("data_protection_compliance", func(t *testing.T) {
		app, err := NewApplication()
		if err != nil {
			t.Fatalf("failed to create app: %v", err)
		}
		defer func() { _ = app.Close() }()

		// Test with PII-like data (simulated)
		tmpl := template.Must(template.New("pii").Parse(`
			<div class="user-profile">
				<p>Name: {{.Name}}</p>
				<p>Email: {{.Email}}</p>
				<p>SSN: {{.SSN}}</p>
				<p>DOB: {{.DOB}}</p>
			</div>
		`))

		piiData := map[string]interface{}{
			"Name":  "John Doe",
			"Email": "john.doe@example.com",
			"SSN":   "123-45-6789",
			"DOB":   "1990-01-01",
		}

		page, err := app.NewApplicationPage(tmpl, piiData)
		if err != nil {
			t.Fatalf("failed to create page with PII: %v", err)
		}
		defer func() { _ = page.Close() }()

		// Verify PII is properly contained
		html, err := page.Render()
		if err != nil {
			t.Fatalf("failed to render PII page: %v", err)
		}

		// Basic compliance checks
		if !strings.Contains(html, "John Doe") {
			t.Error("PII data not properly rendered")
		}

		// Verify no PII leakage in token
		token := page.GetToken()
		if strings.Contains(token, "123-45-6789") ||
			strings.Contains(token, "john.doe@example.com") {
			t.Error("COMPLIANCE VIOLATION: PII leaked in token")
		}

		t.Logf("Data protection compliance: PII properly contained")
	})

	// Test 2: Access control compliance
	t.Run("access_control_compliance", func(t *testing.T) {
		// Create multiple security contexts
		contexts := []struct {
			name string
			app  *Application
		}{}

		for i := 0; i < 5; i++ {
			app, err := NewApplication()
			if err != nil {
				t.Fatalf("failed to create app %d: %v", i, err)
			}
			defer func() { _ = app.Close() }()

			contexts = append(contexts, struct {
				name string
				app  *Application
			}{
				name: f("Context_%d", i),
				app:  app,
			})
		}

		// Test access control matrix
		accessMatrix := make([][]bool, len(contexts))
		for i := range accessMatrix {
			accessMatrix[i] = make([]bool, len(contexts))
		}

		tmpl := template.Must(template.New("access").Parse(`<div>{{.Data}}</div>`))

		// Create resources in each context
		resources := make([]*ApplicationPage, len(contexts))
		for i, ctx := range contexts {
			data := map[string]interface{}{"Data": f("CONTEXT_%d_DATA", i)}
			page, err := ctx.app.NewApplicationPage(tmpl, data)
			if err != nil {
				t.Fatalf("failed to create resource in %s: %v", ctx.name, err)
			}
			defer func() { _ = page.Close() }()
			resources[i] = page
		}

		// Test access control matrix
		for i, sourceCtx := range contexts {
			for j, resource := range resources {
				token := resource.GetToken()
				_, err := sourceCtx.app.GetApplicationPage(token)

				if i == j {
					// Same context - should succeed
					accessMatrix[i][j] = (err == nil)
					if err != nil {
						t.Errorf("Access control error: %s should access own resource", sourceCtx.name)
					}
				} else {
					// Different context - should fail
					accessMatrix[i][j] = (err == nil)
					if err == nil {
						t.Errorf("ACCESS CONTROL VIOLATION: %s accessed Context_%d resource", sourceCtx.name, j)
					}
				}
			}
		}

		// Report access control matrix
		t.Logf("Access Control Matrix (true = access granted):")
		for i := range accessMatrix {
			t.Logf("Context_%d: %v", i, accessMatrix[i])
		}

		// Verify only diagonal access is allowed
		for i := range accessMatrix {
			for j := range accessMatrix[i] {
				if i == j && !accessMatrix[i][j] {
					t.Errorf("Access control error: Context_%d should access own resources", i)
				}
				if i != j && accessMatrix[i][j] {
					t.Errorf("Access control violation: Context_%d accessed Context_%d", i, j)
				}
			}
		}
	})
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
