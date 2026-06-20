package livetemplate

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestExtractFormSchema_BasicAttributes(t *testing.T) {
	statics := []string{
		`<form><input type="email" name="Email" required minlength="5" maxlength="100">`,
		`<input type="number" name="Age" min="18" max="120">`,
		`<input type="text" name="Name" required pattern="[A-Za-z ]+">`,
		`</form>`,
	}

	schema := ExtractFormSchema(statics)
	if len(schema.Rules) != 3 {
		t.Fatalf("Expected 3 rules, got %d", len(schema.Rules))
	}

	// Email rule
	email := schema.Rules[0]
	if email.Field != "Email" {
		t.Errorf("Rule 0 field = %q, want Email", email.Field)
	}
	if !email.Required {
		t.Error("Email should be required")
	}
	if email.InputType != "email" {
		t.Errorf("Email type = %q, want email", email.InputType)
	}
	if !email.HasMinLength || email.MinLength != 5 {
		t.Errorf("Email minlength = %v/%d, want true/5", email.HasMinLength, email.MinLength)
	}
	if !email.HasMaxLength || email.MaxLength != 100 {
		t.Errorf("Email maxlength = %v/%d, want true/100", email.HasMaxLength, email.MaxLength)
	}

	// Age rule
	age := schema.Rules[1]
	if age.Field != "Age" {
		t.Errorf("Rule 1 field = %q, want Age", age.Field)
	}
	if !age.HasMin || age.Min != 18 {
		t.Errorf("Age min = %v/%v, want true/18", age.HasMin, age.Min)
	}
	if !age.HasMax || age.Max != 120 {
		t.Errorf("Age max = %v/%v, want true/120", age.HasMax, age.Max)
	}

	// Name rule
	name := schema.Rules[2]
	if name.Field != "Name" {
		t.Errorf("Rule 2 field = %q, want Name", name.Field)
	}
	if !name.Required {
		t.Error("Name should be required")
	}
	if name.Pattern != "[A-Za-z ]+" {
		t.Errorf("Name pattern = %q, want [A-Za-z ]+", name.Pattern)
	}
}

func TestExtractFormSchema_NoValidationAttrs(t *testing.T) {
	statics := []string{`<input type="text" name="search" placeholder="Search...">`}
	schema := ExtractFormSchema(statics)
	if len(schema.Rules) != 0 {
		t.Errorf("Expected 0 rules for input without validation attrs, got %d", len(schema.Rules))
	}
}

func TestExtractFormSchema_NoNameAttr(t *testing.T) {
	statics := []string{`<input type="text" required>`}
	schema := ExtractFormSchema(statics)
	if len(schema.Rules) != 0 {
		t.Errorf("Expected 0 rules for input without name, got %d", len(schema.Rules))
	}
}

func TestFormSchema_Validate_Required(t *testing.T) {
	schema := &FormSchema{
		Rules: []FormRule{
			{Field: "Title", Required: true},
		},
	}

	err := schema.Validate(map[string]interface{}{"Title": ""})
	if err == nil {
		t.Fatal("Expected error for empty required field")
	}

	err = schema.Validate(map[string]interface{}{})
	if err == nil {
		t.Fatal("Expected error for missing required field")
	}

	err = schema.Validate(map[string]interface{}{"Title": "hello"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestFormSchema_Validate_Email(t *testing.T) {
	schema := &FormSchema{
		Rules: []FormRule{
			{Field: "Email", InputType: "email"},
		},
	}

	err := schema.Validate(map[string]interface{}{"Email": "notanemail"})
	if err == nil {
		t.Fatal("Expected error for invalid email")
	}

	err = schema.Validate(map[string]interface{}{"Email": "user@example.com"})
	if err != nil {
		t.Fatalf("Unexpected error for valid email: %v", err)
	}
}

func TestFormSchema_Validate_MinMaxLength(t *testing.T) {
	schema := &FormSchema{
		Rules: []FormRule{
			{Field: "Name", MinLength: 3, HasMinLength: true, MaxLength: 10, HasMaxLength: true},
		},
	}

	err := schema.Validate(map[string]interface{}{"Name": "ab"})
	if err == nil {
		t.Fatal("Expected error for too short")
	}

	err = schema.Validate(map[string]interface{}{"Name": "this is way too long"})
	if err == nil {
		t.Fatal("Expected error for too long")
	}

	err = schema.Validate(map[string]interface{}{"Name": "hello"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestFormSchema_Validate_MinMax(t *testing.T) {
	schema := &FormSchema{
		Rules: []FormRule{
			{Field: "Age", HasMin: true, Min: 18, HasMax: true, Max: 120},
		},
	}

	err := schema.Validate(map[string]interface{}{"Age": "10"})
	if err == nil {
		t.Fatal("Expected error for age < 18")
	}

	err = schema.Validate(map[string]interface{}{"Age": "200"})
	if err == nil {
		t.Fatal("Expected error for age > 120")
	}

	err = schema.Validate(map[string]interface{}{"Age": "25"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestFormSchema_Validate_Pattern(t *testing.T) {
	schema := &FormSchema{
		Rules: []FormRule{
			{Field: "Code", Pattern: "[A-Z]{3}", PatternRe: regexp.MustCompile("^(?:[A-Z]{3})$")},
		},
	}

	err := schema.Validate(map[string]interface{}{"Code": "abc"})
	if err == nil {
		t.Fatal("Expected error for lowercase")
	}

	err = schema.Validate(map[string]interface{}{"Code": "ABC"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestFormSchema_Validate_MultipleErrors(t *testing.T) {
	schema := &FormSchema{
		Rules: []FormRule{
			{Field: "Name", Required: true},
			{Field: "Email", Required: true, InputType: "email"},
		},
	}

	err := schema.Validate(map[string]interface{}{})
	if err == nil {
		t.Fatal("Expected errors")
	}

	var multi MultiError
	if !errors.As(err, &multi) {
		t.Fatalf("Expected MultiError, got %T", err)
	}
	if len(multi) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(multi))
	}
}

func TestFormSchema_Validate_NilSchema(t *testing.T) {
	var schema *FormSchema
	err := schema.Validate(map[string]interface{}{"anything": "value"})
	if err != nil {
		t.Fatalf("Nil schema should return nil: %v", err)
	}
}

func TestContext_ValidateForm(t *testing.T) {
	schema := &FormSchema{
		Rules: []FormRule{
			{Field: "Title", Required: true, MinLength: 3, HasMinLength: true},
		},
	}

	ctx := NewContext(context.TODO(), "submit", map[string]interface{}{"Title": "ab"})
	ctx = ctx.WithFormSchema(schema)

	err := ctx.ValidateForm()
	if err == nil {
		t.Fatal("Expected validation error for short title")
	}
}

func TestExtractFormSchemaFromTemplateStr_StripsDirectives(t *testing.T) {
	// Mixed static + dynamic attributes: name="title" is literal, value
	// is dynamic. The directive must be stripped so the literal name is found.
	src := `<form><input type="email" name="title" value="{{.Title}}" required minlength="5"></form>`
	schema := extractFormSchemaFromTemplateStr(src)
	if schema == nil {
		t.Fatal("Expected schema, got nil")
	}
	if len(schema.Rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(schema.Rules))
	}
	if schema.Rules[0].Field != "title" || !schema.Rules[0].Required {
		t.Errorf("unexpected rule: %+v", schema.Rules[0])
	}

	// Template with no validation attributes returns nil so callers can skip.
	none := extractFormSchemaFromTemplateStr(`<div>{{.Body}}</div>`)
	if none != nil {
		t.Errorf("Expected nil for template with no input rules, got %+v", none)
	}
}

func TestExtractFormSchemaFromTemplateStr_SkipsDynamicNames(t *testing.T) {
	// Names that contain a template directive are dynamic — even partially —
	// and must produce no rule. Without the dynamic-name pre-pass, a partial
	// directive like name="user_{{.ID}}" would collapse to name="user_" after
	// the global strip and yield a rule for a field that never exists, causing
	// ValidateForm to emit spurious required/format errors.
	cases := []struct {
		name string
		src  string
	}{
		{"fully dynamic", `<input type="email" name="{{.Field}}" required>`},
		{"prefix + directive", `<input type="email" name="user_{{.ID}}" required>`},
		{"directive + suffix", `<input type="email" name="{{.Prefix}}_field" required>`},
		{"directive in middle", `<input type="text" name="a{{.Mid}}b" required minlength="3">`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schema := extractFormSchemaFromTemplateStr(tc.src)
			if schema != nil {
				t.Errorf("Expected nil schema for dynamic name (%s), got rules: %+v", tc.src, schema.Rules)
			}
		})
	}

	// Sanity: a fully literal name alongside a dynamic value still yields a rule.
	mixed := `<form><input type="email" name="literal" value="{{.X}}" required></form>`
	schema := extractFormSchemaFromTemplateStr(mixed)
	if schema == nil || len(schema.Rules) != 1 || schema.Rules[0].Field != "literal" {
		t.Fatalf("Expected single rule for literal name, got %+v", schema)
	}
}

// validateFormController exercises ctx.ValidateForm() inside an action method
// without ever calling WithFormSchema manually — this regression-tests
// issue #236 ("ValidateForm silently a no-op for real users").
type validateFormController struct{}

type validateFormState struct {
	Errors map[string]string
}

func (c *validateFormController) Mount(s validateFormState, ctx *Context) (validateFormState, error) {
	return s, nil
}

func (c *validateFormController) Submit(s validateFormState, ctx *Context) (validateFormState, error) {
	if err := ctx.ValidateForm(); err != nil {
		var multi MultiError
		if errors.As(err, &multi) {
			out := make(map[string]string, len(multi))
			for _, fe := range multi {
				out[fe.Field] = fe.Message
			}
			s.Errors = out
		}
		return s, err
	}
	s.Errors = nil
	return s, nil
}

func TestMount_AutoWiresFormSchema_HTTP(t *testing.T) {
	tmpl, err := New("autowire-http")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>
		<form method="POST">
			<input type="email" name="email" required>
			<input type="text" name="name" required minlength="3">
		</form>
	</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if tmpl.formSchema == nil {
		t.Fatal("Template.formSchema should be populated after Parse")
	}
	if len(tmpl.formSchema.Rules) != 2 {
		t.Fatalf("Expected 2 rules cached on template, got %d", len(tmpl.formSchema.Rules))
	}

	handler := tmpl.Handle(&validateFormController{}, AsState(&validateFormState{}))

	form := url.Values{}
	form.Set("lvt-action", "Submit")
	form.Set("email", "not-an-email")
	form.Set("name", "ab") // too short

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	// JSON response should carry both per-field validation errors emitted by
	// ValidateForm — proving the schema was auto-wired without the controller
	// ever calling WithFormSchema.
	if !strings.Contains(body, "valid email") {
		t.Errorf("Expected email validation error in response, got: %s", body)
	}
	if !strings.Contains(body, "at least 3 characters") {
		t.Errorf("Expected minlength validation error in response, got: %s", body)
	}
}

func TestMount_AutoWiresFormSchema_WS(t *testing.T) {
	tmpl, err := New("autowire-ws")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>
		<form method="POST">
			<input type="email" name="email" required>
			<input type="text" name="name" required minlength="3">
		</form>
	</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if tmpl.formSchema == nil {
		t.Fatal("Template.formSchema should be populated after Parse")
	}

	handler := tmpl.Handle(&validateFormController{}, AsState(&validateFormState{}))
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ws := connectWS(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	sendWSAction(t, ws, "Submit", map[string]interface{}{
		"email": "not-an-email",
		"name":  "ab",
	})
	resp := readWSUpdate(t, ws, 3*time.Second)

	meta, ok := resp["meta"].(map[string]any)
	if !ok {
		t.Fatalf("Submit response has no meta: %#v", resp)
	}
	if success, _ := meta["success"].(bool); success {
		t.Errorf("meta.success = true, want false (validation should have failed)")
	}
	errs, ok := meta["errors"].(map[string]any)
	if !ok {
		t.Fatalf("meta.errors missing or wrong type: %#v", meta)
	}
	emailMsg, _ := errs["email"].(string)
	if !strings.Contains(emailMsg, "valid email") {
		t.Errorf("meta.errors[email] = %q, want substring 'valid email'", emailMsg)
	}
	nameMsg, _ := errs["name"].(string)
	if !strings.Contains(nameMsg, "at least 3 characters") {
		t.Errorf("meta.errors[name] = %q, want substring 'at least 3 characters'", nameMsg)
	}
}

func TestMount_AutoWiresFormSchema_NoFormFields(t *testing.T) {
	// Templates without input/textarea/select rules should leave formSchema
	// nil so the auto-wire branch in mount.go is a no-op (preserves existing
	// behavior for non-form templates).
	tmpl, err := New("autowire-noform")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if _, err := tmpl.Parse(`<div>{{.Title}}</div>`); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if tmpl.formSchema != nil {
		t.Errorf("Expected formSchema=nil for template without inputs, got %+v", tmpl.formSchema)
	}
}
