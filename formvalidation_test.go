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

// --- #239: server-side formnovalidate -------------------------------------

func TestExtractFormSchema_FormNoValidate(t *testing.T) {
	statics := []string{
		`<form method="POST">`,
		`<input type="text" name="name" required minlength="3">`,
		`<button name="save">Save</button>`,
		`<button name="save-draft" formnovalidate>Save Draft</button>`,          // default type=submit
		`<input type="submit" name="quick-draft" formnovalidate value="Quick">`, // submit input
		`<button formnovalidate>Unnamed</button>`,                               // no name: not recorded
		`<button type="button" name="clear" formnovalidate>Clear</button>`,      // type=button: can't submit
		`<button type="reset" name="reset-btn" formnovalidate>Reset</button>`,   // type=reset: can't submit
		`<input type="text" name="weird" formnovalidate>`,                       // non-submit input
		`</form>`,
	}
	schema := ExtractFormSchema(statics)

	want := map[string]bool{"save-draft": true, "quick-draft": true}
	if len(schema.NoValidateSubmitters) != len(want) {
		t.Fatalf("NoValidateSubmitters = %v, want keys %v", schema.NoValidateSubmitters, want)
	}
	for name := range want {
		if !schema.NoValidateSubmitters[name] {
			t.Errorf("expected %q in NoValidateSubmitters, got %v", name, schema.NoValidateSubmitters)
		}
	}
	// Controls that can't submit a form must never be recorded as skip-submitters,
	// or a crafted lvt-submitter could bypass validation.
	for _, name := range []string{"save", "clear", "reset-btn", "weird"} {
		if schema.NoValidateSubmitters[name] {
			t.Errorf("%q must not be a skip-submitter", name)
		}
	}
}

// TestExtractFormSchema_SubmitInputNoSpuriousRule guards against a submit/image/
// button/reset input being treated as a data field: its name carries no value in
// the POST body, so a `required` on it would raise a spurious error.
func TestExtractFormSchema_SubmitInputNoSpuriousRule(t *testing.T) {
	schema := ExtractFormSchema([]string{
		`<form><input type="text" name="title" required>`,
		`<input type="submit" name="save-draft" required formnovalidate value="Draft"></form>`,
	})
	for _, r := range schema.Rules {
		if r.Field == "save-draft" {
			t.Errorf("submit input must not produce a validation rule, got %+v", r)
		}
	}
	if !schema.NoValidateSubmitters["save-draft"] {
		t.Error("submit input with formnovalidate should still be a skip-submitter")
	}
}

func TestExtractFormSchema_NoFormNoValidate(t *testing.T) {
	// A plain submit button leaves the set nil (not an empty map) so the common
	// case allocates nothing.
	schema := ExtractFormSchema([]string{
		`<form><input name="name" required><button name="save">Save</button></form>`,
	})
	if schema.NoValidateSubmitters != nil {
		t.Errorf("NoValidateSubmitters = %v, want nil when no formnovalidate present", schema.NoValidateSubmitters)
	}
}

// TestValidateForm_FormNoValidate is the unit-level proof that the skip keys
// solely on the submitter. The submitter-differs case (action not in the set,
// submitter is) justifies the separate Context.submitter field — under
// lvt-on:submit routing the action is the handler, not the button — and the
// empty-submitter case proves a bare action no longer skips.
func TestValidateForm_FormNoValidate(t *testing.T) {
	schema := &FormSchema{
		Rules:                []FormRule{{Field: "name", Required: true, MinLength: 3, HasMinLength: true}},
		NoValidateSubmitters: map[string]bool{"save-draft": true},
	}
	data := map[string]interface{}{"name": "ab"} // too short → invalid

	cases := []struct {
		name      string
		action    string
		submitter string
		wantSkip  bool
	}{
		{name: "normal submitter validates", action: "save", submitter: "save", wantSkip: false},
		{name: "formnovalidate submitter skips (button-name routing: submitter==action)", action: "save-draft", submitter: "save-draft", wantSkip: true},
		{name: "formnovalidate submitter, action differs (lvt-on:submit)", action: "save", submitter: "save-draft", wantSkip: true},
		{name: "no submitter does not skip even if action matches", action: "save-draft", submitter: "", wantSkip: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := NewContext(context.TODO(), tc.action, data).withSubmitter(tc.submitter)
			ctx = ctx.WithFormSchema(schema)
			err := ctx.ValidateForm()
			if tc.wantSkip && err != nil {
				t.Errorf("expected validation skipped, got error: %v", err)
			}
			if !tc.wantSkip && err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

// fnvController validates inside every action; the only reason a submission
// skips validation is the formnovalidate attribute on the submitting control.
// A skipped submission therefore reports meta.success=true (the action ran and
// validation was bypassed) where a validated one reports the field errors.
type fnvController struct{}
type fnvState struct{ Errors map[string]string }

func (fnvController) Mount(s fnvState, ctx *Context) (fnvState, error) { return s, nil }

func (fnvController) validate(s fnvState, ctx *Context) (fnvState, error) {
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

func (c fnvController) Save(s fnvState, ctx *Context) (fnvState, error) { return c.validate(s, ctx) }
func (c fnvController) SaveDraft(s fnvState, ctx *Context) (fnvState, error) {
	return c.validate(s, ctx)
}
func (c fnvController) Submit(s fnvState, ctx *Context) (fnvState, error) { return c.validate(s, ctx) }

// Uses the canonical kebab-case button name (<button name="save-draft">), which
// routes to SaveDraft via methodNameToActions' kebab variant — exercising the
// no-JS button-name path end-to-end alongside formnovalidate.
const fnvTemplate = `<div>
	<form method="POST">
		<input type="text" name="name" required minlength="3">
		<button name="save">Save</button>
		<button name="save-draft" formnovalidate>Save Draft</button>
	</form>
</div>`

func newFNVHandler(t *testing.T) http.Handler {
	t.Helper()
	tmpl, err := New("fnv")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if _, err := tmpl.Parse(fnvTemplate); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if tmpl.formSchema == nil || !tmpl.formSchema.NoValidateSubmitters["save-draft"] {
		t.Fatalf("expected save-draft auto-wired into NoValidateSubmitters, got %+v", tmpl.formSchema)
	}
	return tmpl.Handle(&fnvController{}, AsState(&fnvState{}))
}

func postFNV(t *testing.T, handler http.Handler, form url.Values) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Body.String()
}

// TestFormNoValidate_NoJS_ButtonName covers the pure no-JS tier: native POST
// where the clicked button arrives as an empty-value form field and is resolved
// to the action via detectSubmitButtonName. The formnovalidate button skips
// validation; the normal button does not.
func TestFormNoValidate_NoJS_ButtonName(t *testing.T) {
	handler := newFNVHandler(t)

	// formnovalidate button (empty value → detected as submitter) → skipped.
	// Assert the SaveDraft action actually ran (ran:SaveDraft) AND no error, so
	// this can't pass on a routing miss (ErrMethodNotFound never sets Ran).
	draft := postFNV(t, handler, url.Values{"save-draft": {""}, "name": {"ab"}})
	if strings.Contains(draft, "at least 3 characters") {
		t.Errorf("formnovalidate button should skip validation, got error in: %s", draft)
	}
	// meta.success is true only if the action ran AND validation passed/skipped —
	// a routing miss (ErrMethodNotFound) never reports success.
	if !strings.Contains(draft, `"success":true`) {
		t.Errorf("expected success (action ran, validation skipped), body: %s", draft)
	}

	// normal button → validation runs.
	save := postFNV(t, handler, url.Values{"save": {""}, "name": {"ab"}})
	if !strings.Contains(save, "at least 3 characters") {
		t.Errorf("normal button should validate, got no error in: %s", save)
	}
}

// TestFormNoValidate_ExplicitSubmitter covers the JS tiers where the client
// sends lvt-action (the handler) separately from lvt-submitter (the clicked
// button). Skipping here depends on the submitter, not the action — this is the
// case that earns the Context.submitter field.
func TestFormNoValidate_ExplicitSubmitter(t *testing.T) {
	handler := newFNVHandler(t)

	// action routes to Save (not formnovalidate) but the submitter is the
	// formnovalidate button → validation skipped. Only the submitter differs
	// between this and the next case, so it isolates the Context.submitter field.
	skip := postFNV(t, handler, url.Values{"lvt-action": {"Save"}, "lvt-submitter": {"save-draft"}, "name": {"ab"}})
	if strings.Contains(skip, "at least 3 characters") {
		t.Errorf("formnovalidate submitter should skip validation, got error in: %s", skip)
	}
	if !strings.Contains(skip, `"success":true`) {
		t.Errorf("expected success (Save ran, validation skipped via submitter), body: %s", skip)
	}

	// same action, ordinary submitter → validation runs.
	run := postFNV(t, handler, url.Values{"lvt-action": {"Save"}, "lvt-submitter": {"save"}, "name": {"ab"}})
	if !strings.Contains(run, "at least 3 characters") {
		t.Errorf("ordinary submitter should validate, got no error in: %s", run)
	}
}

// TestFormNoValidate_NoJS_ValuedButtonBoundary locks the documented no-JS
// boundary: detectSubmitButtonName only treats an EMPTY-value field as the
// submitter, so a formnovalidate button carrying a value is not recognized as
// the submitter on the no-JS tier. The action falls through to the default
// "submit", which validates. (JS tiers are unaffected — they send an explicit
// submitter; see TestFormNoValidate_ExplicitSubmitter.)
func TestFormNoValidate_NoJS_ValuedButtonBoundary(t *testing.T) {
	handler := newFNVHandler(t)

	valued := url.Values{"save-draft": {"1"}, "name": {"ab"}}
	body := postFNV(t, handler, valued)
	if !strings.Contains(body, "at least 3 characters") {
		t.Errorf("value-bearing formnovalidate button is not the no-JS submitter, validation should run; got: %s", body)
	}
}

// TestFormNoValidate_WS_ExplicitSubmitter covers the WebSocket action path
// (mount.go threads msg.Submitter there too). The frame routes to Save but the
// submitter is the formnovalidate button, so validation is skipped — exercising
// submitter≠action over WS, distinct from the HTTP path.
func TestFormNoValidate_WS_ExplicitSubmitter(t *testing.T) {
	tmpl, err := New("fnv-ws")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if _, err := tmpl.Parse(fnvTemplate); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	handler := tmpl.Handle(&fnvController{}, AsState(&fnvState{}))
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ws := connectWS(t, wsURL)
	defer func() { _ = ws.Close() }()

	// action=Save (ordinary), submitter=save-draft (formnovalidate) → skip.
	sendWSActionWithSubmitter(t, ws, "Save", "save-draft", map[string]interface{}{"name": "ab"})
	resp := readWSUpdate(t, ws, 3*time.Second)
	meta, ok := resp["meta"].(map[string]any)
	if !ok {
		t.Fatalf("response has no meta: %#v", resp)
	}
	if success, _ := meta["success"].(bool); !success {
		t.Errorf("meta.success = false, want true (formnovalidate submitter should skip validation): %#v", meta)
	}

	// action=Save, submitter=save (ordinary) → validation runs.
	sendWSActionWithSubmitter(t, ws, "Save", "save", map[string]interface{}{"name": "ab"})
	resp = readWSUpdate(t, ws, 3*time.Second)
	meta, _ = resp["meta"].(map[string]any)
	if success, _ := meta["success"].(bool); success {
		t.Errorf("meta.success = true, want false (ordinary submitter should validate): %#v", meta)
	}
}

func TestExtractFormSchemaFromTemplateStr_KeepsFormNoValidateOnlySchema(t *testing.T) {
	// A form with a formnovalidate button but no validation rules still yields a
	// schema (NoValidateSubmitters populated) — the nil guard must not discard it.
	schema := extractFormSchemaFromTemplateStr(`<form method="POST"><button name="save-draft" formnovalidate>Draft</button></form>`)
	if schema == nil {
		t.Fatal("expected non-nil schema for a formnovalidate-only form")
	}
	if !schema.NoValidateSubmitters["save-draft"] {
		t.Errorf("expected save-draft in NoValidateSubmitters, got %+v", schema)
	}
}

func TestContext_Submitter(t *testing.T) {
	ctx := NewContext(context.TODO(), "Save", nil)
	if got := ctx.Submitter(); got != "" {
		t.Errorf("default Submitter() = %q, want empty", got)
	}
	ctx = ctx.withSubmitter("save-draft")
	if got := ctx.Submitter(); got != "save-draft" {
		t.Errorf("Submitter() = %q, want save-draft", got)
	}
}
