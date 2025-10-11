package parser

import (
	"testing"
)

func TestParseFields(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    int
		wantErr bool
	}{
		{
			name:  "valid fields",
			input: []string{"name:string", "email:string", "age:int"},
			want:  3,
		},
		{
			name:  "single field",
			input: []string{"title:string"},
			want:  1,
		},
		{
			name:    "empty input",
			input:   []string{},
			wantErr: true,
		},
		{
			name:    "invalid format - missing colon",
			input:   []string{"name"},
			wantErr: true,
		},
		{
			name:    "invalid format - empty name",
			input:   []string{":string"},
			wantErr: true,
		},
		{
			name:    "invalid format - empty type",
			input:   []string{"name:"},
			wantErr: true,
		},
		{
			name:    "unsupported type",
			input:   []string{"id:uuid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, err := ParseFields(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if len(fields) != tt.want {
				t.Errorf("got %d fields, want %d", len(fields), tt.want)
			}
		})
	}
}

func TestMapType(t *testing.T) {
	tests := []struct {
		input        string
		wantGo       string
		wantSQL      string
		wantTextarea bool
		wantErr      bool
	}{
		{"string", "string", "TEXT", false, false},
		{"str", "string", "TEXT", false, false},
		{"text", "string", "TEXT", true, false},
		{"textarea", "string", "TEXT", true, false},
		{"longtext", "string", "TEXT", true, false},
		{"int", "int64", "INTEGER", false, false},
		{"integer", "int64", "INTEGER", false, false},
		{"bool", "bool", "BOOLEAN", false, false},
		{"boolean", "bool", "BOOLEAN", false, false},
		{"float", "float64", "REAL", false, false},
		{"float64", "float64", "REAL", false, false},
		{"time", "time.Time", "DATETIME", false, false},
		{"datetime", "time.Time", "DATETIME", false, false},
		{"uuid", "", "", false, true},
		{"unknown", "", "", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			goType, sqlType, isTextarea, err := MapType(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if goType != tt.wantGo {
				t.Errorf("got Go type %s, want %s", goType, tt.wantGo)
			}
			if sqlType != tt.wantSQL {
				t.Errorf("got SQL type %s, want %s", sqlType, tt.wantSQL)
			}
			if isTextarea != tt.wantTextarea {
				t.Errorf("got isTextarea %v, want %v", isTextarea, tt.wantTextarea)
			}
		})
	}
}

func TestFieldsToGoStruct(t *testing.T) {
	fields := []Field{
		{Name: "name", GoType: "string"},
		{Name: "age", GoType: "int64"},
	}

	result := FieldsToGoStruct(fields)

	// Check for expected field declarations
	if !contains(result, "Name string") {
		t.Error("expected 'Name string' in struct")
	}
	if !contains(result, "Age int64") {
		t.Error("expected 'Age int64' in struct")
	}
	if !contains(result, "json:\"name\"") {
		t.Error("expected json tag for name")
	}
	if !contains(result, "json:\"age\"") {
		t.Error("expected json tag for age")
	}
}

func TestFieldsToSQLColumns(t *testing.T) {
	fields := []Field{
		{Name: "name", SQLType: "TEXT"},
		{Name: "age", SQLType: "INTEGER"},
	}

	result := FieldsToSQLColumns(fields)

	if !contains(result, "name TEXT NOT NULL") {
		t.Error("expected 'name TEXT NOT NULL' in SQL")
	}
	if !contains(result, "age INTEGER NOT NULL") {
		t.Error("expected 'age INTEGER NOT NULL' in SQL")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
