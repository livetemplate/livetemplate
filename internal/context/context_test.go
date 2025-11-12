package context

import (
	"html/template"
	"reflect"
	"strings"
	"testing"
)

func TestNewTemplateContext(t *testing.T) {
	tests := []struct {
		name    string
		errors  map[string]string
		devMode bool
	}{
		{
			name:    "nil errors",
			errors:  nil,
			devMode: false,
		},
		{
			name:    "empty errors",
			errors:  map[string]string{},
			devMode: true,
		},
		{
			name: "with errors",
			errors: map[string]string{
				"email": "Invalid email",
				"name":  "Required",
			},
			devMode: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(tt.errors, tt.devMode)
			if ctx == nil {
				t.Fatal("NewTemplateContext returned nil")
			}
			if ctx.DevMode != tt.devMode {
				t.Errorf("DevMode = %v, want %v", ctx.DevMode, tt.devMode)
			}
		})
	}
}

func TestTemplateContext_Error(t *testing.T) {
	tests := []struct {
		name   string
		errors map[string]string
		field  string
		want   string
	}{
		{
			name:   "nil errors map",
			errors: nil,
			field:  "email",
			want:   "",
		},
		{
			name:   "empty errors map",
			errors: map[string]string{},
			field:  "email",
			want:   "",
		},
		{
			name: "field exists",
			errors: map[string]string{
				"email": "Invalid email",
			},
			field: "email",
			want:  "Invalid email",
		},
		{
			name: "field does not exist",
			errors: map[string]string{
				"email": "Invalid email",
			},
			field: "name",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(tt.errors, false)
			got := ctx.Error(tt.field)
			if got != tt.want {
				t.Errorf("Error(%q) = %q, want %q", tt.field, got, tt.want)
			}
		})
	}
}

func TestTemplateContext_HasError(t *testing.T) {
	tests := []struct {
		name   string
		errors map[string]string
		field  string
		want   bool
	}{
		{
			name:   "nil errors map",
			errors: nil,
			field:  "email",
			want:   false,
		},
		{
			name:   "empty errors map",
			errors: map[string]string{},
			field:  "email",
			want:   false,
		},
		{
			name: "field exists",
			errors: map[string]string{
				"email": "Invalid email",
			},
			field: "email",
			want:  true,
		},
		{
			name: "field does not exist",
			errors: map[string]string{
				"email": "Invalid email",
			},
			field: "name",
			want:  false,
		},
		{
			name: "field exists with empty value",
			errors: map[string]string{
				"email": "",
			},
			field: "email",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(tt.errors, false)
			got := ctx.HasError(tt.field)
			if got != tt.want {
				t.Errorf("HasError(%q) = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}

func TestTemplateContext_HasAnyError(t *testing.T) {
	tests := []struct {
		name   string
		errors map[string]string
		want   bool
	}{
		{
			name:   "nil errors map",
			errors: nil,
			want:   false,
		},
		{
			name:   "empty errors map",
			errors: map[string]string{},
			want:   false,
		},
		{
			name: "one error",
			errors: map[string]string{
				"email": "Invalid email",
			},
			want: true,
		},
		{
			name: "multiple errors",
			errors: map[string]string{
				"email": "Invalid email",
				"name":  "Required",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(tt.errors, false)
			got := ctx.HasAnyError()
			if got != tt.want {
				t.Errorf("HasAnyError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTemplateContext_AllErrors(t *testing.T) {
	tests := []struct {
		name   string
		errors map[string]string
	}{
		{
			name:   "nil errors map",
			errors: nil,
		},
		{
			name:   "empty errors map",
			errors: map[string]string{},
		},
		{
			name: "with errors",
			errors: map[string]string{
				"email": "Invalid email",
				"name":  "Required",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(tt.errors, false)
			got := ctx.AllErrors()

			if got == nil {
				t.Error("AllErrors() returned nil, want non-nil map")
			}

			if tt.errors == nil {
				if len(got) != 0 {
					t.Errorf("AllErrors() returned %d errors, want 0", len(got))
				}
			} else {
				if !reflect.DeepEqual(got, tt.errors) {
					t.Errorf("AllErrors() = %v, want %v", got, tt.errors)
				}
			}

			got["test_mutation"] = "should not affect internal state"
			got2 := ctx.AllErrors()
			if _, exists := got2["test_mutation"]; exists {
				t.Error("Mutation of AllErrors() return value affected internal state")
			}
		})
	}
}

func TestExecuteTemplateWithContext_StructData(t *testing.T) {
	type User struct {
		Name  string
		Email string
		Age   int
	}

	tmpl, err := template.New("test").Parse(`Name: {{.Name}}, Email: {{.Email}}, Age: {{.Age}}, DevMode: {{.lvt.DevMode}}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	data := User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	errors := map[string]string{"Email": "Invalid format"}
	result, err := ExecuteTemplateWithContext(tmpl, data, errors, true, nil)
	if err != nil {
		t.Fatalf("ExecuteTemplateWithContext failed: %v", err)
	}

	expected := "Name: John Doe, Email: john@example.com, Age: 30, DevMode: true"
	if string(result) != expected {
		t.Errorf("Result = %q, want %q", string(result), expected)
	}
}

func TestExecuteTemplateWithContext_StructPointer(t *testing.T) {
	type User struct {
		Name string
	}

	tmpl, err := template.New("test").Parse(`Name: {{.Name}}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	data := &User{Name: "Jane"}
	result, err := ExecuteTemplateWithContext(tmpl, data, nil, false, nil)
	if err != nil {
		t.Fatalf("ExecuteTemplateWithContext failed: %v", err)
	}

	expected := "Name: Jane"
	if string(result) != expected {
		t.Errorf("Result = %q, want %q", string(result), expected)
	}
}

func TestExecuteTemplateWithContext_MapData(t *testing.T) {
	tmpl, err := template.New("test").Parse(`Name: {{.name}}, City: {{.city}}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	data := map[string]interface{}{
		"name": "Alice",
		"city": "NYC",
	}

	result, err := ExecuteTemplateWithContext(tmpl, data, nil, false, nil)
	if err != nil {
		t.Fatalf("ExecuteTemplateWithContext failed: %v", err)
	}

	expected := "Name: Alice, City: NYC"
	if string(result) != expected {
		t.Errorf("Result = %q, want %q", string(result), expected)
	}
}

func TestExecuteTemplateWithContext_JSONTags(t *testing.T) {
	type User struct {
		FullName string `json:"full_name"`
		Email    string `json:"email,omitempty"`
		Password string `json:"-"`
		Age      int    `json:"age"`
	}

	tmpl, err := template.New("test").Parse(`Name: {{.full_name}}, Email: {{.email}}, Age: {{.age}}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	data := User{
		FullName: "Bob Smith",
		Email:    "bob@example.com",
		Password: "secret",
		Age:      25,
	}

	result, err := ExecuteTemplateWithContext(tmpl, data, nil, false, nil)
	if err != nil {
		t.Fatalf("ExecuteTemplateWithContext failed: %v", err)
	}

	expected := "Name: Bob Smith, Email: bob@example.com, Age: 25"
	if string(result) != expected {
		t.Errorf("Result = %q, want %q", string(result), expected)
	}
}

func TestExecuteTemplateWithContext_UnexportedFields(t *testing.T) {
	type User struct {
		Name     string
		password string
	}

	tmpl, err := template.New("test").Parse(`Name: {{.Name}}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	data := User{
		Name:     "Charlie",
		password: "secret",
	}

	result, err := ExecuteTemplateWithContext(tmpl, data, nil, false, nil)
	if err != nil {
		t.Fatalf("ExecuteTemplateWithContext failed: %v", err)
	}

	expected := "Name: Charlie"
	if string(result) != expected {
		t.Errorf("Result = %q, want %q", string(result), expected)
	}
}

func TestExecuteTemplateWithContext_WithErrors(t *testing.T) {
	type Form struct {
		Email string
		Name  string
	}

	tmpl, err := template.New("test").Parse(`{{if .lvt.HasError "Email"}}Error: {{.lvt.Error "Email"}}{{end}}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	data := Form{Email: "invalid", Name: "Test"}
	errors := map[string]string{"Email": "Invalid email format"}

	result, err := ExecuteTemplateWithContext(tmpl, data, errors, false, nil)
	if err != nil {
		t.Fatalf("ExecuteTemplateWithContext failed: %v", err)
	}

	expected := "Error: Invalid email format"
	if string(result) != expected {
		t.Errorf("Result = %q, want %q", string(result), expected)
	}
}

func TestExecuteTemplateWithContext_PrimitiveData(t *testing.T) {
	tmpl, err := template.New("test").Parse(`Value: {{.}}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	tests := []struct {
		name string
		data interface{}
		want string
	}{
		{"string", "hello", "Value: hello"},
		{"int", 42, "Value: 42"},
		{"bool", true, "Value: true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExecuteTemplateWithContext(tmpl, tt.data, nil, false, nil)
			if err != nil {
				t.Fatalf("ExecuteTemplateWithContext failed: %v", err)
			}
			if string(result) != tt.want {
				t.Errorf("Result = %q, want %q", string(result), tt.want)
			}
		})
	}
}

func TestExecuteTemplateWithContext_DevMode(t *testing.T) {
	tmpl, err := template.New("test").Parse(`{{if .lvt.DevMode}}DEV{{else}}PROD{{end}}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	tests := []struct {
		name    string
		devMode bool
		want    string
	}{
		{"dev mode on", true, "DEV"},
		{"dev mode off", false, "PROD"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExecuteTemplateWithContext(tmpl, struct{}{}, nil, tt.devMode, nil)
			if err != nil {
				t.Fatalf("ExecuteTemplateWithContext failed: %v", err)
			}
			if string(result) != tt.want {
				t.Errorf("Result = %q, want %q", string(result), tt.want)
			}
		})
	}
}

func TestExecuteTemplateWithContext_HasAnyError(t *testing.T) {
	tmpl, err := template.New("test").Parse(`{{if .lvt.HasAnyError}}ERRORS{{else}}OK{{end}}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	tests := []struct {
		name   string
		errors map[string]string
		want   string
	}{
		{"no errors", nil, "OK"},
		{"empty errors", map[string]string{}, "OK"},
		{"with errors", map[string]string{"field": "error"}, "ERRORS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExecuteTemplateWithContext(tmpl, struct{}{}, tt.errors, false, nil)
			if err != nil {
				t.Fatalf("ExecuteTemplateWithContext failed: %v", err)
			}
			if string(result) != tt.want {
				t.Errorf("Result = %q, want %q", string(result), tt.want)
			}
		})
	}
}

func ExampleNewTemplateContext() {
	errors := map[string]string{
		"email": "Invalid email format",
		"name":  "Name is required",
	}
	ctx := NewTemplateContext(errors, true)

	if ctx.HasError("email") {
		println(ctx.Error("email"))
	}
}

func ExampleExecuteTemplateWithContext() {
	type User struct {
		Name  string
		Email string
	}

	tmplStr := `Name: {{.Name}}
{{if .lvt.HasError "Email"}}Error: {{.lvt.Error "Email"}}{{end}}`

	tmpl, _ := template.New("user").Parse(tmplStr)
	data := User{Name: "John", Email: "invalid"}
	errors := map[string]string{"Email": "Invalid format"}

	result, _ := ExecuteTemplateWithContext(tmpl, data, errors, false, nil)
	_ = result
}

func BenchmarkExecuteTemplateWithContext_Struct(b *testing.B) {
	type User struct {
		Name  string
		Email string
		Age   int
	}

	tmpl, _ := template.New("test").Parse(`{{.Name}} {{.Email}} {{.Age}}`)
	data := User{Name: "John", Email: "john@example.com", Age: 30}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExecuteTemplateWithContext(tmpl, data, nil, false, nil)
	}
}

func BenchmarkExecuteTemplateWithContext_Map(b *testing.B) {
	tmpl, _ := template.New("test").Parse(`{{.name}} {{.email}} {{.age}}`)
	data := map[string]interface{}{
		"name":  "John",
		"email": "john@example.com",
		"age":   30,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExecuteTemplateWithContext(tmpl, data, nil, false, nil)
	}
}

func TestTemplateContext_ConcurrentReads(t *testing.T) {
	errors := map[string]string{
		"field1": "error1",
		"field2": "error2",
	}
	ctx := NewTemplateContext(errors, false)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = ctx.Error("field1")
				_ = ctx.HasError("field2")
				_ = ctx.HasAnyError()
				_ = ctx.AllErrors()
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestExecuteTemplateWithContext_FieldNameFallback(t *testing.T) {
	type User struct {
		FullName string `json:"full_name"`
	}

	tmpl, err := template.New("test").Parse(`JSON: {{.full_name}}, Field: {{.FullName}}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	data := User{FullName: "John Doe"}
	result, err := ExecuteTemplateWithContext(tmpl, data, nil, false, nil)
	if err != nil {
		t.Fatalf("ExecuteTemplateWithContext failed: %v", err)
	}

	expected := "JSON: John Doe, Field: John Doe"
	if string(result) != expected {
		t.Errorf("Result = %q, want %q", string(result), expected)
	}
}

func TestExecuteTemplateWithContext_EmptyJSONTag(t *testing.T) {
	type User struct {
		Name string `json:""`
	}

	tmpl, err := template.New("test").Parse(`Name: {{.Name}}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	data := User{Name: "Test"}
	result, err := ExecuteTemplateWithContext(tmpl, data, nil, false, nil)
	if err != nil {
		t.Fatalf("ExecuteTemplateWithContext failed: %v", err)
	}

	if !strings.Contains(string(result), "Test") {
		t.Errorf("Result should contain 'Test', got %q", string(result))
	}
}

// Test lvt field collision in struct
func TestExecuteTemplateWithContext_LvtFieldCollision(t *testing.T) {
	type Data struct {
		Name string
		Lvt  string // This field name conflicts with reserved key
	}

	tmpl, err := template.New("test").Parse(`Name: {{.Name}}, Context: {{.lvt.DevMode}}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	data := Data{Name: "Test", Lvt: "ShouldBeSkipped"}
	result, err := ExecuteTemplateWithContext(tmpl, data, nil, true)
	if err != nil {
		t.Fatalf("ExecuteTemplateWithContext failed: %v", err)
	}

	// The lvt context should be accessible, field "Lvt" should be skipped
	expected := "Name: Test, Context: true"
	if string(result) != expected {
		t.Errorf("Result = %q, want %q", string(result), expected)
	}
}

// Test lvt JSON tag collision in struct
func TestExecuteTemplateWithContext_LvtJSONTagCollision(t *testing.T) {
	type Data struct {
		Name    string
		Special string `json:"lvt"` // JSON tag conflicts with reserved key
	}

	tmpl, err := template.New("test").Parse(`Name: {{.Name}}, Context: {{.lvt.DevMode}}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	data := Data{Name: "Test", Special: "ShouldBeSkipped"}
	result, err := ExecuteTemplateWithContext(tmpl, data, nil, false, nil)
	if err != nil {
		t.Fatalf("ExecuteTemplateWithContext failed: %v", err)
	}

	// The lvt context should be accessible, json:"lvt" should be skipped
	expected := "Name: Test, Context: false"
	if string(result) != expected {
		t.Errorf("Result = %q, want %q", string(result), expected)
	}
}

// Test lvt key collision in map
func TestExecuteTemplateWithContext_MapLvtCollision(t *testing.T) {
	tmpl, err := template.New("test").Parse(`Name: {{.name}}, Context: {{.lvt.DevMode}}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	data := map[string]interface{}{
		"name": "Test",
		"lvt":  "ShouldBeSkipped", // This key conflicts with reserved key
	}

	result, err := ExecuteTemplateWithContext(tmpl, data, nil, true)
	if err != nil {
		t.Fatalf("ExecuteTemplateWithContext failed: %v", err)
	}

	// The lvt context should be accessible, map's "lvt" key should be skipped
	expected := "Name: Test, Context: true"
	if string(result) != expected {
		t.Errorf("Result = %q, want %q", string(result), expected)
	}
}

// Test nil pointer handling
func TestExecuteTemplateWithContext_NilPointer(t *testing.T) {
	type User struct {
		Name string
	}

	tmpl, err := template.New("test").Parse(`{{if .lvt}}Context exists: {{.lvt.DevMode}}{{else}}No context{{end}}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	var data *User = nil
	result, err := ExecuteTemplateWithContext(tmpl, data, nil, true)
	if err != nil {
		t.Fatalf("ExecuteTemplateWithContext failed: %v", err)
	}

	expected := "Context exists: true"
	if string(result) != expected {
		t.Errorf("Result = %q, want %q", string(result), expected)
	}
}

// Test comma-only JSON tag
func TestExecuteTemplateWithContext_CommaOnlyJSONTag(t *testing.T) {
	type User struct {
		Name  string `json:",omitempty"`
		Email string `json:",omitempty"`
	}

	tmpl, err := template.New("test").Parse(`Name: {{.Name}}, Email: {{.Email}}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	data := User{Name: "John", Email: "john@example.com"}
	result, err := ExecuteTemplateWithContext(tmpl, data, nil, false, nil)
	if err != nil {
		t.Fatalf("ExecuteTemplateWithContext failed: %v", err)
	}

	expected := "Name: John, Email: john@example.com"
	if string(result) != expected {
		t.Errorf("Result = %q, want %q", string(result), expected)
	}
}
