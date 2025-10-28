package livetemplate

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type testPost struct {
	ID        string
	Title     string
	Content   string
	Published bool
}

type testPostsState struct {
	Title          string
	SearchQuery    string
	SortBy         string
	PaginationMode string
	PaginatedPosts []testPost
	HasMore        bool
	IsLoading      bool
	LoadedCount    int
	TotalCount     int
	CurrentPage    int
	TotalPages     int
	CSSFramework   string
	EditingID      string
	EditingPosts   *testPost
}

// TestRangeDynamicDoesNotAppendContent ensures that range item dynamics keep field boundaries
func TestRangeDynamicDoesNotAppendContent(t *testing.T) {
	tmpl := New("posts", WithDevMode(true))

	templatePath := filepath.Join("cmd", "lvt", "testdata", "golden", "resource_template.tmpl.golden")
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}

	if _, err := tmpl.Parse(string(content)); err != nil {
		t.Fatalf("parse template: %v", err)
	}

	state := &testPostsState{
		Title:          "Posts Management",
		PaginationMode: "infinite",
		CSSFramework:   "tailwind",
		TotalCount:     0,
		LoadedCount:    0,
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteUpdates(&buf, state); err != nil {
		t.Fatalf("initial execute: %v", err)
	}

	state.PaginatedPosts = []testPost{{
		ID:        "posts-1",
		Title:     "My First Blog Post",
		Content:   "This is the content of my first blog post",
		Published: true,
	}}
	state.TotalCount = 1
	state.LoadedCount = 1
	buf.Reset()
	if err := tmpl.ExecuteUpdates(&buf, state); err != nil {
		t.Fatalf("second execute: %v", err)
	}
	if testing.Verbose() {
		t.Logf("update payload: %s", buf.String())
	}

	var tree map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &tree); err != nil {
		t.Fatalf("unmarshal tree: %v", err)
	}

	found := false
	for _, v := range tree {
		node, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		rangeNode, ok := node["0"].(map[string]interface{})
		if !ok {
			continue
		}
		dItems, ok := rangeNode["d"].([]interface{})
		if !ok || len(dItems) == 0 {
			continue
		}
		firstItem, ok := dItems[0].(map[string]interface{})
		if !ok {
			continue
		}
		titleVal, ok := firstItem["1"].(string)
		if !ok {
			continue
		}
		found = true
		if titleVal != "My First Blog Post" {
			t.Fatalf("unexpected title dynamic: %q", titleVal)
		}
		break
	}

	if !found {
		t.Fatalf("range node with title dynamic not found: %v", tree)
	}
}
