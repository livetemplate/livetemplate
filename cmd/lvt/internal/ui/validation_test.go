package ui

import (
	"testing"
)

func TestIsValidGoIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOk  bool
		wantErr string
	}{
		{"valid lowercase", "username", true, ""},
		{"valid underscore", "user_name", true, ""},
		{"valid camelCase", "userName", true, ""},
		{"valid leading underscore", "_private", true, ""},
		{"empty string", "", false, "identifier cannot be empty"},
		{"too short", "a", false, "identifier must be at least 2 characters"},
		{"starts with digit", "123name", false, "identifier cannot start with a digit"},
		{"contains hyphen", "user-name", false, "identifier can only contain letters, digits, and underscores"},
		{"contains space", "user name", false, "identifier can only contain letters, digits, and underscores"},
		{"go keyword if", "if", false, "'if' is a Go reserved keyword"},
		{"go keyword func", "func", false, "'func' is a Go reserved keyword"},
		{"go keyword select", "select", false, "'select' is a Go reserved keyword"},
		{"go keyword type", "type", false, "'type' is a Go reserved keyword"},
		{"valid similar to keyword", "ifelse", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOk, gotErr := IsValidGoIdentifier(tt.input)
			if gotOk != tt.wantOk {
				t.Errorf("IsValidGoIdentifier(%q) ok = %v, want %v", tt.input, gotOk, tt.wantOk)
			}
			if gotErr != tt.wantErr {
				t.Errorf("IsValidGoIdentifier(%q) error = %v, want %v", tt.input, gotErr, tt.wantErr)
			}
		})
	}
}

func TestIsValidResourceName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantValid   bool
		wantError   string
		wantWarning string
	}{
		{"valid lowercase", "users", true, "", ""},
		{"valid plural", "products", true, "", ""},
		{"uppercase warning", "Users", true, "", "resource name will be converted to lowercase: 'users'"},
		{"sql reserved word not go keyword", "insert", true, "", "'insert' is a SQL reserved word - consider renaming"},
		{"sql reserved table not go keyword", "table", true, "", "'table' is a SQL reserved word - consider renaming"},
		{"go keyword also sql", "select", false, "'select' is a Go reserved keyword", "also a SQL reserved word"},
		{"empty", "", false, "resource name cannot be empty", ""},
		{"starts with digit", "9users", false, "identifier cannot start with a digit", ""},
		{"contains hyphen", "user-list", false, "identifier can only contain letters, digits, and underscores", ""},
		{"go keyword", "if", false, "'if' is a Go reserved keyword", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidResourceName(tt.input)
			if result.Valid != tt.wantValid {
				t.Errorf("IsValidResourceName(%q).Valid = %v, want %v", tt.input, result.Valid, tt.wantValid)
			}
			if result.Error != tt.wantError {
				t.Errorf("IsValidResourceName(%q).Error = %q, want %q", tt.input, result.Error, tt.wantError)
			}
			if result.Warning != tt.wantWarning {
				t.Errorf("IsValidResourceName(%q).Warning = %q, want %q", tt.input, result.Warning, tt.wantWarning)
			}
		})
	}
}

func TestIsValidFieldName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantValid   bool
		wantError   string
		wantWarning string
	}{
		{"valid lowercase", "username", true, "", ""},
		{"valid snake_case", "user_name", true, "", ""},
		{"valid with underscore", "created_at", true, "", ""},
		{"uppercase warning", "UserName", true, "", "field name will be converted to lowercase: 'username'"},
		{"sql reserved word", "delete", true, "", "'delete' is a SQL reserved word - may cause issues"},
		{"empty", "", false, "field name cannot be empty", ""},
		{"starts with digit", "1st_name", false, "identifier cannot start with a digit", ""},
		{"contains hyphen", "user-name", false, "identifier can only contain letters, digits, and underscores", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidFieldName(tt.input)
			if result.Valid != tt.wantValid {
				t.Errorf("IsValidFieldName(%q).Valid = %v, want %v", tt.input, result.Valid, tt.wantValid)
			}
			if result.Error != tt.wantError {
				t.Errorf("IsValidFieldName(%q).Error = %q, want %q", tt.input, result.Error, tt.wantError)
			}
			if result.Warning != tt.wantWarning {
				t.Errorf("IsValidFieldName(%q).Warning = %q, want %q", tt.input, result.Warning, tt.wantWarning)
			}
		})
	}
}

func TestIsValidViewName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantValid   bool
		wantError   string
		wantWarning string
	}{
		{"valid lowercase", "dashboard", true, "", ""},
		{"valid underscore", "user_profile", true, "", ""},
		{"uppercase warning", "Dashboard", true, "", "view name will be converted to lowercase: 'dashboard'"},
		{"sql reserved not go keyword", "insert", true, "", "'insert' is a SQL reserved word - consider renaming"},
		{"go keyword also sql", "select", false, "'select' is a Go reserved keyword", "also a SQL reserved word"},
		{"empty", "", false, "view name cannot be empty", ""},
		{"starts with digit", "1dashboard", false, "identifier cannot start with a digit", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidViewName(tt.input)
			if result.Valid != tt.wantValid {
				t.Errorf("IsValidViewName(%q).Valid = %v, want %v", tt.input, result.Valid, tt.wantValid)
			}
			if result.Error != tt.wantError {
				t.Errorf("IsValidViewName(%q).Error = %q, want %q", tt.input, result.Error, tt.wantError)
			}
			if result.Warning != tt.wantWarning {
				t.Errorf("IsValidViewName(%q).Warning = %q, want %q", tt.input, result.Warning, tt.wantWarning)
			}
		})
	}
}

func TestIsValidModulePath(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantValid   bool
		wantError   string
		wantWarning string
	}{
		{"valid github", "github.com/user/repo", true, "", ""},
		{"valid gitlab", "gitlab.com/user/repo", true, "", ""},
		{"valid custom domain", "mycompany.com/projects/app", true, "", ""},
		{"example.com warning", "example.com/user/app", true, "", "using example.com - consider using actual domain"},
		{"empty", "", false, "module path cannot be empty", ""},
		{"no slash", "mymodule", false, "module path should be in format: domain.com/user/repo", ""},
		{"http prefix", "http://github.com/user/repo", false, "module path should not include http:// or https://", ""},
		{"https prefix", "https://github.com/user/repo", false, "module path should not include http:// or https://", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidModulePath(tt.input)
			if result.Valid != tt.wantValid {
				t.Errorf("IsValidModulePath(%q).Valid = %v, want %v", tt.input, result.Valid, tt.wantValid)
			}
			if result.Error != tt.wantError {
				t.Errorf("IsValidModulePath(%q).Error = %q, want %q", tt.input, result.Error, tt.wantError)
			}
			if result.Warning != tt.wantWarning {
				t.Errorf("IsValidModulePath(%q).Warning = %q, want %q", tt.input, result.Warning, tt.wantWarning)
			}
		})
	}
}

func TestIsValidAppName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantValid   bool
		wantError   string
		wantWarning string
	}{
		{"valid lowercase", "myapp", true, "", ""},
		{"valid underscore", "my_app", true, "", ""},
		{"valid hyphen", "my-app", true, "", "app name contains hyphens (directory will use hyphens, package will use underscores)"},
		{"valid mixed", "my-cool_app", true, "", "app name contains hyphens (directory will use hyphens, package will use underscores)"},
		{"empty", "", false, "app name cannot be empty", ""},
		{"starts with digit", "1app", false, "identifier cannot start with a digit", ""},
		{"contains space", "my app", false, "identifier can only contain letters, digits, and underscores", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidAppName(tt.input)
			if result.Valid != tt.wantValid {
				t.Errorf("IsValidAppName(%q).Valid = %v, want %v", tt.input, result.Valid, tt.wantValid)
			}
			if result.Error != tt.wantError {
				t.Errorf("IsValidAppName(%q).Error = %q, want %q", tt.input, result.Error, tt.wantError)
			}
			if result.Warning != tt.wantWarning {
				t.Errorf("IsValidAppName(%q).Warning = %q, want %q", tt.input, result.Warning, tt.wantWarning)
			}
		})
	}
}
