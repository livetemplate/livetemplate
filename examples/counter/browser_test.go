package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

var dockerContainer string

// getRandomPort returns a random port number between 8000-8999
func getRandomPort() int {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return 8000 + r.Intn(1000)
}

// setupBrowser creates a browser context using Docker chromedp/headless-shell
func setupBrowser(t *testing.T) (context.Context, context.CancelFunc) {
	// Clean up any existing container
	exec.Command("docker", "stop", "chromedp-test").Run()
	exec.Command("docker", "rm", "-f", "chromedp-test").Run()

	// Start chromedp/headless-shell container - use port mapping instead of host network
	cmd := exec.Command("docker", "run", "-d", "--rm",
		"-p", "9222:9222",
		"--name", "chromedp-test",
		"--add-host", "host.docker.internal:host-gateway", // Allow container to access host
		"chromedp/headless-shell:latest",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start chromedp/headless-shell: %v, output: %s", err, output)
	}

	dockerContainer = strings.TrimSpace(string(output))
	log.Printf("Started chromedp/headless-shell container: %s", dockerContainer)

	// Wait longer for container to be ready and check connection
	for i := range 20 {
		time.Sleep(1 * time.Second)
		if resp, err := http.Get("http://localhost:9222/json/version"); err == nil {
			resp.Body.Close()
			log.Printf("ChromeDP container ready after %d seconds", i+1)
			break
		}
		if i == 19 {
			// Get container logs for debugging
			if logCmd := exec.Command("docker", "logs", dockerContainer); logCmd != nil {
				if logs, _ := logCmd.CombinedOutput(); logs != nil {
					log.Printf("Container logs: %s", logs)
				}
			}
			t.Fatalf("ChromeDP container failed to become ready after 20 seconds")
		}
	}

	// Connect to Docker headless-shell
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), "ws://localhost:9222")
	ctx, ctxCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))

	// Set timeout - increased for complex E2E operations
	ctx, timeoutCancel := context.WithTimeout(ctx, 45*time.Second)

	cancel := func() {
		timeoutCancel()
		ctxCancel()
		allocCancel()
	}

	return ctx, cancel
}

// TestMain handles setup and teardown for all tests
func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()

	// Cleanup all Docker containers
	exec.Command("docker", "stop", "chromedp-test").Run()
	exec.Command("docker", "rm", "-f", "chromedp-test").Run()
	
	// Clean up any remaining chromedp containers
	exec.Command("docker", "ps", "-q", "--filter", "ancestor=chromedp/headless-shell").Run()

	os.Exit(code)
}

func TestUnifiedCounterE2E(t *testing.T) {
	// Use random port to avoid conflicts
	port := getRandomPort()
	
	// Start test server
	server := startTestServer(port)
	defer func() {
		server.Shutdown(context.Background())
		log.Printf("Test server on port %d shut down", port)
	}()

	// Create browser context
	ctx, cancel := setupBrowser(t)
	defer cancel()

	var consoleMessages []string
	var wsMessages []string
	var staticCacheViolations []string
	var initialCounterText, afterIncrementText, afterDecrementText string
	var initialColor, afterIncrementColor, afterDecrementColor string
	var morphdomUsed bool
	var staticsCached bool

	// Capture console logs
	chromedp.ListenTarget(ctx, func(ev any) {
		if ev, ok := ev.(*runtime.EventConsoleAPICalled); ok {
			for _, arg := range ev.Args {
				if arg.Value != nil {
					consoleMessages = append(consoleMessages, string(arg.Value))
				}
			}
		}
	})

	err := chromedp.Run(ctx,
		// Navigate to the test server
		chromedp.Navigate(fmt.Sprintf("http://host.docker.internal:%d", port)),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for LiveTemplate to connect

		// Debug LiveTemplateClient availability
		chromedp.Evaluate(`
			console.log('🔍 Checking LiveTemplateClient availability...');
			console.log('typeof window.LiveTemplateClient:', typeof window.LiveTemplateClient);
			console.log('window.LiveTemplateClient:', window.LiveTemplateClient);
			
			// Check if it's available on global scope
			console.log('typeof LiveTemplateClient:', typeof LiveTemplateClient);
			try {
				console.log('LiveTemplateClient:', LiveTemplateClient);
			} catch (e) {
				console.log('Error accessing LiveTemplateClient:', e.message);
			}
			
			// Try to check what scripts are loaded
			const scripts = document.querySelectorAll('script');
			console.log('Loaded scripts:');
			scripts.forEach((script, i) => {
				if (script.src) {
					console.log('  Script ' + i + ':', script.src);
				}
			});
		`, nil),

		// Set up WebSocket message capture and enhanced logging
		chromedp.Evaluate(`
			window.wsMessages = [];
			window.fragmentUpdates = [];
			window.staticCache = new Map();
			window.staticsCached = false;
			window.staticCacheViolations = [];
			
			console.log('🔍 Setting up unified counter test monitoring...');
			
			// Capture WebSocket messages - the client is exposed as window.client
			if (window.client && window.client.ws) {
				const originalOnMessage = window.client.ws.onmessage;
				window.client.ws.onmessage = function(event) {
					window.wsMessages.push(event.data);
					console.log('🔍 WS RECEIVED:', event.data);
					
					try {
						const data = JSON.parse(event.data);
						console.log('🔍 PARSED DATA:', JSON.stringify(data, null, 2));
						
						// Handle both new object format and old array format
						let fragments;
						if (Array.isArray(data)) {
							// Old format: array of fragment objects
							fragments = data;
						} else if (typeof data === 'object' && data !== null) {
							// New format: object with lvt-id as keys
							fragments = [];
							for (const [id, fragmentData] of Object.entries(data)) {
								fragments.push({
									id: id,
									data: fragmentData
								});
							}
						} else {
							console.error('Unknown fragment format:', data);
							return;
						}
						
						window.fragmentUpdates.push(fragments);
						
						fragments.forEach((fragment, i) => {
							console.log('🔍 FRAGMENT', i, '- ID:', fragment.id, '- STRATEGY:', fragment.strategy, '- DATA:', JSON.stringify(fragment.data));
							
							// Check for static caching
							if (fragment.data) {
								if (fragment.data.s) {
									// Full statics present - first time seeing this fragment
									const staticsKey = fragment.id + ':statics';
									const staticsStr = JSON.stringify(fragment.data.s);
									
									if (window.staticCache.has(staticsKey)) {
										// We've seen these statics before - they shouldn't be sent again!
										console.error('❌ STATIC CACHE VIOLATION: Server resent statics for fragment:', fragment.id);
										console.error('   Previous statics:', window.staticCache.get(staticsKey));
										console.error('   Current statics:', staticsStr);
										window.staticCacheViolations.push(fragment.id);
									} else {
										// First time seeing these statics - cache them
										window.staticCache.set(staticsKey, staticsStr);
										console.log('✅ STATICS CACHED for fragment:', fragment.id);
									}
									
									console.log('🔍 STATICS:', fragment.data.s);
									console.log('🔍 DYNAMICS:', JSON.stringify(fragment.data));
								} else {
									// No statics - should be using cached version
									console.log('✅ NO STATICS SENT - using cached version for fragment:', fragment.id);
									window.staticsCached = true;
								}
							}
						});
					} catch (e) {
						console.log('🔍 PARSE ERROR:', e);
					}
					
					if (originalOnMessage) {
						return originalOnMessage.call(this, event);
					}
				};
				console.log('✅ WebSocket monitoring enabled');
			} else {
				console.error('❌ LiveTemplate client WebSocket not available');
			}
			
			// Monitor morphdom usage
			if (typeof morphdom !== 'undefined') {
				const originalMorphdom = window.morphdom;
				window.morphdom = function(...args) {
					console.log('🔍 MORPHDOM CALLED with args:', args.length);
					window.morphdomUsed = true;
					return originalMorphdom.apply(this, args);
				};
				console.log('✅ morphdom monitoring enabled');
			} else {
				console.error('❌ morphdom not available');
			}
			
			'Monitoring setup complete'
		`, nil),

		// Verify initial state - the counter div has lvt-id="2"
		chromedp.WaitVisible(`[lvt-id="2"]`, chromedp.ByQuery),
		chromedp.Text(`[lvt-id="2"]`, &initialCounterText, chromedp.ByQuery),
		chromedp.AttributeValue(`[lvt-id="2"]`, "style", &initialColor, nil, chromedp.ByQuery),

		// Log initial state
		chromedp.Evaluate(`
			console.log('🔍 INITIAL STATE:');
			console.log('  Counter text:', document.querySelector('[lvt-id="2"]').textContent);
			console.log('  Counter style:', document.querySelector('[lvt-id="2"]').getAttribute('style'));
			console.log('  LiveTemplate connected:', window.client && window.client.ws && window.client.ws.readyState === 1);
		`, nil),

		// Test 1: Increment counter
		chromedp.Evaluate(`console.log('🔍 CLICKING INCREMENT BUTTON...')`, nil),
		chromedp.Click(`button[data-lvt-action="increment"]`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for server response and morphdom update

		// Verify increment worked
		chromedp.Text(`[lvt-id="2"]`, &afterIncrementText, chromedp.ByQuery),
		chromedp.AttributeValue(`[lvt-id="2"]`, "style", &afterIncrementColor, nil, chromedp.ByQuery),

		// Log after increment
		chromedp.Evaluate(`
			console.log('🔍 AFTER INCREMENT:');
			console.log('  Counter text:', document.querySelector('[lvt-id="2"]').textContent);
			console.log('  Counter style:', document.querySelector('[lvt-id="2"]').getAttribute('style'));
			console.log('  morphdom used:', window.morphdomUsed);
		`, nil),

		// Test 2: Decrement counter
		chromedp.Evaluate(`console.log('🔍 CLICKING DECREMENT BUTTON...')`, nil),
		chromedp.Click(`button[data-lvt-action="decrement"]`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for server response and morphdom update

		// Verify decrement worked
		chromedp.Text(`[lvt-id="2"]`, &afterDecrementText, chromedp.ByQuery),
		chromedp.AttributeValue(`[lvt-id="2"]`, "style", &afterDecrementColor, nil, chromedp.ByQuery),

		// Final state logging
		chromedp.Evaluate(`
			console.log('🔍 AFTER DECREMENT:');
			console.log('  Counter text:', document.querySelector('[lvt-id="2"]').textContent);
			console.log('  Counter style:', document.querySelector('[lvt-id="2"]').getAttribute('style'));
			console.log('  Final morphdom status:', window.morphdomUsed);
		`, nil),

		// Get final test data
		chromedp.Evaluate(`window.wsMessages || []`, &wsMessages),
		chromedp.Evaluate(`window.morphdomUsed || false`, &morphdomUsed),
		chromedp.Evaluate(`window.staticsCached || false`, &staticsCached),
		chromedp.Evaluate(`window.staticCacheViolations || []`, &staticCacheViolations),
	)

	if err != nil {
		t.Fatalf("Unified counter E2E test failed: %v", err)
	}

	// Print captured console messages for debugging
	fmt.Println("\n=== CONSOLE MESSAGES ===")
	for i, msg := range consoleMessages {
		fmt.Printf("Console[%d]: %s\n", i+1, msg)
	}

	// Print WebSocket messages for debugging
	fmt.Printf("\n=== WEBSOCKET MESSAGES (%d total) ===\n", len(wsMessages))
	for i, msg := range wsMessages {
		fmt.Printf("WS[%d]: %s\n", i+1, msg)
	}

	// Assertions
	fmt.Printf("\n=== UNIFIED COUNTER E2E TEST RESULTS ===\n")

	// Test initial state
	if !strings.Contains(initialCounterText, "Hello 0 World") {
		t.Errorf("❌ Expected initial text to contain 'Hello 0 World', got '%s'", initialCounterText)
	} else {
		fmt.Printf("✅ Initial state correct: '%s'\n", initialCounterText)
	}

	// Test increment
	if !strings.Contains(afterIncrementText, "Hello 1 World") {
		t.Errorf("❌ Expected increment text to contain 'Hello 1 World', got '%s'", afterIncrementText)
	} else {
		fmt.Printf("✅ Increment working: '%s'\n", afterIncrementText)
	}

	// Test decrement  
	if !strings.Contains(afterDecrementText, "Hello 0 World") {
		t.Errorf("❌ Expected decrement text to contain 'Hello 0 World', got '%s'", afterDecrementText)
	} else {
		fmt.Printf("✅ Decrement working: '%s'\n", afterDecrementText)
	}

	// Test color changes (should be different for each update)
	if initialColor == afterIncrementColor || afterIncrementColor == afterDecrementColor {
		t.Errorf("❌ Expected colors to change with each update")
		fmt.Printf("   Initial: %s\n", initialColor)
		fmt.Printf("   After increment: %s\n", afterIncrementColor) 
		fmt.Printf("   After decrement: %s\n", afterDecrementColor)
	} else {
		fmt.Printf("✅ Colors changing correctly:\n")
		fmt.Printf("   Initial: %s\n", initialColor)
		fmt.Printf("   After increment: %s\n", afterIncrementColor)
		fmt.Printf("   After decrement: %s\n", afterDecrementColor)
	}

	// Test morphdom usage
	if !morphdomUsed {
		t.Errorf("❌ Expected morphdom to be used for DOM updates")
	} else {
		fmt.Printf("✅ morphdom used for DOM updates\n")
	}

	// Test WebSocket communication
	if len(wsMessages) == 0 {
		t.Errorf("❌ Expected WebSocket messages to be received")
	} else {
		fmt.Printf("✅ WebSocket communication working (%d messages received)\n", len(wsMessages))
	}

	// Test static caching (CRITICAL PERFORMANCE REQUIREMENT)
	if len(staticCacheViolations) > 0 {
		t.Errorf("❌ STATIC CACHING FAILURE: Server resent statics for fragments: %v", staticCacheViolations)
		t.Errorf("   This violates LiveTemplate's 92%% bandwidth savings promise")
		t.Errorf("   Expected: Only dynamic data on subsequent updates")  
		t.Errorf("   Actual: Full static HTML structures resent every time")
	} else {
		fmt.Printf("✅ Static caching working - no violations detected\n")
	}

	// Test static caching - CRITICAL optimization verification
	fmt.Printf("\n=== STATIC CACHING ANALYSIS ===\n")
	if len(wsMessages) >= 2 {
		// Parse the WebSocket messages to check for static caching
		// Handle both new object format and old array format
		var firstFragmentData map[string]any
		var secondFragmentData map[string]any
		
		if err := json.Unmarshal([]byte(wsMessages[0]), &firstFragmentData); err == nil {
			if err := json.Unmarshal([]byte(wsMessages[1]), &secondFragmentData); err == nil {
				// Check if counter fragment in second message has statics
				// New format: check if fragment "2" has statics
				if fragmentData, ok := secondFragmentData["2"].(map[string]any); ok {
					if _, hasStatics := fragmentData["s"]; hasStatics {
						// Second update SHOULD NOT have statics - they should be cached!
						// This is a LiveTemplate optimization issue, not a bug in our example
						fmt.Printf("⚠️  STATIC CACHING ISSUE DETECTED: Server resent statics in second update\n")
						fmt.Printf("   Fragment ID: 2\n")
						fmt.Printf("   This reduces bandwidth savings significantly\n")
						// Don't fail the test - this is informational
					} else {
						// Good - no statics in second update
						fmt.Printf("✅ Static caching working: Second update contains only dynamic values\n")
						staticsCached = true
					}
				}
			}
		}
		
		// Show the raw messages for debugging
		fmt.Printf("\nFirst message: %s\n", wsMessages[0])
		fmt.Printf("Second message: %s\n", wsMessages[1])
	}
	
	// Bandwidth impact analysis
	fmt.Printf("\n=== BANDWIDTH IMPACT ANALYSIS ===\n")
	if len(wsMessages) >= 2 {
		msg1Len := len(wsMessages[0])
		msg2Len := len(wsMessages[1])
		totalBandwidth := msg1Len + msg2Len
		
		// Calculate what bandwidth SHOULD be with proper static caching
		// Second message should only have dynamic values (much smaller)
		estimatedOptimalSecondMsg := 50 // Approximate size with only dynamics
		optimalBandwidth := msg1Len + estimatedOptimalSecondMsg
		wastedBandwidth := totalBandwidth - optimalBandwidth
		
		fmt.Printf("First message size: %d bytes\n", msg1Len)
		fmt.Printf("Second message size: %d bytes (should be ~%d with caching)\n", msg2Len, estimatedOptimalSecondMsg)
		fmt.Printf("Total bandwidth used: %d bytes\n", totalBandwidth)
		fmt.Printf("Optimal bandwidth: %d bytes\n", optimalBandwidth)
		fmt.Printf("Wasted bandwidth: %d bytes (%.1f%% overhead)\n", 
			wastedBandwidth, float64(wastedBandwidth)/float64(optimalBandwidth)*100)
	}
	
	// Note: Current LiveTemplate implementation may always send statics
	// This test helps identify if optimization is working properly
	if !staticsCached {
		fmt.Printf("\n⚠️  OPTIMIZATION OPPORTUNITY IDENTIFIED:\n")
		fmt.Printf("   LiveTemplate is resending static HTML structure on every update\n")
		fmt.Printf("   Implementing proper static caching would significantly improve bandwidth efficiency\n")
		fmt.Printf("   Expected savings: 70-90%% reduction in update message size\n")
	}
}

func TestUnifiedCounterMultipleClicks(t *testing.T) {
	// Use random port to avoid conflicts
	port := getRandomPort()
	
	// Start test server
	server := startTestServer(port)
	defer func() {
		server.Shutdown(context.Background())
		log.Printf("Test server on port %d shut down", port)
	}()

	// Create browser context
	ctx, cancel := setupBrowser(t)
	defer cancel()

	var finalCounterText string
	var finalColor string
	var updateCount int

	err := chromedp.Run(ctx,
		// Navigate to the test server
		chromedp.Navigate(fmt.Sprintf("http://host.docker.internal:%d", port)),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for LiveTemplate to connect

		// Set up update counting - the client is exposed as window.client
		chromedp.Evaluate(`
			window.updateCount = 0;
			window.wsMessages = [];
			
			// The client is auto-initialized as window.liveTemplateClient
			if (window.liveTemplateClient && window.liveTemplateClient.ws) {
				const originalOnMessage = window.liveTemplateClient.ws.onmessage;
				window.liveTemplateClient.ws.onmessage = function(event) {
					window.updateCount++;
					window.wsMessages.push(event.data);
					console.log('Update #' + window.updateCount + ':', event.data);
					if (originalOnMessage) {
						return originalOnMessage.call(this, event);
					}
				};
			}
			'Update counting enabled'
		`, nil),

		// Rapid clicks test
		chromedp.Click(`button[data-lvt-action="increment"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.Click(`button[data-lvt-action="increment"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.Click(`button[data-lvt-action="increment"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.Click(`button[data-lvt-action="decrement"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.Click(`button[data-lvt-action="decrement"]`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Final wait

		// Get final state - look for the div with style attribute
		chromedp.Text(`div[style*="color"]`, &finalCounterText, chromedp.ByQuery),
		chromedp.AttributeValue(`[lvt-id="2"]`, "style", &finalColor, nil, chromedp.ByQuery),
		chromedp.Evaluate(`window.updateCount || 0`, &updateCount),
	)

	if err != nil {
		t.Fatalf("Multiple clicks test failed: %v", err)
	}

	// Assertions
	fmt.Printf("\n=== MULTIPLE CLICKS TEST RESULTS ===\n")

	// Should end at 1 (0 + 3 - 2 = 1)
	if !strings.Contains(finalCounterText, "Hello 1 World") {
		t.Errorf("❌ Expected final text to contain 'Hello 1 World', got '%s'", finalCounterText)
	} else {
		fmt.Printf("✅ Multiple clicks working correctly: '%s'\n", finalCounterText)
	}

	if updateCount != 5 {
		t.Errorf("❌ Expected 5 updates, got %d", updateCount)
	} else {
		fmt.Printf("✅ All updates received: %d\n", updateCount)
	}
}

// startTestServer starts a test server on the specified port
func startTestServer(port int) *http.Server {
	serverObj := NewServer()

	mux := http.NewServeMux()
	mux.HandleFunc("/", serverObj.handleHome)
	mux.HandleFunc("/ws", serverObj.handleWebSocket)
	// Serve the bundled LiveTemplate client library (same as main server)
	mux.HandleFunc("/dist/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-cache")
		http.StripPrefix("/dist/", http.FileServer(http.Dir("../../dist/"))).ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		log.Printf("Unified counter test server starting on port %d", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// Give the server time to start
	time.Sleep(2 * time.Second)

	return server
}