package livetemplate

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestChatJoinFlow tests the complete chat join flow with conditional branch switching
func TestChatJoinFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E chat test in short mode")
	}

	// Start the chat server in a subprocess
	chatDir := "./examples/chat"
	if _, err := os.Stat(chatDir); os.IsNotExist(err) {
		t.Skip("Chat example not found")
	}

	cmd := exec.Command("go", "run", "main.go")
	cmd.Dir = chatDir
	cmd.Env = append(os.Environ(), "PORT=8095")

	// Capture output for debugging
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start chat server: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	// Wait for server to start
	time.Sleep(2 * time.Second)

	// Verify server is running
	resp, err := http.Get("http://localhost:8095")
	if err != nil {
		t.Fatalf("Server not reachable: %v", err)
	}
	resp.Body.Close()

	// Create browser context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Test the join flow
	var initialStatsText string
	var initialFormVisible bool
	var afterStatsText string
	var afterChatVisible bool
	var afterFormVisible bool

	err = chromedp.Run(ctx,
		// Load the page
		chromedp.Navigate("http://localhost:8095"),
		chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// Capture initial state
		chromedp.Text(".stats", &initialStatsText, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('form[lvt-submit="join"]') !== null`, &initialFormVisible),

		// Fill and submit join form
		chromedp.SetValue(`input[name="username"]`, "testuser", chromedp.ByQuery),
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for WebSocket update

		// Capture after-join state
		chromedp.Text(".stats", &afterStatsText, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('.messages') !== null`, &afterChatVisible),
		chromedp.Evaluate(`document.querySelector('form[lvt-submit="join"]') !== null`, &afterFormVisible),
	)
	if err != nil {
		t.Fatalf("Browser automation failed: %v", err)
	}

	// Verify initial state
	if !strings.Contains(initialStatsText, "Welcome") {
		t.Errorf("Initial stats should show welcome message, got: %q", initialStatsText)
	}
	if !initialFormVisible {
		t.Error("Join form should be visible initially")
	}

	// Verify after-join state
	if !strings.Contains(afterStatsText, "Logged in as testuser") {
		t.Errorf("After join, stats should show logged in state, got: %q", afterStatsText)
	}
	if !strings.Contains(afterStatsText, "user") && !strings.Contains(afterStatsText, "online") {
		t.Errorf("After join, stats should show online users, got: %q", afterStatsText)
	}
	if !strings.Contains(afterStatsText, "message") {
		t.Errorf("After join, stats should show message count, got: %q", afterStatsText)
	}
	if !afterChatVisible {
		t.Error("Chat interface should be visible after join")
	}
	if afterFormVisible {
		t.Error("Join form should NOT be visible after join")
	}

	fmt.Printf("✅ Chat join flow test passed\n")
	fmt.Printf("   Initial: %q\n", initialStatsText)
	fmt.Printf("   After:   %q\n", afterStatsText)
}

// TestChatMessageIncremental tests that adding messages uses insert operations, not full array sends
func TestChatMessageIncremental(t *testing.T) {
	// This tests the specific scenario from the user's bug report:
	// Nested conditional ({{if not .CurrentUser}}...{{else}}...{{end}}) contains
	// another conditional ({{if eq (len .Messages) 0}}...{{else}}{{range}}...{{end}})
	// When adding messages, should use insert operations, not send full array

	tmplStr := `<!DOCTYPE html>
<html>
<body>
<div class="wrapper">
{{if not .CurrentUser}}
	<form>Login Form</form>
{{else}}
	<div class="messages">
	{{if eq (len .Messages) 0}}
		<div class="empty">No messages</div>
	{{else}}
		{{range .Messages}}
		<div class="msg">{{.Username}}: {{.Text}}</div>
		{{end}}
	{{end}}
	</div>
{{end}}
</div>
</body>
</html>`

	tmpl := New("chat-incremental-test", WithDevMode(true))
	_, err := tmpl.Parse(tmplStr)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Message struct
	type Message struct {
		Username string
		Text     string
	}

	// Data struct
	type Data struct {
		CurrentUser string
		Messages    []Message
	}

	// Step 1: Initial render - logged in but no messages
	data1 := Data{
		CurrentUser: "testuser",
		Messages:    []Message{},
	}

	var buf1 bytes.Buffer
	err = tmpl.Execute(&buf1, data1)
	if err != nil {
		t.Fatalf("Execute step 1 failed: %v", err)
	}

	// Step 2: Add first message (empty-state → range transition)
	data2 := Data{
		CurrentUser: "testuser",
		Messages: []Message{
			{Username: "testuser", Text: "First message"},
		},
	}

	var buf2 bytes.Buffer
	err = tmpl.ExecuteUpdates(&buf2, data2)
	if err != nil {
		t.Fatalf("ExecuteUpdates step 2 failed: %v", err)
	}

	update2 := buf2.String()
	// This should be a structure change (empty-state div → range)
	// So we expect to see the full new structure
	if update2 == "" || update2 == "{}" {
		t.Error("Step 2: Expected update for empty-state → range transition")
	}

	// Step 3: Add second message - this is the critical test
	// Should use insert operation, NOT send full array
	data3 := Data{
		CurrentUser: "testuser",
		Messages: []Message{
			{Username: "testuser", Text: "First message"},
			{Username: "testuser", Text: "Second message"},
		},
	}

	var buf3 bytes.Buffer
	err = tmpl.ExecuteUpdates(&buf3, data3)
	if err != nil {
		t.Fatalf("ExecuteUpdates step 3 failed: %v", err)
	}

	update3 := buf3.String()

	// CRITICAL CHECK: Parse the JSON to verify structure
	// An insert operation looks like: {"d": [["i", key, data]]}
	// A full array looks like: {"d": [obj1, obj2]}
	t.Logf("Step 3 update JSON: %s", update3)

	// Count how many times each message appears to detect full array sends
	firstMsgCount := strings.Count(update3, "First message")
	secondMsgCount := strings.Count(update3, "Second message")

	if firstMsgCount > 0 {
		t.Errorf("Step 3: Update contains 'First message' %d times - should be 0 (incremental insert expected)", firstMsgCount)
	}

	if secondMsgCount == 0 && update3 != "" && update3 != "{}" {
		t.Error("Step 3: Update should contain 'Second message' (new)")
	}

	// Step 4: Add third message - verify incremental again
	data4 := Data{
		CurrentUser: "testuser",
		Messages: []Message{
			{Username: "testuser", Text: "First message"},
			{Username: "testuser", Text: "Second message"},
			{Username: "testuser", Text: "Third message"},
		},
	}

	var buf4 bytes.Buffer
	err = tmpl.ExecuteUpdates(&buf4, data4)
	if err != nil {
		t.Fatalf("ExecuteUpdates step 4 failed: %v", err)
	}

	update4 := buf4.String()

	// CRITICAL CHECK: Verify incremental insert for third message
	t.Logf("Step 4 update JSON: %s", update4)

	firstMsgCount4 := strings.Count(update4, "First message")
	secondMsgCount4 := strings.Count(update4, "Second message")
	thirdMsgCount4 := strings.Count(update4, "Third message")

	if firstMsgCount4 > 0 || secondMsgCount4 > 0 {
		t.Errorf("Step 4: Update contains previous messages (First: %d, Second: %d) - should be 0 (incremental insert expected)",
			firstMsgCount4, secondMsgCount4)
	}

	if thirdMsgCount4 == 0 && update4 != "" && update4 != "{}" {
		t.Error("Step 4: Update should contain 'Third message' (new)")
	}

	fmt.Printf("✅ Chat message incremental test passed\n")
	fmt.Printf("   Step 2 (empty→range): Has update\n")
	fmt.Printf("   Step 3 (add 2nd msg): Incremental insert\n")
	fmt.Printf("   Step 4 (add 3rd msg): Incremental insert\n")
}
