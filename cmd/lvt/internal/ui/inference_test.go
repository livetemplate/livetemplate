package ui

import "testing"

func TestInferType(t *testing.T) {
	tests := []struct {
		fieldName string
		want      string
	}{
		// String fields
		{"name", "string"},
		{"title", "string"},
		{"email", "string"},
		{"description", "string"},
		{"user_email", "string"},
		{"product_url", "string"},

		// Integer fields
		{"age", "int"},
		{"count", "int"},
		{"view_count", "int"},
		{"item_number", "int"},
		{"page_index", "int"},

		// Float fields
		{"price", "float"},
		{"product_price", "float"},
		{"total_amount", "float"},
		{"rating", "float"},
		{"discount_rate", "float"},

		// Boolean fields
		{"enabled", "bool"},
		{"is_active", "bool"},
		{"has_permission", "bool"},
		{"can_edit", "bool"},
		{"published", "bool"},

		// Time fields
		{"created_at", "time"},
		{"updated_at", "time"},
		{"published_date", "time"},
		{"start_time", "time"},

		// Default to string
		{"unknown_field", "string"},
		{"random", "string"},
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			got := InferType(tt.fieldName)
			if got != tt.want {
				t.Errorf("InferType(%q) = %q, want %q", tt.fieldName, got, tt.want)
			}
		})
	}
}

func TestParseFieldInput(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantType string
	}{
		// Without type - inferred
		{"name", "name", "string"},
		{"age", "age", "int"},
		{"price", "price", "float"},
		{"enabled", "enabled", "bool"},
		{"created_at", "created_at", "time"},

		// With type - explicit
		{"name:string", "name", "string"},
		{"age:int", "age", "int"},
		{"score:float", "score", "float"},
		{"active:bool", "active", "bool"},
		{"birthdate:time", "birthdate", "time"},

		// Override inference
		{"age:float", "age", "float"}, // age would be int, but overridden
		{"name:int", "name", "int"},   // name would be string, but overridden

		// Whitespace handling
		{" name ", "name", "string"},
		{" age : int ", "age", "int"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotName, gotType := ParseFieldInput(tt.input)
			if gotName != tt.wantName {
				t.Errorf("ParseFieldInput(%q) name = %q, want %q", tt.input, gotName, tt.wantName)
			}
			if gotType != tt.wantType {
				t.Errorf("ParseFieldInput(%q) type = %q, want %q", tt.input, gotType, tt.wantType)
			}
		})
	}
}
