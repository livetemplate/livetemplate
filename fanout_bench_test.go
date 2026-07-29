package livetemplate

import (
	"fmt"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

// This file holds the composite fan-out benchmarks: full
// dispatch → controller → render → diff → serialize → real-writePump cycles
// fanned out to N live sessions, each bench modeled on a named consumer
// workload. High-N sub-benches (N ≥ 1000) are capacity-planning sweeps, not
// CI-gate material — the gate covers N ≤ 100 (measure high-N, gate modest-N).
//
// Consumer-app citations in this file (docs-repo examples, muster,
// checklistkit, lvt components) are workload provenance recorded as of
// 2026-07 (livetemplate v0.22) — they live in other repositories and are not
// verifiable code references here.

// discardLogs silences slog for the duration of the benchmark: high-N setup
// logs one "Client connected" INFO per connection, and per-op Debug/Warn
// logging would pollute both the timing and the output.
func discardLogs(tb testing.TB) {
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.DiscardHandler))
	tb.Cleanup(func() { slog.SetDefault(prev) })
}

// awaitAll blocks until every session's pipeline has pushed one frame to the
// write boundary. The receives are sequential but the pipelines complete
// concurrently, so the driver-side tax is one channel receive per session
// (~tens of ns each, <1% of per-op cost even at N=10000), not a
// serialization of the measured work.
func awaitAll(tb testing.TB, sessions []*compositeSession) {
	for _, s := range sessions {
		s.awaitWrite(tb)
	}
}

// fanoutSweep is the subscriber-count axis shared by the fan-out benches.
var fanoutSweep = []int{1, 10, 100, 1000, 10000}

// ---------------------------------------------------------------------------
// BenchmarkTopicFanout_FullPipeline — room-topic push fan-out
// (docs/examples/{seat-picker,live-dashboard,greet-wall}; checklistkit's
// reviewer↔submitter push). One publisher action Publishes real data to a
// developer topic; each of N subscribers runs the dispatched action against
// its OWN state through its real event loop and sends its diff through the
// real pump. Full-pipeline counterpart of the enqueue-only
// BenchmarkTopicFanoutByN_EnqueueOnly micro-bench.

const fanoutBenchTopic = "bench/room"

type topicFanoutState struct {
	Value string
}

type topicFanoutController struct{}

func (c *topicFanoutController) Mount(state topicFanoutState, ctx *Context) (topicFanoutState, error) {
	if err := ctx.Subscribe(fanoutBenchTopic); err != nil {
		return state, err
	}
	return state, nil
}

func (c *topicFanoutController) SetValue(state topicFanoutState, ctx *Context) (topicFanoutState, error) {
	state.Value = ctx.GetString("value")
	if err := ctx.Publish(fanoutBenchTopic, "SyncValue", map[string]interface{}{"value": state.Value}); err != nil {
		return state, err
	}
	return state, nil
}

func (c *topicFanoutController) SyncValue(state topicFanoutState, ctx *Context) (topicFanoutState, error) {
	state.Value = ctx.GetString("value")
	return state, nil
}

const topicFanoutTemplate = `<div>{{.Value}}</div>`

// topicFanoutFrames alternate the published value so every op measures a
// non-trivial diff on every subscriber (the pipeline writes a frame either
// way; an unchanged value would just measure an emptier one).
var topicFanoutFrames = [2][]byte{
	[]byte(`{"action":"SetValue","data":{"value":"tick"}}`),
	[]byte(`{"action":"SetValue","data":{"value":"tock"}}`),
}

func BenchmarkTopicFanout_FullPipeline(b *testing.B) {
	discardLogs(b)
	for _, n := range fanoutSweep {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			app := newCompositeApp(b, topicFanoutTemplate,
				&topicFanoutController{}, AsState(&topicFanoutState{}), WithOpenTopics())
			publisher := app.connect(b, "")
			subscribers := make([]*compositeSession, n)
			for i := range subscribers {
				subscribers[i] = app.connect(b, "")
			}
			all := append([]*compositeSession{publisher}, subscribers...)
			startBytes := wireBytesTotal(all...)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				publisher.dispatch(b, topicFanoutFrames[i%2])
				awaitAll(b, subscribers)
			}
			b.StopTimer()
			b.ReportMetric(float64(wireBytesTotal(all...)-startBytes)/float64(b.N), "wireB/op")
		})
	}
}

// BenchmarkTopicFanout_LoopbackWS is the fidelity spot-check for the fan-out
// harness at N=10: the same app and publish driven over real loopback
// WebSockets. Compare per-op cost against BenchmarkTopicFanout_FullPipeline
// N=10 — the delta is genuine socket/kernel I/O, deliberately excluded from
// the harness numbers. Not part of the CI gate.
func BenchmarkTopicFanout_LoopbackWS(b *testing.B) {
	discardLogs(b)
	const n = 10

	tmpl := Must(New("fanout-loopback", WithMessageRateLimit(0, 0), WithOpenTopics()))
	if _, err := tmpl.Parse(topicFanoutTemplate); err != nil {
		b.Fatalf("Parse failed: %v", err)
	}
	server := httptest.NewServer(tmpl.Handle(&topicFanoutController{}, AsState(&topicFanoutState{})))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	publisher := benchDialWS(b, wsURL)
	subscribers := make([]*websocket.Conn, n)
	for i := range subscribers {
		subscribers[i] = benchDialWS(b, wsURL)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := publisher.WriteMessage(websocket.TextMessage, topicFanoutFrames[i%2]); err != nil {
			b.Fatalf("publish write failed: %v", err)
		}
		if _, _, err := publisher.ReadMessage(); err != nil {
			b.Fatalf("publisher response read failed: %v", err)
		}
		for _, ws := range subscribers {
			if _, _, err := ws.ReadMessage(); err != nil {
				b.Fatalf("subscriber read failed: %v", err)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// BenchmarkTriggerActionFanout — background-driven session-registry fan-out
// (muster session_hub.go: one scan loop TriggerActions a refresh on every
// registered Session handle; each handle re-renders its OWN session's state).
// Sessions live in separate groups (one browser each), exactly muster's shape.

type triggerFanoutState struct {
	Tick int
}

type triggerFanoutController struct {
	mu       sync.Mutex
	sessions []Session
}

func (c *triggerFanoutController) OnConnect(state triggerFanoutState, ctx *Context) (triggerFanoutState, error) {
	c.mu.Lock()
	c.sessions = append(c.sessions, ctx.Session())
	c.mu.Unlock()
	return state, nil
}

func (c *triggerFanoutController) Refresh(state triggerFanoutState, ctx *Context) (triggerFanoutState, error) {
	state.Tick++
	return state, nil
}

// fanout mirrors muster's hub.fanout(): snapshot the handles under the lock,
// TriggerAction outside it. All bench sessions are live, so an error is a
// bench bug, not a prune candidate (the dead-handle path is measured
// separately below).
func (c *triggerFanoutController) fanout(tb testing.TB) {
	c.mu.Lock()
	targets := make([]Session, len(c.sessions))
	copy(targets, c.sessions)
	c.mu.Unlock()
	for _, s := range targets {
		if err := s.TriggerAction("Refresh", nil); err != nil {
			tb.Fatalf("TriggerAction failed: %v", err)
		}
	}
}

const triggerFanoutTemplate = `<div>{{.Tick}}</div>`

func BenchmarkTriggerActionFanout(b *testing.B) {
	discardLogs(b)
	for _, n := range fanoutSweep {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			ctrl := &triggerFanoutController{}
			app := newCompositeApp(b, triggerFanoutTemplate, ctrl, AsState(&triggerFanoutState{}))
			conns := make([]*compositeSession, n)
			for i := range conns {
				conns[i] = app.connect(b, "")
			}
			startBytes := wireBytesTotal(conns...)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				ctrl.fanout(b)
				awaitAll(b, conns)
			}
			b.StopTimer()
			b.ReportMetric(float64(wireBytesTotal(conns...)-startBytes)/float64(b.N), "wireB/op")
		})
	}
}

// BenchmarkTriggerAction_DeadHandle measures the disconnected-session error
// path — the per-handle cost muster's hub pays before pruning a stale entry.
func BenchmarkTriggerAction_DeadHandle(b *testing.B) {
	discardLogs(b)
	ctrl := &triggerFanoutController{}
	app := newCompositeApp(b, triggerFanoutTemplate, ctrl, AsState(&triggerFanoutState{}))
	s := app.connect(b, "")
	s.close() // the session group now has no live connections
	handle := ctrl.sessions[0]

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := handle.TriggerAction("Refresh", nil); err == nil {
			b.Fatal("expected error for disconnected session")
		}
	}
}

// ---------------------------------------------------------------------------
// BenchmarkChatAppendFanout — chat-room append + peer fan-out
// (docs/examples/chat; docs/examples/patterns PubSub): shared history lives
// on the controller; the sender's action appends and Publishes a nil-data
// "NewMessage" to the group's self-topic (pull model); every peer copies the
// full history into its state and re-renders the whole range. History is held
// at cap L (append + trim), so every op measures the steady-state at-capacity
// cost — the growing phase is bounded above by it.

type chatBenchState struct {
	Messages []string
}

type chatBenchController struct {
	mu       sync.Mutex
	messages []string
	cap      int
}

// snapshot copies the shared history — the chat example's copyMessages,
// the full-slice clone that makes each peer re-render the whole range.
// Caller must hold mu.
func (c *chatBenchController) snapshot() []string {
	return append([]string(nil), c.messages...)
}

func (c *chatBenchController) Mount(state chatBenchState, ctx *Context) (chatBenchState, error) {
	if err := ctx.Subscribe(ctx.SelfTopic()); err != nil {
		return state, err
	}
	c.mu.Lock()
	state.Messages = c.snapshot()
	c.mu.Unlock()
	return state, nil
}

func (c *chatBenchController) Send(state chatBenchState, ctx *Context) (chatBenchState, error) {
	c.mu.Lock()
	c.messages = append(c.messages, ctx.GetString("text"))
	if len(c.messages) > c.cap {
		c.messages = c.messages[1:]
	}
	state.Messages = c.snapshot()
	c.mu.Unlock()
	if err := ctx.Publish(ctx.SelfTopic(), "NewMessage", nil); err != nil {
		return state, err
	}
	return state, nil
}

func (c *chatBenchController) NewMessage(state chatBenchState, ctx *Context) (chatBenchState, error) {
	c.mu.Lock()
	state.Messages = c.snapshot()
	c.mu.Unlock()
	return state, nil
}

const chatBenchTemplate = `<ul>{{range .Messages}}<li>{{.}}</li>{{end}}</ul>`

func BenchmarkChatAppendFanout(b *testing.B) {
	discardLogs(b)
	run := func(histLen, peers int) func(*testing.B) {
		return func(b *testing.B) {
			ctrl := &chatBenchController{cap: histLen}
			for i := 0; i < histLen; i++ {
				ctrl.messages = append(ctrl.messages, fmt.Sprintf("seed-%d", i))
			}
			app := newCompositeApp(b, chatBenchTemplate, ctrl, AsState(&chatBenchState{}))
			sender := app.connect(b, "chat-bench-group")
			peerConns := make([]*compositeSession, peers)
			for i := range peerConns {
				peerConns[i] = app.connect(b, "chat-bench-group")
			}
			// Cycle more distinct texts than the history holds so the live
			// window never contains duplicates — duplicate items collide on
			// content-hash range keys, which would change the diff shape
			// being measured.
			frames := make([][]byte, histLen+16)
			for i := range frames {
				frames[i] = []byte(fmt.Sprintf(`{"action":"Send","data":{"text":"msg-%d"}}`, i))
			}
			all := append([]*compositeSession{sender}, peerConns...)
			startBytes := wireBytesTotal(all...)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				sender.dispatch(b, frames[i%len(frames)])
				awaitAll(b, peerConns)
			}
			b.StopTimer()
			b.ReportMetric(float64(wireBytesTotal(all...)-startBytes)/float64(b.N), "wireB/op")
		}
	}
	// Two axes, swept independently: history length at a fixed 10-peer room,
	// then peer count at a fixed 100-message history.
	for _, l := range []int{10, 100, 1000, 10000} {
		b.Run(fmt.Sprintf("hist=%d/peers=10", l), run(l, 10))
	}
	for _, n := range []int{1, 100, 1000} {
		b.Run(fmt.Sprintf("hist=100/peers=%d", n), run(100, n))
	}
}

// ---------------------------------------------------------------------------
// BenchmarkWideTableAction — wide-grid re-projection per action
// (docs/examples/seat-picker; lvt/components/datatable): one action flips one
// seat, the controller re-projects the ENTIRE 50×20 grid into fresh view
// structs (the seat-picker pattern — the template only reads precomputed
// Status), and the diff isolates the one changed cell. Single connection:
// the fan-out axis is covered by the benches above.

type seatView struct {
	ID     string
	Status string
}

type rowView struct {
	Label string
	Seats []seatView
}

type wideTableState struct {
	Rows []rowView
}

type wideTableController struct {
	mu     sync.Mutex
	labels []string // fixed row labels, like seat-picker's preset row names
	cols   int
	taken  map[string]bool
}

func newWideTableController(rows, cols int) *wideTableController {
	c := &wideTableController{cols: cols, taken: map[string]bool{}}
	for r := 0; r < rows; r++ {
		c.labels = append(c.labels, fmt.Sprintf("R%d", r))
	}
	return c
}

// project rebuilds the whole grid projection, as seat-picker's project()
// does on every action — including the per-seat ID Sprintf, which the real
// app also does per projection. Caller must hold mu.
func (c *wideTableController) project(state *wideTableState) {
	rows := make([]rowView, 0, len(c.labels))
	for _, label := range c.labels {
		row := rowView{Label: label, Seats: make([]seatView, 0, c.cols)}
		for n := 0; n < c.cols; n++ {
			id := fmt.Sprintf("%s-%d", label, n)
			status := "available"
			if c.taken[id] {
				status = "taken"
			}
			row.Seats = append(row.Seats, seatView{ID: id, Status: status})
		}
		rows = append(rows, row)
	}
	state.Rows = rows
}

func (c *wideTableController) Mount(state wideTableState, _ *Context) (wideTableState, error) {
	c.mu.Lock()
	c.project(&state)
	c.mu.Unlock()
	return state, nil
}

func (c *wideTableController) ToggleSeat(state wideTableState, ctx *Context) (wideTableState, error) {
	c.mu.Lock()
	id := ctx.GetString("seat")
	c.taken[id] = !c.taken[id]
	c.project(&state)
	c.mu.Unlock()
	return state, nil
}

const wideTableTemplate = `<div>{{range .Rows}}<div class="row"><span>{{.Label}}</span>{{range .Seats}}<button class="seat {{.Status}}" value="{{.ID}}">{{.ID}}</button>{{end}}</div>{{end}}</div>`

func BenchmarkWideTableAction(b *testing.B) {
	discardLogs(b)
	ctrl := newWideTableController(50, 20)
	app := newCompositeApp(b, wideTableTemplate, ctrl, AsState(&wideTableState{}))
	s := app.connect(b, "")
	// Alternate two seats so consecutive ops never cancel out.
	frames := [2][]byte{
		[]byte(`{"action":"ToggleSeat","data":{"seat":"R12-7"}}`),
		[]byte(`{"action":"ToggleSeat","data":{"seat":"R31-4"}}`),
	}
	startBytes := wireBytesTotal(s)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s.dispatch(b, frames[i%2])
	}
	b.StopTimer()
	b.ReportMetric(float64(wireBytesTotal(s)-startBytes)/float64(b.N), "wireB/op")
}

// TestFanoutHarnessOneFramePerSession is the fan-out counterpart of
// TestCompositeHarnessSerializes: awaitAll assumes each op produces exactly
// one frame per session (publisher response + one dispatched update per
// subscriber). This pins that invariant so a future extra or suppressed
// frame in the dispatch path fails loudly here instead of silently skewing
// the fan-out benchmarks.
func TestFanoutHarnessOneFramePerSession(t *testing.T) {
	discardLogs(t)
	app := newCompositeApp(t, topicFanoutTemplate,
		&topicFanoutController{}, AsState(&topicFanoutState{}), WithOpenTopics())
	publisher := app.connect(t, "")
	subscribers := []*compositeSession{app.connect(t, ""), app.connect(t, ""), app.connect(t, "")}
	all := append([]*compositeSession{publisher}, subscribers...)

	// Each session has written exactly its initial render so far.
	for i, s := range all {
		if got := s.conn.MsgsWritten(); got != 1 {
			t.Fatalf("session %d: expected 1 initial frame, got %d", i, got)
		}
	}

	publisher.dispatch(t, topicFanoutFrames[0])
	awaitAll(t, subscribers)

	for i, s := range all {
		if got := s.conn.MsgsWritten(); got != 2 {
			t.Fatalf("session %d: expected exactly 1 frame per op (2 total), got %d", i, got)
		}
	}
}
