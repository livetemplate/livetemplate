package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestWebSocketCaching tests the static content caching directly via WebSocket
func TestWebSocketCaching(t *testing.T) {
	// Start test server
	server := NewServer()
	
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleHome)
	mux.HandleFunc("/ws", server.handleWebSocket)
	
	httpServer := &http.Server{
		Addr:    ":8089",
		Handler: mux,
	}
	
	go func() {
		httpServer.ListenAndServe()
	}()
	defer httpServer.Shutdown(context.Background())
	
	// Wait for server to start
	time.Sleep(1 * time.Second)
	
	// Get initial page to get token
	resp, err := http.Get("http://localhost:8089/")
	if err != nil {
		t.Fatalf("Failed to get initial page: %v", err)
	}
	defer resp.Body.Close()
	
	// Extract token from HTML meta tag
	body := make([]byte, 10000)
	n, _ := resp.Body.Read(body)
	html := string(body[:n])
	
	var token string
	if start := strings.Index(html, `<meta name="livetemplate-token" content="`); start != -1 {
		start += len(`<meta name="livetemplate-token" content="`)
		if end := strings.Index(html[start:], `"`); end != -1 {
			token = html[start:start+end]
		}
	}
	
	if token == "" {
		t.Fatalf("Could not extract token from HTML")
	}
	
	fmt.Printf("Extracted token: %s\n", token)
	
	// Connect to WebSocket
	u := url.URL{Scheme: "ws", Host: "localhost:8089", Path: "/ws", RawQuery: "token=" + token}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()
	
	fmt.Printf("WebSocket connected to: %s\n", u.String())
	
	// Send first increment action
	firstAction := map[string]interface{}{
		"action": "increment",
		"cache_info": map[string]interface{}{
			"has_cache": false,
			"cached_fragments": []string{},
		},
	}
	
	if err := conn.WriteJSON(firstAction); err != nil {
		t.Fatalf("Failed to send first action: %v", err)
	}
	
	fmt.Printf("Sent first action: %+v\n", firstAction)
	
	// Read first response
	var firstResponse map[string]interface{}
	if err := conn.ReadJSON(&firstResponse); err != nil {
		t.Fatalf("Failed to read first response: %v", err)
	}
	
	firstBytes, _ := json.MarshalIndent(firstResponse, "", "  ")
	fmt.Printf("First response:\n%s\n", string(firstBytes))
	
	// Analyze first response for fragment IDs and statics
	var cachedFragments []string
	var hasStatics bool
	
	for fragmentID, fragmentDataInterface := range firstResponse {
		if fragmentData, ok := fragmentDataInterface.(map[string]interface{}); ok {
			if statics, exists := fragmentData["s"]; exists && statics != nil {
				hasStatics = true
				fmt.Printf("Fragment %s has statics: %v\n", fragmentID, statics)
			}
			cachedFragments = append(cachedFragments, fragmentID)
		}
	}
	
	if !hasStatics {
		t.Errorf("Expected first response to have statics")
	}
	
	// Send second increment action WITH cache info
	secondAction := map[string]interface{}{
		"action": "increment", 
		"cache_info": map[string]interface{}{
			"has_cache": true,
			"cached_fragments": cachedFragments,
		},
	}
	
	if err := conn.WriteJSON(secondAction); err != nil {
		t.Fatalf("Failed to send second action: %v", err)
	}
	
	fmt.Printf("Sent second action: %+v\n", secondAction)
	
	// Read second response
	var secondResponse map[string]interface{}
	if err := conn.ReadJSON(&secondResponse); err != nil {
		t.Fatalf("Failed to read second response: %v", err)
	}
	
	secondBytes, _ := json.MarshalIndent(secondResponse, "", "  ")
	fmt.Printf("Second response:\n%s\n", string(secondBytes))
	
	// Analyze second response - should NOT have statics
	var secondHasStatics bool
	for fragmentID, fragmentDataInterface := range secondResponse {
		if fragmentData, ok := fragmentDataInterface.(map[string]interface{}); ok {
			if statics, exists := fragmentData["s"]; exists && statics != nil {
				secondHasStatics = true
				fmt.Printf("❌ Fragment %s STILL has statics: %v (should be cached!)\n", fragmentID, statics)
			} else {
				fmt.Printf("✅ Fragment %s has no statics (using cache)\n", fragmentID)
			}
		}
	}
	
	// Calculate bandwidth savings
	firstSize := len(firstBytes)
	secondSize := len(secondBytes)
	savings := float64(firstSize - secondSize) / float64(firstSize) * 100
	
	fmt.Printf("\n=== CACHING ANALYSIS ===\n")
	fmt.Printf("First response size: %d bytes\n", firstSize)
	fmt.Printf("Second response size: %d bytes\n", secondSize)
	fmt.Printf("Bandwidth savings: %.1f%%\n", savings)
	
	// Test assertions
	if secondHasStatics {
		t.Errorf("❌ CACHING FAILURE: Second response should NOT have statics - they should be cached on client")
		t.Errorf("Expected: Only dynamic data in second response")
		t.Errorf("Actual: Full static HTML structures resent")
		t.Errorf("This defeats the purpose of the caching system")
	} else {
		fmt.Printf("✅ Caching working correctly: Second response omits cached statics\n")
	}
	
	if savings < 50 {
		t.Errorf("❌ Expected >50%% bandwidth savings with caching, got %.1f%%", savings)
	} else {
		fmt.Printf("✅ Good bandwidth savings: %.1f%%\n", savings)
	}
}