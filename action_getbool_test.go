package livetemplate

import (
	"math"
	"testing"
)

// TestGetBoolOk_CheckboxWireShapes pins the value shapes a checkbox actually
// reaches a handler as. Which shape arrives depends on how the form was
// submitted, and none of them is discoverable from the handler's side.
func TestGetBoolOk_CheckboxWireShapes(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    bool
		wantOk  bool
		comment string
	}{
		// WebSocket: the client sends input.checked.
		{"ws checked", true, true, true, ""},
		{"ws unchecked", false, false, true, ""},

		// No-JS POST: every field is a string. <input type="checkbox" value="1">
		// posts "1"; with no value attribute the browser posts "on".
		{"post value attr", "1", true, true, ""},
		{"post default on", "on", true, true, ""},
		{"post uppercase On", "On", true, true, "matching is case-insensitive"},
		{"post explicit zero", "0", false, true, "a hidden input can carry 0"},
		{"post off", "off", false, true, ""},
		{"post true", "true", true, true, "still accepted"},
		{"post FALSE", "FALSE", false, true, "still accepted"},

		// parseValue coerces numeric-looking input values to JSON numbers, so a
		// hidden <input value="1"> arrives as float64 over the socket and as
		// "1" over a plain POST. Both must read the same.
		{"ws numeric one", float64(1), true, true, ""},
		{"ws numeric zero", float64(0), false, true, ""},

		// Session.TriggerAction takes a map[string]interface{} a caller fills
		// with native values, so the numeric widths GetIntOk and GetFloatOk
		// accept have to read the same here.
		{"native int", 1, true, true, ""},
		{"native int32", int32(1), true, true, ""},
		{"native int64 zero", int64(0), false, true, ""},
		{"native uint8", uint8(1), true, true, ""},
		{"native float32", float32(1), true, true, ""},

		// Anything else is not a boolean, and saying so is the point of the Ok.
		{"free text", "yes", false, false, ""},
		{"numeric string", "2", false, false, "a number that is not 1/0 is data, not a flag"},
		{"empty string", "", false, false, ""},
		{"nil", nil, false, false, ""},
		{"NaN", math.NaN(), false, false, "NaN != 0 is true in Go; it must not read as ticked"},
		{"+Inf", math.Inf(1), false, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewActionData(map[string]interface{}{"box": tt.value})
			got, ok := a.GetBoolOk("box")
			if got != tt.want || ok != tt.wantOk {
				t.Errorf("GetBoolOk(box) with %#v = (%v, %v), want (%v, %v)%s",
					tt.value, got, ok, tt.want, tt.wantOk, note(tt.comment))
			}
		})
	}

	// An unchecked box is not posted at all on the no-JS path. The absent key
	// must read false, which is what lets one handler write the "off" branch
	// without a separate "was it submitted" check.
	a := NewActionData(map[string]interface{}{})
	if got, ok := a.GetBoolOk("box"); got || ok {
		t.Errorf("GetBoolOk(absent) = (%v, %v), want (false, false)", got, ok)
	}
}

// TestGetBoolOk_TransportParity asserts the property directly: the same
// checkbox, submitted over the socket and as a plain form POST, reads the same.
// A handler that is only correct on one transport defeats progressive
// enhancement, which promises the server handles plain POSTs end-to-end.
func TestGetBoolOk_TransportParity(t *testing.T) {
	for _, tc := range []struct {
		name string
		ws   interface{} // what the client sends
		post interface{} // what the browser posts with JS off
		want bool
	}{
		{"checked", true, "1", true},
		{"checked, no value attr", true, "on", true},
		{"unchecked", false, nil, false}, // nil stands for "key absent"
	} {
		t.Run(tc.name, func(t *testing.T) {
			wsData := map[string]interface{}{"box": tc.ws}
			postData := map[string]interface{}{}
			if tc.post != nil {
				postData["box"] = tc.post
			}
			// GetBoolOk, not GetBool: whether the value was recognized as
			// boolean-shaped at all is half the parity property. An absent key
			// reads (false, false) and a sent `false` reads (false, true) —
			// both mean "unchecked", so only the value has to agree.
			ws, wsOk := NewActionData(wsData).GetBoolOk("box")
			post, postOk := NewActionData(postData).GetBoolOk("box")
			if ws != post {
				t.Errorf("transport disagreement: websocket=%v (ok=%v), POST=%v (ok=%v)",
					ws, wsOk, post, postOk)
			}
			if ws != tc.want {
				t.Errorf("GetBoolOk = %v, want %v", ws, tc.want)
			}
			// A value that WAS sent must be recognized on both sides; only the
			// unchecked-and-therefore-absent case may report not-ok.
			if !wsOk {
				t.Errorf("websocket value %#v not recognized as boolean", tc.ws)
			}
			if tc.post != nil && !postOk {
				t.Errorf("POST value %#v not recognized as boolean", tc.post)
			}
		})
	}
}

func note(s string) string {
	if s == "" {
		return ""
	}
	return " — " + s
}
