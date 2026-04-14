package livetemplate

import (
	"context"
	"math"
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

// TestContext_GetInt_NativeTypes verifies that GetInt accepts native Go
// numeric types. This matters for Session.TriggerAction, which passes
// Go-native values rather than JSON-unmarshaled ones.
func TestContext_GetInt_NativeTypes(t *testing.T) {
	data := map[string]interface{}{
		"native_int":     int(10),
		"native_int32":   int32(20),
		"native_int64":   int64(30),
		"native_uint":    uint(40),
		"native_float32": float32(50),
		"native_float64": float64(60),
		"numeric_string": "70",
	}
	ctx := NewContext(context.Background(), "test", data)

	cases := map[string]int{
		"native_int":     10,
		"native_int32":   20,
		"native_int64":   30,
		"native_uint":    40,
		"native_float32": 50,
		"native_float64": 60,
		"numeric_string": 70,
	}
	for key, want := range cases {
		if got := ctx.GetInt(key); got != want {
			t.Errorf("GetInt(%q) = %d, want %d", key, got, want)
		}
	}
}

// TestContext_GetInt_UnsignedOverflow verifies that GetInt rejects unsigned
// integers that exceed math.MaxInt on the current platform rather than
// silently wrapping to a negative value. The GetIntOk variant must return
// (0, false) for these cases so callers can detect the overflow.
func TestContext_GetInt_UnsignedOverflow(t *testing.T) {
	data := map[string]interface{}{
		"max_int":       uint64(math.MaxInt),     // exactly fits
		"overflow_u64":  uint64(math.MaxInt) + 1, // one past
		"overflow_uint": uint(math.MaxInt64) + 1, // one past on 64-bit; always overflows
		"int64_ok":      int64(math.MaxInt32),    // fits on any platform
	}
	ctx := NewContext(context.Background(), "test", data)
	d := ctx.data

	// Exact-fit uint64: should succeed.
	if got, ok := d.GetIntOk("max_int"); !ok || got != math.MaxInt {
		t.Errorf("GetIntOk(max_int) = (%d, %v), want (%d, true)", got, ok, math.MaxInt)
	}

	// Overflowing uint64: should return (0, false), NOT a wrapped negative.
	if got, ok := d.GetIntOk("overflow_u64"); ok || got != 0 {
		t.Errorf("GetIntOk(overflow_u64) = (%d, %v), want (0, false)", got, ok)
	}

	// Overflowing uint (platform-dependent but always overflows on 64-bit).
	if got, ok := d.GetIntOk("overflow_uint"); ok || got != 0 {
		t.Errorf("GetIntOk(overflow_uint) = (%d, %v), want (0, false)", got, ok)
	}

	// In-range int64: should succeed.
	if got, ok := d.GetIntOk("int64_ok"); !ok || got != math.MaxInt32 {
		t.Errorf("GetIntOk(int64_ok) = (%d, %v), want (%d, true)", got, ok, math.MaxInt32)
	}
}

// TestContext_GetFloat_NativeTypes mirrors TestContext_GetInt_NativeTypes
// for the float path. Verifies that all Go numeric types convert cleanly
// to float64 via GetFloat.
func TestContext_GetFloat_NativeTypes(t *testing.T) {
	data := map[string]interface{}{
		"native_int":     int(10),
		"native_int32":   int32(20),
		"native_int64":   int64(30),
		"native_uint":    uint(40),
		"native_uint32":  uint32(45),
		"native_float32": float32(50.5),
		"native_float64": float64(60.25),
		"numeric_string": "70.5",
	}
	ctx := NewContext(context.Background(), "test", data)

	cases := map[string]float64{
		"native_int":     10,
		"native_int32":   20,
		"native_int64":   30,
		"native_uint":    40,
		"native_uint32":  45,
		"native_float32": 50.5,
		"native_float64": 60.25,
		"numeric_string": 70.5,
	}
	for key, want := range cases {
		if got := ctx.GetFloat(key); got != want {
			t.Errorf("GetFloat(%q) = %v, want %v", key, got, want)
		}
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

func TestContext_GetBool_StringValues(t *testing.T) {
	// HTTP form submissions send booleans as strings "true"/"false"
	// GetBool should handle both boolean and string representations
	data := map[string]interface{}{
		"bool_true":   true,
		"bool_false":  false,
		"str_true":    "true",
		"str_false":   "false",
		"str_invalid": "yes",
		"str_empty":   "",
	}
	ctx := NewContext(context.Background(), "test", data)

	// Boolean values should work
	if got := ctx.GetBool("bool_true"); !got {
		t.Error("GetBool(bool_true) = false, want true")
	}
	if got := ctx.GetBool("bool_false"); got {
		t.Error("GetBool(bool_false) = true, want false")
	}

	// String "true"/"false" should work (HTTP form path)
	if got := ctx.GetBool("str_true"); !got {
		t.Error("GetBool(str_true) = false, want true")
	}
	if got := ctx.GetBool("str_false"); got {
		t.Error("GetBool(str_false) = true, want false")
	}

	// Invalid strings should return false
	if got := ctx.GetBool("str_invalid"); got {
		t.Error("GetBool(str_invalid) = true, want false")
	}
	if got := ctx.GetBool("str_empty"); got {
		t.Error("GetBool(str_empty) = true, want false")
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

func TestContext_GetString_NumericValues(t *testing.T) {
	// The client-side parseValue() converts numeric strings like "1" to numbers.
	// GetString should handle both string and numeric values.
	data := map[string]interface{}{
		"id_string": "123",
		"id_int":    float64(456), // JSON numbers are float64
		"id_float":  float64(3.14),
		"negative":  float64(-42),
		"safe_int":  float64(9007199254740991), // Max safe integer in JavaScript
		"zero":      float64(0),
	}
	ctx := NewContext(context.Background(), "test", data)

	// String value should work as before
	if got := ctx.GetString("id_string"); got != "123" {
		t.Errorf("GetString(id_string) = %q, want %q", got, "123")
	}

	// Integer as float64 should convert to string
	if got := ctx.GetString("id_int"); got != "456" {
		t.Errorf("GetString(id_int) = %q, want %q", got, "456")
	}

	// Float should convert to string without scientific notation
	if got := ctx.GetString("id_float"); got != "3.14" {
		t.Errorf("GetString(id_float) = %q, want %q", got, "3.14")
	}

	// Negative integers should work
	if got := ctx.GetString("negative"); got != "-42" {
		t.Errorf("GetString(negative) = %q, want %q", got, "-42")
	}

	// Large integers within safe range should work
	if got := ctx.GetString("safe_int"); got != "9007199254740991" {
		t.Errorf("GetString(safe_int) = %q, want %q", got, "9007199254740991")
	}

	// Zero should work
	if got := ctx.GetString("zero"); got != "0" {
		t.Errorf("GetString(zero) = %q, want %q", got, "0")
	}
}
