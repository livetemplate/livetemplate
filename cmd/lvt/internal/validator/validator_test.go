package validator

import (
	"strings"
	"testing"
)

func TestValidationResult_AddError(t *testing.T) {
	result := NewValidationResult()

	if !result.Valid {
		t.Error("Expected new result to be valid initially")
	}

	result.AddError("test error", "test.go", 10)

	if result.Valid {
		t.Error("Expected result to be invalid after adding error")
	}

	if len(result.Issues) != 1 {
		t.Errorf("Expected 1 issue, got %d", len(result.Issues))
	}

	issue := result.Issues[0]
	if issue.Level != LevelError {
		t.Errorf("Expected error level, got %s", issue.Level)
	}
	if issue.Message != "test error" {
		t.Errorf("Expected 'test error', got '%s'", issue.Message)
	}
	if issue.File != "test.go" {
		t.Errorf("Expected 'test.go', got '%s'", issue.File)
	}
	if issue.Line != 10 {
		t.Errorf("Expected line 10, got %d", issue.Line)
	}
}

func TestValidationResult_AddWarning(t *testing.T) {
	result := NewValidationResult()
	result.AddWarning("test warning", "test.go", 20)

	if !result.Valid {
		t.Error("Expected result to remain valid after adding warning")
	}

	if len(result.Issues) != 1 {
		t.Errorf("Expected 1 issue, got %d", len(result.Issues))
	}

	issue := result.Issues[0]
	if issue.Level != LevelWarning {
		t.Errorf("Expected warning level, got %s", issue.Level)
	}
}

func TestValidationResult_AddInfo(t *testing.T) {
	result := NewValidationResult()
	result.AddInfo("test info", "test.go", 30)

	if !result.Valid {
		t.Error("Expected result to remain valid after adding info")
	}

	if len(result.Issues) != 1 {
		t.Errorf("Expected 1 issue, got %d", len(result.Issues))
	}

	issue := result.Issues[0]
	if issue.Level != LevelInfo {
		t.Errorf("Expected info level, got %s", issue.Level)
	}
}

func TestValidationResult_Merge(t *testing.T) {
	result1 := NewValidationResult()
	result1.AddWarning("warning 1", "file1.go", 1)

	result2 := NewValidationResult()
	result2.AddError("error 1", "file2.go", 2)

	result1.Merge(result2)

	if result1.Valid {
		t.Error("Expected merged result to be invalid")
	}

	if len(result1.Issues) != 2 {
		t.Errorf("Expected 2 issues after merge, got %d", len(result1.Issues))
	}
}

func TestValidationResult_HasErrors(t *testing.T) {
	result := NewValidationResult()

	if result.HasErrors() {
		t.Error("Expected no errors initially")
	}

	result.AddWarning("warning", "test.go", 1)
	if result.HasErrors() {
		t.Error("Expected no errors with only warnings")
	}

	result.AddError("error", "test.go", 2)
	if !result.HasErrors() {
		t.Error("Expected HasErrors to return true")
	}
}

func TestValidationResult_ErrorCount(t *testing.T) {
	result := NewValidationResult()
	result.AddError("error 1", "test.go", 1)
	result.AddWarning("warning 1", "test.go", 2)
	result.AddError("error 2", "test.go", 3)
	result.AddInfo("info 1", "test.go", 4)

	count := result.ErrorCount()
	if count != 2 {
		t.Errorf("Expected 2 errors, got %d", count)
	}
}

func TestValidationResult_WarningCount(t *testing.T) {
	result := NewValidationResult()
	result.AddError("error 1", "test.go", 1)
	result.AddWarning("warning 1", "test.go", 2)
	result.AddWarning("warning 2", "test.go", 3)
	result.AddInfo("info 1", "test.go", 4)

	count := result.WarningCount()
	if count != 2 {
		t.Errorf("Expected 2 warnings, got %d", count)
	}
}

func TestValidationResult_Format(t *testing.T) {
	t.Run("valid result with no issues", func(t *testing.T) {
		result := NewValidationResult()
		output := result.Format()

		if !strings.Contains(output, "✅ Validation passed") {
			t.Error("Expected success message")
		}
		if !strings.Contains(output, "No issues found") {
			t.Error("Expected 'No issues found' message")
		}
	})

	t.Run("invalid result with errors", func(t *testing.T) {
		result := NewValidationResult()
		result.AddError("test error", "test.go", 10)
		output := result.Format()

		if !strings.Contains(output, "❌ Validation failed") {
			t.Error("Expected failure message")
		}
		if !strings.Contains(output, "test error") {
			t.Error("Expected error message in output")
		}
		if !strings.Contains(output, "test.go") {
			t.Error("Expected file name in output")
		}
	})

	t.Run("grouped by severity", func(t *testing.T) {
		result := NewValidationResult()
		result.AddError("error 1", "test.go", 1)
		result.AddWarning("warning 1", "test.go", 2)
		result.AddInfo("info 1", "test.go", 3)
		output := result.Format()

		if !strings.Contains(output, "❌ Errors (1)") {
			t.Error("Expected errors section")
		}
		if !strings.Contains(output, "⚠️  Warnings (1)") {
			t.Error("Expected warnings section")
		}
		if !strings.Contains(output, "ℹ️  Info (1)") {
			t.Error("Expected info section")
		}
	})
}
