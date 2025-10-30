package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type WSMessageLogger struct {
	mu       sync.RWMutex
	messages []WSMessage
}

type WSMessage struct {
	Timestamp time.Time
	Direction string
	Type      string
	Data      string
	Parsed    map[string]interface{}
}

func NewWSMessageLogger() *WSMessageLogger {
	return &WSMessageLogger{
		messages: make([]WSMessage, 0),
	}
}

func (wl *WSMessageLogger) Start(ctx context.Context) {
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *network.EventWebSocketFrameSent:
			wl.mu.Lock()
			defer wl.mu.Unlock()

			msg := WSMessage{
				Timestamp: time.Now(),
				Direction: "sent",
				Data:      ev.Response.PayloadData,
			}

			msg.parseData()
			wl.messages = append(wl.messages, msg)

		case *network.EventWebSocketFrameReceived:
			wl.mu.Lock()
			defer wl.mu.Unlock()

			msg := WSMessage{
				Timestamp: time.Now(),
				Direction: "received",
				Data:      ev.Response.PayloadData,
			}

			msg.parseData()
			wl.messages = append(wl.messages, msg)
		}
	})
}

func (m *WSMessage) parseData() {
	m.Data = strings.TrimSpace(m.Data)

	if strings.HasPrefix(m.Data, "{") || strings.HasPrefix(m.Data, "[") {
		m.Type = "json"
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(m.Data), &parsed); err == nil {
			m.Parsed = parsed
		}
	} else {
		m.Type = "text"
	}
}

func (wl *WSMessageLogger) GetMessages() []WSMessage {
	wl.mu.RLock()
	defer wl.mu.RUnlock()

	result := make([]WSMessage, len(wl.messages))
	copy(result, wl.messages)
	return result
}

func (wl *WSMessageLogger) GetSent() []WSMessage {
	wl.mu.RLock()
	defer wl.mu.RUnlock()

	sent := make([]WSMessage, 0)
	for _, msg := range wl.messages {
		if msg.Direction == "sent" {
			sent = append(sent, msg)
		}
	}
	return sent
}

func (wl *WSMessageLogger) GetReceived() []WSMessage {
	wl.mu.RLock()
	defer wl.mu.RUnlock()

	received := make([]WSMessage, 0)
	for _, msg := range wl.messages {
		if msg.Direction == "received" {
			received = append(received, msg)
		}
	}
	return received
}

func (wl *WSMessageLogger) Clear() {
	wl.mu.Lock()
	defer wl.mu.Unlock()

	wl.messages = make([]WSMessage, 0)
}

func (wl *WSMessageLogger) FindMessage(pattern string) (WSMessage, bool) {
	wl.mu.RLock()
	defer wl.mu.RUnlock()

	for _, msg := range wl.messages {
		if strings.Contains(msg.Data, pattern) {
			return msg, true
		}
	}
	return WSMessage{}, false
}

func (wl *WSMessageLogger) FindMessages(pattern string) []WSMessage {
	wl.mu.RLock()
	defer wl.mu.RUnlock()

	matches := make([]WSMessage, 0)
	for _, msg := range wl.messages {
		if strings.Contains(msg.Data, pattern) {
			matches = append(matches, msg)
		}
	}
	return matches
}

func (wl *WSMessageLogger) HasMessage(pattern string) bool {
	wl.mu.RLock()
	defer wl.mu.RUnlock()

	for _, msg := range wl.messages {
		if strings.Contains(msg.Data, pattern) {
			return true
		}
	}
	return false
}

func (wl *WSMessageLogger) Count() int {
	wl.mu.RLock()
	defer wl.mu.RUnlock()

	return len(wl.messages)
}

func (wl *WSMessageLogger) CountByDirection(direction string) int {
	wl.mu.RLock()
	defer wl.mu.RUnlock()

	count := 0
	for _, msg := range wl.messages {
		if msg.Direction == direction {
			count++
		}
	}
	return count
}

func (wl *WSMessageLogger) CountMatching(pattern string) int {
	wl.mu.RLock()
	defer wl.mu.RUnlock()

	count := 0
	for _, msg := range wl.messages {
		if strings.Contains(msg.Data, pattern) {
			count++
		}
	}
	return count
}

func (wl *WSMessageLogger) GetLastN(n int) []WSMessage {
	wl.mu.RLock()
	defer wl.mu.RUnlock()

	if n <= 0 || len(wl.messages) == 0 {
		return []WSMessage{}
	}

	if n >= len(wl.messages) {
		result := make([]WSMessage, len(wl.messages))
		copy(result, wl.messages)
		return result
	}

	result := make([]WSMessage, n)
	copy(result, wl.messages[len(wl.messages)-n:])
	return result
}

func (wl *WSMessageLogger) GetMessagesSince(since time.Time) []WSMessage {
	wl.mu.RLock()
	defer wl.mu.RUnlock()

	messages := make([]WSMessage, 0)
	for _, msg := range wl.messages {
		if msg.Timestamp.After(since) {
			messages = append(messages, msg)
		}
	}
	return messages
}

func (wl *WSMessageLogger) WaitForMessage(pattern string, timeout time.Duration) (WSMessage, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if msg, found := wl.FindMessage(pattern); found {
			return msg, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return WSMessage{}, fmt.Errorf("timeout waiting for message matching '%s'", pattern)
}

func (wl *WSMessageLogger) Print() {
	wl.mu.RLock()
	defer wl.mu.RUnlock()

	fmt.Println("\n=== WebSocket Messages ===")
	for i, msg := range wl.messages {
		fmt.Printf("[%d] [%s] [%s] %s %s\n",
			i,
			msg.Timestamp.Format("15:04:05.000"),
			msg.Direction,
			msg.Type,
			truncate(msg.Data, 80))
	}
	fmt.Println("==========================")
}

func (wl *WSMessageLogger) PrintLast(n int) {
	messages := wl.GetLastN(n)
	if len(messages) == 0 {
		return
	}

	fmt.Printf("\n=== Last %d WebSocket Messages ===\n", n)
	for i, msg := range messages {
		fmt.Printf("[%d] [%s] [%s] %s %s\n",
			i,
			msg.Timestamp.Format("15:04:05.000"),
			msg.Direction,
			msg.Type,
			truncate(msg.Data, 80))
	}
	fmt.Println("====================================")
}

func (wl *WSMessageLogger) PrintMatching(pattern string) {
	matches := wl.FindMessages(pattern)
	if len(matches) == 0 {
		return
	}

	fmt.Printf("\n=== WebSocket Messages Matching '%s' ===\n", pattern)
	for i, msg := range matches {
		fmt.Printf("[%d] [%s] [%s] %s %s\n",
			i,
			msg.Timestamp.Format("15:04:05.000"),
			msg.Direction,
			msg.Type,
			truncate(msg.Data, 80))
		if msg.Type == "json" && msg.Parsed != nil {
			jsonBytes, _ := json.MarshalIndent(msg.Parsed, "    ", "  ")
			fmt.Printf("    %s\n", string(jsonBytes))
		}
	}
	fmt.Println("==========================================")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
