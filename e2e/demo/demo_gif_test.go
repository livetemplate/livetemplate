package demo

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/livetemplate/livetemplate"
)

// ---------------------------------------------------------------------------
// Todo app with broadcast — the demo subject
// ---------------------------------------------------------------------------

const demoTemplate = `<!DOCTYPE html>
<html>
<head>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: system-ui, sans-serif; background: #fafafa; padding: 24px; max-width: 380px; }
h2 { font-size: 18px; margin-bottom: 12px; color: #333; }
form { display: flex; gap: 8px; margin-bottom: 16px; }
input[type="text"] { flex: 1; padding: 8px 12px; border: 1px solid #ddd; border-radius: 6px; font-size: 14px; }
input[type="text"]:focus { outline: none; border-color: #4f46e5; }
button { padding: 8px 16px; background: #4f46e5; color: white; border: none; border-radius: 6px; font-size: 14px; cursor: pointer; }
button:hover { background: #4338ca; }
ul { list-style: none; }
li { padding: 10px 12px; background: white; border: 1px solid #e5e7eb; border-radius: 6px; margin-bottom: 6px; font-size: 14px; color: #374151; }
.empty { color: #9ca3af; font-style: italic; font-size: 14px; }
.tab-label { font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; color: #6b7280; margin-bottom: 10px; }
</style>
</head>
<body>
<div class="tab-label" id="tab-label"></div>
<h2>Todos</h2>
<form method="POST">
    <input type="text" name="title" id="title-input" placeholder="New todo..." autocomplete="off">
    <button name="add" id="add-btn">Add</button>
</form>
<ul id="todo-list">
{{range .Items}}
    <li data-key="{{.ID}}">{{.Title}}</li>
{{end}}
{{if not .Items}}
    <li class="empty">No todos yet</li>
{{end}}
</ul>

<script src="https://cdn.jsdelivr.net/npm/@livetemplate/client@0.8.7/dist/livetemplate-client.browser.js"></script>
</body>
</html>`

type demoTodo struct {
	ID    string
	Title string
}

type demoState struct {
	Items []demoTodo
}

type demoController struct {
	mu      sync.Mutex
	nextID  int
	allItems []demoTodo
}

func (c *demoController) Add(state demoState, ctx *livetemplate.Context) (demoState, error) {
	title := ctx.GetString("title")
	if title == "" {
		return state, nil
	}
	c.mu.Lock()
	c.nextID++
	todo := demoTodo{ID: fmt.Sprintf("t%d", c.nextID), Title: title}
	c.allItems = append(c.allItems, todo)
	items := make([]demoTodo, len(c.allItems))
	copy(items, c.allItems)
	c.mu.Unlock()

	state.Items = items
	ctx.BroadcastAction("Refresh", nil)
	return state, nil
}

func (c *demoController) Refresh(state demoState, ctx *livetemplate.Context) (demoState, error) {
	c.mu.Lock()
	items := make([]demoTodo, len(c.allItems))
	copy(items, c.allItems)
	c.mu.Unlock()
	state.Items = items
	return state, nil
}

// sharedGroupAuth puts all connections into the same session group so
// BroadcastAction reaches every tab (simulates multiple users viewing the same page).
type sharedGroupAuth struct{}

func (a *sharedGroupAuth) Identify(_ *http.Request) (string, error)                    { return "", nil }
func (a *sharedGroupAuth) GetSessionGroup(_ *http.Request, _ string) (string, error)   { return "demo", nil }
func (a *sharedGroupAuth) OnUnauthorized(w http.ResponseWriter, _ *http.Request, _ error) {
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}

// ---------------------------------------------------------------------------
// Test: record the GIF
// ---------------------------------------------------------------------------

func TestDemoGif(t *testing.T) {
	// Write template to temp file (New() requires files for auto-discovery)
	tmpFile, err := os.CreateTemp("", "demo-*.tmpl")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(demoTemplate); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	tmpFile.Close()

	// Start the demo server with shared group auth so broadcast reaches both tabs
	ctrl := &demoController{}
	tmpl, err := livetemplate.New("demo",
		livetemplate.WithParseFiles(tmpFile.Name()),
		livetemplate.WithAuthenticator(&sharedGroupAuth{}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	handler := tmpl.Handle(ctrl, livetemplate.AsState(&demoState{}))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Set up chromedp
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.WindowSize(420, 380),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	// Create two browser contexts (two "tabs")
	tab1Ctx, tab1Cancel := chromedp.NewContext(allocCtx)
	defer tab1Cancel()
	tab2Ctx, tab2Cancel := chromedp.NewContext(allocCtx)
	defer tab2Cancel()

	url := server.URL

	// Navigate both tabs and set labels
	if err := chromedp.Run(tab1Ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`#todo-list`, chromedp.ByID),
		chromedp.Evaluate(`document.getElementById('tab-label').textContent = 'Tab A'`, nil),
	); err != nil {
		t.Fatalf("Tab A navigate: %v", err)
	}
	if err := chromedp.Run(tab2Ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`#todo-list`, chromedp.ByID),
		chromedp.Evaluate(`document.getElementById('tab-label').textContent = 'Tab B'`, nil),
	); err != nil {
		t.Fatalf("Tab B navigate: %v", err)
	}

	// Give WebSocket time to connect
	time.Sleep(1 * time.Second)

	var frames []image.Image

	// Frame 1: Both tabs empty
	frame1, err := captureComposite(t, tab1Ctx, tab2Ctx)
	if err != nil {
		t.Fatalf("Frame 1: %v", err)
	}
	frames = append(frames, frame1)

	// Tab A: type "Buy groceries" and click Add
	if err := chromedp.Run(tab1Ctx,
		chromedp.SendKeys(`#title-input`, "Buy groceries", chromedp.ByID),
	); err != nil {
		t.Fatalf("Tab A type: %v", err)
	}

	// Frame 2: Tab A has text typed
	frame2, err := captureComposite(t, tab1Ctx, tab2Ctx)
	if err != nil {
		t.Fatalf("Frame 2: %v", err)
	}
	frames = append(frames, frame2)

	// Click the Add button
	if err := chromedp.Run(tab1Ctx,
		chromedp.Evaluate(`document.getElementById('add-btn').click()`, nil),
	); err != nil {
		t.Fatalf("Tab A click: %v", err)
	}

	// Wait for the item to appear in Tab A
	time.Sleep(500 * time.Millisecond)

	// Frame 3: Tab A shows item
	frame3, err := captureComposite(t, tab1Ctx, tab2Ctx)
	if err != nil {
		t.Fatalf("Frame 3: %v", err)
	}
	frames = append(frames, frame3)

	// Wait for broadcast to reach Tab B
	time.Sleep(1 * time.Second)

	// Frame 4: Tab B shows item too (the reactive moment)
	frame4, err := captureComposite(t, tab1Ctx, tab2Ctx)
	if err != nil {
		t.Fatalf("Frame 4: %v", err)
	}
	frames = append(frames, frame4)

	// Add a second todo for a richer demo
	if err := chromedp.Run(tab1Ctx,
		chromedp.SendKeys(`#title-input`, "Walk the dog", chromedp.ByID),
	); err != nil {
		t.Fatalf("Tab A type 2: %v", err)
	}
	if err := chromedp.Run(tab1Ctx,
		chromedp.Evaluate(`document.getElementById('add-btn').click()`, nil),
	); err != nil {
		t.Fatalf("Tab A click 2: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Frame 5: Both tabs show two items
	frame5, err := captureComposite(t, tab1Ctx, tab2Ctx)
	if err != nil {
		t.Fatalf("Frame 5: %v", err)
	}
	frames = append(frames, frame5)

	// Verify Tab B actually received the items (the E2E assertion)
	var listHTML string
	if err := chromedp.Run(tab2Ctx,
		chromedp.InnerHTML(`#todo-list`, &listHTML, chromedp.ByID),
	); err != nil {
		t.Fatalf("Tab B read: %v", err)
	}
	if !containsAll(listHTML, "Buy groceries", "Walk the dog") {
		t.Errorf("Tab B missing items, got: %s", listHTML)
	}

	// Encode GIF
	outPath := "../../assets/reactive-demo.gif"
	if err := encodeGIF(outPath, frames, []int{200, 150, 120, 200, 300}); err != nil {
		t.Fatalf("Encode GIF: %v", err)
	}
	t.Logf("GIF written to %s (%d frames)", outPath, len(frames))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func captureComposite(t *testing.T, ctx1, ctx2 context.Context) (image.Image, error) {
	t.Helper()

	var buf1, buf2 []byte
	if err := chromedp.Run(ctx1, chromedp.FullScreenshot(&buf1, 100)); err != nil {
		return nil, fmt.Errorf("screenshot tab1: %w", err)
	}
	if err := chromedp.Run(ctx2, chromedp.FullScreenshot(&buf2, 100)); err != nil {
		return nil, fmt.Errorf("screenshot tab2: %w", err)
	}

	img1, err := png.Decode(bytes.NewReader(buf1))
	if err != nil {
		return nil, fmt.Errorf("decode tab1: %w", err)
	}
	img2, err := png.Decode(bytes.NewReader(buf2))
	if err != nil {
		return nil, fmt.Errorf("decode tab2: %w", err)
	}

	return composeSideBySide(img1, img2, 4), nil
}

func composeSideBySide(left, right image.Image, gap int) image.Image {
	lb := left.Bounds()
	rb := right.Bounds()
	h := lb.Dy()
	if rb.Dy() > h {
		h = rb.Dy()
	}
	w := lb.Dx() + gap + rb.Dx()

	composite := image.NewRGBA(image.Rect(0, 0, w, h))
	// Fill background
	draw.Draw(composite, composite.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	// Draw left
	draw.Draw(composite, image.Rect(0, 0, lb.Dx(), lb.Dy()), left, lb.Min, draw.Over)
	// Draw right
	draw.Draw(composite, image.Rect(lb.Dx()+gap, 0, w, rb.Dy()), right, rb.Min, draw.Over)

	return composite
}

func encodeGIF(path string, frames []image.Image, delays []int) error {
	g := &gif.GIF{}
	for i, frame := range frames {
		bounds := frame.Bounds()
		palettedImg := image.NewPaletted(bounds, palette.Plan9)
		draw.FloydSteinberg.Draw(palettedImg, bounds, frame, bounds.Min)
		g.Image = append(g.Image, palettedImg)
		delay := 100 // default 1 second
		if i < len(delays) {
			delay = delays[i]
		}
		g.Delay = append(g.Delay, delay)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return gif.EncodeAll(f, g)
}

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
