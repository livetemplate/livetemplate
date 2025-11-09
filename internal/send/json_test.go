package send

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMarshalOrderedJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
		wantErr  bool
	}{
		{
			name:     "simple object",
			input:    map[string]interface{}{"key": "value"},
			expected: `{"key":"value"}`,
			wantErr:  false,
		},
		{
			name:     "HTML content not escaped",
			input:    map[string]string{"html": "<div>content</div>"},
			expected: `{"html":"<div>content</div>"}`,
			wantErr:  false,
		},
		{
			name:     "nested object",
			input:    map[string]interface{}{"outer": map[string]string{"inner": "value"}},
			expected: `{"outer":{"inner":"value"}}`,
			wantErr:  false,
		},
		{
			name:     "array",
			input:    []string{"a", "b", "c"},
			expected: `["a","b","c"]`,
			wantErr:  false,
		},
		{
			name:     "string with special characters",
			input:    "Hello & \"World\" <tag>",
			expected: `"Hello & \"World\" <tag>"`,
			wantErr:  false,
		},
		{
			name:     "nil value",
			input:    nil,
			expected: `null`,
			wantErr:  false,
		},
		{
			name:     "empty object",
			input:    map[string]interface{}{},
			expected: `{}`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MarshalOrderedJSON(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalOrderedJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				resultStr := string(result)
				if resultStr != tt.expected {
					t.Errorf("MarshalOrderedJSON() = %q, want %q", resultStr, tt.expected)
				}

				// Verify no trailing newline
				if strings.HasSuffix(resultStr, "\n") {
					t.Error("MarshalOrderedJSON() should not have trailing newline")
				}

				// Verify HTML is not escaped
				if tt.name == "HTML content not escaped" && strings.Contains(resultStr, "\\u003c") {
					t.Error("MarshalOrderedJSON() should not escape HTML")
				}
			}
		})
	}
}

func TestMarshalValue(t *testing.T) {
	// Test that MarshalValue is correctly aliased to MarshalOrderedJSON
	input := map[string]string{"test": "<div>"}

	result1, err1 := MarshalOrderedJSON(input)
	result2, err2 := MarshalValue(input)

	if err1 != err2 {
		t.Errorf("MarshalValue error differs from MarshalOrderedJSON: %v vs %v", err2, err1)
	}

	if string(result1) != string(result2) {
		t.Errorf("MarshalValue() = %q, want %q (same as MarshalOrderedJSON)", result2, result1)
	}

	// Verify HTML is not escaped in MarshalValue
	if strings.Contains(string(result2), "\\u003c") {
		t.Error("MarshalValue() should not escape HTML")
	}
}

func TestMarshalOrderedJSONNoTrailingNewline(t *testing.T) {
	// Explicitly test that no trailing newline is present
	tests := []interface{}{
		"simple string",
		123,
		true,
		map[string]int{"count": 42},
		[]int{1, 2, 3},
	}

	for _, input := range tests {
		result, err := MarshalOrderedJSON(input)
		if err != nil {
			t.Errorf("MarshalOrderedJSON(%v) unexpected error: %v", input, err)
			continue
		}

		if len(result) > 0 && result[len(result)-1] == '\n' {
			t.Errorf("MarshalOrderedJSON(%v) has trailing newline: %q", input, result)
		}
	}
}

func TestMarshalOrderedJSONHTMLPreservation(t *testing.T) {
	// Test various HTML entities and tags
	htmlStrings := []string{
		"<div>",
		"</div>",
		"<script>alert('test')</script>",
		"&amp; &lt; &gt;",
		"<a href=\"test\">link</a>",
	}

	for _, html := range htmlStrings {
		input := map[string]string{"html": html}
		result, err := MarshalOrderedJSON(input)
		if err != nil {
			t.Errorf("MarshalOrderedJSON(%q) error: %v", html, err)
			continue
		}

		// Decode back to verify content is preserved
		var decoded map[string]string
		if err := json.Unmarshal(result, &decoded); err != nil {
			t.Errorf("Failed to decode result for %q: %v", html, err)
			continue
		}

		if decoded["html"] != html {
			t.Errorf("HTML not preserved: got %q, want %q", decoded["html"], html)
		}

		// Verify HTML entities are in the JSON as literal characters
		resultStr := string(result)
		if html == "<div>" && strings.Contains(resultStr, "\\u003c") {
			t.Errorf("HTML was escaped when it shouldn't be: %q", resultStr)
		}
	}
}

func TestMarshalOrderedJSONInvalidInput(t *testing.T) {
	// Test input that cannot be marshaled
	invalidInput := make(chan int) // channels cannot be marshaled

	_, err := MarshalOrderedJSON(invalidInput)
	if err == nil {
		t.Error("Expected error when marshaling invalid input (channel), got nil")
	}
}
