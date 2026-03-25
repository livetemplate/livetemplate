package livetemplate

import (
	"context"
	"errors"
	"regexp"
	"testing"
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
	if email.MinLength != 5 {
		t.Errorf("Email minlength = %d, want 5", email.MinLength)
	}
	if email.MaxLength != 100 {
		t.Errorf("Email maxlength = %d, want 100", email.MaxLength)
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
			{Field: "Title", Required: true, MinLength: -1, MaxLength: -1},
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
			{Field: "Email", InputType: "email", MinLength: -1, MaxLength: -1},
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
			{Field: "Name", MinLength: 3, MaxLength: 10},
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
			{Field: "Age", HasMin: true, Min: 18, HasMax: true, Max: 120, MinLength: -1, MaxLength: -1},
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
			{Field: "Code", Pattern: "^[A-Z]{3}$", PatternRe: regexp.MustCompile("^[A-Z]{3}$"), MinLength: -1, MaxLength: -1},
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
			{Field: "Name", Required: true, MinLength: -1, MaxLength: -1},
			{Field: "Email", Required: true, InputType: "email", MinLength: -1, MaxLength: -1},
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
			{Field: "Title", Required: true, MinLength: 3, MaxLength: -1},
		},
	}

	ctx := NewContext(context.TODO(), "submit", map[string]interface{}{"Title": "ab"})
	ctx = ctx.WithFormSchema(schema)

	err := ctx.ValidateForm()
	if err == nil {
		t.Fatal("Expected validation error for short title")
	}
}
