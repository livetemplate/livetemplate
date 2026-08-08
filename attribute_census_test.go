package livetemplate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gorilla/websocket"
)

func TestExtractAttributeNames(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "no lvt attributes returns nil",
			input: `<div class="card"><p>hello</p></div>`,
			want:  nil,
		},
		{
			name:  "simple attribute",
			input: `<div lvt-fx:scroll="bottom"></div>`,
			want:  []string{"lvt-fx:scroll"},
		},
		{
			name:  "boolean attribute with no value",
			input: `<div lvt-ignore></div>`,
			want:  []string{"lvt-ignore"},
		},
		{
			name:  "sorted and deduplicated across elements",
			input: `<div lvt-on:click="B"></div><div lvt-on:click="C"></div><span lvt-mod:debounce="300"></span>`,
			want:  []string{"lvt-mod:debounce", "lvt-on:click"},
		},
		{
			// The failure this whole extractor exists to avoid: a census that
			// warned about CSS classes would be pure noise. Real templates carry
			// lvt-modal__overlay, lvt-fade-in and friends as class values.
			name:  "css class values are not attributes",
			input: `<div class="lvt-modal__overlay lvt-fade-in lvt-sr-only"></div>`,
			want:  nil,
		},
		{
			// data-lvt-* is the framework's own marker namespace, and the wrapper
			// ID it carries is itself an lvt--prefixed token.
			name:  "framework data- markers and their values are excluded",
			input: `<div data-lvt-id="lvt-06af684d6b9c7483" data-lvt-loading="true" data-lvt-target="x"></div>`,
			want:  nil,
		},
		{
			name:  "single-quoted and unquoted values do not leak into names",
			input: `<div lvt-fx:scroll='lvt-not-an-attr' lvt-key=lvt-also-not></div>`,
			want:  []string{"lvt-fx:scroll", "lvt-key"},
		},
		{
			// The lifecycle tail names an action and a state, not a handler.
			// Collapsing it is what keeps the census from growing with the
			// action list after bracket expansion.
			name:  "lifecycle suffix collapses to the handler name",
			input: `<div lvt-fx:animate:on:save:pending="x" lvt-fx:animate:on:delete:pending="x"></div>`,
			want:  []string{"lvt-fx:animate"},
		},
		{
			name:  "dom event trigger suffix collapses",
			input: `<div lvt-el:addClass:on:click="open" lvt-el:removeClass:on:click-away="open"></div>`,
			want:  []string{"lvt-el:addClass", "lvt-el:removeClass"},
		},
		{
			// lvt-on:click contains "-on:", not ":on:", so the separator must not
			// truncate event routing attributes down to "lvt".
			name:  "event routing attributes are not truncated",
			input: `<div lvt-on:click="Save" lvt-on:window:keydown="Close"></div>`,
			want:  []string{"lvt-on:click", "lvt-on:window:keydown"},
		},
		{
			name:  "dynamic attribute name is discarded not reported as bare namespace",
			input: `<div lvt-fx:{{.Kind}}="x" lvt-{{.Name}}="y"></div>`,
			want:  nil,
		},
		{
			name:  "dynamic method segment with lifecycle tail is discarded",
			input: `<div lvt-fx:{{.Kind}}:on:save:pending="x"></div>`,
			want:  nil,
		},
		{
			name:  "dynamic action segment keeps the handler name",
			input: `<div lvt-fx:animate:on:{{.Action}}:pending="x"></div>`,
			want:  []string{"lvt-fx:animate"},
		},
		{
			name:  "attribute inside a range body is censused",
			input: `<ul>{{range .Items}}<li lvt-key="{{.ID}}" lvt-fx:highlight="new"></li>{{end}}</ul>`,
			want:  []string{"lvt-fx:highlight", "lvt-key"},
		},
		{
			// Case is preserved so the warning echoes what the developer wrote
			// and can be grepped for. The DOM lowercases attribute names, so the
			// client must compare case-insensitively — see lvtAttributePrefix.
			name:  "camelCase method segment is preserved",
			input: `<div lvt-el:toggleAttr:on:click="aria-expanded"></div>`,
			want:  []string{"lvt-el:toggleAttr"},
		},
		{
			name:  "app defined namespace is censused like any other",
			input: `<div lvt-x:copy-to-clipboard="token"></div>`,
			want:  []string{"lvt-x:copy-to-clipboard"},
		},
		{
			name:  "attributes outside a tag are not censused",
			input: `<p>Use lvt-fx:scroll to pin the viewport.</p>`,
			want:  nil,
		},
		{
			name:  "script body is not censused",
			input: `<script>const a = "lvt-fx:scroll"; let lvt-x = 1;</script>`,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAttributeNames(tt.input)
			if !slices.Equal(got, tt.want) {
				t.Errorf("extractAttributeNames(%q)\n  got:  %v\n  want: %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractAttributeNames_SortedIndependentOfSourceOrder(t *testing.T) {
	// Map iteration order is randomized per process, so an unsorted census would
	// make the wire field differ run to run for no reason.
	forward := extractAttributeNames(`<div lvt-a lvt-b lvt-c lvt-d lvt-e lvt-f lvt-g lvt-h></div>`)
	reverse := extractAttributeNames(`<div lvt-h lvt-g lvt-f lvt-e lvt-d lvt-c lvt-b lvt-a></div>`)
	if !slices.Equal(forward, reverse) {
		t.Errorf("census depends on source order:\n  forward: %v\n  reverse: %v", forward, reverse)
	}
	if !slices.IsSorted(forward) {
		t.Errorf("census is not sorted: %v", forward)
	}
}

// Associated templates are flattened into templateStr before the census runs,
// so an attribute that only appears inside a {{define}} block still lands in the
// census. This is measured rather than assumed because the whole value of a
// server-side census over a client-side DOM scan is that it sees template text
// the browser has not rendered yet.
func TestAttributeCensus_CoversAssociatedTemplates(t *testing.T) {
	tmpl, err := New("assoc")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`
{{define "row"}}<li lvt-fx:highlight="new"></li>{{end}}
<ul lvt-scroll-sentinel>{{range .Items}}{{template "row" .}}{{end}}</ul>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	want := []string{"lvt-fx:highlight", "lvt-scroll-sentinel"}
	if !slices.Equal(tmpl.attributeCensus, want) {
		t.Errorf("census over associated templates\n  got:  %v\n  want: %v", tmpl.attributeCensus, want)
	}
}

// The harder half of the same claim, and the shape actually worth pinning: the
// {{define}} lives in a *separately parsed source*, not in the main template's
// string. ParseFiles/ParseFS parse every source into one set and parseInternal
// flattens the set, so the census still reaches it — but that is a property of
// the flatten step, not of the scan, and nothing else would notice if it
// changed. A regression here silently hollows out the diagnostic for exactly
// the multi-file template sets most likely to need it.
func TestAttributeCensus_CoversSeparatelyParsedSources(t *testing.T) {
	fsys := fstest.MapFS{
		// Lexical order decides which source is the main template.
		"a_main.tmpl": &fstest.MapFile{Data: []byte(
			`<ul lvt-scroll-sentinel>{{range .Items}}{{template "row" .}}{{end}}</ul>`)},
		"b_row.tmpl": &fstest.MapFile{Data: []byte(
			`{{define "row"}}<li lvt-fx:highlight="new"></li>{{end}}`)},
	}

	tmpl, err := New("multisource")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.ParseFS(fsys, "*.tmpl")
	if err != nil {
		t.Fatalf("ParseFS failed: %v", err)
	}

	want := []string{"lvt-fx:highlight", "lvt-scroll-sentinel"}
	if !slices.Equal(tmpl.attributeCensus, want) {
		t.Errorf("census across separately parsed sources\n  got:  %v\n  want: %v", tmpl.attributeCensus, want)
	}
}

// The census must survive Clone: production renders through per-session clones,
// so a field the master carries and the clone drops is invisible to every unit
// test that only inspects the master.
func TestAttributeCensus_SurvivesClone(t *testing.T) {
	tmpl, err := New("clone")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div lvt-fx:scroll="bottom" lvt-on:click="Save"></div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	clone, err := tmpl.Clone()
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}
	if !slices.Equal(clone.attributeCensus, tmpl.attributeCensus) {
		t.Errorf("clone census = %v, want %v", clone.attributeCensus, tmpl.attributeCensus)
	}
	if len(clone.attributeCensus) == 0 {
		t.Fatal("clone carries an empty census — the test would pass vacuously")
	}
}

// Bracket expansion runs before the census, so one authored bracket attribute
// becomes N expanded ones — and the census still reports a single handler name.
func TestAttributeCensus_CollapsesExpandedBracketSyntax(t *testing.T) {
	tmpl, err := New("bracket")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div lvt-fx:animate:on:[save,delete,archive]:pending="pulse"></div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	want := []string{"lvt-fx:animate"}
	if !slices.Equal(tmpl.attributeCensus, want) {
		t.Errorf("census = %v, want %v", tmpl.attributeCensus, want)
	}
}

type censusState struct {
	Name string
}

type censusController struct{}

func (c *censusController) Save(state censusState, _ *Context) (censusState, error) {
	state.Name = "saved"
	return state, nil
}

// readInitialMeta issues an Accept: application/json request and returns the
// initial-render meta object, mirroring readCapabilities.
func readInitialMeta(t *testing.T, handler http.Handler) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}
	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected meta field in response")
	}
	return meta
}

func metaAttributes(t *testing.T, meta map[string]interface{}) []string {
	t.Helper()
	raw, ok := meta["attributes"].([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, len(raw))
	for i, v := range raw {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("attributes[%d] = %T, want string", i, v)
		}
		out[i] = s
	}
	return out
}

func censusHandler(t *testing.T, templateStr string) http.Handler {
	t.Helper()
	tmpl, err := New("census", WithProgressiveEnhancement(false))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(templateStr)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	return tmpl.Handle(&censusController{}, AsState(&censusState{Name: "test"}))
}

func TestHandle_AttributesInHTTPInitialRender(t *testing.T) {
	handler := censusHandler(t, `<div lvt-fx:scroll="bottom"><button lvt-on:click="Save">{{.Name}}</button></div>`)

	got := metaAttributes(t, readInitialMeta(t, handler))
	want := []string{"lvt-fx:scroll", "lvt-on:click"}
	if !slices.Equal(got, want) {
		t.Errorf("meta.attributes = %v, want %v", got, want)
	}
}

// Both initial-render transports must carry the field. Setting one and not the
// other ships a diagnostic that fires over WebSocket and stays silent over the
// HTTP fallback — the exact asymmetry that makes a warning untrustworthy.
func TestHandle_AttributesInWebSocketInitialRender(t *testing.T) {
	handler := censusHandler(t, `<div lvt-fx:scroll="bottom"><button lvt-on:click="Save">{{.Name}}</button></div>`)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("WebSocket close error: %v", err)
		}
	}()

	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("WebSocket read failed: %v", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(msg, &resp); err != nil {
		t.Fatalf("Failed to parse WebSocket response: %v", err)
	}
	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected meta field in WebSocket initial render")
	}

	got := metaAttributes(t, meta)
	want := []string{"lvt-fx:scroll", "lvt-on:click"}
	if !slices.Equal(got, want) {
		t.Errorf("meta.attributes = %v, want %v", got, want)
	}
}

func TestHandle_AttributesOmittedWhenTemplateHasNone(t *testing.T) {
	handler := censusHandler(t, `<div class="lvt-fade-in">{{.Name}}</div>`)

	meta := readInitialMeta(t, handler)
	if _, exists := meta["attributes"]; exists {
		t.Errorf("Expected attributes to be omitted for a template with no lvt-* attributes, got %v", meta["attributes"])
	}
}

// Action responses are not initial renders, so they must not repeat the census.
func TestHandle_AttributesNotRepeatedOnActionResponse(t *testing.T) {
	handler := censusHandler(t, `<div lvt-fx:scroll="bottom"><button lvt-on:click="Save">{{.Name}}</button></div>`)

	form := url.Values{}
	form.Set("lvt-action", "Save")

	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse action response JSON: %v (body %q)", err, rec.Body.String())
	}
	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected meta on the action response — without one this test cannot distinguish absent from unreached")
	}
	if got := meta["action"]; got != "Save" {
		t.Fatalf("Expected the action response for Save, got action=%v — the assertion below would be vacuous", got)
	}
	if _, exists := meta["attributes"]; exists {
		t.Errorf("Expected attributes to be absent from an action response, got %v", meta["attributes"])
	}
}
