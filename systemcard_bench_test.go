package livetemplate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Known WebSocket overhead constants (from BenchmarkMemoryUsage: ~980B base connection,
// plus goroutine stacks and send buffer). These cannot be measured in library tests
// since real WebSocket connections are created in the lvt repository.
const (
	wsConnectionBaseBytes  = 980      // Measured in BenchmarkMemoryUsage
	wsGoroutineStackBytes  = 2 * 2048 // 2 goroutines (readPump + writePump) × ~2KB stack
	wsDefaultBufferSize    = 50       // Default send buffer size
	wsMessageOverheadBytes = 100      // Average message overhead in buffer
	wsOverheadPerConn      = wsConnectionBaseBytes + wsGoroutineStackBytes + wsDefaultBufferSize*wsMessageOverheadBytes
	ramHeadroomFactor      = 1.5 // 50% headroom for GC, runtime, OS
)

// Scale points for capacity planning
var scalePoints = []int{100, 1_000, 5_000, 10_000}

// =============================================================================
// State Types for App Scenarios
// =============================================================================

type dashboardState struct {
	Title   string `json:"title"`
	Users   int    `json:"users"`
	Revenue string `json:"revenue"`
	Active  int    `json:"active"`
	Updated string `json:"updated"`
}

type todoItem struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Done     bool   `json:"done"`
	Priority string `json:"priority"`
}

type todoState struct {
	Filter string     `json:"filter"`
	Items  []todoItem `json:"items"`
}

type feedPost struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	Content   string `json:"content"`
	Likes     int    `json:"likes"`
	Timestamp string `json:"timestamp"`
	LikedByMe bool   `json:"liked_by_me"`
}

type feedState struct {
	Username  string     `json:"username"`
	PostCount int        `json:"post_count"`
	Posts     []feedPost `json:"posts"`
}

type chatMessage struct {
	ID   string `json:"id"`
	User string `json:"user"`
	Text string `json:"text"`
	Time string `json:"time"`
}

type chatUser struct {
	Name   string `json:"name"`
	Online bool   `json:"online"`
}

type chatState struct {
	RoomName string        `json:"room_name"`
	Typing   string        `json:"typing"`
	Unread   int           `json:"unread"`
	Users    []chatUser    `json:"users"`
	Messages []chatMessage `json:"messages"`
}

// =============================================================================
// App Scenario Definition
// =============================================================================

type appScenario struct {
	Name          string
	Description   string
	TemplateStr   string
	InitialState  func() any
	UpdateState   func(iteration int) any
	MakeState     func() (State, func() any) // For serialization measurement
	UpdatesPerSec float64
}

func allScenarios() []appScenario {
	return []appScenario{
		counterDashboardScenario(),
		todoAppScenario(),
		socialFeedScenario(),
		chatCollabScenario(),
	}
}

func counterDashboardScenario() appScenario {
	tmpl := `<div class="dashboard">` +
		`<h1>{{.Title}}</h1>` +
		`<div class="stats">` +
		`<div class="stat"><span class="label">Users</span><span class="value">{{.Users}}</span></div>` +
		`<div class="stat"><span class="label">Revenue</span><span class="value">${{.Revenue}}</span></div>` +
		`<div class="stat"><span class="label">Active</span><span class="value">{{.Active}}</span></div>` +
		`</div>` +
		`<footer>Last updated: {{.Updated}}</footer>` +
		`</div>`

	return appScenario{
		Name:        "Counter/Dashboard",
		Description: "5 scalar fields, infrequent updates",
		TemplateStr: tmpl,
		InitialState: func() any {
			return &dashboardState{
				Title: "Analytics Dashboard", Users: 1234,
				Revenue: "45,678", Active: 89, Updated: "2026-03-23 10:00",
			}
		},
		UpdateState: func(i int) any {
			return &dashboardState{
				Title: "Analytics Dashboard", Users: 1234 + i,
				Revenue: fmt.Sprintf("%d,%03d", 45+i/1000, 678+i%1000),
				Active:  89 + i%20, Updated: fmt.Sprintf("2026-03-23 10:%02d", i%60),
			}
		},
		MakeState: func() (State, func() any) {
			s := &dashboardState{
				Title: "Analytics Dashboard", Users: 1234,
				Revenue: "45,678", Active: 89, Updated: "2026-03-23 10:00",
			}
			return AsState(s), func() any { return s }
		},
		UpdatesPerSec: 0.2,
	}
}

func todoAppScenario() appScenario {
	tmpl := `<div class="todo-app">` +
		`<h2>Tasks ({{.Filter}})</h2>` +
		`<ul class="todo-list">` +
		`{{range .Items}}<li data-key="{{.ID}}" class="todo-item">` +
		`{{if .Done}}<s>{{.Text}}</s>{{else}}<span>{{.Text}}</span>{{end}}` +
		` <span class="priority priority-{{.Priority}}">{{.Priority}}</span>` +
		`</li>{{end}}` +
		`</ul>` +
		`</div>`

	makeItems := func(count int) []todoItem {
		items := make([]todoItem, count)
		for i := range items {
			items[i] = todoItem{
				ID:       fmt.Sprintf("todo-%d", i),
				Text:     fmt.Sprintf("Task number %d with a reasonable description", i),
				Done:     i%3 == 0,
				Priority: []string{"low", "medium", "high"}[i%3],
			}
		}
		return items
	}

	return appScenario{
		Name:        "Todo App",
		Description: "25 items, range with conditionals",
		TemplateStr: tmpl,
		InitialState: func() any {
			return &todoState{Filter: "all", Items: makeItems(25)}
		},
		UpdateState: func(i int) any {
			items := makeItems(25)
			// Toggle a done flag and add an item
			items[i%25].Done = !items[i%25].Done
			if i%5 == 0 {
				items = append(items, todoItem{
					ID: fmt.Sprintf("new-%d", i), Text: fmt.Sprintf("New task %d", i),
					Done: false, Priority: "high",
				})
			}
			return &todoState{Filter: "all", Items: items}
		},
		MakeState: func() (State, func() any) {
			s := &todoState{Filter: "all", Items: makeItems(25)}
			return AsState(s), func() any { return s }
		},
		UpdatesPerSec: 0.5,
	}
}

func socialFeedScenario() appScenario {
	tmpl := `<div class="feed">` +
		`<header><h1>@{{.Username}}</h1><span>{{.PostCount}} posts</span></header>` +
		`<div class="timeline">` +
		`{{range .Posts}}<article data-key="{{.ID}}" class="post">` +
		`<div class="post-header"><strong>{{.Author}}</strong><time>{{.Timestamp}}</time></div>` +
		`<p class="post-body">{{.Content}}</p>` +
		`<div class="post-actions">` +
		`{{if .LikedByMe}}<button class="liked">Liked ({{.Likes}})</button>` +
		`{{else}}<button class="like">Like ({{.Likes}})</button>{{end}}` +
		`</div>` +
		`</article>{{end}}` +
		`</div>` +
		`</div>`

	makePosts := func(count int) []feedPost {
		posts := make([]feedPost, count)
		for i := range posts {
			posts[i] = feedPost{
				ID:        fmt.Sprintf("post-%d", i),
				Author:    fmt.Sprintf("user_%d", i%20),
				Content:   fmt.Sprintf("This is post #%d. It contains some text that simulates a real social media post with enough content to be realistic. #topic%d", i, i%10),
				Likes:     i * 3,
				Timestamp: fmt.Sprintf("2026-03-23 %02d:%02d", i%24, i%60),
				LikedByMe: i%4 == 0,
			}
		}
		return posts
	}

	return appScenario{
		Name:        "Social Feed",
		Description: "50 posts, nested conditionals, range ops",
		TemplateStr: tmpl,
		InitialState: func() any {
			return &feedState{Username: "current_user", PostCount: 50, Posts: makePosts(50)}
		},
		UpdateState: func(i int) any {
			posts := makePosts(50)
			// Like/unlike a post
			posts[i%50].LikedByMe = !posts[i%50].LikedByMe
			posts[i%50].Likes += 1
			// Insert new post at top every 3rd iteration
			if i%3 == 0 {
				newPost := feedPost{
					ID: fmt.Sprintf("new-%d", i), Author: "new_user",
					Content: fmt.Sprintf("Fresh post at iteration %d!", i),
					Likes:   0, Timestamp: "2026-03-23 12:00", LikedByMe: false,
				}
				posts = append([]feedPost{newPost}, posts[:49]...)
			}
			return &feedState{Username: "current_user", PostCount: 50 + i/3, Posts: posts}
		},
		MakeState: func() (State, func() any) {
			s := &feedState{Username: "current_user", PostCount: 50, Posts: makePosts(50)}
			return AsState(s), func() any { return s }
		},
		UpdatesPerSec: 1.0,
	}
}

func chatCollabScenario() appScenario {
	tmpl := `<div class="chat">` +
		`<div class="sidebar"><h3>{{.RoomName}}</h3>` +
		`<ul class="users">{{range .Users}}<li data-key="{{.Name}}" class="{{if .Online}}online{{else}}offline{{end}}">{{.Name}}</li>{{end}}</ul>` +
		`</div>` +
		`<div class="messages">` +
		`{{range .Messages}}<div data-key="{{.ID}}" class="message">` +
		`<span class="author">{{.User}}</span>` +
		`<span class="text">{{.Text}}</span>` +
		`<span class="time">{{.Time}}</span>` +
		`</div>{{end}}` +
		`</div>` +
		`<div class="footer">` +
		`{{if .Typing}}<span class="typing">{{.Typing}} is typing...</span>{{end}}` +
		`<span class="unread">{{.Unread}} unread</span>` +
		`</div>` +
		`</div>`

	makeMessages := func(count int) []chatMessage {
		msgs := make([]chatMessage, count)
		for i := range msgs {
			msgs[i] = chatMessage{
				ID:   fmt.Sprintf("msg-%d", i),
				User: fmt.Sprintf("user_%d", i%10),
				Text: fmt.Sprintf("Message %d: Hello, this is a chat message with typical length.", i),
				Time: fmt.Sprintf("%02d:%02d", i%24, i%60),
			}
		}
		return msgs
	}

	makeUsers := func(count int) []chatUser {
		users := make([]chatUser, count)
		for i := range users {
			users[i] = chatUser{
				Name:   fmt.Sprintf("user_%d", i),
				Online: i%3 != 0,
			}
		}
		return users
	}

	return appScenario{
		Name:        "Chat/Collab",
		Description: "100 messages, 20 users, frequent updates",
		TemplateStr: tmpl,
		InitialState: func() any {
			return &chatState{
				RoomName: "general", Typing: "", Unread: 0,
				Users: makeUsers(20), Messages: makeMessages(100),
			}
		},
		UpdateState: func(i int) any {
			msgs := makeMessages(100)
			users := makeUsers(20)
			// Append new message, drop oldest
			newMsg := chatMessage{
				ID: fmt.Sprintf("new-%d", i), User: fmt.Sprintf("user_%d", i%10),
				Text: fmt.Sprintf("New message at iteration %d", i),
				Time: fmt.Sprintf("12:%02d", i%60),
			}
			msgs = append(msgs[1:], newMsg)
			// Toggle a user online/offline
			users[i%20].Online = !users[i%20].Online
			typing := ""
			if i%2 == 0 {
				typing = fmt.Sprintf("user_%d", (i+1)%10)
			}
			return &chatState{
				RoomName: "general", Typing: typing, Unread: i % 5,
				Users: users, Messages: msgs,
			}
		},
		MakeState: func() (State, func() any) {
			s := &chatState{
				RoomName: "general", Typing: "", Unread: 0,
				Users: makeUsers(20), Messages: makeMessages(100),
			}
			return AsState(s), func() any { return s }
		},
		UpdatesPerSec: 2.0,
	}
}

// =============================================================================
// Measurement Result Types
// =============================================================================

type sessionMemoryResult struct {
	PerSessionBytes    uint64
	TotalMeasuredBytes uint64
	SessionCount       int
	GoroutineCount     int
}

type payloadResult struct {
	InitialRenderBytes int
	UpdateBytes        int
	SavingsPercent     float64
}

type serializationResult struct {
	MarshalTimeNs   int64
	UnmarshalTimeNs int64
	SerializedBytes int
}

type throughputResult struct {
	TotalOps      int64
	OpsPerSec     float64
	MeanLatencyNs int64
	Duration      time.Duration
	SessionCount  int
}

type gcResult struct {
	PausePerSecNs float64
	GCFrequencyHz float64
	AllocRateMBps float64
	Duration      time.Duration
}

type capacityEstimate struct {
	RAM          uint64
	CPUFraction  float64
	BandwidthBps float64
	Goroutines   int
}

// =============================================================================
// Measurement Functions
// =============================================================================

func measureSessionMemory(scenario appScenario, sessionCount int) sessionMemoryResult {
	// Parse master template
	master := Must(New("systemcard-mem"))
	if _, err := master.Parse(scenario.TemplateStr); err != nil {
		panic(fmt.Sprintf("parse failed: %v", err))
	}

	// Initial render on master to ensure it's fully initialized
	var buf bytes.Buffer
	if err := master.Execute(&buf, scenario.InitialState()); err != nil {
		panic(fmt.Sprintf("execute failed: %v", err))
	}

	// Baseline measurement
	runtime.GC()
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	// Create N sessions: clone + execute (populates lastTree)
	templates := make([]*Template, sessionCount)
	for i := 0; i < sessionCount; i++ {
		t, err := master.Clone()
		if err != nil {
			panic(fmt.Sprintf("clone failed: %v", err))
		}
		buf.Reset()
		if err := t.Execute(&buf, scenario.InitialState()); err != nil {
			panic(fmt.Sprintf("execute failed: %v", err))
		}
		templates[i] = t
	}

	// After measurement
	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	totalMem := after.HeapInuse - before.HeapInuse
	perSession := totalMem / uint64(sessionCount)

	// Keep templates alive until after measurement
	runtime.KeepAlive(templates)

	return sessionMemoryResult{
		PerSessionBytes:    perSession,
		TotalMeasuredBytes: totalMem,
		SessionCount:       sessionCount,
		GoroutineCount:     runtime.NumGoroutine(),
	}
}

func measurePayloadSizes(scenario appScenario) payloadResult {
	tmpl := Must(New("systemcard-payload"))
	if _, err := tmpl.Parse(scenario.TemplateStr); err != nil {
		panic(fmt.Sprintf("parse failed: %v", err))
	}

	// Initial render
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, scenario.InitialState()); err != nil {
		panic(fmt.Sprintf("execute failed: %v", err))
	}
	initialSize := buf.Len()

	// Update render
	buf.Reset()
	if err := tmpl.ExecuteUpdates(&buf, scenario.UpdateState(1)); err != nil {
		panic(fmt.Sprintf("update failed: %v", err))
	}
	updateSize := buf.Len()

	savings := 0.0
	if initialSize > 0 {
		savings = (1.0 - float64(updateSize)/float64(initialSize)) * 100
	}

	return payloadResult{
		InitialRenderBytes: initialSize,
		UpdateBytes:        updateSize,
		SavingsPercent:     savings,
	}
}

func measureStateSerialization(scenario appScenario) serializationResult {
	state, getInner := scenario.MakeState()

	// Measure marshal
	const iterations = 1000
	start := time.Now()
	var lastData []byte
	for i := 0; i < iterations; i++ {
		data, err := state.MarshalBinary()
		if err != nil {
			panic(fmt.Sprintf("marshal failed: %v", err))
		}
		lastData = data
	}
	marshalTotal := time.Since(start)

	// Measure unmarshal
	start = time.Now()
	for i := 0; i < iterations; i++ {
		if err := state.UnmarshalBinary(lastData); err != nil {
			panic(fmt.Sprintf("unmarshal failed: %v", err))
		}
	}
	unmarshalTotal := time.Since(start)

	// Keep inner alive
	runtime.KeepAlive(getInner())

	return serializationResult{
		MarshalTimeNs:   marshalTotal.Nanoseconds() / int64(iterations),
		UnmarshalTimeNs: unmarshalTotal.Nanoseconds() / int64(iterations),
		SerializedBytes: len(lastData),
	}
}

func measureUpdateThroughput(scenario appScenario, sessionCount int) throughputResult {
	// Create sessions
	templates := make([]*Template, sessionCount)
	for i := 0; i < sessionCount; i++ {
		tmpl := Must(New("systemcard-throughput"))
		if _, err := tmpl.Parse(scenario.TemplateStr); err != nil {
			panic(fmt.Sprintf("parse failed: %v", err))
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, scenario.InitialState()); err != nil {
			panic(fmt.Sprintf("execute failed: %v", err))
		}
		templates[i] = tmpl
	}

	duration := 2 * time.Second
	var totalOps atomic.Int64
	var totalLatencyNs atomic.Int64
	var wg sync.WaitGroup

	deadline := time.Now().Add(duration)

	for idx := 0; idx < sessionCount; idx++ {
		wg.Add(1)
		go func(tmpl *Template, session int) {
			defer wg.Done()
			var buf bytes.Buffer
			iteration := 0
			for time.Now().Before(deadline) {
				iteration++
				state := scenario.UpdateState(iteration + session*10000)
				buf.Reset()
				start := time.Now()
				if err := tmpl.ExecuteUpdates(&buf, state); err != nil {
					return
				}
				elapsed := time.Since(start)
				totalOps.Add(1)
				totalLatencyNs.Add(elapsed.Nanoseconds())
			}
		}(templates[idx], idx)
	}

	wg.Wait()

	ops := totalOps.Load()
	meanLatency := int64(0)
	if ops > 0 {
		meanLatency = totalLatencyNs.Load() / ops
	}

	return throughputResult{
		TotalOps:      ops,
		OpsPerSec:     float64(ops) / duration.Seconds(),
		MeanLatencyNs: meanLatency,
		Duration:      duration,
		SessionCount:  sessionCount,
	}
}

func measureGCPressure(scenario appScenario, sessionCount int) gcResult {
	// Create sessions
	templates := make([]*Template, sessionCount)
	for i := 0; i < sessionCount; i++ {
		tmpl := Must(New("systemcard-gc"))
		if _, err := tmpl.Parse(scenario.TemplateStr); err != nil {
			panic(fmt.Sprintf("parse failed: %v", err))
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, scenario.InitialState()); err != nil {
			panic(fmt.Sprintf("execute failed: %v", err))
		}
		templates[i] = tmpl
	}

	// Baseline GC stats
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	duration := 3 * time.Second
	var wg sync.WaitGroup
	deadline := time.Now().Add(duration)

	for idx := 0; idx < sessionCount; idx++ {
		wg.Add(1)
		go func(tmpl *Template, session int) {
			defer wg.Done()
			var buf bytes.Buffer
			iteration := 0
			for time.Now().Before(deadline) {
				iteration++
				state := scenario.UpdateState(iteration + session*10000)
				buf.Reset()
				_ = tmpl.ExecuteUpdates(&buf, state)
			}
		}(templates[idx], idx)
	}

	wg.Wait()

	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	runtime.KeepAlive(templates)

	secs := duration.Seconds()
	pauseDelta := float64(after.PauseTotalNs - before.PauseTotalNs)
	gcDelta := float64(after.NumGC - before.NumGC)
	allocDelta := float64(after.TotalAlloc - before.TotalAlloc)

	return gcResult{
		PausePerSecNs: pauseDelta / secs,
		GCFrequencyHz: gcDelta / secs,
		AllocRateMBps: allocDelta / secs / (1024 * 1024),
		Duration:      duration,
	}
}

func measureSingleUpdateLatency(scenario appScenario) int64 {
	tmpl := Must(New("systemcard-latency"))
	if _, err := tmpl.Parse(scenario.TemplateStr); err != nil {
		panic(fmt.Sprintf("parse failed: %v", err))
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, scenario.InitialState()); err != nil {
		panic(fmt.Sprintf("execute failed: %v", err))
	}

	// Warm up
	for i := 0; i < 100; i++ {
		buf.Reset()
		_ = tmpl.ExecuteUpdates(&buf, scenario.UpdateState(i))
	}

	// Measure single-goroutine latency (no contention = actual CPU cost)
	const iterations = 1000
	start := time.Now()
	for i := 0; i < iterations; i++ {
		buf.Reset()
		_ = tmpl.ExecuteUpdates(&buf, scenario.UpdateState(i+100))
	}
	return time.Since(start).Nanoseconds() / int64(iterations)
}

// =============================================================================
// Capacity Calculation
// =============================================================================

func calculateCapacity(mem sessionMemoryResult, payload payloadResult, singleUpdateLatencyNs int64, scenario appScenario, sessionCount int) capacityEstimate {
	perSessionTotal := mem.PerSessionBytes + wsOverheadPerConn

	ram := uint64(float64(perSessionTotal) * float64(sessionCount) * ramHeadroomFactor)

	// CPU from single-goroutine latency (no contention = actual CPU cost per update)
	cpuFraction := float64(singleUpdateLatencyNs) * scenario.UpdatesPerSec * float64(sessionCount) / 1e9

	bandwidth := float64(payload.UpdateBytes) * scenario.UpdatesPerSec * float64(sessionCount)

	goroutines := 2*sessionCount + runtime.NumGoroutine()

	return capacityEstimate{
		RAM:          ram,
		CPUFraction:  cpuFraction,
		BandwidthBps: bandwidth,
		Goroutines:   goroutines,
	}
}

func recommendTier(est capacityEstimate) (string, string) {
	ramGB := float64(est.RAM) / (1024 * 1024 * 1024)
	switch {
	case ramGB < 2 && est.CPUFraction < 1:
		return "Tier 1: Hobby", "1 vCPU, 1-2 GB RAM, $5-20/month"
	case ramGB < 4 && est.CPUFraction < 2:
		return "Tier 2: Startup", "2 vCPUs, 4 GB RAM, $50-200/month"
	case ramGB < 16 && est.CPUFraction < 8:
		return "Tier 3: SaaS", "4-8 vCPUs, 8-16 GB RAM, 2-10 instances"
	default:
		return "Tier 4: Enterprise", "8+ vCPUs, 32+ GB RAM, 10+ instances"
	}
}

// =============================================================================
// Formatting Helpers
// =============================================================================

func formatBytes(b uint64) string {
	switch {
	case b >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(b)/(1024*1024*1024))
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func formatBytesFloat(b float64) string {
	switch {
	case b >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB/s", b/(1024*1024*1024))
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB/s", b/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1f KB/s", b/1024)
	default:
		return fmt.Sprintf("%.0f B/s", b)
	}
}

func formatDuration(ns int64) string {
	switch {
	case ns >= 1_000_000:
		return fmt.Sprintf("%.1f ms", float64(ns)/1e6)
	case ns >= 1_000:
		return fmt.Sprintf("%.1f us", float64(ns)/1e3)
	default:
		return fmt.Sprintf("%d ns", ns)
	}
}

func formatScale(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%dK", n/1000)
	}
	return fmt.Sprintf("%d", n)
}

func formatCPU(fraction float64) string {
	if fraction < 0.01 {
		return "<1%"
	}
	if fraction >= 1.0 {
		return fmt.Sprintf("%.1f cores", fraction)
	}
	return fmt.Sprintf("%.0f%%", fraction*100)
}

func padLeft(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return strings.Repeat(" ", width-len(s)) + s
}

// =============================================================================
// System Card Output
// =============================================================================

func printSystemCard(t *testing.T, scenarios []appScenario) {
	const memSessions = 1000
	const throughputSessions = 100

	type scenarioResults struct {
		scenario              appScenario
		mem                   sessionMemoryResult
		payload               payloadResult
		serialization         serializationResult
		throughput            throughputResult
		gc                    gcResult
		singleUpdateLatencyNs int64
	}

	results := make([]scenarioResults, len(scenarios))
	for i, sc := range scenarios {
		t.Logf("Measuring %s...", sc.Name)
		results[i] = scenarioResults{
			scenario:              sc,
			mem:                   measureSessionMemory(sc, memSessions),
			payload:               measurePayloadSizes(sc),
			serialization:         measureStateSerialization(sc),
			throughput:            measureUpdateThroughput(sc, throughputSessions),
			gc:                    measureGCPressure(sc, throughputSessions),
			singleUpdateLatencyNs: measureSingleUpdateLatency(sc),
		}
	}

	// Print the card
	var out strings.Builder
	sep := strings.Repeat("=", 80)

	out.WriteString("\n" + sep + "\n")
	fmt.Fprintf(&out, "%s\n", padLeft("LiveTemplate System Card", 52))
	fmt.Fprintf(&out, "%s\n", padLeft(
		fmt.Sprintf("Generated: %s | Go %s | %s/%s",
			time.Now().Format("2006-01-02"), runtime.Version(), runtime.GOOS, runtime.GOARCH),
		68))
	out.WriteString(sep + "\n\n")

	// Section 1: Per-Session Memory
	fmt.Fprintf(&out, "Per-Session Memory (measured with %d sessions)\n", memSessions)
	fmt.Fprintf(&out, "  %-18s %10s %10s %12s %12s\n",
		"App Scenario", "Per-Sess", "State", "Tmpl+Tree", "WS Overhead")
	fmt.Fprintf(&out, "  %-18s %10s %10s %12s %12s\n",
		strings.Repeat("-", 18), strings.Repeat("-", 10), strings.Repeat("-", 10),
		strings.Repeat("-", 12), strings.Repeat("-", 12))
	for _, r := range results {
		wsOH := formatBytes(wsOverheadPerConn)
		totalPerSess := formatBytes(r.mem.PerSessionBytes + wsOverheadPerConn)
		stateSize := formatBytes(uint64(r.serialization.SerializedBytes))
		tmplTree := formatBytes(r.mem.PerSessionBytes)
		fmt.Fprintf(&out, "  %-18s %10s %10s %12s %12s\n",
			r.scenario.Name, totalPerSess, stateSize, tmplTree, wsOH)
	}
	out.WriteString("\n")

	// Section 2: Update Performance
	out.WriteString("Update Performance (per update operation)\n")
	fmt.Fprintf(&out, "  %-18s %12s %12s %14s %10s\n",
		"App Scenario", "Latency", "Initial", "Update", "Savings")
	fmt.Fprintf(&out, "  %-18s %12s %12s %14s %10s\n",
		strings.Repeat("-", 18), strings.Repeat("-", 12), strings.Repeat("-", 12),
		strings.Repeat("-", 14), strings.Repeat("-", 10))
	for _, r := range results {
		fmt.Fprintf(&out, "  %-18s %12s %12s %14s %9.0f%%\n",
			r.scenario.Name,
			formatDuration(r.singleUpdateLatencyNs),
			formatBytes(uint64(r.payload.InitialRenderBytes)),
			formatBytes(uint64(r.payload.UpdateBytes)),
			r.payload.SavingsPercent)
	}
	out.WriteString("\n")

	// Section 3: State Serialization
	out.WriteString("State Serialization (AsState clone overhead)\n")
	fmt.Fprintf(&out, "  %-18s %12s %12s %14s\n",
		"App Scenario", "Marshal", "Unmarshal", "Size")
	fmt.Fprintf(&out, "  %-18s %12s %12s %14s\n",
		strings.Repeat("-", 18), strings.Repeat("-", 12), strings.Repeat("-", 12),
		strings.Repeat("-", 14))
	for _, r := range results {
		fmt.Fprintf(&out, "  %-18s %12s %12s %14s\n",
			r.scenario.Name,
			formatDuration(r.serialization.MarshalTimeNs),
			formatDuration(r.serialization.UnmarshalTimeNs),
			formatBytes(uint64(r.serialization.SerializedBytes)))
	}
	out.WriteString("\n")

	// Section 4: GC Pressure
	fmt.Fprintf(&out, "GC Pressure (%ds window, %d sessions with sustained updates)\n",
		int(results[0].gc.Duration.Seconds()), throughputSessions)
	fmt.Fprintf(&out, "  %-18s %14s %14s %14s\n",
		"App Scenario", "GC Pause/s", "GC Freq", "Alloc Rate")
	fmt.Fprintf(&out, "  %-18s %14s %14s %14s\n",
		strings.Repeat("-", 18), strings.Repeat("-", 14), strings.Repeat("-", 14),
		strings.Repeat("-", 14))
	for _, r := range results {
		pauseMs := r.gc.PausePerSecNs / 1e6
		fmt.Fprintf(&out, "  %-18s %13.1f ms %12.1f/s %12.1f MB/s\n",
			r.scenario.Name, pauseMs, r.gc.GCFrequencyHz, r.gc.AllocRateMBps)
	}
	out.WriteString("\n")

	// Section 5: Capacity Planning
	out.WriteString("Capacity Planning\n")
	fmt.Fprintf(&out, "  %-18s %8s %10s %8s %12s %10s\n",
		"Scenario @ Scale", "Users", "RAM", "CPU", "Bandwidth", "Goroutines")
	fmt.Fprintf(&out, "  %-18s %8s %10s %8s %12s %10s\n",
		strings.Repeat("-", 18), strings.Repeat("-", 8), strings.Repeat("-", 10),
		strings.Repeat("-", 8), strings.Repeat("-", 12), strings.Repeat("-", 10))
	for _, r := range results {
		for _, n := range scalePoints {
			cap := calculateCapacity(r.mem, r.payload, r.singleUpdateLatencyNs, r.scenario, n)
			label := r.scenario.Name
			if n > 100 {
				label = ""
			}
			fmt.Fprintf(&out, "  %-18s %8s %10s %8s %12s %10d\n",
				label,
				formatScale(n),
				formatBytes(cap.RAM),
				formatCPU(cap.CPUFraction),
				formatBytesFloat(cap.BandwidthBps),
				cap.Goroutines)
		}
		out.WriteString("\n")
	}

	// Section 6: Recommended Infrastructure
	out.WriteString("Recommended Infrastructure (at 10K concurrent users)\n")
	fmt.Fprintf(&out, "  %-18s %s\n",
		strings.Repeat("-", 18), strings.Repeat("-", 56))
	for _, r := range results {
		cap := calculateCapacity(r.mem, r.payload, r.singleUpdateLatencyNs, r.scenario, 10_000)
		tier, spec := recommendTier(cap)
		fmt.Fprintf(&out, "  %-18s RAM: %-10s  CPU: %-10s  BW: %-12s\n",
			r.scenario.Name,
			formatBytes(cap.RAM),
			formatCPU(cap.CPUFraction),
			formatBytesFloat(cap.BandwidthBps))
		fmt.Fprintf(&out, "  %-18s %s (%s)\n", "", tier, spec)
		out.WriteString("\n")
	}

	// Notes
	out.WriteString(strings.Repeat("-", 80) + "\n")
	out.WriteString("Notes:\n")
	out.WriteString("  - Timing may vary +/-30%; allocation-derived memory is deterministic\n")
	fmt.Fprintf(&out, "  - WS overhead: ~%s/conn (struct + goroutine stacks + send buffer)\n",
		formatBytes(wsOverheadPerConn))
	fmt.Fprintf(&out, "  - RAM includes %.0f%% headroom for GC and runtime\n",
		(ramHeadroomFactor-1)*100)
	out.WriteString("  - CPU assumes typical update frequency per scenario\n")
	out.WriteString("  - Bandwidth is wire payload only (excludes WebSocket framing, TLS)\n")
	out.WriteString("  - All numbers are server-side only (no network latency)\n")
	out.WriteString(sep + "\n")

	t.Log(out.String())
}

// =============================================================================
// TestSystemCard - Formatted System Card Output
// =============================================================================

func TestSystemCard(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping system card generation in short mode")
	}
	printSystemCard(t, allScenarios())
}

// =============================================================================
// BenchmarkSystemCard - Standard Go Benchmarks for CI Integration
// =============================================================================

func BenchmarkSystemCard(b *testing.B) {
	scenarios := allScenarios()

	for _, sc := range scenarios {
		sc := sc

		b.Run(sc.Name+"/session-memory", func(b *testing.B) {
			master := Must(New("sc-bench"))
			if _, err := master.Parse(sc.TemplateStr); err != nil {
				b.Fatal(err)
			}
			var buf bytes.Buffer
			if err := master.Execute(&buf, sc.InitialState()); err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				t, err := master.Clone()
				if err != nil {
					b.Fatal(err)
				}
				buf.Reset()
				if err := t.Execute(&buf, sc.InitialState()); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(sc.Name+"/update", func(b *testing.B) {
			tmpl := Must(New("sc-bench"))
			if _, err := tmpl.Parse(sc.TemplateStr); err != nil {
				b.Fatal(err)
			}
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, sc.InitialState()); err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				buf.Reset()
				if err := tmpl.ExecuteUpdates(&buf, sc.UpdateState(i)); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(sc.Name+"/state-clone", func(b *testing.B) {
			state, _ := sc.MakeState()
			data, err := state.MarshalBinary()
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := state.MarshalBinary(); err != nil {
					b.Fatal(err)
				}
				if err := state.UnmarshalBinary(data); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(sc.Name+"/payload-size", func(b *testing.B) {
			tmpl := Must(New("sc-bench"))
			if _, err := tmpl.Parse(sc.TemplateStr); err != nil {
				b.Fatal(err)
			}
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, sc.InitialState()); err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				buf.Reset()
				if err := tmpl.ExecuteUpdates(&buf, sc.UpdateState(i)); err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(buf.Len()), "payload-bytes")
			}
		})
	}
}

// Ensure state types are JSON-serializable (compile-time check)
var _ = func() bool {
	for _, check := range []any{
		&dashboardState{}, &todoState{}, &feedState{}, &chatState{},
	} {
		if _, err := json.Marshal(check); err != nil {
			panic(fmt.Sprintf("state type not serializable: %v", err))
		}
	}
	return true
}()
