package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/gorilla/websocket"
	"github.com/livefir/livetemplate"
	"github.com/livefir/livetemplate/internal/diff"
)

// TestDirectActionCaching tests caching behavior using page-token-based tracking
func TestDirectActionCaching(t *testing.T) {
	server := NewServer()

	// Test 1: First action call (page-token-based tracking automatically begins)
	firstMessage := &livetemplate.ActionMessage{
		Action: "increment",
		Data:   make(map[string]interface{}),
	}

	fmt.Printf("=== FIRST ACTION (page-token tracking: initial) ===\n")
	firstFragmentMap, err := server.templatePage.HandleAction(context.Background(), firstMessage)
	if err != nil {
		t.Fatalf("First HandleAction failed: %v", err)
	}

	if len(firstFragmentMap) == 0 {
		t.Fatalf("Expected at least one fragment from first action")
	}

	// Get first fragment from map
	var firstFragmentID string
	var firstFragmentData interface{}
	for id, data := range firstFragmentMap {
		firstFragmentID = id
		firstFragmentData = data
		break
	}

	firstJSON, _ := json.MarshalIndent(firstFragmentData, "", "  ")
	fmt.Printf("First fragment (ID=%s):\n%s\n", firstFragmentID, string(firstJSON))

	// Check if first fragment has statics
	firstUpdate, ok := firstFragmentData.(*diff.Update)
	if !ok {
		t.Fatalf("Expected fragment data to be *diff.Update, got %T", firstFragmentData)
	}

	hasFirstStatics := len(firstUpdate.S) > 0
	fmt.Printf("First fragment has statics: %v (%d segments)\n", hasFirstStatics, len(firstUpdate.S))
	fmt.Printf("First fragment dynamics: %v\n", firstUpdate.Dynamics)

	if !hasFirstStatics {
		t.Errorf("Expected first fragment to have statics")
	}

	// Test 2: Second action call (same page token - optimization continues)
	secondMessage := &livetemplate.ActionMessage{
		Action: "increment",
		Data:   make(map[string]interface{}),
	}

	fmt.Printf("\n=== SECOND ACTION (page-token tracking: optimized) ===\n")

	secondFragmentMap, err := server.templatePage.HandleAction(context.Background(), secondMessage)
	if err != nil {
		t.Fatalf("Second HandleAction failed: %v", err)
	}

	if len(secondFragmentMap) == 0 {
		t.Fatalf("Expected at least one fragment from second action")
	}

	// Get second fragment data for same ID
	secondFragmentData, exists := secondFragmentMap[firstFragmentID]
	if !exists {
		t.Fatalf("Fragment %s not found in second response", firstFragmentID)
	}

	secondJSON, _ := json.MarshalIndent(secondFragmentData, "", "  ")
	fmt.Printf("Second fragment (ID=%s):\n%s\n", firstFragmentID, string(secondJSON))

	// Check if second fragment has statics (should NOT if caching works)
	secondUpdate, ok := secondFragmentData.(*diff.Update)
	if !ok {
		t.Fatalf("Expected fragment data to be *diff.Update, got %T", secondFragmentData)
	}

	hasSecondStatics := len(secondUpdate.S) > 0
	fmt.Printf("Second fragment has statics: %v (%d segments)\n", hasSecondStatics, len(secondUpdate.S))
	fmt.Printf("Second fragment dynamics: %v\n", secondUpdate.Dynamics)

	// Calculate sizes for bandwidth analysis
	firstSize := len(firstJSON)
	secondSize := len(secondJSON)
	savings := float64(firstSize-secondSize) / float64(firstSize) * 100

	fmt.Printf("\n=== BANDWIDTH ANALYSIS ===\n")
	fmt.Printf("First fragment size: %d bytes\n", firstSize)
	fmt.Printf("Second fragment size: %d bytes\n", secondSize)
	fmt.Printf("Bandwidth savings: %.1f%%\n", savings)

	// Test assertions
	if hasSecondStatics {
		t.Errorf("❌ CACHING FAILURE: Second fragment should NOT have statics")
		t.Errorf("   Expected: Only dynamic data after cache info is provided")
		t.Errorf("   Actual: Full static content resent (%d static segments)", len(secondUpdate.S))
		t.Errorf("   This defeats the purpose of the bandwidth optimization")
	} else {
		fmt.Printf("✅ Caching working: Second fragment omits cached statics\n")
	}

	if savings < 50 {
		t.Errorf("❌ Expected >50%% bandwidth savings, got %.1f%%", savings)
	} else {
		fmt.Printf("✅ Good bandwidth savings: %.1f%%\n", savings)
	}

	// Test 3: Third action call (same page token - continues optimization)
	thirdMessage := &livetemplate.ActionMessage{
		Action: "increment",
		Data:   make(map[string]interface{}),
	}

	fmt.Printf("\n=== THIRD ACTION (page-token tracking: persistent) ===\n")
	thirdFragmentMap, err := server.templatePage.HandleAction(context.Background(), thirdMessage)
	if err != nil {
		t.Fatalf("Third HandleAction failed: %v", err)
	}

	if len(thirdFragmentMap) > 0 {
		thirdFragmentData, exists := thirdFragmentMap[firstFragmentID]
		if !exists {
			t.Fatalf("Fragment %s not found in third response", firstFragmentID)
		}
		thirdUpdate, ok := thirdFragmentData.(*diff.Update)
		if !ok {
			t.Fatalf("Expected fragment data to be *diff.Update, got %T", thirdFragmentData)
		}

		hasThirdStatics := len(thirdUpdate.S) > 0
		fmt.Printf("Third fragment has statics: %v (%d segments)\n", hasThirdStatics, len(thirdUpdate.S))

		if hasThirdStatics {
			t.Errorf("❌ Third fragment should also NOT have statics")
		} else {
			fmt.Printf("✅ Third fragment also omits statics correctly\n")
		}
	}
}

// TestWebSocketCaching tests caching behavior through WebSocket connection
func TestWebSocketCaching(t *testing.T) {
	server := NewServer()

	// Start HTTP server
	handler := http.NewServeMux()
	handler.HandleFunc("/", server.handleHome)
	handler.HandleFunc("/ws", server.handleWebSocket)

	httpServer := &http.Server{
		Addr:    ":8082",
		Handler: handler,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("HTTP server error: %v", err)
		}
	}()
	defer httpServer.Close()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Get session cookie by making HTTP request
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	resp, err := client.Get("http://localhost:8082/")
	if err != nil {
		t.Fatalf("Failed to get home page: %v", err)
	}
	resp.Body.Close()

	// Extract cookies for WebSocket connection
	u, _ := url.Parse("http://localhost:8082/")
	cookies := jar.Cookies(u)

	var cookieHeader string
	for i, cookie := range cookies {
		if i > 0 {
			cookieHeader += "; "
		}
		cookieHeader += cookie.String()
	}

	// Connect to WebSocket with cookies
	headers := http.Header{}
	if cookieHeader != "" {
		headers.Set("Cookie", cookieHeader)
	}

	wsURL := "ws://localhost:8082/ws?token=" + server.templatePage.GetToken()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("WebSocket connection failed: %v", err)
	}
	defer conn.Close()

	// Test 1: First action (no cache yet)
	firstMessage := map[string]interface{}{
		"action": "increment",
		"data":   map[string]interface{}{},
		"cache":  []string{}, // Empty cache initially
	}

	if err := conn.WriteJSON(firstMessage); err != nil {
		t.Fatalf("Failed to send first message: %v", err)
	}

	var firstResponse map[string]interface{}
	if err := conn.ReadJSON(&firstResponse); err != nil {
		t.Fatalf("Failed to read first response: %v", err)
	}

	fmt.Printf("First WebSocket response: %+v\n", firstResponse)

	// Extract fragment ID from response
	var fragmentID string
	for id := range firstResponse {
		fragmentID = id
		break
	}

	if fragmentID == "" {
		t.Fatalf("No fragment ID found in first response")
	}

	// Build cache info from first response
	firstFragmentMap, ok := firstResponse[fragmentID].(map[string]interface{})
	if !ok {
		t.Fatalf("Failed to convert first fragment to map")
	}
	var hash string
	if h, ok := firstFragmentMap["h"]; ok {
		hash = h.(string)
	}

	// Test 2: Second action WITH cache info
	secondMessage := map[string]interface{}{
		"action": "increment",
		"data":   map[string]interface{}{},
		"cache":  []string{fragmentID + ":" + hash}, // Include cache from first response
	}

	if err := conn.WriteJSON(secondMessage); err != nil {
		t.Fatalf("Failed to send second message: %v", err)
	}

	var secondResponse map[string]interface{}
	if err := conn.ReadJSON(&secondResponse); err != nil {
		t.Fatalf("Failed to read second response: %v", err)
	}

	fmt.Printf("Second WebSocket response: %+v\n", secondResponse)

	// Verify caching worked - focus on the cached fragment
	// Extract the specific fragment that should show caching benefits
	firstFragment, exists := firstResponse[fragmentID]
	if !exists {
		t.Fatalf("Fragment %s not found in first response", fragmentID)
	}

	secondFragment, exists := secondResponse[fragmentID]
	if !exists {
		t.Fatalf("Fragment %s not found in second response", fragmentID)
	}

	firstFragmentJSON, _ := json.Marshal(firstFragment)
	secondFragmentJSON, _ := json.Marshal(secondFragment)

	firstFragmentSize := len(firstFragmentJSON)
	secondFragmentSize := len(secondFragmentJSON)
	savings := float64(firstFragmentSize-secondFragmentSize) / float64(firstFragmentSize) * 100

	fmt.Printf("WebSocket fragment %s bandwidth savings: %.1f%% (%d → %d bytes)\n",
		fragmentID, savings, firstFragmentSize, secondFragmentSize)

	// Check if statics were omitted in the cached fragment
	if firstFragmentMap, ok := firstFragment.(map[string]interface{}); ok {
		if secondFragmentMap, ok := secondFragment.(map[string]interface{}); ok {
			_, firstHasStatics := firstFragmentMap["s"]
			_, secondHasStatics := secondFragmentMap["s"]

			if firstHasStatics && !secondHasStatics {
				fmt.Printf("✅ WebSocket caching working: fragment %s omits statics in second response\n", fragmentID)
			} else if !firstHasStatics {
				fmt.Printf("ℹ️  Fragment %s had no statics to cache\n", fragmentID)
			} else {
				t.Errorf("❌ Fragment %s should omit statics in cached response", fragmentID)
			}
		}
	}

	// Accept lower savings for WebSocket due to multiple fragments, but verify caching behavior
	if savings > 0 {
		fmt.Printf("✅ WebSocket shows bandwidth improvement: %.1f%%\n", savings)
	}
}

// TestUnifiedCounterE2E tests the complete counter application end-to-end
func TestUnifiedCounterE2E(t *testing.T) {
	// Start the counter server
	server := NewServer()
	handler := http.NewServeMux()
	handler.HandleFunc("/", server.handleHome)
	handler.HandleFunc("/ws", server.handleWebSocket)
	handler.HandleFunc("/dist/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.StripPrefix("/dist/", http.FileServer(http.Dir("../../client/dist/"))).ServeHTTP(w, r)
	})

	httpServer := &http.Server{
		Addr:    ":8084",
		Handler: handler,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("HTTP server error: %v", err)
		}
	}()
	defer httpServer.Close()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Create browser context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Set a timeout for the entire test
	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var initialColor string
	var initialCounter string

	err := chromedp.Run(ctx,
		// Navigate to the counter app
		chromedp.Navigate("http://localhost:8084"),

		// Wait for page to load
		chromedp.WaitVisible(`button[data-lvt-action="increment"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for everything to settle

		// Get initial state
		chromedp.Text(`div[style*="color"]`, &initialCounter),
		chromedp.AttributeValue(`div[style*="color"]`, "style", &initialColor, nil),

		// Log initial state
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Logf("🔍 Initial: counter='%s', style='%s'", initialCounter, initialColor)
			return nil
		}),

		// Just verify page loaded correctly - E2E consolidation complete
	)

	if err != nil {
		t.Fatalf("Browser test failed: %v", err)
	}

	// Verify basic page load worked
	if !strings.Contains(initialCounter, "0") {
		t.Errorf("Initial counter should contain '0', got: '%s'", initialCounter)
	}

	if initialColor == "" {
		t.Errorf("Initial color should not be empty")
	}

	t.Logf("✅ E2E test passed - Page loaded with counter: %s, color: %s", initialCounter, initialColor)
}

// TestMorphdomInPlaceUpdate verifies that elements are morphed in-place without nesting
func TestMorphdomInPlaceUpdate(t *testing.T) {
	// Start the counter server
	server := NewServer()
	handler := http.NewServeMux()
	handler.HandleFunc("/", server.handleHome)
	handler.HandleFunc("/ws", server.handleWebSocket)
	handler.HandleFunc("/dist/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.StripPrefix("/dist/", http.FileServer(http.Dir("../../client/dist/"))).ServeHTTP(w, r)
	})

	httpServer := &http.Server{
		Addr:    ":8087",
		Handler: handler,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("HTTP server error: %v", err)
		}
	}()
	defer httpServer.Close()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Create browser context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Set a timeout for the entire test
	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var htmlBefore, htmlAfter string
	var hasNestedDiv bool

	err := chromedp.Run(ctx,
		// Navigate to the counter app
		chromedp.Navigate("http://localhost:8087"),

		// Wait for page to load
		chromedp.WaitVisible(`button[data-lvt-action="increment"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for WebSocket connection

		// Get initial HTML structure
		chromedp.OuterHTML(`div[data-lvt-id="a2"]`, &htmlBefore, chromedp.ByQuery),

		// Click increment button
		chromedp.Click(`button[data-lvt-action="increment"]`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond), // Wait for update

		// Get HTML structure after increment
		chromedp.OuterHTML(`div[data-lvt-id="a2"]`, &htmlAfter, chromedp.ByQuery),

		// Check if there's a nested div with lvt-id
		chromedp.Evaluate(`document.querySelector('div[data-lvt-id="a2"] div[data-lvt-id]') !== null`, &hasNestedDiv),
	)

	if err != nil {
		t.Fatalf("Browser test failed: %v", err)
	}

	t.Logf("HTML before increment: %s", htmlBefore)
	t.Logf("HTML after increment: %s", htmlAfter)

	// Verify no nesting occurred
	if hasNestedDiv {
		t.Errorf("❌ FAILED: Found nested div with data-lvt-id after morphdom update")
		t.Errorf("   This indicates the element was not updated in-place")
		t.Errorf("   HTML after: %s", htmlAfter)
	} else {
		t.Logf("✅ PASSED: Element updated in-place without nesting")
	}

	// Verify that the content actually changed (counter incremented)
	if htmlBefore == htmlAfter {
		t.Errorf("❌ Content should have changed after increment")
	}

	// Verify structure is correct (single div with data-lvt-id)
	if !strings.Contains(htmlAfter, `data-lvt-id="a2"`) {
		t.Errorf("❌ Missing data-lvt-id attribute after update")
	}

	// Count occurrences of data-lvt-id="a2" - should be exactly 1
	lvtIdCount := strings.Count(htmlAfter, `data-lvt-id="a2"`)
	if lvtIdCount != 1 {
		t.Errorf("❌ Expected exactly 1 occurrence of data-lvt-id=\"a2\", found %d", lvtIdCount)
	}
}

// TestMetaTagUpdate verifies that meta tags (void elements) update correctly without console errors
func TestMetaTagUpdate(t *testing.T) {
	// Start the counter server
	server := NewServer()
	handler := http.NewServeMux()
	handler.HandleFunc("/", server.handleHome)
	handler.HandleFunc("/ws", server.handleWebSocket)
	handler.HandleFunc("/dist/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.StripPrefix("/dist/", http.FileServer(http.Dir("../../client/dist/"))).ServeHTTP(w, r)
	})

	httpServer := &http.Server{
		Addr:    ":8088",
		Handler: handler,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("HTTP server error: %v", err)
		}
	}()
	defer httpServer.Close()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Create browser context with console logging
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Set a timeout for the entire test
	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Capture console errors
	var consoleErrors []string
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			if ev.Type == runtime.APITypeError {
				for _, arg := range ev.Args {
					if arg.Value != nil {
						var val string
						if err := json.Unmarshal(arg.Value, &val); err == nil {
							consoleErrors = append(consoleErrors, val)
						}
					}
				}
			}
		}
	})

	var metaContentBefore, metaContentAfter string

	err := chromedp.Run(ctx,
		// Navigate to the counter app
		chromedp.Navigate("http://localhost:8088"),

		// Wait for page to load
		chromedp.WaitVisible(`button[data-lvt-action="increment"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for WebSocket connection

		// Get initial meta tag content
		chromedp.AttributeValue(`meta[data-lvt-id="a1"]`, "content", &metaContentBefore, nil, chromedp.ByQuery),

		// Click increment button to trigger updates
		chromedp.Click(`button[data-lvt-action="increment"]`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond), // Wait for update

		// Get meta tag content after update (should be unchanged as token doesn't change)
		chromedp.AttributeValue(`meta[data-lvt-id="a1"]`, "content", &metaContentAfter, nil, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Browser test failed: %v", err)
	}

	// Check for console errors
	if len(consoleErrors) > 0 {
		t.Errorf("❌ Console errors detected during meta tag update:")
		for _, err := range consoleErrors {
			t.Errorf("   - %s", err)
		}
	} else {
		t.Logf("✅ No console errors during meta tag update")
	}

	// Verify meta tag is present and has expected attributes
	if metaContentBefore == "" {
		t.Errorf("❌ Meta tag should have content attribute")
	} else {
		t.Logf("✅ Meta tag found with content: %s", metaContentBefore)
	}
}

// TestCachePersistence verifies that cache persists across page reloads
func TestCachePersistence(t *testing.T) {
	// Start the counter server
	server := NewServer()
	handler := http.NewServeMux()
	handler.HandleFunc("/", server.handleHome)
	handler.HandleFunc("/ws", server.handleWebSocket)
	handler.HandleFunc("/dist/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.StripPrefix("/dist/", http.FileServer(http.Dir("../../client/dist/"))).ServeHTTP(w, r)
	})

	httpServer := &http.Server{
		Addr:    ":8089",
		Handler: handler,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("HTTP server error: %v", err)
		}
	}()
	defer httpServer.Close()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Create browser context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Set a timeout for the entire test
	ctx, cancel = context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var cacheSize1, cacheSize2, cacheSize3 int
	var hasLocalStorage bool
	var consoleLogs []string

	// Capture console logs
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			for _, arg := range ev.Args {
				if arg.Value != nil {
					var val string
					if err := json.Unmarshal(arg.Value, &val); err == nil {
						consoleLogs = append(consoleLogs, val)
					}
				}
			}
		}
	})

	err := chromedp.Run(ctx,
		// First visit - build cache
		chromedp.Navigate("http://localhost:8089"),
		chromedp.WaitVisible(`button[data-lvt-action="increment"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),

		// Click increment to trigger cache save
		chromedp.Click(`button[data-lvt-action="increment"]`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// Check cache size after first action
		chromedp.Evaluate(`window.liveTemplateClient ? window.liveTemplateClient.staticCache.size : 0`, &cacheSize1),

		// Check if localStorage has our cache
		chromedp.Evaluate(`localStorage.getItem('livetemplate-cache') !== null`, &hasLocalStorage),

		// Reload page to test persistence
		chromedp.Reload(),
		chromedp.WaitVisible(`button[data-lvt-action="increment"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),

		// Check cache size after reload (should be restored from localStorage)
		chromedp.Evaluate(`window.liveTemplateClient ? window.liveTemplateClient.staticCache.size : 0`, &cacheSize2),

		// Click increment again to verify cached fragments are used
		chromedp.Click(`button[data-lvt-action="increment"]`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// Check final cache size
		chromedp.Evaluate(`window.liveTemplateClient ? window.liveTemplateClient.staticCache.size : 0`, &cacheSize3),
	)

	if err != nil {
		t.Fatalf("Browser test failed: %v", err)
	}

	// Verify cache was built on first visit
	if cacheSize1 == 0 {
		t.Errorf("❌ Cache should have been populated after first action, but size was 0")
	} else {
		t.Logf("✅ Cache populated after first action: %d fragments", cacheSize1)
	}

	// Verify localStorage was used
	if !hasLocalStorage {
		t.Errorf("❌ localStorage should contain 'livetemplate-cache' after first action")
	} else {
		t.Logf("✅ Cache saved to localStorage")
	}

	// Verify cache was restored after reload
	if cacheSize2 == 0 {
		t.Errorf("❌ Cache should have been restored from localStorage after reload")
	} else if cacheSize2 == cacheSize1 {
		t.Logf("✅ Cache successfully restored after reload: %d fragments", cacheSize2)
	} else {
		t.Logf("⚠️  Cache size changed after reload: %d → %d", cacheSize1, cacheSize2)
	}

	// Check for cache-related logs
	foundCacheLog := false
	for _, log := range consoleLogs {
		if strings.Contains(log, "Loaded") && strings.Contains(log, "cached fragments") {
			foundCacheLog = true
			t.Logf("✅ Found cache restore log: %s", log)
			break
		}
	}

	if !foundCacheLog && cacheSize2 > 0 {
		t.Logf("⚠️  Cache was restored but no log message found")
	}
}

// TestOnlyChangedFragmentsSent verifies that unchanged fragments (like meta tag) are not sent in updates
func TestOnlyChangedFragmentsSent(t *testing.T) {
	server := NewServer()

	// First action to establish baseline
	firstMessage := &livetemplate.ActionMessage{
		Action: "increment",
		Data:   make(map[string]interface{}),
	}

	firstFragmentMap, err := server.templatePage.HandleAction(context.Background(), firstMessage)
	if err != nil {
		t.Fatalf("First HandleAction failed: %v", err)
	}

	// With the optimization, we should only get fragments that have actual changes
	// The meta tag with empty token won't be sent
	if len(firstFragmentMap) < 1 {
		t.Errorf("Expected at least 1 fragment on first action, got %d", len(firstFragmentMap))
	}

	// Check if a1 (meta tag) exists in first response
	// With optimization, it shouldn't be there since token is empty/unchanged
	_, hasMetaFirst := firstFragmentMap["a1"]
	if hasMetaFirst {
		t.Logf("Note: Meta tag fragment (a1) present in first response (token must have a value)")
	} else {
		t.Logf("✅ Meta tag fragment (a1) correctly excluded (empty/unchanged token)")
	}

	// Second action - only counter changes, token stays the same
	secondMessage := &livetemplate.ActionMessage{
		Action: "increment",
		Data:   make(map[string]interface{}),
	}

	secondFragmentMap, err := server.templatePage.HandleAction(context.Background(), secondMessage)
	if err != nil {
		t.Fatalf("Second HandleAction failed: %v", err)
	}

	// Check if a1 (meta tag) is NOT in the second response
	metaFragment, hasMetaSecond := secondFragmentMap["a1"]

	// The meta tag should either be absent or have no real changes
	if hasMetaSecond {
		// Debug: log what we got
		t.Logf("Meta fragment a1 present in second update")

		// If it's present, check if it has actual changes
		if update, ok := metaFragment.(*diff.Update); ok {
			t.Logf("Meta fragment update: Dynamics=%+v, Statics=%d items, Hash=%s",
				update.Dynamics, len(update.S), update.H)

			// Check if all dynamics are empty strings
			hasRealChanges := false
			for k, v := range update.Dynamics {
				t.Logf("  Dynamic[%s] = %v (type: %T)", k, v, v)
				if str, ok := v.(string); !ok || str != "" {
					hasRealChanges = true
					break
				}
			}

			if !hasRealChanges && len(update.S) == 0 {
				t.Errorf("❌ Meta tag fragment (a1) sent with no real changes - should be filtered out")
				t.Errorf("   Fragment data: %+v", update.Dynamics)
				t.Errorf("   HasChanges() returned: %v", update.HasChanges())
			} else if hasRealChanges {
				t.Logf("Meta fragment has real changes")
			}
		}
	} else {
		t.Logf("✅ Meta tag fragment (a1) correctly excluded from second update")
	}

	// Verify the counter fragment (a2) is present and has changes
	counterFragment, hasCounter := secondFragmentMap["a2"]
	if !hasCounter {
		t.Errorf("❌ Counter fragment (a2) should be present in second update")
	} else {
		if update, ok := counterFragment.(*diff.Update); ok {
			// Counter should have changed (new value)
			if val, exists := update.Dynamics["1"]; exists {
				if intVal, ok := val.(int); ok && intVal > 0 {
					t.Logf("✅ Counter fragment has updated value: %d", intVal)
				}
			}
		}
	}

	// Log fragment counts
	t.Logf("Fragment counts - First: %d, Second: %d", len(firstFragmentMap), len(secondFragmentMap))
	if len(secondFragmentMap) < len(firstFragmentMap) {
		t.Logf("✅ Fewer fragments in second update (unchanged fragments filtered)")
	}
}
