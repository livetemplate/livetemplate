package context

import (
	"html/template"
	"reflect"
	"strings"
	"testing"
)

func TestNewTemplateContext(t *testing.T) {
	tests := []struct {
		name     string
		messages map[string]string
		devMode  bool
	}{
		{
			name:     "nil messages",
			messages: nil,
			devMode:  false,
		},
		{
			name:     "empty messages",
			messages: map[string]string{},
			devMode:  true,
		},
		{
			name: "with errors",
			messages: map[string]string{
				"email": "Invalid email",
				"name":  "Required",
			},
			devMode: false,
		},
		{
			name: "with flash messages",
			messages: map[string]string{
				"_flash:success": "Changes saved!",
				"_flash:error":   "Something went wrong",
			},
			devMode: true,
		},
		{
			name: "with mixed errors and flash",
			messages: map[string]string{
				"email":          "Invalid email",
				"_flash:success": "Partial save completed",
			},
			devMode: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(tt.messages, tt.devMode)
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
		name     string
		messages map[string]string
		field    string
		want     string
	}{
		{
			name:     "nil messages map",
			messages: nil,
			field:    "email",
			want:     "",
		},
		{
			name:     "empty messages map",
			messages: map[string]string{},
			field:    "email",
			want:     "",
		},
		{
			name: "field exists",
			messages: map[string]string{
				"email": "Invalid email",
			},
			field: "email",
			want:  "Invalid email",
		},
		{
			name: "field does not exist",
			messages: map[string]string{
				"email": "Invalid email",
			},
			field: "name",
			want:  "",
		},
		{
			name: "flash message not returned as error",
			messages: map[string]string{
				"_flash:success": "Changes saved!",
			},
			field: "success",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(tt.messages, false)
			got := ctx.Error(tt.field)
			if got != tt.want {
				t.Errorf("Error(%q) = %q, want %q", tt.field, got, tt.want)
			}
		})
	}
}

func TestTemplateContext_HasError(t *testing.T) {
	tests := []struct {
		name     string
		messages map[string]string
		field    string
		want     bool
	}{
		{
			name:     "nil messages map",
			messages: nil,
			field:    "email",
			want:     false,
		},
		{
			name:     "empty messages map",
			messages: map[string]string{},
			field:    "email",
			want:     false,
		},
		{
			name: "field exists",
			messages: map[string]string{
				"email": "Invalid email",
			},
			field: "email",
			want:  true,
		},
		{
			name: "field does not exist",
			messages: map[string]string{
				"email": "Invalid email",
			},
			field: "name",
			want:  false,
		},
		{
			name: "field exists with empty value",
			messages: map[string]string{
				"email": "",
			},
			field: "email",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(tt.messages, false)
			got := ctx.HasError(tt.field)
			if got != tt.want {
				t.Errorf("HasError(%q) = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}

func TestTemplateContext_ErrorTag(t *testing.T) {
	tests := []struct {
		name     string
		messages map[string]string
		field    string
		want     template.HTML
	}{
		{
			name:     "nil messages map",
			messages: nil,
			field:    "email",
			want:     "",
		},
		{
			name:     "field not in map",
			messages: map[string]string{},
			field:    "email",
			want:     "",
		},
		{
			name: "field exists",
			messages: map[string]string{
				"email": "Invalid email",
			},
			field: "email",
			want:  "<small>Invalid email</small>",
		},
		{
			name: "field does not exist",
			messages: map[string]string{
				"email": "Invalid email",
			},
			field: "name",
			want:  "",
		},
		{
			name: "HTML chars in message are escaped",
			messages: map[string]string{
				"count": "must be <10 & >0",
			},
			field: "count",
			want:  "<small>must be &lt;10 &amp; &gt;0</small>",
		},
		{
			name: "flash message key returns empty",
			messages: map[string]string{
				"_flash:success": "Saved!",
			},
			field: "_flash:success",
			want:  "",
		},
		{
			name: "empty error message returns empty",
			messages: map[string]string{
				"email": "",
			},
			field: "email",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(tt.messages, false)
			got := ctx.ErrorTag(tt.field)
			if got != tt.want {
				t.Errorf("ErrorTag(%q) = %q, want %q", tt.field, got, tt.want)
			}
		})
	}
}

func TestTemplateContext_Redact(t *testing.T) {
	tests := []struct {
		name  string
		field string
		want  template.HTML
	}{
		{
			name:  "simple field",
			field: "passport",
			want:  `<span data-lvt-redact="passport"></span>`,
		},
		{
			name:  "snake_case field",
			field: "tax_id",
			want:  `<span data-lvt-redact="tax_id"></span>`,
		},
		{
			name:  "empty field name",
			field: "",
			want:  `<span data-lvt-redact=""></span>`,
		},
		{
			name:  "HTML-special chars in name are escaped",
			field: `x"><script>alert(1)</script>`,
			want:  `<span data-lvt-redact="x&#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;"></span>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(nil, false)
			got := ctx.Redact(tt.field)
			if got != tt.want {
				t.Errorf("Redact(%q) = %q, want %q", tt.field, got, tt.want)
			}
		})
	}
}

// TestTemplateContext_Redact_RendersAsElement guards that Redact emits a live
// element in content context (not escaped to text), so the client can find it
// by attribute. The name stays escaped inside the attribute.
func TestTemplateContext_Redact_RendersAsElement(t *testing.T) {
	ctx := NewTemplateContext(nil, false)
	data := map[string]any{"lvt": ctx}
	tmpl := template.Must(template.New("c").Parse(`<p>{{.lvt.Redact "passport"}}</p>`))
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got, want := buf.String(), `<p><span data-lvt-redact="passport"></span></p>`; got != want {
		t.Errorf("got %q, want %q — Redact must render as a live element in content", got, want)
	}
}

func TestTemplateContext_AriaInvalid(t *testing.T) {
	tests := []struct {
		name     string
		messages map[string]string
		field    string
		want     template.HTMLAttr
	}{
		{
			name:     "nil messages",
			messages: nil,
			field:    "email",
			want:     "",
		},
		{
			name:     "no error for field",
			messages: map[string]string{"title": "Required"},
			field:    "email",
			want:     "",
		},
		{
			name:     "error exists",
			messages: map[string]string{"email": "Invalid"},
			field:    "email",
			want:     `aria-invalid="true"`,
		},
		{
			name:     "flash key returns empty",
			messages: map[string]string{"_flash:success": "OK"},
			field:    "_flash:success",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(tt.messages, false)
			got := ctx.AriaInvalid(tt.field)
			if got != tt.want {
				t.Errorf("AriaInvalid(%q) = %q, want %q", tt.field, got, tt.want)
			}
		})
	}
}

func TestTemplateContext_HasAnyError(t *testing.T) {
	tests := []struct {
		name     string
		messages map[string]string
		want     bool
	}{
		{
			name:     "nil messages map",
			messages: nil,
			want:     false,
		},
		{
			name:     "empty messages map",
			messages: map[string]string{},
			want:     false,
		},
		{
			name: "one error",
			messages: map[string]string{
				"email": "Invalid email",
			},
			want: true,
		},
		{
			name: "multiple errors",
			messages: map[string]string{
				"email": "Invalid email",
				"name":  "Required",
			},
			want: true,
		},
		{
			name: "only flash messages - no errors",
			messages: map[string]string{
				"_flash:success": "Changes saved!",
				"_flash:info":    "Welcome back",
			},
			want: false,
		},
		{
			name: "mixed errors and flash",
			messages: map[string]string{
				"email":          "Invalid email",
				"_flash:success": "Partial save",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(tt.messages, false)
			got := ctx.HasAnyError()
			if got != tt.want {
				t.Errorf("HasAnyError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTemplateContext_AllErrors(t *testing.T) {
	tests := []struct {
		name       string
		messages   map[string]string
		wantErrors map[string]string
	}{
		{
			name:       "nil messages map",
			messages:   nil,
			wantErrors: map[string]string{},
		},
		{
			name:       "empty messages map",
			messages:   map[string]string{},
			wantErrors: map[string]string{},
		},
		{
			name: "with errors only",
			messages: map[string]string{
				"email": "Invalid email",
				"name":  "Required",
			},
			wantErrors: map[string]string{
				"email": "Invalid email",
				"name":  "Required",
			},
		},
		{
			name: "flash messages excluded",
			messages: map[string]string{
				"email":          "Invalid email",
				"_flash:success": "Changes saved!",
				"_flash:info":    "Welcome",
			},
			wantErrors: map[string]string{
				"email": "Invalid email",
			},
		},
		{
			name: "only flash messages",
			messages: map[string]string{
				"_flash:success": "Changes saved!",
			},
			wantErrors: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(tt.messages, false)
			got := ctx.AllErrors()

			if got == nil {
				t.Error("AllErrors() returned nil, want non-nil map")
			}

			if !reflect.DeepEqual(got, tt.wantErrors) {
				t.Errorf("AllErrors() = %v, want %v", got, tt.wantErrors)
			}

			got["test_mutation"] = "should not affect internal state"
			got2 := ctx.AllErrors()
			if _, exists := got2["test_mutation"]; exists {
				t.Error("Mutation of AllErrors() return value affected internal state")
			}
		})
	}
}

// Flash message tests

func TestTemplateContext_Flash(t *testing.T) {
	tests := []struct {
		name     string
		messages map[string]string
		key      string
		want     string
	}{
		{
			name:     "nil messages map",
			messages: nil,
			key:      "success",
			want:     "",
		},
		{
			name:     "empty messages map",
			messages: map[string]string{},
			key:      "success",
			want:     "",
		},
		{
			name: "flash key exists",
			messages: map[string]string{
				"_flash:success": "Changes saved!",
			},
			key:  "success",
			want: "Changes saved!",
		},
		{
			name: "flash key does not exist",
			messages: map[string]string{
				"_flash:success": "Changes saved!",
			},
			key:  "error",
			want: "",
		},
		{
			name: "error not returned as flash",
			messages: map[string]string{
				"email": "Invalid email",
			},
			key:  "email",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(tt.messages, false)
			got := ctx.Flash(tt.key)
			if got != tt.want {
				t.Errorf("Flash(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestTemplateContext_HasFlash(t *testing.T) {
	tests := []struct {
		name     string
		messages map[string]string
		key      string
		want     bool
	}{
		{
			name:     "nil messages map",
			messages: nil,
			key:      "success",
			want:     false,
		},
		{
			name:     "empty messages map",
			messages: map[string]string{},
			key:      "success",
			want:     false,
		},
		{
			name: "flash key exists",
			messages: map[string]string{
				"_flash:success": "Changes saved!",
			},
			key:  "success",
			want: true,
		},
		{
			name: "flash key does not exist",
			messages: map[string]string{
				"_flash:success": "Changes saved!",
			},
			key:  "error",
			want: false,
		},
		{
			name: "flash key exists with empty value",
			messages: map[string]string{
				"_flash:warning": "",
			},
			key:  "warning",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(tt.messages, false)
			got := ctx.HasFlash(tt.key)
			if got != tt.want {
				t.Errorf("HasFlash(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestTemplateContext_HasAnyFlash(t *testing.T) {
	tests := []struct {
		name     string
		messages map[string]string
		want     bool
	}{
		{
			name:     "nil messages map",
			messages: nil,
			want:     false,
		},
		{
			name:     "empty messages map",
			messages: map[string]string{},
			want:     false,
		},
		{
			name: "one flash message",
			messages: map[string]string{
				"_flash:success": "Changes saved!",
			},
			want: true,
		},
		{
			name: "multiple flash messages",
			messages: map[string]string{
				"_flash:success": "Changes saved!",
				"_flash:info":    "Welcome back",
			},
			want: true,
		},
		{
			name: "only errors - no flash",
			messages: map[string]string{
				"email": "Invalid email",
				"name":  "Required",
			},
			want: false,
		},
		{
			name: "mixed errors and flash",
			messages: map[string]string{
				"email":          "Invalid email",
				"_flash:success": "Partial save",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(tt.messages, false)
			got := ctx.HasAnyFlash()
			if got != tt.want {
				t.Errorf("HasAnyFlash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTemplateContext_AllFlash(t *testing.T) {
	tests := []struct {
		name      string
		messages  map[string]string
		wantFlash map[string]string
	}{
		{
			name:      "nil messages map",
			messages:  nil,
			wantFlash: map[string]string{},
		},
		{
			name:      "empty messages map",
			messages:  map[string]string{},
			wantFlash: map[string]string{},
		},
		{
			name: "with flash only",
			messages: map[string]string{
				"_flash:success": "Changes saved!",
				"_flash:info":    "Welcome",
			},
			wantFlash: map[string]string{
				"success": "Changes saved!",
				"info":    "Welcome",
			},
		},
		{
			name: "errors excluded from flash",
			messages: map[string]string{
				"email":          "Invalid email",
				"_flash:success": "Changes saved!",
			},
			wantFlash: map[string]string{
				"success": "Changes saved!",
			},
		},
		{
			name: "only errors",
			messages: map[string]string{
				"email": "Invalid email",
			},
			wantFlash: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(tt.messages, false)
			got := ctx.AllFlash()

			if got == nil {
				t.Error("AllFlash() returned nil, want non-nil map")
			}

			if !reflect.DeepEqual(got, tt.wantFlash) {
				t.Errorf("AllFlash() = %v, want %v", got, tt.wantFlash)
			}

			got["test_mutation"] = "should not affect internal state"
			got2 := ctx.AllFlash()
			if _, exists := got2["test_mutation"]; exists {
				t.Error("Mutation of AllFlash() return value affected internal state")
			}
		})
	}
}

func TestTemplateContext_AriaDisabled(t *testing.T) {
	tests := []struct {
		name     string
		messages map[string]string
		fields   []string
		want     template.HTMLAttr
	}{
		{
			name:     "nil messages",
			messages: nil,
			fields:   []string{"email"},
			want:     "",
		},
		{
			name:     "empty messages",
			messages: map[string]string{},
			fields:   []string{"email"},
			want:     "",
		},
		{
			name:     "no error for field",
			messages: map[string]string{"title": "Required"},
			fields:   []string{"email"},
			want:     "",
		},
		{
			name:     "error exists",
			messages: map[string]string{"email": "Invalid"},
			fields:   []string{"email"},
			want:     `aria-disabled="true"`,
		},
		{
			name:     "flash key returns empty",
			messages: map[string]string{"_flash:success": "OK"},
			fields:   []string{"_flash:success"},
			want:     "",
		},
		{
			name:     "multiple fields none with error",
			messages: map[string]string{},
			fields:   []string{"email", "name", "phone"},
			want:     "",
		},
		{
			name:     "multiple fields one with error",
			messages: map[string]string{"name": "Required"},
			fields:   []string{"email", "name", "phone"},
			want:     `aria-disabled="true"`,
		},
		{
			name:     "multiple fields all with errors",
			messages: map[string]string{"email": "Invalid", "name": "Required"},
			fields:   []string{"email", "name"},
			want:     `aria-disabled="true"`,
		},
		{
			name:     "no fields provided",
			messages: map[string]string{"email": "Invalid"},
			fields:   []string{},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(tt.messages, false)
			got := ctx.AriaDisabled(tt.fields...)
			if got != tt.want {
				t.Errorf("AriaDisabled(%v) = %q, want %q", tt.fields, got, tt.want)
			}
		})
	}
}

func TestTemplateContext_FlashTag(t *testing.T) {
	tests := []struct {
		name     string
		messages map[string]string
		key      string
		want     template.HTML
	}{
		{
			name:     "nil messages",
			messages: nil,
			key:      "success",
			want:     "",
		},
		{
			name:     "key not in map",
			messages: map[string]string{},
			key:      "success",
			want:     "",
		},
		{
			name:     "success flash uses output with status role",
			messages: map[string]string{"_flash:success": "Saved!"},
			key:      "success",
			want:     `<output role="status" data-flash="success">Saved!</output>`,
		},
		{
			name:     "error flash uses output with alert role",
			messages: map[string]string{"_flash:error": "Failed"},
			key:      "error",
			want:     `<output role="alert" data-flash="error">Failed</output>`,
		},
		{
			name:     "warning flash uses output with status role",
			messages: map[string]string{"_flash:warning": "Watch out"},
			key:      "warning",
			want:     `<output role="status" data-flash="warning">Watch out</output>`,
		},
		{
			name:     "info flash uses output with status role",
			messages: map[string]string{"_flash:info": "FYI"},
			key:      "info",
			want:     `<output role="status" data-flash="info">FYI</output>`,
		},
		{
			name:     "HTML chars escaped in message",
			messages: map[string]string{"_flash:success": "<script>alert(1)</script>"},
			key:      "success",
			want:     `<output role="status" data-flash="success">&lt;script&gt;alert(1)&lt;/script&gt;</output>`,
		},
		{
			name:     "empty flash message returns empty",
			messages: map[string]string{"_flash:success": ""},
			key:      "success",
			want:     "",
		},
		{
			name:     "custom key uses output with status role",
			messages: map[string]string{"_flash:custom": "Hello"},
			key:      "custom",
			want:     `<output role="status" data-flash="custom">Hello</output>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTemplateContext(tt.messages, false)
			got := ctx.FlashTag(tt.key)
			if got != tt.want {
				t.Errorf("FlashTag(%q) = %q, want %q", tt.key, got, tt.want)
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
	result, err := ExecuteTemplateWithContext(tmpl, data, errors, true, nil, nil)
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
	result, err := ExecuteTemplateWithContext(tmpl, data, nil, false, nil, nil)
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

	result, err := ExecuteTemplateWithContext(tmpl, data, nil, false, nil, nil)
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

	result, err := ExecuteTemplateWithContext(tmpl, data, nil, false, nil, nil)
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

	result, err := ExecuteTemplateWithContext(tmpl, data, nil, false, nil, nil)
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

	result, err := ExecuteTemplateWithContext(tmpl, data, errors, false, nil, nil)
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
			result, err := ExecuteTemplateWithContext(tmpl, tt.data, nil, false, nil, nil)
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
			result, err := ExecuteTemplateWithContext(tmpl, struct{}{}, nil, tt.devMode, nil, nil)
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
			result, err := ExecuteTemplateWithContext(tmpl, struct{}{}, tt.errors, false, nil, nil)
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

	result, _ := ExecuteTemplateWithContext(tmpl, data, errors, false, nil, nil)
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
		_, _ = ExecuteTemplateWithContext(tmpl, data, nil, false, nil, nil)
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
		_, _ = ExecuteTemplateWithContext(tmpl, data, nil, false, nil, nil)
	}
}

// precomputeBenchState carries three zero-arg methods, two of which do real work, so
// the benchmark can show the cost of eagerly precomputing methods a template does not
// reference.
type precomputeBenchState struct {
	Name string
	Age  int
}

func (s precomputeBenchState) Cheap() string { return s.Name }

func (s precomputeBenchState) Expensive() int {
	sum := 0
	for i := 0; i < 10000; i++ {
		sum += i
	}
	return sum
}

func (s precomputeBenchState) Report() []int {
	out := make([]int, 128)
	for i := range out {
		out[i] = i * i
	}
	return out
}

// BenchmarkPrecompute compares the eager-all path (nil allow-set, the pre-scoping
// behavior) against a referenced-only set. nil == the "before"; a set that omits the
// unreferenced methods == the "after". The all_referenced case guards against the
// allow-set map lookup itself being a regression.
func BenchmarkPrecompute(b *testing.B) {
	state := precomputeBenchState{Name: "John", Age: 30}
	cases := []struct {
		name  string
		allow map[string]struct{}
	}{
		{"eager_all_methods", nil},
		{"referenced_only", map[string]struct{}{"Name": {}}},
		{"all_referenced", map[string]struct{}{"Cheap": {}, "Expensive": {}, "Report": {}}},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = BuildDataMap(state, nil, false, nil, tc.allow)
			}
		})
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
	result, err := ExecuteTemplateWithContext(tmpl, data, nil, false, nil, nil)
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
	result, err := ExecuteTemplateWithContext(tmpl, data, nil, false, nil, nil)
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
	result, err := ExecuteTemplateWithContext(tmpl, data, nil, true, nil, nil)
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
	result, err := ExecuteTemplateWithContext(tmpl, data, nil, false, nil, nil)
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

	result, err := ExecuteTemplateWithContext(tmpl, data, nil, true, nil, nil)
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
	result, err := ExecuteTemplateWithContext(tmpl, data, nil, true, nil, nil)
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
	result, err := ExecuteTemplateWithContext(tmpl, data, nil, false, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteTemplateWithContext failed: %v", err)
	}

	expected := "Name: John, Email: john@example.com"
	if string(result) != expected {
		t.Errorf("Result = %q, want %q", string(result), expected)
	}
}
