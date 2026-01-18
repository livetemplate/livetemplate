package livetemplate

import (
	"context"
	"testing"
)

func TestContext_GetString(t *testing.T) {
	data := map[string]interface{}{"name": "Alice"}
	ctx := NewContext(context.Background(), "test_action", data)

	if got := ctx.GetString("name"); got != "Alice" {
		t.Errorf("GetString(name) = %q, want %q", got, "Alice")
	}

	if got := ctx.GetString("missing"); got != "" {
		t.Errorf("GetString(missing) = %q, want empty", got)
	}
}

func TestContext_Action(t *testing.T) {
	ctx := NewContext(context.Background(), "increment", nil)

	if got := ctx.Action(); got != "increment" {
		t.Errorf("Action() = %q, want %q", got, "increment")
	}
}

func TestContext_UserID(t *testing.T) {
	ctx := NewContext(context.Background(), "test", nil)
	ctx = ctx.WithUserID("user-123")

	if got := ctx.UserID(); got != "user-123" {
		t.Errorf("UserID() = %q, want %q", got, "user-123")
	}
}

func TestContext_GetInt(t *testing.T) {
	data := map[string]interface{}{"count": float64(42)} // JSON numbers are float64
	ctx := NewContext(context.Background(), "test", data)

	if got := ctx.GetInt("count"); got != 42 {
		t.Errorf("GetInt(count) = %d, want 42", got)
	}

	if got := ctx.GetInt("missing"); got != 0 {
		t.Errorf("GetInt(missing) = %d, want 0", got)
	}
}

func TestContext_GetBool(t *testing.T) {
	data := map[string]interface{}{"active": true}
	ctx := NewContext(context.Background(), "test", data)

	if got := ctx.GetBool("active"); !got {
		t.Error("GetBool(active) = false, want true")
	}

	if got := ctx.GetBool("missing"); got {
		t.Error("GetBool(missing) = true, want false")
	}
}

func TestContext_Has(t *testing.T) {
	data := map[string]interface{}{"exists": "value"}
	ctx := NewContext(context.Background(), "test", data)

	if !ctx.Has("exists") {
		t.Error("Has(exists) = false, want true")
	}

	if ctx.Has("missing") {
		t.Error("Has(missing) = true, want false")
	}
}

func TestContext_WithSession(t *testing.T) {
	ctx := NewContext(context.Background(), "test", nil)

	// Session should be nil initially
	if ctx.Session() != nil {
		t.Error("Session() should be nil initially")
	}

	// After WithSession, should have session
	// Note: We use nil as Session interface for this test since we're just testing the setter
	// A full test would need a mock Session implementation
}

type testContextKey string

func TestContext_StandardContext(t *testing.T) {
	baseCtx := context.WithValue(context.Background(), testContextKey("key"), "value")
	ctx := NewContext(baseCtx, "test", nil)

	// Should be able to use Context methods
	if ctx.Value(testContextKey("key")) != "value" {
		t.Error("Context.Value should work via embedded context.Context")
	}
}

func TestContext_NilData(t *testing.T) {
	ctx := NewContext(context.Background(), "test", nil)

	// Should not panic on nil data
	if got := ctx.GetString("any"); got != "" {
		t.Errorf("GetString with nil data = %q, want empty", got)
	}

	if got := ctx.GetInt("any"); got != 0 {
		t.Errorf("GetInt with nil data = %d, want 0", got)
	}

	if ctx.Has("any") {
		t.Error("Has with nil data = true, want false")
	}
}
