package build

import (
	"strings"
	"testing"
)

func TestInjectAriaInvalid_NoErrors(t *testing.T) {
	html := `<input name="title" type="text">`
	result := InjectAriaInvalid(html, nil)
	if strings.Contains(result, "aria-invalid") {
		t.Errorf("expected no aria-invalid with nil errors, got: %s", result)
	}

	result = InjectAriaInvalid(html, map[string]string{})
	if strings.Contains(result, "aria-invalid") {
		t.Errorf("expected no aria-invalid with empty errors, got: %s", result)
	}
}

func TestInjectAriaInvalid_DirectMatch(t *testing.T) {
	html := `<html><body><input name="title" type="text"></body></html>`
	errors := map[string]string{"title": "Required"}
	result := InjectAriaInvalid(html, errors)
	if !strings.Contains(result, `aria-invalid="true"`) {
		t.Errorf("expected aria-invalid on input with matching error, got: %s", result)
	}
}

func TestInjectAriaInvalid_SnakeCaseConversion(t *testing.T) {
	html := `<html><body><input name="DisplayName" type="text"></body></html>`
	errors := map[string]string{"display_name": "Required"}
	result := InjectAriaInvalid(html, errors)
	if !strings.Contains(result, `aria-invalid="true"`) {
		t.Errorf("expected aria-invalid via snake_case conversion, got: %s", result)
	}
}

func TestInjectAriaInvalid_Textarea(t *testing.T) {
	html := `<html><body><textarea name="Bio"></textarea></body></html>`
	errors := map[string]string{"bio": "Too long"}
	result := InjectAriaInvalid(html, errors)
	if !strings.Contains(result, `aria-invalid="true"`) {
		t.Errorf("expected aria-invalid on textarea, got: %s", result)
	}
}

func TestInjectAriaInvalid_Select(t *testing.T) {
	html := `<html><body><select name="Category"><option>A</option></select></body></html>`
	errors := map[string]string{"category": "Required"}
	result := InjectAriaInvalid(html, errors)
	if !strings.Contains(result, `aria-invalid="true"`) {
		t.Errorf("expected aria-invalid on select, got: %s", result)
	}
}

func TestInjectAriaInvalid_NoNameAttribute(t *testing.T) {
	html := `<html><body><input type="hidden"></body></html>`
	errors := map[string]string{"title": "Required"}
	result := InjectAriaInvalid(html, errors)
	if strings.Contains(result, "aria-invalid") {
		t.Errorf("expected no aria-invalid on input without name, got: %s", result)
	}
}

func TestInjectAriaInvalid_AlreadyHasAriaInvalid(t *testing.T) {
	html := `<html><body><input name="title" aria-invalid="true" type="text"></body></html>`
	errors := map[string]string{"title": "Required"}
	result := InjectAriaInvalid(html, errors)
	count := strings.Count(result, "aria-invalid")
	if count != 1 {
		t.Errorf("expected exactly 1 aria-invalid (no duplication), found %d in: %s", count, result)
	}
}

func TestInjectAriaInvalid_NoMatchingError(t *testing.T) {
	html := `<html><body><input name="username" type="text"></body></html>`
	errors := map[string]string{"email": "Required"}
	result := InjectAriaInvalid(html, errors)
	if strings.Contains(result, "aria-invalid") {
		t.Errorf("expected no aria-invalid when no matching error, got: %s", result)
	}
}

func TestInjectAriaInvalid_MultipleElements(t *testing.T) {
	html := `<html><body><form><input name="title" type="text"><input name="email" type="email"><input name="bio" type="text"></form></body></html>`
	errors := map[string]string{"title": "Required", "email": "Invalid"}
	result := InjectAriaInvalid(html, errors)

	// title and email should have aria-invalid, bio should not
	count := strings.Count(result, `aria-invalid="true"`)
	if count != 2 {
		t.Errorf("expected 2 aria-invalid attributes, found %d in: %s", count, result)
	}
	// Verify bio doesn't have it
	if strings.Contains(result, `name="bio" aria-invalid`) || strings.Contains(result, `name="bio" type="text" aria-invalid`) {
		t.Errorf("bio should not have aria-invalid, got: %s", result)
	}
}

func TestInjectAriaInvalid_FullDocument(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>Test</title></head><body><form><label>Name<input name="DisplayName" type="text"></label></form></body></html>`
	errors := map[string]string{"display_name": "Required"}
	result := InjectAriaInvalid(html, errors)
	if !strings.Contains(result, `aria-invalid="true"`) {
		t.Errorf("expected aria-invalid in full document, got: %s", result)
	}
}
